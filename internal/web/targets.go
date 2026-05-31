package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

// 친구 IP 수동 관리 + 전체 스캔 핸들러.
//
// WHY: 브로드캐스트가 막힌 망에서는 친구 IP를 직접 등록해 발견한다. 등록 목록은
// 실행 파일 폴더의 peers.json에 영속화되며(app의 PeerControl 구현이 담당),
// 여기서는 추가/수정/삭제/조회/스캔만 노출한다.

// handleListTargets returns the current manual friend-IP list.
func (s *Server) handleListTargets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"targets": s.targetList()})
}

// handleAddTarget registers one friend IP. 잘못된 IP는 400.
func (s *Server) handleAddTarget(w http.ResponseWriter, r *http.Request) {
	if s.opt.Targets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "targets disabled"})
		return
	}
	var body struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	if err := s.opt.Targets.AddTarget(strings.TrimSpace(body.IP)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"targets": s.targetList()})
}

// handleEditTarget replaces one IP with another. 새 IP가 유효할 때만 옛 IP를 지운다.
func (s *Server) handleEditTarget(w http.ResponseWriter, r *http.Request) {
	if s.opt.Targets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "targets disabled"})
		return
	}
	var body struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	// 새 IP를 먼저 검증·추가하고 성공하면 옛 IP를 제거한다 — 검증 실패 시
	// 기존 등록이 유실되지 않게 한다.
	if err := s.opt.Targets.AddTarget(strings.TrimSpace(body.New)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if old := strings.TrimSpace(body.Old); old != "" && old != strings.TrimSpace(body.New) {
		s.opt.Targets.RemoveTarget(old)
	}
	writeJSON(w, http.StatusOK, map[string]any{"targets": s.targetList()})
}

// handleRemoveTarget drops one friend IP by path value.
func (s *Server) handleRemoveTarget(w http.ResponseWriter, r *http.Request) {
	if s.opt.Targets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "targets disabled"})
		return
	}
	s.opt.Targets.RemoveTarget(r.PathValue("ip"))
	writeJSON(w, http.StatusOK, map[string]any{"targets": s.targetList()})
}

// handleScan triggers one full-subnet sweep (opt-in feature).
func (s *Server) handleScan(w http.ResponseWriter, _ *http.Request) {
	if s.opt.Targets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "targets disabled"})
		return
	}
	n, err := s.opt.Targets.ScanLAN()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": n})
}

// targetList returns the friend-IP list, never nil so JSON renders [] not null.
func (s *Server) targetList() []string {
	if s.opt.Targets == nil {
		return []string{}
	}
	if t := s.opt.Targets.Targets(); t != nil {
		return t
	}
	return []string{}
}
