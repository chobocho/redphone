package message

import (
	"sync"
	"testing"
)

func mustAll(t *testing.T, h *History) []Entry {
	t.Helper()
	all, err := h.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	return all
}

func TestHistoryAddAndAll(t *testing.T) {
	h := NewHistory()
	defer h.Close()

	if _, err := h.Add(Note{FromID: "a", From: "alpha", Text: "hi", TS: 1}); err != nil {
		t.Fatalf("Add #1: %v", err)
	}
	if _, err := h.Add(Note{FromID: "b", From: "bravo", Text: "yo", TS: 2}); err != nil {
		t.Fatalf("Add #2: %v", err)
	}

	all := mustAll(t, h)
	if len(all) != 2 || all[0].Text != "hi" || all[1].Text != "yo" {
		t.Fatalf("unexpected history: %+v", all)
	}
	if all[0].Dir != "in" || all[0].PeerID != "a" || all[0].From != "alpha" {
		t.Fatalf("unexpected first entry: %+v", all[0])
	}
}

func TestHistoryAllIsDefensiveCopy(t *testing.T) {
	h := NewHistory()
	defer h.Close()

	if _, err := h.Add(Note{FromID: "a", Text: "orig"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := mustAll(t, h)
	got[0].Text = "tampered"
	if mustAll(t, h)[0].Text != "orig" {
		t.Fatal("All leaked internal slice")
	}
}

func TestHistoryByPeerFiltersEntries(t *testing.T) {
	h := NewHistory()
	defer h.Close()

	if _, err := h.AddEntry(Entry{PeerID: "a", Dir: "out", Text: "one"}); err != nil {
		t.Fatalf("AddEntry #1: %v", err)
	}
	if _, err := h.AddEntry(Entry{PeerID: "b", Dir: "in", Text: "two"}); err != nil {
		t.Fatalf("AddEntry #2: %v", err)
	}
	if _, err := h.AddEntry(Entry{PeerID: "a", Dir: "in", Text: "three"}); err != nil {
		t.Fatalf("AddEntry #3: %v", err)
	}

	got, err := h.ByPeer("a")
	if err != nil {
		t.Fatalf("ByPeer: %v", err)
	}
	if len(got) != 2 || got[0].Text != "one" || got[1].Text != "three" {
		t.Fatalf("unexpected peer history: %+v", got)
	}
}

func TestHistoryConcurrentAdd(t *testing.T) {
	h := NewHistory()
	defer h.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.Add(Note{FromID: "peer", Text: "x"}); err != nil {
				t.Errorf("Add: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := len(mustAll(t, h)); got != 100 {
		t.Fatalf("want 100 notes, got %d", got)
	}
}

func TestHistoryClearByPeer(t *testing.T) {
	h := NewHistory()
	defer h.Close()

	if _, err := h.AddEntry(Entry{PeerID: "a", Dir: "out", Text: "one"}); err != nil {
		t.Fatalf("AddEntry #1: %v", err)
	}
	if _, err := h.AddEntry(Entry{PeerID: "b", Dir: "out", Text: "two"}); err != nil {
		t.Fatalf("AddEntry #2: %v", err)
	}
	if err := h.Clear("a"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	all := mustAll(t, h)
	if len(all) != 1 || all[0].PeerID != "b" {
		t.Fatalf("unexpected remaining history: %+v", all)
	}
}

func TestHistoryDeleteEntry(t *testing.T) {
	h := NewHistory()
	defer h.Close()

	keep, err := h.AddEntry(Entry{PeerID: "a", Dir: "out", Text: "keep"})
	if err != nil {
		t.Fatalf("AddEntry keep: %v", err)
	}
	drop, err := h.AddEntry(Entry{PeerID: "b", Dir: "in", Text: "drop"})
	if err != nil {
		t.Fatalf("AddEntry drop: %v", err)
	}

	deleted, err := h.DeleteEntry(drop.ID)
	if err != nil {
		t.Fatalf("DeleteEntry: %v", err)
	}
	if deleted.ID != drop.ID || deleted.PeerID != "b" || deleted.Text != "drop" {
		t.Fatalf("unexpected deleted entry: %+v", deleted)
	}

	all := mustAll(t, h)
	if len(all) != 1 || all[0].ID != keep.ID {
		t.Fatalf("unexpected remaining history: %+v", all)
	}
}

func TestHistoryDeleteEntryNotFound(t *testing.T) {
	h := NewHistory()
	defer h.Close()

	if _, err := h.DeleteEntry(999); err != ErrEntryNotFound {
		t.Fatalf("want ErrEntryNotFound, got %v", err)
	}
}
