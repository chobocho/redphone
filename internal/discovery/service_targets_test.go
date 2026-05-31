package discovery_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/chobocho/redphone/internal/discovery"
)

// fakePkt is one captured/queued datagram.
type fakePkt struct {
	b    []byte
	addr net.Addr
}

// fakeConn is an in-memory net.PacketConn so we can drive listen()/broadcast()
// deterministically without real UDP — inbound via in, outbound captured in out.
type fakeConn struct {
	in     chan fakePkt
	out    chan fakePkt
	closed chan struct{}
	once   sync.Once
}

func newFakeConn() *fakeConn {
	return &fakeConn{
		in:     make(chan fakePkt, 32),
		out:    make(chan fakePkt, 64),
		closed: make(chan struct{}),
	}
}

func (f *fakeConn) ReadFrom(p []byte) (int, net.Addr, error) {
	select {
	case pkt := <-f.in:
		return copy(p, pkt.b), pkt.addr, nil
	case <-f.closed:
		return 0, nil, net.ErrClosed
	}
}

func (f *fakeConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	b := append([]byte(nil), p...)
	select {
	case f.out <- fakePkt{b: b, addr: addr}:
	case <-f.closed:
		return 0, net.ErrClosed
	}
	return len(p), nil
}

func (f *fakeConn) Close() error                       { f.once.Do(func() { close(f.closed) }); return nil }
func (f *fakeConn) LocalAddr() net.Addr                { return &net.UDPAddr{} }
func (f *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

// TestAddTargetValidatesNormalizesAndSorts는 친구 IP 등록의 검증·정규화·정렬과
// 삭제(idempotent)를 본다.
func TestAddTargetValidatesNormalizesAndSorts(t *testing.T) {
	svc := &discovery.Service{}
	if err := svc.AddTarget("nope"); err == nil {
		t.Error("invalid IP를 받아들였다")
	}
	for _, ip := range []string{"192.168.0.9", "192.168.0.5", "192.168.0.5"} {
		if err := svc.AddTarget(ip); err != nil {
			t.Fatalf("AddTarget(%q): %v", ip, err)
		}
	}
	got := svc.Targets()
	if len(got) != 2 || got[0] != "192.168.0.5" || got[1] != "192.168.0.9" {
		t.Fatalf("Targets = %v, want [192.168.0.5 192.168.0.9]", got)
	}
	svc.RemoveTarget("192.168.0.9")
	if got := svc.Targets(); len(got) != 1 || got[0] != "192.168.0.5" {
		t.Errorf("after remove = %v, want [192.168.0.5]", got)
	}
}

// TestScanLANNotRunning는 소켓이 없을 때(미기동) ErrNotRunning을 주는지 본다.
func TestScanLANNotRunning(t *testing.T) {
	svc := &discovery.Service{}
	if _, err := svc.ScanLAN(); !errors.Is(err, discovery.ErrNotRunning) {
		t.Errorf("err = %v, want ErrNotRunning", err)
	}
}

// TestRunRepliesToProbeOnce는 reply=false HELLO를 받으면 src로 reply=true HELLO를
// 1회 되쏘는지 본다. 이게 한쪽만 등록해도 양방향 발견을 성립시킨다.
func TestRunRepliesToProbeOnce(t *testing.T) {
	svc := &discovery.Service{SelfID: "me", Name: "me", HTTPPort: 1, HTTPSPort: 2, FP: "fp", Port: 17000, Interval: time.Hour}
	fc := newFakeConn()
	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: 17000}
	gotHello := make(chan string, 4)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = svc.Run(ctx, fc, dst, discovery.Handlers{
			OnHello: func(m discovery.Message, ip string) { select { case gotHello <- m.ID: default: } },
		})
		close(done)
	}()

	src := &net.UDPAddr{IP: net.ParseIP("192.0.2.5"), Port: 17000}
	probe := discovery.Hello("peerX", "x", 9, 9, "fpx", 1) // reply=false
	pb, _ := probe.Encode()
	fc.in <- fakePkt{b: pb, addr: src}

	waitHelloID(t, gotHello, "peerX")
	if !drainForReply(fc, src, true, 800*time.Millisecond) {
		t.Error("src로 향하는 reply=true HELLO를 관측하지 못했다")
	}

	cancel()
	<-done
}

// TestRunDoesNotReplyToReply는 reply=true HELLO에는 재응답하지 않는지 본다 —
// 응답의 응답이 없어야 발견 루프가 무한히 돌지 않는다.
func TestRunDoesNotReplyToReply(t *testing.T) {
	svc := &discovery.Service{SelfID: "me", Name: "me", HTTPPort: 1, HTTPSPort: 2, FP: "fp", Port: 17000, Interval: time.Hour}
	fc := newFakeConn()
	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: 17000}
	gotHello := make(chan string, 4)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = svc.Run(ctx, fc, dst, discovery.Handlers{
			OnHello: func(m discovery.Message, ip string) { select { case gotHello <- m.ID: default: } },
		})
		close(done)
	}()

	src := &net.UDPAddr{IP: net.ParseIP("192.0.2.7"), Port: 17000}
	reply := discovery.Hello("peerY", "y", 9, 9, "fpy", 1)
	reply.Reply = true
	rb, _ := reply.Encode()
	fc.in <- fakePkt{b: rb, addr: src}

	waitHelloID(t, gotHello, "peerY")
	if drainForReply(fc, src, true, 300*time.Millisecond) {
		t.Error("reply=true HELLO에 또 응답했다(루프 위험)")
	}

	cancel()
	<-done
}

func waitHelloID(t *testing.T, ch <-chan string, want string) {
	t.Helper()
	select {
	case id := <-ch:
		if id != want {
			t.Fatalf("OnHello id = %q, want %q", id, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("OnHello(%q)를 받지 못했다", want)
	}
}

// drainForReply는 window 동안 out을 훑어 want=Reply 값이고 src로 향하는 HELLO가
// 있는지 본다.
func drainForReply(fc *fakeConn, src net.Addr, wantReply bool, window time.Duration) bool {
	deadline := time.After(window)
	for {
		select {
		case pkt := <-fc.out:
			m, err := discovery.Decode(pkt.b)
			if err != nil || m.Type != discovery.TypeHello {
				continue
			}
			if m.Reply == wantReply && pkt.addr.String() == src.String() {
				return true
			}
		case <-deadline:
			return false
		}
	}
}
