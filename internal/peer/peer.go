// Package peer maintains the live registry of discovered LAN peers.
//
// WHY: Discovery(UDP)는 HELLO/BYE를 받아 이 레지스트리를 갱신하고, web 계층은
// 스냅샷을 읽어 UI에 뿌린다. 레지스트리는 그 사이의 유일한 공유 상태이므로
// 동시 접근에 안전해야 한다.
package peer

import (
	"sort"
	"sync"
	"time"
)

// Peer is one discovered instance on the LAN.
type Peer struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	IP       string    `json:"ip"`       // UDP 패킷 src addr에서 취득(페이로드 신뢰 금지)
	HTTPPort int       `json:"httpPort"` // 쪽지/파일 중계를 위한 상대 HTTP 포트
	LastSeen time.Time `json:"lastSeen"`
}

// Registry is a concurrency-safe set of peers keyed by ID.
type Registry struct {
	mu    sync.RWMutex
	peers map[string]Peer
	nowFn func() time.Time // 테스트 주입용 시계
}

// NewRegistry returns an empty registry using the wall clock.
func NewRegistry() *Registry {
	return &Registry{peers: make(map[string]Peer), nowFn: time.Now}
}

// Upsert inserts or refreshes a peer, stamping LastSeen with the current time.
//
// WHY: 신선도는 레지스트리가 단일 진실원이 되도록 수신 시점으로 직접 찍는다.
// 송신자가 보낸 ts를 신뢰하지 않는다(시계 불일치/스푸핑 방지).
func (r *Registry) Upsert(p Peer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.LastSeen = r.nowFn()
	r.peers[p.ID] = p
}

// Get returns the peer with the given ID, if present.
//
// WHY: 쪽지/파일 전송은 대상 피어의 IP·포트가 필요하다. 전체 스냅샷을
// 훑지 않고 단건 조회한다.
func (r *Registry) Get(id string) (Peer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.peers[id]
	return p, ok
}

// Remove deletes a peer by ID. Removing an unknown ID is a no-op.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.peers, id)
}

// Snapshot returns a name-sorted defensive copy of the current peers.
func (r *Registry) Snapshot() []Peer {
	r.mu.RLock()
	out := make([]Peer, 0, len(r.peers))
	for _, p := range r.peers {
		out = append(out, p)
	}
	r.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID // 동명이인 안정 정렬
	})
	return out
}

// Expire removes peers whose LastSeen is older than ttl and returns them.
//
// WHY: BYE를 놓쳐도(패킷 유실) 조용히 떠난 피어가 영원히 남지 않도록 하는
// 안전망. 호출자(peerSweep)가 주기적으로 돌린다.
func (r *Registry) Expire(ttl time.Duration) []Peer {
	cutoff := r.nowFn().Add(-ttl)
	r.mu.Lock()
	defer r.mu.Unlock()
	var removed []Peer
	for id, p := range r.peers {
		if p.LastSeen.Before(cutoff) {
			removed = append(removed, p)
			delete(r.peers, id)
		}
	}
	return removed
}
