// Package web serves the local UI and the HTTP control/transfer plane.
//
// WHY: 발견만 UDP이고 그 외 전부 HTTP다. 정적 UI 서빙, /api/* 제어, 피어 간
// inbox 중계, /s 공유를 단일 mux로 묶어 포트/코드를 단순화한다.
package web

import (
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path"
	"sync"

	"github.com/chobocho/redphone/internal/message"
	"github.com/chobocho/redphone/internal/peer"
	"github.com/chobocho/redphone/internal/share"
)

//go:embed static
var staticFS embed.FS

// Options carries the dependencies the server needs from the rest of the app.
type Options struct {
	Reg         *peer.Registry
	SelfID      string
	Name        string
	NameGet     func() string
	Rename      func(string) (string, error)
	History     *message.History
	Client      *http.Client                 // 평문 HTTP(파일 본문 PUT 등)용; nil이면 http.DefaultClient
	PeerTLS     func(fp string) *http.Client // 피어로 TLS 송신할 때 핀닝된 클라이언트
	NowMs       func() int64                 // 테스트 주입용; nil이면 wall clock
	DownloadDir string                       // 수신 파일 저장 위치; ""이면 "downloads"
	Shares      *share.Store                 // URL 공유 저장소; nil이면 공유 비활성
	ShareHost   string                       // 공유 URL의 host:port 강제값; ""이면 r.Host
	Shutdown    func()                       // 종료 버튼 콜백(보통 root ctx의 cancel)
	Targets     PeerControl                  // 친구 IP 수동 관리 + 전체 스캔; nil이면 비활성
}

// PeerControl is the discovery side of manual friend-IP management, kept as an
// interface so the web layer doesn't import discovery (배선은 app이 한다).
type PeerControl interface {
	AddTarget(ip string) error // 잘못된 IP면 에러
	RemoveTarget(ip string)
	Targets() []string
	ScanLAN() (int, error) // 미기동이면 에러
}

// Server owns the HTTP routing surface and the WS hub.
type Server struct {
	opt Options
	hub *Hub

	// 업로드 토큰 저장소(handleInboxFileAnnounce ↔ handleInboxFileBody).
	// 첫 사용 시 lazy 초기화 → 테스트 격리 유지.
	uploadOnce  sync.Once
	uploadStore *uploadStore
}

// New constructs a Server with a fresh WS hub. 핸들러는 Handler()에서 조립한다.
func New(opt Options) *Server {
	return &Server{opt: opt, hub: NewHub()}
}

// Hub exposes the WS hub so the app lifecycle can Run it and push events.
func (s *Server) Hub() *Hub { return s.hub }

// Handler builds the routing mux. Go 1.22+ 메서드 패턴으로 메서드/경로를 분리한다.
//
// 보안 레이아웃(2026-05):
//   - /inbox/message            : TLS-only (requireTLS 가드)
//   - /inbox/file/announce      : TLS-only (파일명·크기만 암호화)
//   - /inbox/file/{token}       : 평문 HTTP 본문 스트림 (의도된 평문)
//   - 나머지 /api/*, /s/*, /, /ws : 둘 다 허용(평문 HTTP에서도 동작)
//
// 같은 mux를 HTTP·HTTPS 두 리스너에 동시에 붙이고, requireTLS가
// 평문 요청을 421로 거부한다.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/peers", s.handlePeers)
	mux.HandleFunc("GET /api/self", s.handleSelf)
	mux.HandleFunc("PUT /api/self/name", s.handleRenameSelf)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("DELETE /api/history/{peerID}", s.handleDeleteHistory)
	mux.HandleFunc("DELETE /api/history/entry/{entryID}", s.handleDeleteEntry)
	mux.HandleFunc("POST /api/send", s.handleSend)
	mux.HandleFunc("POST /api/broadcast", s.handleBroadcast)
	mux.HandleFunc("POST /api/sendfile", s.handleSendFile)
	mux.HandleFunc("POST /inbox/message", requireTLS(s.handleInboxMessage))
	mux.HandleFunc("POST /inbox/file/announce", requireTLS(s.handleInboxFileAnnounce))
	mux.HandleFunc("PUT /inbox/file/{token}", s.handleInboxFileBody)
	mux.HandleFunc("GET /downloads/{name}", s.handleDownload)
	mux.HandleFunc("GET /api/targets", s.handleListTargets)
	mux.HandleFunc("POST /api/targets", s.handleAddTarget)
	mux.HandleFunc("PUT /api/targets", s.handleEditTarget)
	mux.HandleFunc("DELETE /api/targets/{ip}", s.handleRemoveTarget)
	mux.HandleFunc("POST /api/scan", s.handleScan)
	mux.HandleFunc("POST /api/share", s.handleShareUpload)
	mux.HandleFunc("GET /api/shares", s.handleShareList)
	mux.HandleFunc("DELETE /api/share/{token}", s.handleShareRevoke)
	mux.HandleFunc("GET /s/{token}", s.handleServeShare)
	mux.HandleFunc("POST /api/shutdown", s.handleShutdown)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.Handle("GET /", s.staticHandler())
	return mux
}

