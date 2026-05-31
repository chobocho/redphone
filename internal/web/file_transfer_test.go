package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chobocho/redphone/internal/peer"
	"github.com/chobocho/redphone/internal/tlsid"
)

// multipartFile builds a peerId+file multipart body in the order the handler expects.
func multipartFile(t *testing.T, peerID, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("peerId", peerID); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	mw.Close()
	return &buf, mw.FormDataContentType()
}

// startReceiverDual spins up B with HTTP and HTTPS on the same mux, returning
// (ip, httpPort, httpsPort, fp, downloadDir). 같은 mux를 양쪽에 붙이는 이유:
// requireTLS는 /inbox/file/announce만 막고 본문 PUT은 HTTP로 받아야 한다.
func startReceiverDual(t *testing.T, ctx context.Context) (string, int, int, string, string, *Server) {
	t.Helper()
	tid, _ := tlsid.Generate("bob")
	dir := t.TempDir()
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "B", Name: "bob", DownloadDir: dir})
	go srv.Hub().Run(ctx)

	tsHTTPS := httptest.NewUnstartedServer(srv.Handler())
	tsHTTPS.TLS = tid.ServerTLSConfig()
	tsHTTPS.StartTLS()
	t.Cleanup(tsHTTPS.Close)
	tsHTTP := httptest.NewServer(srv.Handler())
	t.Cleanup(tsHTTP.Close)

	ip, httpsPort := hostPort(t, tsHTTPS.URL)
	_, httpPort := hostPort(t, tsHTTP.URL)
	return ip, httpPort, httpsPort, tid.Fingerprint, dir, srv
}

// A→B 파일 전송: announce(HTTPS) → 토큰 발급 → 본문 PUT(HTTP) → 저장.
// 저장된 파일의 SHA-256이 보낸 것과 동일해야 한다.
func TestSendFileRoundTripSHA256(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bIP, bHTTP, bHTTPS, bFP, bDir, _ := startReceiverDual(t, ctx)

	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice"})
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()

	aReg.Upsert(peer.Peer{
		ID: "B", Name: "bob", IP: bIP,
		HTTPPort: bHTTP, HTTPSPort: bHTTPS, Fingerprint: bFP,
	})

	blob := make([]byte, 2*1024*1024)
	if _, err := rand.Read(blob); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(blob)

	body, ct := multipartFile(t, "B", "photo.bin", blob)
	resp, err := http.Post(aTS.URL+"/api/sendfile", ct, body)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sendfile status = %d", resp.StatusCode)
	}

	saved := filepath.Join(bDir, "photo.bin")
	got, err := os.ReadFile(saved)
	if err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
	if sha256.Sum256(got) != want {
		t.Fatal("SHA-256 mismatch after transfer")
	}
}

// 경로 탈출 파일명은 downloads 밖으로 나가면 안 된다.
// announce에서 파일명을 수신해 저장 측에서 SafeName으로 정화한다.
func TestInboxFileBlocksPathEscape(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bIP, bHTTP, bHTTPS, bFP, bDir, _ := startReceiverDual(t, ctx)

	aReg := peer.NewRegistry()
	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: bIP, HTTPPort: bHTTP, HTTPSPort: bHTTPS, Fingerprint: bFP})
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice"})
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()

	body, ct := multipartFile(t, "B", "../../escape.bin", []byte("evil"))
	resp, err := http.Post(aTS.URL+"/api/sendfile", ct, body)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sendfile status = %d", resp.StatusCode)
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(bDir), "escape.bin")); err == nil {
		t.Fatal("file escaped the download dir")
	}
	if _, err := os.Stat(filepath.Join(bDir, "escape.bin")); err != nil {
		t.Fatalf("file not saved inside dir: %v", err)
	}
}

// announce는 TLS 전용 — 평문 HTTP로 호출하면 421.
func TestAnnounceRejectsPlainHTTP(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "B", Name: "bob", DownloadDir: t.TempDir()})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/inbox/file/announce",
		strings.NewReader(`{"filename":"x.bin"}`))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("want 421 on plain HTTP announce, got %d", rec.Code)
	}
}

// 미공지 토큰으로 PUT하면 404.
func TestInboxFileBodyRejectsUnknownToken(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "B", Name: "bob", DownloadDir: t.TempDir()})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/inbox/file/deadbeef", strings.NewReader("x"))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for unknown token, got %d", rec.Code)
	}
}

// 토큰은 1회용 — 같은 토큰으로 두 번 PUT하면 두 번째는 404.
func TestUploadTokenIsSingleUse(t *testing.T) {
	store := newUploadStore()
	tok, err := store.reserve("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.claim(tok); !ok {
		t.Fatal("first claim should succeed")
	}
	if _, ok := store.claim(tok); ok {
		t.Fatal("second claim should fail (single-use)")
	}
}

// 토큰은 TTL 후 만료된다.
func TestUploadTokenExpires(t *testing.T) {
	store := newUploadStore()
	now := time.Now()
	store.nowFn = func() time.Time { return now }
	store.ttl = 50 * time.Millisecond

	tok, _ := store.reserve("x.bin")
	now = now.Add(time.Second) // 시계를 TTL 너머로
	if _, ok := store.claim(tok); ok {
		t.Fatal("expired token should not claim")
	}
}
