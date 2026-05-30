package transfer

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeNameStripsPathAndEscape(t *testing.T) {
	cases := map[string]string{
		"a.txt":              "a.txt",
		"../../etc/passwd":   "passwd",
		`..\..\windows\x.ini`: "x.ini",
		"sub/dir/file.png":   "file.png",
		"":                   "file",
		"..":                 "file",
		".":                  "file",
	}
	for in, want := range cases {
		if got := SafeName(in); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// WHY: ../ 가 섞여도 저장 경로는 반드시 dir 내부여야 한다(경로 탈출 차단).
func TestSaveStaysInsideDir(t *testing.T) {
	dir := t.TempDir()
	path, err := Save(dir, "../../escape.txt", bytes.NewReader([]byte("x")))
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(path)
	root, _ := filepath.Abs(dir)
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		t.Fatalf("file escaped dir: %s not under %s", abs, root)
	}
}

func TestSaveAvoidsCollision(t *testing.T) {
	dir := t.TempDir()
	p1, _ := Save(dir, "a.txt", bytes.NewReader([]byte("one")))
	p2, _ := Save(dir, "a.txt", bytes.NewReader([]byte("two")))
	if p1 == p2 {
		t.Fatal("collision not avoided: same path twice")
	}
	if filepath.Base(p2) != "a (2).txt" {
		t.Fatalf("want 'a (2).txt', got %q", filepath.Base(p2))
	}
	// 두 파일 내용이 각각 보존돼야 한다.
	if b, _ := os.ReadFile(p1); string(b) != "one" {
		t.Fatal("first file content lost")
	}
	if b, _ := os.ReadFile(p2); string(b) != "two" {
		t.Fatal("second file content lost")
	}
}

// WHY: 임의 바이너리 왕복 후 SHA-256이 동일해야 무결성이 보장된다(스트리밍 저장).
func TestSaveRoundTripSHA256(t *testing.T) {
	dir := t.TempDir()
	blob := make([]byte, 3*1024*1024) // 3MB
	if _, err := rand.Read(blob); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(blob)

	path, err := Save(dir, "blob.bin", bytes.NewReader(blob))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(got) != want {
		t.Fatal("SHA-256 mismatch after round-trip")
	}
}
