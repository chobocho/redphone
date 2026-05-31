// Package app wires the RedPhone process together and owns its lifecycle.
//
// WHY: 발견·HTTP·WS 등 모든 하위 시스템은 하나의 root context를 공유하고,
// 세 갈래 종료 경로(stdin "exit" / POST /api/shutdown / SIGINT)는 전부 그
// context 취소로 수렴한다. app 패키지는 그 배선과 종료 절차만 책임진다.
package app

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chobocho/redphone/internal/discovery"
	"github.com/chobocho/redphone/internal/message"
	"github.com/chobocho/redphone/internal/peer"
	"github.com/chobocho/redphone/internal/peerbook"
	"github.com/chobocho/redphone/internal/share"
	"github.com/chobocho/redphone/internal/tlsid"
	"github.com/chobocho/redphone/internal/web"
)

const (
	peerTTL        = 15 * time.Second // 이 시간 HELLO 없으면 오프라인
	sweepInterval  = 3 * time.Second
	shutdownGrace  = 3 * time.Second
	instanceIDFile = "redphone.id"
	nameFile       = "redphone-name.txt"
)

// Config holds the runtime knobs for a RedPhone instance.
type Config struct {
	Name           string    // 화면에 표시될 사용자 이름
	HTTPPort       int       // 우선 HTTP 포트(사용 중이면 OS 자동 폴백)
	HTTPSPort      int       // 우선 HTTPS 포트(사용 중이면 OS 자동 폴백)
	DiscoveryPort  int       // 0 → discovery.Port(17000)
	HistoryPath    string    // SQLite DB path; ""이면 redphone.db
	OpenBrowser    bool      // 기동 시 기본 브라우저 자동 오픈
	ScanAll        bool      // true면 주기적으로 서브넷 전체에 유니캐스트 HELLO 스캔
	InstanceIDPath string    // 안정 논리 ID 파일 경로; ""이면 실행 파일 옆 redphone.id
	NamePath       string    // 표시 이름 저장 경로; ""이면 실행 파일 옆 redphone-name.txt
	Stdin          io.Reader // 테스트 주입용; nil이면 os.Stdin
	OnReady        func(httpURL string)
}

