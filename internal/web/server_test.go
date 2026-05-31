package web

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chobocho/redphone/internal/peer"
)

func TestPeersHandlerReturnsSnapshot(t *testing.T) {
	reg := peer.NewRegistry()
	reg.Upsert(peer.Peer{ID: "a", Name: "alpha", IP: "10.0.0.1", HTTPPort: 17080})
	reg.Upsert(peer.Peer{ID: "b", Name: "bravo", IP: "10.0.0.2", HTTPPort: 17081})

	srv := New(Options{Reg: reg, SelfID: "me", Name: "me"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/peers", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("want json content-type, got %q", ct)
	}
	var peers []peer.Peer
	if err := json.Unmarshal(rec.Body.Bytes(), &peers); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(peers) != 2 || peers[0].Name != "alpha" {
		t.Fatalf("unexpected peers: %+v", peers)
	}
}

func TestStaticIndexServed(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry()})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for /, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "RedPhone") {
		t.Fatalf("embedded index not served: %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("want utf-8 html content-type, got %q", ct)
	}
}

func TestStaticAssetsServedAsUTF8(t *testing.T) {
	srv := New(Options{Reg: peer.NewRegistry()})

	cases := []struct {
		path string
		want string
	}{
		{path: "/style.css", want: "text/css; charset=utf-8"},
		{path: "/app.js", want: "text/javascript; charset=utf-8"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			srv.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("want 200 for %s, got %d", tc.path, rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != tc.want {
				t.Fatalf("want %q, got %q", tc.want, ct)
			}
		})
	}
}

// WHY: 선호 포트가 점유돼 있어도 앱은 떠야 한다 → OS 자동 폴백.
func TestListenFallsBackWhenPortBusy(t *testing.T) {
	// Listen이 바인딩하는 것과 동일한 ":port"(전체 인터페이스)로 점유해야
	// 실제 충돌이 발생한다. 127.0.0.1 한정 바인딩은 0.0.0.0:port를 막지 못한다.
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port

	ln, err := Listen(busyPort)
	if err != nil {
		t.Fatalf("Listen should fall back, got error: %v", err)
	}
	defer ln.Close()

	if got := ln.Addr().(*net.TCPAddr).Port; got == busyPort {
		t.Fatalf("expected a different port than busy %d, got same", busyPort)
	}
}
