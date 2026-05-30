package web

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const writeTimeout = 10 * time.Second

// handleWS upgrades the connection and bridges it to the hub.
//
// WHY: hub는 전송 방식을 모른다(순수 채널). 여기서 client.send를 읽어 ws로
// 밀고(write pump), 읽기 루프는 브라우저 종료를 감지해 등록 해제한다.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// LAN 한정 도구라 origin은 검사하지 않는다(IP/호스트명 혼용 접속 허용).
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	client := NewClient()
	if !s.hub.Register(client) {
		c.Close(websocket.StatusGoingAway, "shutting down")
		return
	}
	ctx := r.Context()

	// write pump: hub가 채널을 닫으면(range 종료) 연결도 닫는다.
	go func() {
		for msg := range client.Send() {
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				break
			}
		}
		c.Close(websocket.StatusNormalClosure, "")
	}()

	// read pump: 제어 프레임 처리 + 종료 감지. 수신 페이로드는 현재 쓰지 않는다.
	for {
		if _, _, err := c.Read(ctx); err != nil {
			break
		}
	}
	s.hub.Unregister(client) // 이미 제거됐으면 no-op
}
