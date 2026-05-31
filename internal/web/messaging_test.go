package web

import (
	"bytes"
	"context"
	"crypto/tls"
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
	"github.com/chobocho/redphone/internal/tlsid"
	"github.com/coder/websocket"
)

// hostPort splits an httptest URL into IP and port for registry registration.
// http:// 와 https:// 양쪽 모두 다룬다.
func hostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	stripped := strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
	host, port, err := net.SplitHostPort(stripped)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	return host, p
}

// startReceiver spins up a TLS httptest server backed by a fresh Identity and
// returns the host/port the sender's registry should pin to.
func startReceiver(t *testing.T, ctx context.Context, id, name string, hist *message.History, dir string) (string, int, string) {
	t.Helper()
	tid, err := tlsid.Generate(name)
	if err != nil {
		t.Fatalf("tlsid: %v", err)
	}
	reg := peer.NewRegistry()
	srv := New(Options{Reg: reg, SelfID: id, Name: name, History: hist, DownloadDir: dir})
	go srv.Hub().Run(ctx)

	ts := httptest.NewUnstartedServer(srv.Handler())
	ts.TLS = tid.ServerTLSConfig()
	ts.StartTLS()
	t.Cleanup(ts.Close)

	host, port := hostPort(t, ts.URL)
	return host, port, tid.Fingerprint
}

// A→B 라운드트립: A의 /api/send가 B의 HTTPS /inbox/message로 (지문 핀닝)
// 중계되고, B는 히스토리에 저장 + WS로 푸시한다.
func TestSendRoundTripAndWSPush(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 수신자 B (HTTPS)
	bHist := message.NewHistory()
	bIP, bPort, bFP := startReceiver(t, ctx, "B", "bob", bHist, "")

	// 송신자 A — B를 피어로 등록(지문 포함)
	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice", NowMs: func() int64 { return 42 }})
	go aSrv.Hub().Run(ctx)
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()

	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: bIP, HTTPSPort: bPort, Fingerprint: bFP})

	// B의 브라우저(WS) 연결 — 평문 HTTP 쪽으로 새 ts 만들어 WS push 받기.
	// httptest.NewServer는 같은 핸들러여도 새 인스턴스라 hub가 다르므로
	// B는 이미 TLS로만 떠 있다. WS 검증은 히스토리로 대체.
	body, _ := json.Marshal(map[string]string{"peerId": "B", "text": "hello bob"})
	resp, err := http.Post(aTS.URL+"/api/send", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send status = %d", resp.StatusCode)
	}

	// B의 히스토리에 저장 확인(WS push도 같은 코드 경로).
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(bHist.All()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	all := bHist.All()
	if len(all) != 1 || all[0].Text != "hello bob" || all[0].From != "alice" || all[0].TS != 42 {
		t.Fatalf("history mismatch: %+v", all)
	}
}

// WS 푸시까지 같이 검증하는 별도 케이스 — 송신자가 자기 자신의 hub로
// 자기 메시지를 푸시하지 않으므로, 수신자 B에서 같은 hub를 WS로 본다.
func TestSendPushesToReceiverWS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// B를 한 번에 같은 mux로 HTTP+HTTPS 두 서버에 붙여, HTTPS로는 inbox를
	// 받고 평문 HTTP로는 WS만 잡는다.
	tid, _ := tlsid.Generate("bob")
	bHist := message.NewHistory()
	bReg := peer.NewRegistry()
	bSrv := New(Options{Reg: bReg, SelfID: "B", Name: "bob", History: bHist})
	go bSrv.Hub().Run(ctx)

	bHTTPS := httptest.NewUnstartedServer(bSrv.Handler())
	bHTTPS.TLS = tid.ServerTLSConfig()
	bHTTPS.StartTLS()
	defer bHTTPS.Close()
	bHTTP := httptest.NewServer(bSrv.Handler()) // WS용 평문 채널
	defer bHTTP.Close()

	bIP, bsPort := hostPort(t, bHTTPS.URL)

	// A
	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice"})
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()
	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: bIP, HTTPSPort: bsPort, Fingerprint: tid.Fingerprint})

	// B의 WS 구독
	wsURL := "ws" + strings.TrimPrefix(bHTTP.URL, "http") + "/ws"
	dctx, dcancel := context.WithTimeout(ctx, 2*time.Second)
	defer dcancel()
	wsConn, _, err := websocket.Dial(dctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer wsConn.Close(websocket.StatusNormalClosure, "")
	time.Sleep(30 * time.Millisecond)

	body, _ := json.Marshal(map[string]string{"peerId": "B", "text": "hi"})
	resp, err := http.Post(aTS.URL+"/api/send", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	rctx, rcancel := context.WithTimeout(ctx, 2*time.Second)
	defer rcancel()
	_, data, err := wsConn.Read(rctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var ev wsEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Type != "message" || ev.Note == nil || ev.Note.Text != "hi" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

// 전체 쪽지: 모든 피어에 전달. 각 수신자는 자체 지문 + HTTPS 포트를 갖는다.
func TestBroadcastReachesAllPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h1 := message.NewHistory()
	ip1, p1, fp1 := startReceiver(t, ctx, "B", "bob", h1, "")
	h2 := message.NewHistory()
	ip2, p2, fp2 := startReceiver(t, ctx, "C", "carol", h2, "")

	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice"})
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()

	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: ip1, HTTPSPort: p1, Fingerprint: fp1})
	aReg.Upsert(peer.Peer{ID: "C", Name: "carol", IP: ip2, HTTPSPort: p2, Fingerprint: fp2})
	// 죽은 피어(연결 거부) — 지문은 있어야 송신 시도가 일어남.
	aReg.Upsert(peer.Peer{ID: "D", Name: "dead", IP: "127.0.0.1", HTTPSPort: 1, Fingerprint: "deadbeef"})

	body, _ := json.Marshal(map[string]string{"text": "all hands"})
	resp, err := http.Post(aTS.URL+"/api/broadcast", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("broadcast status = %d", resp.StatusCode)
	}
	var got struct{ Sent, Failed int }
	json.NewDecoder(resp.Body).Decode(&got)
	if got.Sent != 2 || got.Failed != 1 {
		t.Fatalf("want sent=2 failed=1, got %+v", got)
	}

	// 수신 확인(짧은 폴 — TLS 핸드셰이크 변동성 흡수)
	waitFor(t, func() bool { return len(h1.All()) == 1 && len(h2.All()) == 1 })
	if h1.All()[0].Text != "all hands" || h2.All()[0].Text != "all hands" {
		t.Fatalf("payload mismatch: %+v %+v", h1.All(), h2.All())
	}
}

