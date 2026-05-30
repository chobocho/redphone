package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chobocho/redphone/internal/peer"
	"github.com/coder/websocket"
)

// 실제 websocket 핸드셰이크를 거쳐 hub.Broadcast가 브라우저까지 도달하는지 검증.
func TestWSEndpointDeliversBroadcast(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "me"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Hub().Run(ctx)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	dialCtx, dialCancel := context.WithTimeout(ctx, 2*time.Second)
	defer dialCancel()
	c, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// 클라이언트가 hub에 등록될 시간을 잠깐 준 뒤 브로드캐스트.
	time.Sleep(50 * time.Millisecond)
	srv.Hub().Broadcast([]byte(`{"type":"hello"}`))

	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()
	typ, data, err := c.Read(readCtx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if typ != websocket.MessageText || string(data) != `{"type":"hello"}` {
		t.Fatalf("unexpected ws frame: typ=%v data=%s", typ, data)
	}
}

func TestWSRejectsAfterHubStopped(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry()})
	hubCtx, stopHub := context.WithCancel(context.Background())
	go srv.Hub().Run(hubCtx)
	stopHub() // hub 즉시 정지
	time.Sleep(20 * time.Millisecond)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	dialCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		// 핸드셰이크 자체가 실패할 수도 있으나, 성공하면 즉시 닫혀야 한다.
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	// hub가 멈췄으니 등록 실패 → 서버가 곧 close. Read가 에러로 끝나야 한다.
	readCtx, rc := context.WithTimeout(context.Background(), 2*time.Second)
	defer rc()
	if _, _, err := c.Read(readCtx); err == nil {
		t.Fatal("expected connection to be closed when hub stopped")
	}
}
