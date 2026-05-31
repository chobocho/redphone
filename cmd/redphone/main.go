// Command redphone is the single-binary LAN messenger entrypoint.
//
// WHY: main은 플래그 파싱과 app.Run 호출만 한다. 실제 배선·생명주기는
// internal/app이 소유해 테스트 가능성을 확보한다.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chobocho/redphone/internal/app"
)

func main() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "RedPhone - LAN messenger")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  redphone [options]")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Web UI:")
		fmt.Fprintln(out, "  F1   도움말")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Examples:")
		fmt.Fprintln(out, "  redphone")
		fmt.Fprintln(out, "  redphone --name alice --open=false")
		fmt.Fprintln(out, "  redphone --port 18080 --sport 18443 --scan")
	}

	name := flag.String("name", defaultName(), "표시 이름")
	port := flag.Int("port", 17080, "HTTP 포트(사용 중이면 OS 자동 폴백)")
	sport := flag.Int("sport", 17443, "HTTPS 포트(쪽지·파일메타 전용, 사용 중이면 OS 자동 폴백)")
	dport := flag.Int("dport", 17000, "Discovery UDP 포트")
	dbPath := flag.String("db", "redphone.db", "SQLite 히스토리 DB 경로")
	open := flag.Bool("open", true, "기동 시 기본 브라우저 자동 오픈")
	scan := flag.Bool("scan", false, "서브넷 전체에 주기적 유니캐스트 HELLO 스캔(기본 꺼짐; 막히는 망 주의)")
	flag.Parse()

	if err := app.Run(context.Background(), app.Config{
		Name:          *name,
		HTTPPort:      *port,
		HTTPSPort:     *sport,
		DiscoveryPort: *dport,
		HistoryPath:   *dbPath,
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
