package app

import (
	"log"
	"os/exec"
	"runtime"
)

// openBrowser opens url in the OS default browser. best-effort — 실패해도
// 콘솔에 URL이 찍혀 있으니 앱은 계속 동작한다.
//
// WHY: 원본 빨간전화기의 "더블클릭하면 바로 뜨는" 경험을 재현한다.
func openBrowser(url string) {
	var (
		cmd  string
		args []string
	)
	switch runtime.GOOS {
	case "windows":
		// rundll32은 따옴표/특수문자 이슈가 적어 cmd start보다 안전하다.
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd, args = "open", []string{url}
	default: // linux, *bsd
		cmd, args = "xdg-open", []string{url}
	}
	if err := exec.Command(cmd, args...).Start(); err != nil {
		log.Printf("redphone: 브라우저 자동 오픈 실패(%v) — 수동으로 %s 열어주세요", err, url)
	}
}
