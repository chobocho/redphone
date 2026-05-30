package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chobocho/redphone/internal/message"
	"github.com/chobocho/redphone/internal/peer"
	"github.com/coder/websocket"
)

// hostPort splits an httptest URL into IP and port for registry registration.
func hostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	host, port, err := net.SplitHostPort(strings.TrimPrefix(rawURL, "http://"))
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	return host, p
}

// A→B 라운드트립: A의 /api/send가 B의 /inbox/message로 중계되고, B는
// 히스토리에 저장 + WS로 브라우저에 푸시한다.
func TestSendRoundTripAndWSPush(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 수신자 B
	bReg := peer.NewRegistry()
	bHist := message.NewHistory()
	bSrv := New(Options{Reg: bReg, SelfID: "B", Name: "bob", History: bHist})
	go bSrv.Hub().Run(ctx)
	bTS := httptest.NewServer(bSrv.Handler())
	defer bTS.Close()

	// 송신자 A — B를 피어로 등록
	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice", NowMs: func() int64 { return 42 }})
	go aSrv.Hub().Run(ctx)
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()

	bIP, bPort := hostPort(t, bTS.URL)
	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: bIP, HTTPPort: bPort})

	// B의 브라우저(WS) 연결
	wsURL := "ws" + strings.TrimPrefix(bTS.URL, "http") + "/ws"
	dctx, dcancel := context.WithTimeout(ctx, 2*time.Second)
	defer dcancel()
	wsConn, _, err := websocket.Dial(dctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer wsConn.Close(websocket.StatusNormalClosure, "")
	time.Sleep(50 * time.Millisecond)

	// A → send
	body, _ := json.Marshal(map[string]string{"peerId": "B", "text": "hello bob"})
	resp, err := http.Post(aTS.URL+"/api/send", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send status = %d", resp.StatusCode)
	}

	// B의 WS로 푸시가 도착해야 한다
	rctx, rcancel := context.WithTimeout(ctx, 2*time.Second)
	defer rcancel()
	_, data, err := wsConn.Read(rctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var ev wsEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("event decode: %v", err)
	}
	if ev.Type != "message" || ev.Note == nil || ev.Note.Text != "hello bob" || ev.Note.From != "alice" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	// B의 히스토리에도 저장돼야 한다
	all := bHist.All()
	if len(all) != 1 || all[0].Text != "hello bob" || all[0].TS != 42 {
		t.Fatalf("history mismatch: %+v", all)
	}
}

// 오프라인 피어로 보내면 404로 실패를 명시해야 한다.
func TestSendToUnknownPeerReturns404(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "A", Name: "alice"})
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"peerId": "ghost", "text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for offline peer, got %d", rec.Code)
	}
}

// 피어는 발견됐지만 전송이 실패(주소 죽음)하면 502로 표시.
func TestSendDeliveryFailureReturns502(t *testing.T) {
	reg := peer.NewRegistry()
	// 닫힌 포트로 유도(연결 거부). 127.0.0.1:1 은 거의 항상 실패.
	reg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: "127.0.0.1", HTTPPort: 1})
	srv := New(Options{Reg: reg, SelfID: "A", Name: "alice",
		Client: &http.Client{Timeout: 500 * time.Millisecond}})
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"peerId": "B", "text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 on delivery failure, got %d", rec.Code)
	}
}
