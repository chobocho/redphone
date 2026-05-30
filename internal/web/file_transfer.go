package web

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/chobocho/redphone/internal/transfer"
)

const defaultDownloadDir = "downloads"

// handleSendFile streams an uploaded file straight to the target peer's
// /inbox/file without buffering the whole body in memory.
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
			url := fmt.Sprintf("http://%s:%d/inbox/file", p.IP, p.HTTPPort)
			if err := s.relayFile(r, url, part.FileName(), part); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "delivery failed"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
			return
		}
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file part"})
}

// relayFile pipes src into a fresh multipart body POSTed to url. io.Pipe로
// 들어오는 스트림을 그대로 흘려보내 메모리 사용을 일정하게 유지한다.
func (s *Server) relayFile(r *http.Request, url, filename string, src io.Reader) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		fw, err := mw.CreateFormFile("file", filepath.Base(filename))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(fw, src); err != nil {
			pw.CloseWithError(err)
			return
		}
		// 두 Close 모두 성공 시에만 EOF로 마감.
		if err := mw.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
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

// handleInboxFile streams a received file to the download dir and notifies UI.
func (s *Server) handleInboxFile(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected multipart"})
		return
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad multipart"})
			return
		}
		if part.FormName() != "file" {
			continue
		}
		path, err := transfer.Save(s.downloadDir(), part.FileName(), part)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save failed"})
			return
		}
		s.pushEvent(wsEvent{Type: "file", Text: filepath.Base(path)})
		writeJSON(w, http.StatusOK, map[string]string{"saved": filepath.Base(path)})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file part"})
}

func (s *Server) downloadDir() string {
	if s.opt.DownloadDir != "" {
		return s.opt.DownloadDir
	}
	return defaultDownloadDir
}
