package web

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chobocho/redphone/internal/peer"
	"github.com/chobocho/redphone/internal/share"
)

func newShareServer(t *testing.T) (*Server, *share.Store) {
	t.Helper()
	st := share.NewStore(t.TempDir())
	srv := New(Options{Reg: peer.NewRegistry(), Shares: st})
	return srv, st
}

func get(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// 1x1 투명 PNG.
var pngBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

func TestServeShareImageInline(t *testing.T) {
	srv, st := newShareServer(t)
	sh, _ := st.Add("dot.png", bytes.NewReader(pngBytes))

	rec := get(t, srv, "/s/"+sh.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Fatalf("want image content-type, got %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Fatalf("want inline disposition, got %q", cd)
	}
}

func TestServeShareTextPreviewEscapes(t *testing.T) {
	srv, st := newShareServer(t)
	sh, _ := st.Add("notes.txt", strings.NewReader("<script>alert(1)</script>"))

	rec := get(t, srv, "/s/"+sh.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("want text/html preview, got %q", ct)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("text not escaped — XSS risk")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("escaped content not present in preview")
	}
}

func TestServeShareOtherDownloads(t *testing.T) {
	srv, st := newShareServer(t)
	sh, _ := st.Add("data.bin", bytes.NewReader([]byte{0x00, 0x01, 0x02, 0x03, 0xff}))

	rec := get(t, srv, "/s/"+sh.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Fatalf("want attachment disposition, got %q", cd)
	}
}

func TestServeShareUnknownToken404(t *testing.T) {
	srv, _ := newShareServer(t)
	if rec := get(t, srv, "/s/doesnotexist"); rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for bad token, got %d", rec.Code)
	}
}

func TestShareUploadReturnsTokenAndURL(t *testing.T) {
	srv, _ := newShareServer(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "pic.png")
	fw.Write(pngBytes)
	mw.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/share", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d", rec.Code)
	}
	var got struct {
		Token, Kind, Name, URL string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Token == "" || got.Kind != "image" || !strings.Contains(got.URL, "/s/"+got.Token) {
		t.Fatalf("unexpected upload response: %+v", got)
	}
}

func TestShareRevokeThen404(t *testing.T) {
	srv, st := newShareServer(t)
	sh, _ := st.Add("x.bin", bytes.NewReader([]byte("data")))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/share/"+sh.Token, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d", rec.Code)
	}
	if r2 := get(t, srv, "/s/"+sh.Token); r2.Code != http.StatusNotFound {
		t.Fatalf("want 404 after revoke, got %d", r2.Code)
	}
}
