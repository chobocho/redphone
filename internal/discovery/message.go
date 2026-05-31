// Package discovery implements serverless LAN peer discovery over UDP.
//
// WHY: 중앙 서버 0대 원칙. 각 인스턴스가 17000/udp로 HELLO를 브로드캐스트하고
// 서로의 HELLO를 수신해 피어 목록을 구성한다. 떠날 때는 BYE 1회를 보낸다.
package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Protocol constants. 버전을 올리면 구버전 패킷은 Decode에서 거부된다.
//
// v2 변경(2026-05): TLS 지문(FP)·HTTPS 포트 추가. v1 패킷은 거부된다 —
// 메시지/파일 메타가 HTTPS로 가야 하므로 구버전과의 부분 호환은 위험하다.
const (
	Version = 2
	Port    = 17000 // DISCOVERY_PORT (고정)

	TypeHello = "hello"
	TypeBye   = "bye"
)

// Message is the UDP wire payload (JSON). 작은 평면 구조로 유지한다.
type Message struct {
	V         int    `json:"v"`
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	HTTPPort  int    `json:"httpPort,omitempty"`
	HTTPSPort int    `json:"httpsPort,omitempty"` // v2: 메시지/파일메타 전용 TLS 포트
	FP        string `json:"fp,omitempty"`        // v2: leaf cert SHA-256 hex
	TS        int64  `json:"ts,omitempty"`
	Reply     bool   `json:"reply,omitempty"` // 유니캐스트 probe에 대한 1회 응답 표시. 발견 루프를 끊는다.
}

// Hello builds a HELLO advertisement. ip는 의도적으로 싣지 않는다 —
// 수신 측이 패킷 src addr에서 취해 스푸핑을 막는다.
func Hello(id, name string, httpPort, httpsPort int, fp string, ts int64) Message {
	return Message{
		V: Version, Type: TypeHello, ID: id, Name: name,
		HTTPPort: httpPort, HTTPSPort: httpsPort, FP: fp, TS: ts,
	}
}

// Bye builds a BYE notice sent once before graceful shutdown.
func Bye(id string) Message {
	return Message{V: Version, Type: TypeBye, ID: id}
}

// Encode marshals the message to its JSON datagram form.
func (m Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// Decode parses and validates a datagram. 검증 실패 패킷은 호출자가 버린다.
func Decode(b []byte) (Message, error) {
	var m Message
	if err := json.Unmarshal(b, &m); err != nil {
		return Message{}, fmt.Errorf("discovery: bad json: %w", err)
	}
	if m.V != Version {
		return Message{}, fmt.Errorf("discovery: unsupported version %d", m.V)
	}
	if m.Type != TypeHello && m.Type != TypeBye {
		return Message{}, fmt.Errorf("discovery: unknown type %q", m.Type)
	}
	if m.ID == "" {
		return Message{}, errors.New("discovery: empty id")
	}
	// HELLO는 TLS 지문이 반드시 있어야 한다. 지문 없는 피어로는
	// 메시지/파일메타 송신 자체가 불가능하므로 받자마자 거른다.
	if m.Type == TypeHello && m.FP == "" {
		return Message{}, errors.New("discovery: hello missing fp")
	}
	return m, nil
}
