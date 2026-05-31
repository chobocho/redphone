package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/chobocho/redphone/internal/message"
	"github.com/chobocho/redphone/internal/transfer"
)

const (
	defaultDownloadDir = "downloads"

	// uploadTokenTTL — announce 후 본문 PUT까지 허용되는 윈도.
	// 짧으면 LAN의 일시적 지연에 약하고 길면 DoS 표면이 커진다 → 60초 절충.
	uploadTokenTTL = 60 * time.Second
)

// handleSendFile is the browser-facing endpoint. It splits the upload into a
// TLS announce (filename only) followed by a plain HTTP body PUT. 본문 평문은
// 의도된 설계 — 메타데이터(파일명/대상)만 암호화한다.
//
// 멀티파트 파트 순서 전제: 먼저 "peerId", 그다음 "file"(UI가 보장).
func (s *Server) handleSendFile(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected multipart"})
		return
	}

	var peerID string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad multipart"})
			return
		}
		switch part.FormName() {
		case "peerId":
			b, _ := io.ReadAll(io.LimitReader(part, 256))
			peerID = string(b)
		case "file":
			p, ok := s.opt.Reg.Get(peerID)
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "peer offline"})
				return
			}
			if p.Fingerprint == "" {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "peer has no fingerprint"})
				return
			}
			filename := filepath.Base(part.FileName())

			// 1) TLS announce — 파일명만 암호화 채널로 전달, 토큰 회신.
			token, err := s.announceFile(r.Context(), p.IP, p.HTTPSPort, p.Fingerprint, filename)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "announce failed"})
				return
			}

			// 2) 평문 HTTP PUT — 본문 스트림 그대로 흘려보낸다.
			bodyURL := fmt.Sprintf("http://%s:%d/inbox/file/%s", p.IP, p.HTTPPort, token)
			if err := s.putFileBody(r.Context(), bodyURL, part); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "delivery failed"})
				return
			}
			entry, _ := s.persistEntry(message.Entry{
				PeerID: peerID,
				Dir:    "sys",
				Text:   fmt.Sprintf("파일 전송: %s", filename),
				TS:     s.now(),
			})
			writeJSON(w, http.StatusOK, map[string]any{"status": "sent", "entry": entry})
			return
		}
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file part"})
}

// announceFile POSTs the filename to the peer's TLS announce endpoint and
// returns the one-shot upload token.
func (s *Server) announceFile(ctx context.Context, ip string, httpsPort int, fp, filename string) (string, error) {
	url := fmt.Sprintf("https://%s:%d/inbox/file/announce", ip, httpsPort)
	body, _ := json.Marshal(map[string]string{"filename": filename})

	ctx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.peerTLS(fp).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("announce status %d", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Token == "" {
		return "", fmt.Errorf("announce: bad response")
	}
	return out.Token, nil
}

// putFileBody streams the file part to the peer's plain-HTTP token endpoint.
func (s *Server) putFileBody(ctx context.Context, url string, src io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, src)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("body PUT status %d", resp.StatusCode)
	}
	return nil
}

// handleInboxFileAnnounce reserves a one-shot upload token for an incoming
// file body. TLS-only — requireTLS가 평문 호출을 막는다.
func (s *Server) handleInboxFileAnnounce(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Filename == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	token, err := s.uploads().reserve(req.Filename)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token gen"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// handleInboxFileBody consumes a previously announced token and stores the
// body under the registered filename. 평문 HTTP — 본문은 LAN 내 도청 가능
// (의도된 설계). 토큰은 1회용 + TTL이라 임의 PUT을 방어한다.
func (s *Server) handleInboxFileBody(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	entry, ok := s.uploads().claim(token)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown or expired token"})
		return
	}
	path, err := transfer.Save(s.downloadDir(), entry.filename, r.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save failed"})
		return
	}
	s.pushEvent(wsEvent{Type: "file", Text: filepath.Base(path)})
	writeJSON(w, http.StatusOK, map[string]string{"saved": filepath.Base(path)})
}

func (s *Server) downloadDir() string {
	if s.opt.DownloadDir != "" {
		return s.opt.DownloadDir
	}
	return defaultDownloadDir
}

// uploads returns the lazy-initialized upload token store for this server.
func (s *Server) uploads() *uploadStore {
	s.uploadOnce.Do(func() { s.uploadStore = newUploadStore() })
	return s.uploadStore
}

// uploadStore holds reserved-but-not-yet-uploaded file tokens.
//
// WHY: announce(HTTPS)와 본문 PUT(HTTP)을 잇는 단일 진실원. 토큰이 곧 인가
// 토큰이므로 추측 불가능(128bit 난수)해야 하고, 1회용·TTL이어야 한다.
type uploadStore struct {
	mu      sync.Mutex
	entries map[string]uploadEntry
	nowFn   func() time.Time
	ttl     time.Duration
}

type uploadEntry struct {
	filename  string
	expiresAt time.Time
}

func newUploadStore() *uploadStore {
	return &uploadStore{entries: map[string]uploadEntry{}, nowFn: time.Now, ttl: uploadTokenTTL}
}

func (u *uploadStore) reserve(filename string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b[:])
	u.mu.Lock()
	defer u.mu.Unlock()
	u.gcLocked()
	u.entries[tok] = uploadEntry{filename: filename, expiresAt: u.nowFn().Add(u.ttl)}
	return tok, nil
}

// claim consumes a token if present and unexpired. 토큰은 단일 사용 — claim
// 직후 제거해 중복 PUT을 막는다.
func (u *uploadStore) claim(token string) (uploadEntry, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	e, ok := u.entries[token]
	if !ok {
		return uploadEntry{}, false
	}
	delete(u.entries, token)
	if u.nowFn().After(e.expiresAt) {
		return uploadEntry{}, false
	}
	return e, true
}

// gcLocked drops expired entries; caller holds u.mu.
func (u *uploadStore) gcLocked() {
	now := u.nowFn()
	for k, e := range u.entries {
		if now.After(e.expiresAt) {
			delete(u.entries, k)
		}
	}
}
