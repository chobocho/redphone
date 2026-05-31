package discovery

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeConn은 net.PacketConn을 흉내내 실제 소켓 없이 listen/broadcast를 검증한다.
type fakeConn struct {
	in     chan rxPacket
	closed chan struct{}
	once   sync.Once

	mu     sync.Mutex
	writes [][]byte
}

type rxPacket struct {
	data []byte
	addr net.Addr
}

func newFakeConn() *fakeConn {
	return &fakeConn{in: make(chan rxPacket, 16), closed: make(chan struct{})}
}

func (f *fakeConn) ReadFrom(p []byte) (int, net.Addr, error) {
	select {
	case pk := <-f.in:
		return copy(p, pk.data), pk.addr, nil
	case <-f.closed:
		return 0, nil, net.ErrClosed
	}
}

func (f *fakeConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	select {
	case <-f.closed:
		return 0, net.ErrClosed
	default:
	}
	f.mu.Lock()
	f.writes = append(f.writes, append([]byte(nil), p...))
	f.mu.Unlock()
	return len(p), nil
}

func (f *fakeConn) Close() error                       { f.once.Do(func() { close(f.closed) }); return nil }
func (f *fakeConn) LocalAddr() net.Addr                { return &net.UDPAddr{} }
func (f *fakeConn) SetDeadline(time.Time) error        { return nil }
func (f *fakeConn) SetReadDeadline(time.Time) error    { return nil }
func (f *fakeConn) SetWriteDeadline(time.Time) error   { return nil }
func (f *fakeConn) written() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.writes))
	copy(out, f.writes)
	return out
}

func udpAddr(ip string) net.Addr { return &net.UDPAddr{IP: net.ParseIP(ip), Port: Port} }

func TestListenExtractsIPFromSrcAddrAndFiltersSelf(t *testing.T) {
	conn := newFakeConn()
	svc := &Service{SelfID: "me", Name: "me", HTTPPort: 17080}

	var gotIP, gotID string
	var helloCalls int
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		svc.listen(conn, Handlers{
			OnHello: func(m Message, ip string) {
				mu.Lock()
				helloCalls++
				gotIP, gotID = ip, m.ID
				mu.Unlock()
			},
		})
		close(done)
	}()

	// 자기 자신의 HELLO → 필터링되어 콜백 없음.
	self, _ := Hello("me", "me", 17080, 17443, "fp-self", 1).Encode()
	conn.in <- rxPacket{self, udpAddr("10.0.0.1")}
	// 피어의 HELLO → ip는 페이로드가 아니라 src addr(10.0.0.5)에서.
	other, _ := Hello("peerB", "bravo", 17081, 17444, "fp-peer", 2).Encode()
	conn.in <- rxPacket{other, udpAddr("10.0.0.5")}
	// 깨진 패킷 → 무시(크래시 없음).
	conn.in <- rxPacket{[]byte("garbage"), udpAddr("10.0.0.9")}

	// 콜백이 처리될 시간을 준 뒤 종료.
	time.Sleep(40 * time.Millisecond)
	conn.Close()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if helloCalls != 1 {
		t.Fatalf("want exactly 1 hello callback, got %d", helloCalls)
	}
	if gotID != "peerB" || gotIP != "10.0.0.5" {
		t.Fatalf("want peerB@10.0.0.5, got %s@%s", gotID, gotIP)
	}
}

func TestListenRoutesBye(t *testing.T) {
	conn := newFakeConn()
	svc := &Service{SelfID: "me"}
	var goneID string
	done := make(chan struct{})
	go func() {
		svc.listen(conn, Handlers{OnBye: func(id string) { goneID = id; conn.Close() }})
		close(done)
	}()
	bye, _ := Bye("peerB").Encode()
	conn.in <- rxPacket{bye, udpAddr("10.0.0.5")}
	<-done
	if goneID != "peerB" {
		t.Fatalf("want bye for peerB, got %q", goneID)
	}
}

func TestBroadcastSendsHelloImmediately(t *testing.T) {
	conn := newFakeConn()
	svc := &Service{SelfID: "me", Name: "alice", HTTPPort: 17080, HTTPSPort: 17443, FP: "fp-me", Interval: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	go svc.broadcast(ctx, conn, udpAddr("255.255.255.255"))

	// 첫 HELLO는 ticker를 기다리지 않고 즉시 나가야 빠른 발견이 가능하다.
	deadline := time.After(300 * time.Millisecond)
	for {
		if len(conn.written()) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("no HELLO broadcast within 300ms")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	cancel()

	m, err := Decode(conn.written()[0])
	if err != nil {
		t.Fatalf("broadcast payload not decodable: %v", err)
	}
	if m.Type != TypeHello || m.ID != "me" || m.Name != "alice" || m.HTTPPort != 17080 ||
		m.HTTPSPort != 17443 || m.FP != "fp-me" {
		t.Fatalf("unexpected hello: %+v", m)
	}
}

func TestRunSendsByeAndReturnsOnCancel(t *testing.T) {
	conn := newFakeConn()
	svc := &Service{SelfID: "me", Name: "me", HTTPPort: 17080, Interval: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() { _ = svc.Run(ctx, conn, udpAddr("255.255.255.255"), Handlers{}); close(done) }()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return promptly after cancel")
	}

	// 마지막 쓰기는 BYE여야 한다(graceful 떠남 통지).
	w := conn.written()
	last, err := Decode(w[len(w)-1])
	if err != nil || last.Type != TypeBye || last.ID != "me" {
		t.Fatalf("expected final BYE, got %+v (err=%v)", last, err)
	}
}