// 피어가 없을 때 전체 쪽지는 sent=0으로 정상 처리(에러 아님).
func TestBroadcastWithNoPeers(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "A", Name: "alice"})
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"text": "hi"})
	req := httptest.NewRequest(http.MethodPost, "/api/broadcast", bytes.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var got struct{ Sent, Failed int }
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Sent != 0 || got.Failed != 0 {
		t.Fatalf("want sent=0 failed=0, got %+v", got)
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

// 피어 발견됐지만 TLS 송신 실패(주소 죽음) → 502.
func TestSendDeliveryFailureReturns502(t *testing.T) {
	reg := peer.NewRegistry()
	reg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: "127.0.0.1", HTTPSPort: 1, Fingerprint: "deadbeef"})
	srv := New(Options{Reg: reg, SelfID: "A", Name: "alice",
		PeerTLS: func(string) *http.Client {
			return &http.Client{Timeout: 300 * time.Millisecond,
				Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
		}})
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"peerId": "B", "text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 on delivery failure, got %d", rec.Code)
	}
}

// 지문이 비어 있으면 송신 자체를 거부해야 한다(v1 피어 차단).
func TestSendRefusesPeerWithoutFingerprint(t *testing.T) {
	reg := peer.NewRegistry()
	reg.Upsert(peer.Peer{ID: "L", Name: "legacy", IP: "127.0.0.1", HTTPSPort: 1}) // FP=""
	srv := New(Options{Reg: reg, SelfID: "A", Name: "alice"})
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"peerId": "L", "text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 for fp-less peer, got %d", rec.Code)
	}
}

// 지문이 일치하지 않으면 송신 실패해야 한다(MITM 방어 단위검증).
func TestSendRejectsFingerprintMismatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 실제 B는 fpReal로 응답.
	tid, _ := tlsid.Generate("bob")
	bSrv := New(Options{Reg: peer.NewRegistry(), SelfID: "B", Name: "bob", History: message.NewHistory()})
	go bSrv.Hub().Run(ctx)
	bTS := httptest.NewUnstartedServer(bSrv.Handler())
	bTS.TLS = tid.ServerTLSConfig()
	bTS.StartTLS()
	defer bTS.Close()
	bIP, bPort := hostPort(t, bTS.URL)

	// A는 잘못된 fp로 핀닝.
	aReg := peer.NewRegistry()
	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: bIP, HTTPSPort: bPort, Fingerprint: "0000"})
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice"})
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"peerId": "B", "text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewReader(body))
	aSrv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 on fp mismatch, got %d", rec.Code)
	}
}

// /inbox/message는 평문 HTTP로 들어오면 421로 거부해야 한다.
func TestInboxMessageRejectsPlainHTTP(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "B", Name: "bob", History: message.NewHistory()})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/inbox/message", strings.NewReader(`{"text":"x"}`))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("want 421 on plain HTTP inbox, got %d", rec.Code)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within 2s")
}
