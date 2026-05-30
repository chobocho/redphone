package share

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	pngHead := []byte("\x89PNG\r\n\x1a\n")
	cases := []struct {
		name string
		head []byte
		kind Kind
	}{
		{"photo.png", pngHead, KindImage},
		{"a.JPG", nil, KindImage},
		{"notes.txt", []byte("hello"), KindText},
		{"main.go", []byte("package main"), KindText},
		{"data.bin", []byte{0x00, 0x01, 0x02, 0x03}, KindOther},
	}
	for _, c := range cases {
		_, kind := classify(c.name, c.head)
		if kind != c.kind {
			t.Errorf("classify(%q) kind = %q, want %q", c.name, kind, c.kind)
		}
	}
}

func TestAddIssuesTokenAndGet(t *testing.T) {
	st := NewStore(t.TempDir())
	sh, err := st.Add("hello.txt", bytes.NewReader([]byte("hi there")))
	if err != nil {
		t.Fatal(err)
	}
	if len(sh.Token) < 16 {
		t.Fatalf("token too short to be unguessable: %q", sh.Token)
	}
	if sh.Kind != KindText || sh.Size != 8 {
		t.Fatalf("unexpected share meta: %+v", sh)
	}
	got, ok := st.Get(sh.Token)
	if !ok || got.Name != "hello.txt" {
		t.Fatalf("Get(token) = %+v, %v", got, ok)
	}
	// 저장 파일이 실제로 존재해야 한다.
	if _, err := os.Stat(got.Path); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
}

func TestGetUnknownToken(t *testing.T) {
	st := NewStore(t.TempDir())
	if _, ok := st.Get("nope"); ok {
		t.Fatal("unknown token should not be found")
	}
}

func TestRevokeRemovesEntryAndFile(t *testing.T) {
	st := NewStore(t.TempDir())
	sh, _ := st.Add("x.bin", bytes.NewReader([]byte("data")))
	if !st.Revoke(sh.Token) {
		t.Fatal("Revoke returned false for existing token")
	}
	if _, ok := st.Get(sh.Token); ok {
		t.Fatal("share still present after revoke")
	}
	if _, err := os.Stat(sh.Path); !os.IsNotExist(err) {
		t.Fatal("file not deleted after revoke")
	}
	if st.Revoke(sh.Token) {
		t.Fatal("double revoke should return false")
	}
}

func TestListReturnsAll(t *testing.T) {
	st := NewStore(t.TempDir())
	st.Add("a.txt", strings.NewReader("a"))
	st.Add("b.txt", strings.NewReader("b"))
	if len(st.List()) != 2 {
		t.Fatalf("want 2 shares, got %d", len(st.List()))
	}
}

// 토큰은 추측 불가해야 한다 — 연속 발급 토큰이 서로 달라야 한다.
func TestTokensAreUnique(t *testing.T) {
	st := NewStore(t.TempDir())
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		sh, _ := st.Add("x", strings.NewReader("x"))
		if seen[sh.Token] {
			t.Fatal("duplicate token issued")
		}
		seen[sh.Token] = true
	}
}
