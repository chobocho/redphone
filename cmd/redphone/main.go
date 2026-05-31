// Command redphone is the single-binary LAN messenger entrypoint.
//
// WHY: main은 플래그 파싱과 app.Run 호출만 한다. 실제 배선·생명주기는
// internal/app이 소유해 테스트 가능성을 확보한다.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/chobocho/redphone/internal/app"
)

func main() {
	name := flag.String("name", defaultName(), "표시 이름")
	port := flag.Int("port", 17080, "HTTP 포트(사용 중이면 OS 자동 폴백)")
	sport := flag.Int("sport", 17443, "HTTPS 포트(쪽지·파일메타 전용, 사용 중이면 OS 자동 폴백)")
	dport := flag.Int("dport", 17000, "Discovery UDP 포트")
	open := flag.Bool("open", true, "기동 시 기본 브라우저 자동 오픈")
	scan := flag.Bool("scan", false, "서브넷 전체에 주기적 유니캐스트 HELLO 스캔(기본 꺼짐; 막히는 망 주의)")
	flag.Parse()

	if err := app.Run(context.Background(), app.Config{
		Name:          *name,
		HTTPPort:      *port,
		HTTPSPort:     *sport,
		DiscoveryPort: *dport,
		OpenBrowser:   *open,
		ScanAll:       *scan,
	}); err != nil {
		log.Fatalf("redphone: %v", err)
	}
}

// defaultName falls back to the host name so peers are distinguishable
// out of the box without requiring --name.
func defaultName() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "redphone"
}
