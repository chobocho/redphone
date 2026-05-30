// Package web serves the local UI and the HTTP control/transfer plane.
//
// WHY: 발견만 UDP이고 그 외 전부 HTTP다. 정적 UI 서빙, /api/* 제어, 피어 간
// inbox 중계, /s 공유를 단일 mux로 묶어 포트/코드를 단순화한다.
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"

	"github.com/chobocho/redphone/internal/message"
	"github.com/chobocho/redphone/internal/peer"
)

//go:embed static
var staticFS embed.FS

// Options carries the dependencies the server needs from the rest of the app.
type Options struct {
	Reg     *peer.Registry
	SelfID  string
	Name    string
	History *message.History
	Client  *http.Client // 피어 중계용; nil이면 http.DefaultClient
	NowMs   func() int64  // 테스트 주입용; nil이면 wall clock
}

// Server owns the HTTP routing surface and the WS hub.
type Server struct {
	opt Options
	hub *Hub
}

// New constructs a Server with a fresh WS hub. 핸들러는 Handler()에서 조립한다.
func New(opt Options) *Server {
	return &Server{opt: opt, hub: NewHub()}
}

// Hub exposes the WS hub so the app lifecycle can Run it and push events.
func (s *Server) Hub() *Hub { return s.hub }

// Handler builds the routing mux. Go 1.22+ 메서드 패턴으로 메서드/경로를 분리한다.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/peers", s.handlePeers)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("POST /api/send", s.handleSend)
	mux.HandleFunc("POST /inbox/message", s.handleInboxMessage)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.Handle("GET /", s.staticHandler())
	return mux
}

func (s *Server) handlePeers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.opt.Reg.Snapshot())
}

// staticHandler serves the embedded web assets at the root.
func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// embed 디렉터리는 빌드 타임에 보장되므로 여기 도달하면 프로그래밍 오류.
		panic(fmt.Sprintf("web: embed sub: %v", err))
	}
	return http.FileServer(http.FS(sub))
}

// Listen binds the preferred port, falling back to an OS-assigned one.
//
// WHY: 같은 PC에서 두 인스턴스를 띄우거나 포트가 점유돼도 기동을 막지 않는다.
// 실제 포트는 호출자가 받아 HELLO에 실어 광고한다.
func Listen(port int) (net.Listener, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err == nil {
		return ln, nil
	}
	return net.Listen("tcp", ":0")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
