package message

import (
	"sync"
	"testing"
)

func TestHistoryAddAndAll(t *testing.T) {
	h := NewHistory()
	h.Add(Note{FromID: "a", From: "alpha", Text: "hi", TS: 1})
	h.Add(Note{FromID: "b", From: "bravo", Text: "yo", TS: 2})

	all := h.All()
	if len(all) != 2 || all[0].Text != "hi" || all[1].Text != "yo" {
		t.Fatalf("unexpected history: %+v", all)
	}
}

// WHY: All은 방어적 복사여야 한다 — 반환 슬라이스 변경이 내부를 오염시키면 안 됨.
func TestHistoryAllIsDefensiveCopy(t *testing.T) {
	h := NewHistory()
	h.Add(Note{Text: "orig"})
	got := h.All()
	got[0].Text = "tampered"
	if h.All()[0].Text != "orig" {
		t.Fatal("All leaked internal slice")
	}
}

func TestHistoryConcurrentAdd(t *testing.T) {
	h := NewHistory()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); h.Add(Note{Text: "x"}) }()
	}
	wg.Wait()
	if len(h.All()) != 100 {
		t.Fatalf("want 100 notes, got %d", len(h.All()))
	}
}
