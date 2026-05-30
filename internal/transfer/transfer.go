// Package transfer saves received files safely to a download directory.
//
// WHY: 네트워크에서 온 파일명은 신뢰할 수 없다. 경로 탈출(../)을 차단하고
// 이름 충돌을 회피하며, 100MB급도 메모리 폭증 없이 스트리밍 저장한다.
package transfer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SafeName reduces an arbitrary, possibly hostile name to a bare base name.
//
// "../../etc/passwd" → "passwd", `..\x.ini` → "x.ini", 빈/"."/".." → "file".
func SafeName(name string) string {
	// 윈도/유닉스 구분자 모두를 잘라 base만 취한다.
	name = strings.ReplaceAll(name, `\`, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}

// Save streams r into dir under a collision-free name and returns the path.
func Save(dir, name string, r io.Reader) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("transfer: mkdir: %w", err)
	}
	path := uniquePath(dir, SafeName(name))

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("transfer: create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil { // 스트리밍 — 전체를 메모리에 올리지 않음
		return "", fmt.Errorf("transfer: write: %w", err)
	}
	return path, nil
}

// uniquePath returns base, or "stem (n)ext" for the first n that does not exist.
func uniquePath(dir, base string) string {
	candidate := filepath.Join(dir, base)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for n := 2; ; n++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, n, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
