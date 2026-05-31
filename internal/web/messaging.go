package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/chobocho/redphone/internal/message"
	"github.com/chobocho/redphone/internal/tlsid"
)

const peerRequestTimeout = 5 * time.Second

// wsEvent is the envelope pushed to browsers over the hub.
type wsEvent struct {
	Type   string         `json:"type"`             // "entry" | "entry_deleted" | "file" | "peers" | "history_cleared"
	Entry  *message.Entry `json:"entry,omitempty"`  // type=="entry" | "entry_deleted"
	PeerID string         `json:"peerId,omitempty"` // type=="history_cleared"
	Text   string         `json:"text,omitempty"`
}

// handleSend relays a note to the target peer's inbox and stores a local copy.
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
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "peer offline"})
		return
	}

	ts := s.now()
	note := message.Note{
		FromID: s.opt.SelfID,
		From:   s.selfName(),
		Text:   req.Text,
		TS:     ts,
	}
	url := fmt.Sprintf("https://%s:%d/inbox/message", p.IP, p.HTTPSPort)
	if err := s.postJSONTLS(r.Context(), url, p.Fingerprint, note); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "delivery failed"})
		return
	}
	entry, err := s.persistEntry(message.Entry{PeerID: req.PeerID, Dir: "out", Text: req.Text, TS: ts})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "history failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent", "entry": entry})
}

// handleBroadcast relays one note to every known peer and stores the local copy.
func (s *Server) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	ts := s.now()
	note := message.Note{
		FromID: s.opt.SelfID,
		From:   s.selfName(),
		Text:   req.Text,
		TS:     ts,
	}
	peers := s.opt.Reg.Snapshot()
	sent, failed := 0, 0
	for _, p := range peers {
		url := fmt.Sprintf("https://%s:%d/inbox/message", p.IP, p.HTTPSPort)
		if err := s.postJSONTLS(r.Context(), url, p.Fingerprint, note); err != nil {
			failed++
		} else {
			sent++
		}
	}
	entry, err := s.persistEntry(message.Entry{PeerID: message.BroadcastPeerID, Dir: "out", Text: req.Text, TS: ts})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "history failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": sent, "failed": failed, "entry": entry})
}

// handleInboxMessage receives a note from a peer, stores it, and pushes to UI.
func (s *Server) handleInboxMessage(w http.ResponseWriter, r *http.Request) {
	var note message.Note
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if _, err := s.addIncoming(note); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "history failed"})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleHistory returns the persisted chat history for UI hydration.
func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	var entries []message.Entry
	if s.opt.History != nil {
		all, err := s.opt.History.All()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "history failed"})
			return
		}
		entries = all
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleDeleteHistory(w http.ResponseWriter, r *http.Request) {
	peerID := r.PathValue("peerID")
	if peerID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "peer id required"})
		return
	}
	if s.opt.History != nil {
		if err := s.opt.History.Clear(peerID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "history failed"})
			return
		}
	}
	s.pushEvent(wsEvent{Type: "history_cleared", PeerID: peerID})
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) handleDeleteEntry(w http.ResponseWriter, r *http.Request) {
	entryID, err := strconv.ParseInt(r.PathValue("entryID"), 10, 64)
	if err != nil || entryID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entry id required"})
		return
	}
	if s.opt.History == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found"})
		return
	}
	entry, err := s.opt.History.DeleteEntry(entryID)
	if err != nil {
		if err == message.ErrEntryNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "history failed"})
		return
	}
	s.pushEvent(wsEvent{Type: "entry_deleted", Entry: &entry})
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "entry": entry})
}

// postJSONTLS POSTs v as JSON to a peer URL using a fingerprint-pinned TLS client.
func (s *Server) postJSONTLS(ctx context.Context, url, fp string, v any) error {
	if fp == "" {
		return fmt.Errorf("peer has no fingerprint (legacy/v1)")
	}
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
	resp, err := s.peerTLS(fp).Do(req)
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

func (s *Server) persistEntry(e message.Entry) (message.Entry, error) {
	if s.opt.History == nil {
		s.pushEvent(wsEvent{Type: "entry", Entry: &e})
		return e, nil
	}
	saved, err := s.opt.History.AddEntry(e)
	if err != nil {
		return message.Entry{}, err
	}
	s.pushEvent(wsEvent{Type: "entry", Entry: &saved})
	return saved, nil
}

func (s *Server) addIncoming(note message.Note) (message.Entry, error) {
	if s.opt.History == nil {
		entry := message.Entry{
			PeerID: note.FromID,
			Dir:    "in",
			Text:   note.Text,
			TS:     note.TS,
			FromID: note.FromID,
			From:   note.From,
		}
		s.pushEvent(wsEvent{Type: "entry", Entry: &entry})
		return entry, nil
	}
	entry, err := s.opt.History.Add(note)
	if err != nil {
		return message.Entry{}, err
	}
	s.pushEvent(wsEvent{Type: "entry", Entry: &entry})
	return entry, nil
}

func (s *Server) client() *http.Client {
	if s.opt.Client != nil {
		return s.opt.Client
	}
	return http.DefaultClient
}

// peerTLS returns a fingerprint-pinned HTTPS client for outbound peer calls.
func (s *Server) peerTLS(fp string) *http.Client {
	if s.opt.PeerTLS != nil {
		return s.opt.PeerTLS(fp)
	}
	return &http.Client{
		Timeout:   peerRequestTimeout,
		Transport: &http.Transport{TLSClientConfig: tlsid.PinnedClientConfig(fp)},
	}
}

func (s *Server) now() int64 {
	if s.opt.NowMs != nil {
		return s.opt.NowMs()
	}
	return time.Now().UnixMilli()
}
