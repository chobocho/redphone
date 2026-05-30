// Package share publishes files behind unguessable tokens for LAN-only,
// volatile URL sharing (F4).
//
// WHY: 파일을 "전송"하지 않고 링크로 노출한다. 인증은 토큰 비밀성에 의존하고,
// 인스턴스 종료 시 소멸한다(영속화/외부망 노출은 v2 과제로 분리).
package share

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chobocho/redphone/internal/transfer"
)

// Kind drives how /s/{token} renders the share.
type Kind string

const (
	KindImage Kind = "image"
	KindText  Kind = "text"
	KindOther Kind = "other"
)

const tokenLen = 22 // base62 22자 ≈ 131bit, 추측 불가

// Share is one published file.
type Share struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Kind        Kind   `json:"kind"`
	Path        string `json:"-"` // 내부 저장 경로(토큰 파일명)
	Size        int64  `json:"size"`
}

// Store keeps published shares and their backing files in dir.
type Store struct {
	mu      sync.Mutex
	dir     string
	items   map[string]Share
	tokenFn func() (string, error) // 테스트 주입용
}

// NewStore returns a store backed by dir (created on first Add).
func NewStore(dir string) *Store {
	return &Store{dir: dir, items: make(map[string]Share)}
}

// Add saves r under a fresh token, classifying it by extension + content sniff.
func (s *Store) Add(name string, r io.Reader) (Share, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return Share{}, fmt.Errorf("share: mkdir: %w", err)
	}

	// content-type 판정을 위해 앞부분을 미리 읽고, 나머지와 이어붙여 저장한다.
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return Share{}, fmt.Errorf("share: read head: %w", err)
	}
	head = head[:n]
	ct, kind := classify(name, head)

	token, err := s.newToken()
	if err != nil {
		return Share{}, err
	}
	// 파일은 토큰 이름으로 저장 → 원본명 노출 방지 + 충돌 없음.
	path := filepath.Join(s.dir, token)
	f, err := os.Create(path)
	if err != nil {
		return Share{}, fmt.Errorf("share: create: %w", err)
	}
	size, copyErr := io.Copy(f, io.MultiReader(bytes.NewReader(head), r))
	closeErr := f.Close()
	if copyErr != nil {
		return Share{}, fmt.Errorf("share: write: %w", copyErr)
	}
	if closeErr != nil {
		return Share{}, fmt.Errorf("share: close: %w", closeErr)
	}

	sh := Share{
		Token:       token,
		Name:        transfer.SafeName(name),
		ContentType: ct,
		Kind:        kind,
		Path:        path,
		Size:        size,
	}
	s.mu.Lock()
	s.items[token] = sh
	s.mu.Unlock()
	return sh, nil
}

// Get returns the share for a token, if present.
func (s *Store) Get(token string) (Share, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sh, ok := s.items[token]
	return sh, ok
}

// List returns all current shares.
func (s *Store) List() []Share {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Share, 0, len(s.items))
	for _, sh := range s.items {
		out = append(out, sh)
	}
	return out
}

// Revoke deletes a share and its file. Returns false if the token is unknown.
func (s *Store) Revoke(token string) bool {
	s.mu.Lock()
	sh, ok := s.items[token]
	if ok {
		delete(s.items, token)
	}
	s.mu.Unlock()
	if !ok {
		return false
	}
	_ = os.Remove(sh.Path)
	return true
}

// RemoveAll drops every share and its files (called on shutdown — volatile).
func (s *Store) RemoveAll() {
	s.mu.Lock()
	items := s.items
	s.items = make(map[string]Share)
	s.mu.Unlock()
	for _, sh := range items {
		_ = os.Remove(sh.Path)
	}
}

func (s *Store) newToken() (string, error) {
	if s.tokenFn != nil {
		return s.tokenFn()
	}
	return randToken(tokenLen)
}

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("share: rand: %w", err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

// imageExt / textExt drive extension-first classification before sniffing.
var imageExt = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp",
}

var textExt = map[string]bool{
	".txt": true, ".md": true, ".log": true, ".csv": true, ".json": true,
	".xml": true, ".yaml": true, ".yml": true, ".go": true, ".js": true,
	".css": true, ".html": true, ".htm": true, ".sh": true, ".py": true,
	".java": true, ".c": true, ".cpp": true, ".h": true, ".ts": true, ".rs": true,
}

// classify decides content-type and Kind from extension first, then a
// content sniff fallback (둘 다로 판정).
func classify(name string, head []byte) (string, Kind) {
	ext := strings.ToLower(filepath.Ext(name))
	if ct, ok := imageExt[ext]; ok {
		return ct, KindImage
	}
	if textExt[ext] {
		return "text/plain; charset=utf-8", KindText
	}
	ct := http.DetectContentType(head)
	switch {
	case strings.HasPrefix(ct, "image/"):
		return ct, KindImage
	case strings.HasPrefix(ct, "text/"):
		return ct, KindText
	default:
		return ct, KindOther
	}
}
