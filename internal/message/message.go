// Package message models LAN notes and their persisted chat history.
package message

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	_ "modernc.org/sqlite"
)

const BroadcastPeerID = "__all__"

// Note is the network payload exchanged between peers.
type Note struct {
	FromID string `json:"fromId"`
	From   string `json:"from"`
	Text   string `json:"text"`
	TS     int64  `json:"ts"`
}

// Entry is a UI-facing chat log item stored locally.
type Entry struct {
	ID     int64  `json:"id"`
	PeerID string `json:"peerId"`
	Dir    string `json:"dir"` // "in" | "out" | "sys"
	Text   string `json:"text"`
	TS     int64  `json:"ts"`
	FromID string `json:"fromId,omitempty"`
	From   string `json:"from,omitempty"`
}

// History persists chat entries in SQLite.
type History struct {
	db *sql.DB
}

var memoryHistorySeq atomic.Uint64

// NewHistory returns an in-memory history, used mainly by tests.
func NewHistory() *History {
	h, err := Open(fmt.Sprintf("file:redphone-history-%d?mode=memory&cache=shared", memoryHistorySeq.Add(1)))
	if err != nil {
		panic(err)
	}
	return h
}

// Open opens or creates a SQLite-backed chat history database.
func Open(path string) (*History, error) {
	dsn := path
	if path == "" {
		dsn = "redphone.db"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("message: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA busy_timeout = 5000;
		CREATE TABLE IF NOT EXISTS chat_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			peer_id TEXT NOT NULL,
			dir TEXT NOT NULL,
			text TEXT NOT NULL,
			ts INTEGER NOT NULL,
			from_id TEXT NOT NULL DEFAULT '',
			from_name TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_chat_history_peer_id_id
		ON chat_history(peer_id, id);
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("message: init sqlite: %w", err)
	}
	return &History{db: db}, nil
}

// Close closes the underlying database handle.
func (h *History) Close() error {
	if h == nil || h.db == nil {
		return nil
	}
	return h.db.Close()
}

// Add stores an incoming peer note as a chat entry.
func (h *History) Add(n Note) (Entry, error) {
	return h.AddEntry(Entry{
		PeerID: strings.TrimSpace(n.FromID),
		Dir:    "in",
		Text:   n.Text,
		TS:     n.TS,
		FromID: n.FromID,
		From:   n.From,
	})
}

// AddEntry stores one UI-facing chat entry.
func (h *History) AddEntry(e Entry) (Entry, error) {
	if h == nil || h.db == nil {
		return Entry{}, errors.New("history unavailable")
	}
	e.PeerID = strings.TrimSpace(e.PeerID)
	e.Dir = strings.TrimSpace(e.Dir)
	if e.PeerID == "" {
		return Entry{}, errors.New("peer id required")
	}
	if e.Dir == "" {
		return Entry{}, errors.New("dir required")
	}
	res, err := h.db.Exec(
		`INSERT INTO chat_history(peer_id, dir, text, ts, from_id, from_name) VALUES(?, ?, ?, ?, ?, ?)`,
		e.PeerID, e.Dir, e.Text, e.TS, e.FromID, e.From,
	)
	if err != nil {
		return Entry{}, fmt.Errorf("message: insert entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Entry{}, fmt.Errorf("message: last insert id: %w", err)
	}
	e.ID = id
	return e, nil
}

// All returns chat entries ordered by insertion.
func (h *History) All() ([]Entry, error) {
	if h == nil || h.db == nil {
		return nil, nil
	}
	rows, err := h.db.Query(`
		SELECT id, peer_id, dir, text, ts, from_id, from_name
		FROM chat_history
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("message: select entries: %w", err)
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.PeerID, &e.Dir, &e.Text, &e.TS, &e.FromID, &e.From); err != nil {
			return nil, fmt.Errorf("message: scan entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("message: rows: %w", err)
	}
	return out, nil
}

// Clear deletes chat entries for one peer. Broadcast messages use BroadcastPeerID.
func (h *History) Clear(peerID string) error {
	if h == nil || h.db == nil {
		return nil
	}
	peerID = strings.TrimSpace(peerID)
	if peerID == "" {
		return errors.New("peer id required")
	}
	_, err := h.db.Exec(`DELETE FROM chat_history WHERE peer_id = ?`, peerID)
	if err != nil {
		return fmt.Errorf("message: clear peer history: %w", err)
	}
	return nil
}