// Run starts the application and blocks until one of the shutdown paths
// cancels the context, then performs a graceful teardown and returns.
func Run(ctx context.Context, cfg Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	runtimeID := newID()
	stableID, err := loadOrCreateStableID(cfg.InstanceIDPath)
	if err != nil {
		return fmt.Errorf("app: stable id: %w", err)
	}
	initialName, err := loadSavedName(cfg.NamePath, cfg.Name)
	if err != nil {
		return fmt.Errorf("app: load name: %w", err)
	}
	nameState := newNameState(cfg.NamePath, initialName)

	// ---- 상태/저장소 ----
	reg := peer.NewRegistry()
	hist, err := message.Open(cfg.HistoryPath)
	if err != nil {
		return fmt.Errorf("app: history open: %w", err)
	}
	defer hist.Close()
	shareDir, err := os.MkdirTemp("", "redphone-share-")
	if err != nil {
		return fmt.Errorf("app: share dir: %w", err)
	}
	defer os.RemoveAll(shareDir)
	shares := share.NewStore(shareDir)
	defer shares.RemoveAll()

	// ---- TLS 식별자(self-signed + 지문) ----
	// WHY: HELLO에 광고할 지문이 먼저 정해져야 두 리스너가 일관된다.
	id, err := tlsid.Generate(nameState.Get())
	if err != nil {
		return fmt.Errorf("app: tls identity: %w", err)
	}

	// ---- HTTP 리스너(포트 폴백) ----
	ln, err := web.Listen(cfg.HTTPPort)
	if err != nil {
		return fmt.Errorf("app: http listen: %w", err)
	}
	httpPort := ln.Addr().(*net.TCPAddr).Port

	// ---- HTTPS 리스너(포트 폴백) ----
	lnTLS, err := web.ListenTLS(cfg.HTTPSPort, id.ServerTLSConfig())
	if err != nil {
		_ = ln.Close()
		return fmt.Errorf("app: https listen: %w", err)
	}
	httpsPort := lnTLS.Addr().(*net.TCPAddr).Port

	// ---- Discovery 서비스(소켓은 아래에서 Open) ----
	// 친구 IP 수동 관리·스캔을 위해 web.New보다 먼저 만든다.
	dport := cfg.DiscoveryPort
	if dport == 0 {
		dport = discovery.Port
	}
	dsvc := &discovery.Service{
		SelfID: runtimeID, SelfUID: stableID, Name: nameState.Get(),
		HTTPPort: httpPort, HTTPSPort: httpsPort, FP: id.Fingerprint,
		Port: dport, ScanOnStart: cfg.ScanAll,
	}

	// 저장된 친구 IP를 적재한다. 실제 probe는 Run의 광고 루프가 맡으므로
	// 여기서는 목록만 채운다(소켓이 아직 없다).
	pbPath := peerbook.DefaultPath()
	if saved, lerr := peerbook.Load(pbPath); lerr != nil {
		slog.Warn("peers.json 로드 실패", "err", lerr, "path", pbPath)
	} else {
		for _, ip := range saved {
			if aerr := dsvc.AddTarget(ip); aerr != nil {
				slog.Warn("저장된 친구 IP 무시", "ip", ip, "err", aerr)
			}
		}
	}
	tctl := &targetControl{svc: dsvc, path: pbPath}

	srv := web.New(web.Options{
		Reg:     reg,
		SelfID:  stableID,
		Name:    nameState.Get(),
		NameGet: nameState.Get,
		Rename: func(next string) (string, error) {
			name, err := nameState.Set(next)
			if err != nil {
				return "", err
			}
			dsvc.SetName(name)
			dsvc.AdvertiseNow()
			return name, nil
		},
		History:     hist,
		Shares:      shares,
		DownloadDir: "downloads",
		Shutdown:    cancel, // 종료 버튼 → root ctx 취소
		Targets:     tctl,
	})
	hub := srv.Hub()

	// ---- Discovery 소켓 ----
	dconn, dst, err := discovery.Open(dport)
	if err != nil {
		_ = ln.Close()
		_ = lnTLS.Close()
		return fmt.Errorf("app: discovery open: %w", err)
	}

	// 피어 목록 변동을 브라우저로 실시간 푸시.
	pushPeers := func() {
		if b, err := json.Marshal(map[string]any{"type": "peers", "peers": reg.Snapshot()}); err == nil {
			hub.Broadcast(b)
		}
	}

	var wg sync.WaitGroup
	spawn := func(fn func()) { wg.Add(1); go func() { defer wg.Done(); fn() }() }

	spawn(func() { hub.Run(ctx) })
	spawn(func() {
		_ = dsvc.Run(ctx, dconn, dst, discovery.Handlers{
			OnHello: func(m discovery.Message, ip string) {
				peerID := m.UID
				if peerID == "" {
					peerID = m.ID
				}
				reg.Upsert(peer.Peer{
					ID: peerID, SessionID: m.ID, Name: m.Name, IP: ip,
					HTTPPort: m.HTTPPort, HTTPSPort: m.HTTPSPort,
					Fingerprint: m.FP,
				})
				pushPeers()
			},
			OnBye: func(id string) { reg.RemoveSession(id); pushPeers() },
		})
	})
	spawn(func() { sweepPeers(ctx, reg, pushPeers) })

	httpSrv := &http.Server{Handler: srv.Handler()}
	spawn(func() { _ = httpSrv.Serve(ln) }) // 종료 시 ErrServerClosed로 반환

	// HTTPS 서버는 같은 mux를 공유한다 — requireTLS 가드가 분기를 책임진다.
	httpsSrv := &http.Server{Handler: srv.Handler(), TLSConfig: id.ServerTLSConfig()}
	spawn(func() { _ = httpsSrv.Serve(lnTLS) })

	// stdin 감시는 detached daemon(블로킹 ReadFrom을 기다리지 않음).
	go watchStdin(ctx, stdinOf(cfg), cancel)

	httpURL := fmt.Sprintf("http://localhost:%d", httpPort)
	slog.Info("redphone up",
		"name", nameState.Get(), "runtime_id", runtimeID, "stable_id", stableID, "ui", httpURL,
		"discovery", dport, "https", httpsPort, "fp", id.Fingerprint[:12])
	if cfg.OnReady != nil {
		cfg.OnReady(httpURL)
	}
	if cfg.OpenBrowser {
		openBrowser(httpURL)
	}

	<-ctx.Done()

	// ---- graceful shutdown ----
	// ① BYE 브로드캐스트 + discovery 정리는 dsvc.Run이 ctx.Done에서 수행한다.
	// ② HTTP·HTTPS 서버를 3초 타임아웃 안에 동시에 정지.
	shCtx, shCancel := context.WithTimeout(context.Background(), shutdownGrace)
	_ = httpSrv.Shutdown(shCtx)
	_ = httpsSrv.Shutdown(shCtx)
	shCancel()
	// ③ hub/ticker는 ctx.Done로 이미 멈춘다. ④ 전부 합류.
	wg.Wait()
	slog.Info("redphone stopped")
	return nil
}