// requireTLS rejects plain-HTTP requests on endpoints that must run over the
// pinned TLS channel. 421 Misdirected Request — 다른 채널로 다시 보내라는 뜻.
func requireTLS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			writeJSON(w, http.StatusMisdirectedRequest, map[string]string{"error": "tls required"})
			return
		}
		h(w, r)
	}
}

// handleShutdown triggers graceful shutdown after replying, so the button
// click gets a response before the server stops.
func (s *Server) handleShutdown(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "bye"})
	if s.opt.Shutdown != nil {
		go s.opt.Shutdown()
	}
}

func (s *Server) handlePeers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.opt.Reg.Snapshot())
}

// handleSelf reports this instance's identity and its LAN IP so the user can
// tell a friend which IP to add (UI는 localhost가 아니라 이 IP를 보여준다).
func (s *Server) handleSelf(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"id":   s.opt.SelfID,
		"name": s.selfName(),
		"ip":   localIP(),
	})
}

func (s *Server) selfName() string {
	if s.opt.NameGet != nil {
		if name := s.opt.NameGet(); name != "" {
			return name
		}
	}
	return s.opt.Name
}

func (s *Server) handleRenameSelf(w http.ResponseWriter, r *http.Request) {
	if s.opt.Rename == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "rename unsupported"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	name, err := s.opt.Rename(req.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name})
}

// localIP returns the primary LAN IPv4 chosen by the routing table, falling back
// to the first non-loopback IPv4 on any interface. 알 수 없으면 빈 문자열.
//
// WHY: UDP Dial은 패킷을 보내지 않고 기본 경로의 출발지 IP만 고른다 — 인터넷이
// 없어도(고립 LAN) 폴백이 인터페이스에서 사설 IP를 찾아낸다.
func localIP() string {
	if c, err := net.Dial("udp4", "8.8.8.8:80"); err == nil {
		defer c.Close()
		if ua, ok := c.LocalAddr().(*net.UDPAddr); ok && ua.IP != nil {
			return ua.IP.String()
		}
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				if ip4 := n.IP.To4(); ip4 != nil && !ip4.IsLoopback() {
					return ip4.String()
				}
			}
		}
	}
	return ""
}

// staticHandler serves the embedded web assets at the root.
func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// embed 디렉터리는 빌드 타임에 보장되므로 여기 도달하면 프로그래밍 오류.
		panic(fmt.Sprintf("web: embed sub: %v", err))
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := staticContentType(r.URL.Path); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		files.ServeHTTP(w, r)
	})
}

func staticContentType(name string) string {
	switch path.Ext(path.Clean(name)) {
	case "", ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	default:
		return ""
	}
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

// ListenTLS wraps Listen with a TLS layer using the provided server config.
// 포트 폴백은 Listen이 한다.
func ListenTLS(port int, cfg *tls.Config) (net.Listener, error) {
	raw, err := Listen(port)
	if err != nil {
		return nil, err
	}
	return tls.NewListener(raw, cfg), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
