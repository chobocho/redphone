package web

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/chobocho/redphone/internal/share"
)

// previewLimit caps how much of a text share we render inline.
const previewLimit = 256 * 1024

// handleShareUpload publishes an uploaded file and returns its link.
func (s *Server) handleShareUpload(w http.ResponseWriter, r *http.Request) {
	if s.opt.Shares == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sharing disabled"})
		return
	}
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
		sh, err := s.opt.Shares.Add(part.FileName(), part)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"token": sh.Token,
			"kind":  sh.Kind,
			"name":  sh.Name,
			"url":   fmt.Sprintf("http://%s/s/%s", s.shareHost(r), sh.Token),
		})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file part"})
}

// handleShareList returns the current shares.
func (s *Server) handleShareList(w http.ResponseWriter, _ *http.Request) {
	var list []share.Share
	if s.opt.Shares != nil {
		list = s.opt.Shares.List()
	}
	writeJSON(w, http.StatusOK, list)
}

// handleShareRevoke withdraws a share by token.
func (s *Server) handleShareRevoke(w http.ResponseWriter, r *http.Request) {
	if s.opt.Shares == nil || !s.opt.Shares.Revoke(r.PathValue("token")) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// handleServeShare renders a share: image inline, text preview, else download.
func (s *Server) handleServeShare(w http.ResponseWriter, r *http.Request) {
	if s.opt.Shares == nil {
		http.NotFound(w, r)
		return
	}
	sh, ok := s.opt.Shares.Get(r.PathValue("token"))
	if !ok {
		http.NotFound(w, r) // 잘못된/회수된 토큰 → 404
		return
	}
	f, err := os.Open(sh.Path)
	if err != nil {
		http.Error(w, "gone", http.StatusGone)
		return
	}
	defer f.Close()

	switch sh.Kind {
	case share.KindImage:
		w.Header().Set("Content-Type", sh.ContentType)
		w.Header().Set("Content-Disposition", "inline; filename=\""+sh.Name+"\"")
		http.ServeContent(w, r, sh.Name, modTime(f), f)
	case share.KindText:
		s.serveTextPreview(w, sh, f)
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+sh.Name+"\"")
		http.ServeContent(w, r, sh.Name, modTime(f), f)
	}
}

// serveTextPreview renders escaped text in a minimal HTML page.
//
// WHY: 텍스트를 그대로 text/plain으로 주면 브라우저가 다운로드하거나, 악성
// 마크업이 실행될 여지가 있다. HTML로 감싸되 내용은 반드시 이스케이프한다.
func (s *Server) serveTextPreview(w http.ResponseWriter, sh share.Share, f io.Reader) {
	body, _ := io.ReadAll(io.LimitReader(f, previewLimit))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html lang="ko"><head><meta charset="utf-8">`+
		`<title>%s</title><style>body{margin:0;background:#0c0e11;color:#dfe4ec;`+
		`font-family:ui-monospace,Menlo,monospace;font-size:13px}`+
		`pre{padding:18px;white-space:pre-wrap;word-break:break-word}`+
		`</style></head><body><pre>%s</pre></body></html>`,
		html.EscapeString(sh.Name), html.EscapeString(string(body)))
}

// shareHost picks the host:port to embed in share URLs.
func (s *Server) shareHost(r *http.Request) string {
	if s.opt.ShareHost != "" {
		return s.opt.ShareHost
	}
	return r.Host
}

func modTime(f *os.File) time.Time {
	if fi, err := f.Stat(); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}