// sweepPeers periodically expires silent peers and pushes the new list.
func sweepPeers(ctx context.Context, reg *peer.Registry, pushPeers func()) {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if removed := reg.Expire(peerTTL); len(removed) > 0 {
				pushPeers()
			}
		}
	}
}

// targetControl adapts the discovery service to web.PeerControl and persists the
// friend-IP list to peers.json on every change.
//
// WHY: 발견(discovery)은 순수 네트워킹만, 영속화는 app이 책임진다(peerbook).
// 추가/삭제가 곧바로 디스크에 반영돼 재시작해도 친구 목록이 유지된다.
type targetControl struct {
	svc  *discovery.Service
	path string
	mu   sync.Mutex // 동시 요청의 save 직렬화
}

func (c *targetControl) AddTarget(ip string) error {
	if err := c.svc.AddTarget(ip); err != nil {
		return err
	}
	c.save()
	return nil
}

func (c *targetControl) RemoveTarget(ip string) {
	c.svc.RemoveTarget(ip)
	c.save()
}

func (c *targetControl) Targets() []string     { return c.svc.Targets() }
func (c *targetControl) ScanLAN() (int, error) { return c.svc.ScanLAN() }

func (c *targetControl) save() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := peerbook.Save(c.path, c.svc.Targets()); err != nil {
		slog.Warn("peers.json 저장 실패", "err", err, "path", c.path)
	}
}

func stdinOf(cfg Config) io.Reader {
	if cfg.Stdin != nil {
		return cfg.Stdin
	}
	return os.Stdin
}

func defaultInstanceIDPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), instanceIDFile)
	}
	return instanceIDFile
}

func defaultNamePath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), nameFile)
	}
	return nameFile
}

func loadOrCreateStableID(path string) (string, error) {
	if path == "" {
		path = defaultInstanceIDPath()
	}
	if b, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	id := newID()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(id+"\n"), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return id, nil
}

func loadSavedName(path, fallback string) (string, error) {
	if path == "" {
		path = defaultNamePath()
	}
	if b, err := os.ReadFile(path); err == nil {
		if name := strings.TrimSpace(string(b)); name != "" {
			return name, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return strings.TrimSpace(fallback), nil
}

type nameState struct {
	mu   sync.RWMutex
	path string
	name string
}

func newNameState(path, initial string) *nameState {
	if path == "" {
		path = defaultNamePath()
	}
	return &nameState{path: path, name: strings.TrimSpace(initial)}
}

func (s *nameState) Get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.name
}

func (s *nameState) Set(next string) (string, error) {
	next = strings.TrimSpace(next)
	if next == "" {
		return "", fmt.Errorf("name required")
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(next+"\n"), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	s.mu.Lock()
	s.name = next
	s.mu.Unlock()
	return next, nil
}

// watchStdin reads console lines and cancels on a bare "exit".
func watchStdin(ctx context.Context, r io.Reader, cancel context.CancelFunc) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "exit" {
			cancel()
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// newID returns a random 128-bit hex id used to filter our own broadcasts.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 극히 드문 경우의 폴백 — 시간 기반(충돌 무해, id는 식별용일 뿐).
		return fmt.Sprintf("rp-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
