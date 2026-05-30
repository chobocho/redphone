// Package app wires the RedPhone process together and owns its lifecycle.
//
// WHY: 발견·HTTP·WS 등 모든 하위 시스템은 하나의 root context를 공유하고,
// 세 갈래 종료 경로(stdin "exit" / POST /api/shutdown / SIGINT)는 전부 그
// context 취소로 수렴한다. app 패키지는 그 배선과 종료 절차만 책임진다.
package app

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// Config holds the runtime knobs for a RedPhone instance.
type Config struct {
	Name     string    // 화면에 표시될 사용자 이름
	HTTPPort int       // 우선 HTTP 포트(사용 중이면 OS 자동 폴백은 web 계층에서 처리)
	Stdin    io.Reader // 테스트 주입용. nil이면 os.Stdin.
}

// Run starts the application and blocks until one of the shutdown paths
// cancels the context, then returns nil after a graceful cleanup.
//
// WHY: 반환 = 프로세스 종료 신호. 종료가 어디서 트리거되든(ctx 취소, "exit",
// SIGINT) 단일 지점에서 정리하고 빠르게 반환해 포트를 즉시 해제한다.
func Run(ctx context.Context, cfg Config) error {
	// SIGINT/SIGTERM도 동일한 ctx 취소로 흡수한다.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stdin := cfg.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	// stdin 감시는 detached daemon: os.Stdin.Read는 인터럽트가 어려우므로
	// 종료 시 이 고루틴을 기다리지 않는다(반환 지연 방지).
	go watchStdin(ctx, stdin, cancel)

	<-ctx.Done()
	// Phase 0에는 정리할 하위 시스템이 아직 없다. 이후 Phase에서 여기에
	// BYE 브로드캐스트 → srv.Shutdown → ticker/hub 정지가 추가된다.
	return nil
}

// watchStdin reads console lines and cancels on a bare "exit".
func watchStdin(ctx context.Context, r io.Reader, cancel context.CancelFunc) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "exit" {
			cancel()
			return
		}
		// 이미 다른 경로로 종료됐다면 더 읽지 않는다.
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
