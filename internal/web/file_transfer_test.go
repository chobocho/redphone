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
	"testing"

	"github.com/chobocho/redphone/internal/peer"
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

// A→B 파일 전송: B의 downloads에 저장되고 SHA-256이 동일해야 한다.
func TestSendFileRoundTripSHA256(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bDir := t.TempDir()
	bReg := peer.NewRegistry()
	bSrv := New(Options{Reg: bReg, SelfID: "B", Name: "bob", DownloadDir: bDir})
	go bSrv.Hub().Run(ctx)
	bTS := httptest.NewServer(bSrv.Handler())
	defer bTS.Close()

	aReg := peer.NewRegistry()
	aSrv := New(Options{Reg: aReg, SelfID: "A", Name: "alice"})
	aTS := httptest.NewServer(aSrv.Handler())
	defer aTS.Close()

	bIP, bPort := hostPort(t, bTS.URL)
	aReg.Upsert(peer.Peer{ID: "B", Name: "bob", IP: bIP, HTTPPort: bPort})

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
func TestInboxFileBlocksPathEscape(t *testing.T) {
	dir := t.TempDir()
	srv := New(Options{Reg: peer.NewRegistry(), SelfID: "B", DownloadDir: dir})

	body, ct := multipartFile(t, "", "../../escape.bin", []byte("evil"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/inbox/file", body)
	req.Header.Set("Content-Type", ct)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("inbox/file status = %d", rec.Code)
	}
	// 상위 디렉터리로 새지 않았는지 확인.
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.bin")); err == nil {
		t.Fatal("file escaped the download dir")
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.bin")); err != nil {
		t.Fatalf("file not saved inside dir: %v", err)
	}
}
