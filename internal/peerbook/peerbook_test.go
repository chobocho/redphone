package peerbook_test

import (
	"path/filepath"
	"testing"

	"github.com/chobocho/redphone/internal/peerbook"
)

// TestLoadMissingReturnsEmpty는 파일이 없을 때 에러 없이 빈 목록을 주는지 본다.
// 최초 실행 시 peers.json이 없는 건 정상이다.
func TestLoadMissingReturnsEmpty(t *testing.T) {
	got, err := peerbook.Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

// TestSaveLoadRoundTrip는 저장한 IP가 다시 읽혀 나오며 정렬되는지 본다.
func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.json")
	if err := peerbook.Save(path, []string{"192.168.0.9", "192.168.0.5"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := peerbook.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"192.168.0.5", "192.168.0.9"} // 정렬됨
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestSaveOverwrites는 같은 경로 재저장이 이전 내용을 덮어쓰는지 본다.
func TestSaveOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.json")
	if err := peerbook.Save(path, []string{"10.0.0.1"}); err != nil {
		t.Fatalf("Save 1: %v", err)
	}
	if err := peerbook.Save(path, []string{"10.0.0.2"}); err != nil {
		t.Fatalf("Save 2: %v", err)
	}
	got, err := peerbook.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0] != "10.0.0.2" {
		t.Errorf("got %v, want [10.0.0.2]", got)
	}
}
