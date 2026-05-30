package discovery

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

const defaultInterval = 5 * time.Second

// Service broadcasts this instance's HELLO and listens for peers' HELLO/BYE.
type Service struct {
	SelfID   string
	Name     string
	HTTPPort int
	Interval time.Duration // 0 → 5s
	NowMs    func() int64  // 테스트 주입용; 0이면 wall clock
}

// Handlers receives decoded, self-filtered discovery events. ip는 항상
// 패킷 출발지 주소에서 온 값이다(페이로드의 ip는 신뢰하지 않음).
type Handlers struct {
	OnHello func(m Message, ip string)
	OnBye   func(id string)
}

// Open binds the UDP discovery socket on port and resolves the broadcast dst.
//
// WHY: SO_BROADCAST를 켜지 않으면 일부 OS(특히 Windows)에서 255.255.255.255로의
// 송신이 거부된다(WSAEACCES). best-effort로 설정한다.
func Open(port int) (net.PacketConn, net.Addr, error) {
	conn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, nil, fmt.Errorf("discovery: listen udp: %w", err)
	}
	if uc, ok := conn.(*net.UDPConn); ok {
		if rc, cerr := uc.SyscallConn(); cerr == nil {
			_ = rc.Control(func(fd uintptr) { _ = enableBroadcast(fd) })
		}
	}
	return conn, &net.UDPAddr{IP: net.IPv4bcast, Port: port}, nil
}

// Run drives broadcast + listen until ctx is cancelled, then sends one final
// BYE and tears down.
//
// WHY: 종료 순서가 중요하다 — 먼저 BYE를 보내 피어가 즉시 우리를 지우게 한 뒤
// conn을 닫아 listen의 블로킹 ReadFrom을 해제한다. 닫기를 먼저 하면 BYE가
// 유실된다.
func (s *Service) Run(ctx context.Context, conn net.PacketConn, dst net.Addr, h Handlers) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.listen(conn, h)
	}()

	s.broadcast(ctx, conn, dst) // ctx.Done까지 블로킹

	s.sendBye(conn, dst)
	_ = conn.Close()
	wg.Wait()
	return nil
}

// listen decodes incoming datagrams, drops our own and invalid ones, and
// routes the rest to the handlers. 종료는 conn.Close()로 ReadFrom을 깨워 한다.
func (s *Service) listen(conn net.PacketConn, h Handlers) {
	buf := make([]byte, 2048)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return // conn closed during shutdown
		}
		m, derr := Decode(buf[:n])
		if derr != nil {
			continue // 잡음/구버전/스푸핑 패킷은 조용히 버린다
		}
		if m.ID == s.SelfID {
			continue // 자기 자신의 브로드캐스트 필터
		}
		switch m.Type {
		case TypeHello:
			if h.OnHello != nil {
				h.OnHello(m, hostFromAddr(addr))
			}
		case TypeBye:
			if h.OnBye != nil {
				h.OnBye(m.ID)
			}
		}
	}
}

// broadcast sends a HELLO immediately, then every Interval until ctx is done.
func (s *Service) broadcast(ctx context.Context, conn net.PacketConn, dst net.Addr) {
	s.sendHello(conn, dst) // 즉시 1회 → 신규 인스턴스의 빠른 발견
	t := time.NewTicker(s.interval())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.sendHello(conn, dst)
		}
	}
}

func (s *Service) sendHello(conn net.PacketConn, dst net.Addr) {
	if b, err := Hello(s.SelfID, s.Name, s.HTTPPort, s.now()).Encode(); err == nil {
		_, _ = conn.WriteTo(b, dst)
	}
}

func (s *Service) sendBye(conn net.PacketConn, dst net.Addr) {
	if b, err := Bye(s.SelfID).Encode(); err == nil {
		_, _ = conn.WriteTo(b, dst)
	}
}

func (s *Service) interval() time.Duration {
	if s.Interval > 0 {
		return s.Interval
	}
	return defaultInterval
}

func (s *Service) now() int64 {
	if s.NowMs != nil {
		return s.NowMs()
	}
	return time.Now().UnixMilli()
}

// hostFromAddr pulls the bare IP from a packet source address.
func hostFromAddr(addr net.Addr) string {
	if u, ok := addr.(*net.UDPAddr); ok {
		return u.IP.String()
	}
	if host, _, err := net.SplitHostPort(addr.String()); err == nil {
		return host
	}
	return addr.String()
}
