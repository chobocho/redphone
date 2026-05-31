package peer

import (
	"sync"
	"testing"
	"time"
)

// 고정 시계: 만료 로직을 결정론적으로 검증하기 위해 시간을 주입한다.
func fixedClock(t *time.Time) func() time.Time {
	return func() time.Time { return *t }
}

func TestUpsertAndSnapshot(t *testing.T) {
	r := NewRegistry()
	r.Upsert(Peer{ID: "b", Name: "bravo", IP: "10.0.0.2", HTTPPort: 17080})
	r.Upsert(Peer{ID: "a", Name: "alpha", IP: "10.0.0.1", HTTPPort: 17081})

	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("want 2 peers, got %d", len(snap))
	}
	// WHY: UI 목록이 흔들리지 않도록 Snapshot은 이름순으로 안정 정렬한다.
	if snap[0].Name != "alpha" || snap[1].Name != "bravo" {
		t.Fatalf("snapshot not sorted by name: %+v", snap)
	}
}

func TestUpsertUpdatesExisting(t *testing.T) {
	r := NewRegistry()
	r.Upsert(Peer{ID: "a", Name: "old", IP: "10.0.0.1", HTTPPort: 1})
	r.Upsert(Peer{ID: "a", Name: "new", IP: "10.0.0.9", HTTPPort: 2})

	snap := r.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("want 1 peer after re-upsert, got %d", len(snap))
	}
	if snap[0].Name != "new" || snap[0].IP != "10.0.0.9" || snap[0].HTTPPort != 2 {
		t.Fatalf("existing peer not updated: %+v", snap[0])
	}
}

func TestExpireRemovesStaleKeepsFresh(t *testing.T) {
	now := time.Unix(1_730_000_000, 0)
	r := NewRegistry()
	r.nowFn = fixedClock(&now)

	r.Upsert(Peer{ID: "stale", Name: "stale"})
	now = now.Add(20 * time.Second) // stale의 LastSeen이 20초 전이 되도록 진행
	r.Upsert(Peer{ID: "fresh", Name: "fresh"})

	removed := r.Expire(15 * time.Second)
	if len(removed) != 1 || removed[0].ID != "stale" {
		t.Fatalf("expected only 'stale' removed, got %+v", removed)
	}
	snap := r.Snapshot()
	if len(snap) != 1 || snap[0].ID != "fresh" {
		t.Fatalf("expected only 'fresh' to remain, got %+v", snap)
	}
}

func TestGet(t *testing.T) {
	r := NewRegistry()
	r.Upsert(Peer{ID: "a", Name: "alpha", IP: "10.0.0.1", HTTPPort: 17080})
	if p, ok := r.Get("a"); !ok || p.IP != "10.0.0.1" || p.HTTPPort != 17080 {
		t.Fatalf("Get(a) = %+v, %v", p, ok)
	}
	if _, ok := r.Get("ghost"); ok {
		t.Fatal("Get(ghost) should be false")
	}
}

func TestRemove(t *testing.T) {
	r := NewRegistry()
	r.Upsert(Peer{ID: "a", Name: "a"})
	r.Remove("a")
	if len(r.Snapshot()) != 0 {
		t.Fatal("peer not removed")
	}
	// 존재하지 않는 id 제거는 무해해야 한다.
	r.Remove("ghost")
}

func TestRemoveSession(t *testing.T) {
	r := NewRegistry()
	r.Upsert(Peer{ID: "stable-a", SessionID: "sess-a", Name: "a"})
	r.Upsert(Peer{ID: "stable-b", SessionID: "sess-b", Name: "b"})
	r.RemoveSession("sess-a")

	snap := r.Snapshot()
	if len(snap) != 1 || snap[0].ID != "stable-b" {
		t.Fatalf("unexpected peers after RemoveSession: %+v", snap)
	}
}

// WHY: Snapshot은 내부 맵의 방어적 복사여야 한다. 반환값을 바꿔도
// 레지스트리가 오염되면 안 된다.
func TestSnapshotIsDefensiveCopy(t *testing.T) {
	r := NewRegistry()
	r.Upsert(Peer{ID: "a", Name: "a"})
	snap := r.Snapshot()
	snap[0].Name = "mutated"
	if r.Snapshot()[0].Name != "a" {
		t.Fatal("Snapshot leaked internal state")
	}
}

// 동시 Upsert/Snapshot이 패닉 없이 일관되게 동작하는지(논리 안전성) 확인.
// (이 머신엔 C 컴파일러가 없어 -race는 CI에서. 여기선 경합 자체를 유발해 둔다.)
func TestConcurrentUpsertSnapshot(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) { defer wg.Done(); r.Upsert(Peer{ID: "a", Name: "a"}) }(i)
		go func() { defer wg.Done(); _ = r.Snapshot() }()
	}
	wg.Wait()
}
