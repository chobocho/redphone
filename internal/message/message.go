// Package message models LAN notes and their in-memory history.
//
// WHY: 쪽지는 인스턴스 수명 동안만 메모리에 보관한다(휘발성). 영속화는 v2 과제.
// 송수신의 HTTP 배선은 web 계층이 맡고, 여기는 데이터 모델과 히스토리만 둔다.
package message

import "sync"

// Note is a single chat note. ip가 아니라 보낸 사람의 안정적 id로 식별한다.
type Note struct {
	FromID string `json:"fromId"` // 보낸 피어의 id
	From   string `json:"from"`   // 보낸 피어의 표시 이름
	Text   string `json:"text"`
	TS     int64  `json:"ts"` // epoch millis
}

// History is a concurrency-safe append-only log for the instance lifetime.
type History struct {
	mu    sync.Mutex
	notes []Note
}

// NewHistory returns an empty history.
func NewHistory() *History { return &History{} }

// Add appends a note.
func (h *History) Add(n Note) {
	h.mu.Lock()
	h.notes = append(h.notes, n)
	h.mu.Unlock()
}

// All returns a defensive copy of the notes in arrival order.
func (h *History) All() []Note {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Note, len(h.notes))
	copy(out, h.notes)
	return out
}
