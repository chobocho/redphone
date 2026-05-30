package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/chobocho/redphone/internal/message"
)

const peerRequestTimeout = 5 * time.Second

// wsEvent is the envelope pushed to browsers over the hub.
// type별로 payload 필드를 선택적으로 채운다(단일 채널, 다중 이벤트).
type wsEvent struct {
	Type string        `json:"type"`           // "message" | "file" | "peers" ...
	Note *message.Note `json:"note,omitempty"` // type=="message"
	Text string        `json:"text,omitempty"` // 일반 알림 텍스트
}

// handleSend relays a note to the target peer's inbox.
//
// 흐름: 브라우저 → POST /api/send {peerId,text} → 상대 /inbox/message로 중계.
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peerId"`
		Text   string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	p, ok := s.opt.Reg.Get(req.PeerID)
	if !ok {
		// 오프라인/미발견 → UI가 실패를 명시할 수 있게 404.
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "peer offline"})
		return
	}

	note := message.Note{
		FromID: s.opt.SelfID,
		From:   s.opt.Name,
		Text:   req.Text,
		TS:     s.now(),
	}
	url := fmt.Sprintf("http://%s:%d/inbox/message", p.IP, p.HTTPPort)
	if err := s.postJSON(r.Context(), url, note); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "delivery failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// handleBroadcast relays one note to every known peer (전체 쪽지) — the
// signature feature of the original 빨간전화기.
//
// WHY: 한 명씩 고르지 않고 같은 LAN의 모든 인스턴스에 동시에 공지하는 용도.
// 일부 피어가 오프라인이어도 나머지에는 전달되도록 실패는 집계만 한다.
func (s *Server) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	note := message.Note{
		FromID: s.opt.SelfID,
		From:   s.opt.Name,
		Text:   req.Text,
		TS:     s.now(),
	}
	peers := s.opt.Reg.Snapshot()
	sent, failed := 0, 0
	for _, p := range peers {
		url := fmt.Sprintf("http://%s:%d/inbox/message", p.IP, p.HTTPPort)
		if err := s.postJSON(r.Context(), url, note); err != nil {
			failed++
		} else {
			sent++
		}
	}
	writeJSON(w, http.StatusOK, map[string]int{"sent": sent, "failed": failed})
}

// handleInboxMessage receives a note from a peer, stores it, and pushes to UI.
func (s *Server) handleInboxMessage(w http.ResponseWriter, r *http.Request) {
	var note message.Note
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if s.opt.History != nil {
		s.opt.History.Add(note)
	}
	s.pushEvent(wsEvent{Type: "message", Note: &note})
	w.WriteHeader(http.StatusOK)
}

// handleHistory returns the in-memory note history for UI hydration.
func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	var notes []message.Note
	if s.opt.History != nil {
		notes = s.opt.History.All()
	}
	writeJSON(w, http.StatusOK, notes)
}

// postJSON POSTs v as JSON to url with a bounded timeout.
func (s *Server) postJSON(ctx context.Context, url string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("peer returned %d", resp.StatusCode)
	}
	return nil
}

// pushEvent marshals and broadcasts an event to all browsers.
func (s *Server) pushEvent(ev wsEvent) {
	if b, err := json.Marshal(ev); err == nil {
		s.hub.Broadcast(b)
	}
}

func (s *Server) client() *http.Client {
	if s.opt.Client != nil {
		return s.opt.Client
	}
	return http.DefaultClient
}

func (s *Server) now() int64 {
	if s.opt.NowMs != nil {
		return s.opt.NowMs()
	}
	return time.Now().UnixMilli()
}
