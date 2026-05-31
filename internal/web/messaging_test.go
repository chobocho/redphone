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

func mustEntries(t *testing.T, h *message.History) []message.Entry {
	t.Helper()
	all, err := h.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	return all
}

func TestSendRoundTripAndWSPush(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bHist := message.NewHistory()
	defer bHist.Close()
	bIP, bPort, bFP := startReceiver(t, ctx, "B", "bob", bHist, "")

	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice", NowMs: func() int64 { return 42 }})
	go aSrv.Hub().Run(ctx)
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()

	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: bIP, HTTPSPort: bPort, Fingerprint: bFP})

	body, _ := json.Marshal(map[string]string{"peerId": "B", "text": "hello bob"})
	resp, err := http.Post(aTS.URL+"/api/send", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send status = %d", resp.StatusCode)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(mustEntries(t, bHist)) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	all := mustEntries(t, bHist)
	if len(all) != 1 || all[0].Text != "hello bob" || all[0].From != "alice" || all[0].TS != 42 {
		t.Fatalf("history mismatch: %+v", all)
	}
	if all[0].Dir != "in" || all[0].PeerID != "A" {
		t.Fatalf("unexpected stored entry: %+v", all[0])
	}
}

func TestSendPushesToReceiverWS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tid, _ := tlsid.Generate("bob")
	bHist := message.NewHistory()
	defer bHist.Close()
	bReg := peer.NewRegistry()
	bSrv := New(Options{Reg: bReg, SelfID: "B", Name: "bob", History: bHist})
	go bSrv.Hub().Run(ctx)

	bHTTPS := httptest.NewUnstartedServer(bSrv.Handler())
	bHTTPS.TLS = tid.ServerTLSConfig()
	bHTTPS.StartTLS()
	defer bHTTPS.Close()
	bHTTP := httptest.NewServer(bSrv.Handler())
	defer bHTTP.Close()

	bIP, bsPort := hostPort(t, bHTTPS.URL)

	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice"})
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()
	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: bIP, HTTPSPort: bsPort, Fingerprint: tid.Fingerprint})

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
	if ev.Type != "entry" || ev.Entry == nil || ev.Entry.Text != "hi" || ev.Entry.Dir != "in" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestBroadcastReachesAllPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h1 := message.NewHistory()
	defer h1.Close()
	ip1, p1, fp1 := startReceiver(t, ctx, "B", "bob", h1, "")
	h2 := message.NewHistory()
	defer h2.Close()
	ip2, p2, fp2 := startReceiver(t, ctx, "C", "carol", h2, "")

	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice"})
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()

	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: ip1, HTTPSPort: p1, Fingerprint: fp1})
	aReg.Upsert(peer.Peer{ID: "C", Name: "carol", IP: ip2, HTTPSPort: p2, Fingerprint: fp2})
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
	var got struct {
		Sent   int
		Failed int
		Entry  *message.Entry
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Sent != 2 || got.Failed != 1 {
		t.Fatalf("want sent=2 failed=1, got %+v", got)
	}
	if got.Entry == nil || got.Entry.PeerID != message.BroadcastPeerID || got.Entry.Dir != "out" {
		t.Fatalf("unexpected response entry: %+v", got.Entry)
	}

	waitFor(t, func() bool { return len(mustEntries(t, h1)) == 1 && len(mustEntries(t, h2)) == 1 })
	if mustEntries(t, h1)[0].Text != "all hands" || mustEntries(t, h2)[0].Text != "all hands" {
		t.Fatalf("payload mismatch: %+v %+v", mustEntries(t, h1), mustEntries(t, h2))
	}
}

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
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Sent != 0 || got.Failed != 0 {
		t.Fatalf("want sent=0 failed=0, got %+v", got)
	}
}

func TestDeleteHistoryClearsSelectedPeer(t *testing.T) {
	hist := message.NewHistory()
	defer hist.Close()
	if _, err := hist.AddEntry(message.Entry{PeerID: "A", Dir: "in", Text: "keep"}); err != nil {
		t.Fatalf("AddEntry A: %v", err)
	}
	if _, err := hist.AddEntry(message.Entry{PeerID: "B", Dir: "in", Text: "drop"}); err != nil {
		t.Fatalf("AddEntry B: %v", err)
	}

	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "me", Name: "me", History: hist})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/history/B", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	all := mustEntries(t, hist)
	if len(all) != 1 || all[0].PeerID != "A" {
		t.Fatalf("unexpected history after clear: %+v", all)
	}
}

func TestDeleteEntryRemovesOnlySelectedMessage(t *testing.T) {
	hist := message.NewHistory()
	defer hist.Close()
	keep, err := hist.AddEntry(message.Entry{PeerID: "A", Dir: "in", Text: "keep"})
	if err != nil {
		t.Fatalf("AddEntry keep: %v", err)
	}
	drop, err := hist.AddEntry(message.Entry{PeerID: "B", Dir: "out", Text: "drop"})
	if err != nil {
		t.Fatalf("AddEntry drop: %v", err)
	}

	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "me", Name: "me", History: hist})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/history/entry/"+strconv.FormatInt(drop.ID, 10), nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	all := mustEntries(t, hist)
	if len(all) != 1 || all[0].ID != keep.ID {
		t.Fatalf("unexpected history after delete: %+v", all)
	}
}

func TestDeleteEntryReturns404WhenMissing(t *testing.T) {
	hist := message.NewHistory()
	defer hist.Close()
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "me", Name: "me", History: hist})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/history/entry/999", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

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

func TestSendDeliveryFailureReturns502(t *testing.T) {
	reg := peer.NewRegistry()
	reg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: "127.0.0.1", HTTPSPort: 1, Fingerprint: "deadbeef"})
	srv := New(Options{Reg: reg, SelfID: "A", Name: "alice",
		PeerTLS: func(string) *http.Client {
			return &http.Client{
				Timeout:   300 * time.Millisecond,
				Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
			}
		},
	})
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"peerId": "B", "text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 on delivery failure, got %d", rec.Code)
	}
}

func TestSendRefusesPeerWithoutFingerprint(t *testing.T) {
	reg := peer.NewRegistry()
	reg.Upsert(peer.Peer{ID: "L", Name: "legacy", IP: "127.0.0.1", HTTPSPort: 1})
	srv := New(Options{Reg: reg, SelfID: "A", Name: "alice"})
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"peerId": "L", "text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 for fp-less peer, got %d", rec.Code)
	}
}

func TestSendRejectsFingerprintMismatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tid, _ := tlsid.Generate("bob")
	hist := message.NewHistory()
	defer hist.Close()
	bSrv := New(Options{Reg: peer.NewRegistry(), SelfID: "B", Name: "bob", History: hist})
	go bSrv.Hub().Run(ctx)
	bTS := httptest.NewUnstartedServer(bSrv.Handler())
	bTS.TLS = tid.ServerTLSConfig()
	bTS.StartTLS()
	defer bTS.Close()
	bIP, bPort := hostPort(t, bTS.URL)

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

func TestInboxMessageRejectsPlainHTTP(t *testing.T) {
	hist := message.NewHistory()
	defer hist.Close()
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "B", Name: "bob", History: hist})
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
