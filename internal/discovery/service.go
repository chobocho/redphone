package discovery

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"syscall"
	"time"
)

const (
	defaultInterval = 5 * time.Second
	scanInterval    = 30 * time.Second // -scan 모드의 주기적 서브넷 스윕 간격
)

// ErrNotRunning is returned by ScanLAN when the service has no live socket yet.
var ErrNotRunning = errors.New("discovery: service not running")

// Service broadcasts this instance's HELLO and listens for peers' HELLO/BYE.
//
// 브로드캐스트가 막힌 망을 위해 두 가지 유니캐스트 경로를 추가로 가진다.
//   - 수동 등록 친구 IP(targets): 매 틱마다 직접 HELLO를 보낸다.
//   - 전체 서브넷 스캔(ScanLAN): 옵션으로 켤 때 모든 호스트에 HELLO를 뿌린다.
//
// 유니캐스트 HELLO를 받은 쪽은 1회 응답(reply=true)을 되돌려, 한쪽만 상대를
// 등록해도 양방향으로 서로의 지문/포트를 학습한다.
type Service struct {
	SelfID      string
	SelfUID     string
	Name        string
	HTTPPort    int
	HTTPSPort   int           // v2: 메시지/파일메타 TLS 포트
	FP          string        // v2: 우리 leaf cert SHA-256 hex
	Interval    time.Duration // 0 → 5s
	Port        int           // 유니캐스트 HELLO 대상 포트; 0이면 discovery.Port
	ScanOnStart bool          // true면 Run이 주기적 서브넷 스캔 루프를 띄운다
	NowMs       func() int64  // 테스트 주입용; 0이면 wall clock

	mu      sync.Mutex          // targets/conn 보호
	targets map[string]struct{} // 수동 등록된 친구 IP 집합
	conn    net.PacketConn      // Run 동안만 유효; AddTarget/ScanLAN의 즉시 송신용
}

// Handlers receives decoded, self-filtered discovery events. ip는 항상
// 패킷 출발지 주소에서 온 값이다(페이로드의 ip는 신뢰하지 않음).
type Handlers struct {
	OnHello func(m Message, ip string)
	OnBye   func(id string)
}

// Open binds the UDP discovery socket on port and resolves the broadcast dst.
//
// WHY: 두 가지 소켓 옵션이 필요하다.
//   - SO_REUSEADDR: 같은 PC에서 인스턴스 여럿이 같은 17000을 바인딩해 모두
//     브로드캐스트를 받게 한다(문서화된 같은-PC 2인스턴스 테스트). bind 전에
//     설정해야 하므로 ListenConfig.Control을 쓴다.
//   - SO_BROADCAST: 미설정 시 일부 OS(특히 Windows)에서 255.255.255.255 송신이
//     거부된다(WSAEACCES). bind 후 설정.
func Open(port int) (net.PacketConn, net.Addr, error) {
	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var serr error
			if err := c.Control(func(fd uintptr) { serr = controlReuse(fd) }); err != nil {
				return err
			}
			return serr
		},
	}
	conn, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf(":%d", port))
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
	s.setConn(conn)      // AddTarget/ScanLAN의 즉시 송신이 이 소켓을 쓴다
	defer s.setConn(nil) // Run 종료 후 stale 소켓 송신 방지

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.listen(conn, h)
	}()

	if s.ScanOnStart {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.scanLoop(ctx, conn)
		}()
	}

	s.broadcast(ctx, conn, dst) // ctx.Done까지 블로킹

	s.sendBye(conn, dst)
	_ = conn.Close()
	wg.Wait()
	return nil
}

// setConn stores (or clears) the live socket so AddTarget/ScanLAN can probe
// immediately while Run is active.
func (s *Service) setConn(conn net.PacketConn) {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
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
			// 유니캐스트/브로드캐스트로 받은 첫 HELLO(reply=false)에는 1회만
			// 직접 응답한다. 상대 소켓도 같은 포트에 바인딩돼 있으므로 src addr로
			// 그대로 되쏜다. reply=true에는 응답하지 않아 루프가 끊긴다 —
			// 한쪽만 친구 IP를 등록해도 양방향 발견이 성립한다.
			if !m.Reply {
				s.sendHelloReply(conn, addr)
			}
		case TypeBye:
			if h.OnBye != nil {
				h.OnBye(m.ID)
			}
		}
	}
}

// broadcast advertises immediately, then every Interval until ctx is done.
// 매 주기마다 브로드캐스트 HELLO와 수동 등록 친구 IP로의 유니캐스트 HELLO를
// 함께 보낸다 — 브로드캐스트가 막힌 망에서도 친구를 계속 살려 둔다.
func (s *Service) broadcast(ctx context.Context, conn net.PacketConn, dst net.Addr) {
	s.advertise(conn, dst) // 즉시 1회 → 신규 인스턴스의 빠른 발견
	t := time.NewTicker(s.interval())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.advertise(conn, dst)
		}
	}
}

// advertise broadcasts a HELLO and unicasts one to every manual target.
func (s *Service) advertise(conn net.PacketConn, dst net.Addr) {
	s.sendHello(conn, dst)
	for _, ip := range s.Targets() {
		s.unicastHello(conn, ip, false)
	}
}

// scanLoop periodically sweeps the local subnet(s) with unicast HELLOs while
// the -scan option is on. 즉시 1회 후 scanInterval마다 반복한다.
func (s *Service) scanLoop(ctx context.Context, conn net.PacketConn) {
	s.scanOnce(conn)
	t := time.NewTicker(scanInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.scanOnce(conn)
		}
	}
}

func (s *Service) sendHello(conn net.PacketConn, dst net.Addr) {
	m := Hello(s.SelfID, s.Name, s.HTTPPort, s.HTTPSPort, s.FP, s.now())
	if s.SelfUID != "" {
		m.UID = s.SelfUID
	}
	if b, err := m.Encode(); err == nil {
		_, _ = conn.WriteTo(b, dst)
	}
}

func (s *Service) sendBye(conn net.PacketConn, dst net.Addr) {
	b, err := Bye(s.SelfID).Encode()
	if err != nil {
		return
	}
	_, _ = conn.WriteTo(b, dst)
	// 브로드캐스트가 막힌 친구도 즉시 우리를 지우도록 유니캐스트 BYE도 보낸다.
	for _, ip := range s.Targets() {
		_, _ = conn.WriteTo(b, s.unicastAddr(ip))
	}
}

// sendHelloReply unicasts a one-shot HELLO (reply=true) back to a probing peer.
func (s *Service) sendHelloReply(conn net.PacketConn, addr net.Addr) {
	m := Hello(s.SelfID, s.Name, s.HTTPPort, s.HTTPSPort, s.FP, s.now())
	if s.SelfUID != "" {
		m.UID = s.SelfUID
	}
	m.Reply = true
	if b, err := m.Encode(); err == nil {
		_, _ = conn.WriteTo(b, addr)
	}
}

// unicastHello sends a HELLO to a specific IP at the discovery port.
func (s *Service) unicastHello(conn net.PacketConn, ip string, reply bool) {
	m := Hello(s.SelfID, s.Name, s.HTTPPort, s.HTTPSPort, s.FP, s.now())
	if s.SelfUID != "" {
		m.UID = s.SelfUID
	}
	m.Reply = reply
	if b, err := m.Encode(); err == nil {
		_, _ = conn.WriteTo(b, s.unicastAddr(ip))
	}
}

// unicastAddr builds the UDP destination for a manual/scan target.
func (s *Service) unicastAddr(ip string) *net.UDPAddr {
	port := s.Port
	if port == 0 {
		port = Port
	}
	return &net.UDPAddr{IP: net.ParseIP(ip), Port: port}
}

// AddTarget registers a manual friend IP and probes it immediately if running.
// 잘못된 IP는 거부한다. 같은 IP를 다시 넣어도 무해(idempotent)하다.
func (s *Service) AddTarget(ip string) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("discovery: invalid ip %q", ip)
	}
	norm := parsed.String()
	s.mu.Lock()
	if s.targets == nil {
		s.targets = make(map[string]struct{})
	}
	s.targets[norm] = struct{}{}
	conn := s.conn
	s.mu.Unlock()
	if conn != nil {
		s.unicastHello(conn, norm, false) // 즉시 probe → 빠른 발견
	}
	return nil
}

// RemoveTarget drops a manual friend IP. 모르는 IP면 무해한 no-op이다.
// 더 이상 probe하지 않으면 상대는 TTL(15s) 후 자동으로 사라진다.
func (s *Service) RemoveTarget(ip string) {
	norm := ip
	if parsed := net.ParseIP(ip); parsed != nil {
		norm = parsed.String()
	}
	s.mu.Lock()
	delete(s.targets, norm)
	s.mu.Unlock()
}

// Targets returns a sorted snapshot of the manual friend IPs.
func (s *Service) Targets() []string {
	s.mu.Lock()
	out := make([]string, 0, len(s.targets))
	for ip := range s.targets {
		out = append(out, ip)
	}
	s.mu.Unlock()
	sort.Strings(out)
	return out
}

// ScanLAN unicasts a HELLO to every host on the local subnet(s) once.
// 옵션 기능 — 모든 IP 스캔이 막히는 환경도 있으니 기본은 꺼져 있다.
// 반환값은 보낸 probe 수다.
func (s *Service) ScanLAN() (int, error) {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return 0, ErrNotRunning
	}
	return s.scanOnce(conn), nil
}

// scanOnce probes every local-subnet host and returns how many were sent.
func (s *Service) scanOnce(conn net.PacketConn) int {
	hosts := localSubnetHosts()
	for _, ip := range hosts {
		s.unicastHello(conn, ip, false)
	}
	return len(hosts)
}

// localSubnetHosts enumerates every usable host IP on the machine's IPv4
// subnets, excluding our own addresses, the network, and the broadcast address.
//
// WHY: /16 같은 넓은 마스크를 그대로 돌면 65k probe가 되어 망을 마비시킨다.
// 마스크가 /24보다 넓으면 우리 IP가 속한 /24로 좁혀 최대 254개만 스캔한다.
func localSubnetHosts() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var hosts []string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		for _, ip := range hostsInNet(ipnet) {
			if _, dup := seen[ip]; dup {
				continue
			}
			seen[ip] = struct{}{}
			hosts = append(hosts, ip)
		}
	}
	return hosts
}

// hostsInNet returns the usable host IPs of an IPv4 subnet, excluding ipnet.IP
// itself, the network address, and the broadcast address. /24보다 넓은 마스크는
// ipnet.IP가 속한 /24로 좁혀 최대 253개만 돌린다(망 마비 방지). 루프백·IPv6는 nil.
func hostsInNet(ipnet *net.IPNet) []string {
	ip4 := ipnet.IP.To4()
	if ip4 == nil || ip4.IsLoopback() {
		return nil
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return nil // IPv4만 스캔
	}
	mask := ipnet.Mask
	if ones < 24 {
		ones = 24
		mask = net.CIDRMask(24, 32)
	}
	base := binary.BigEndian.Uint32(ip4.Mask(mask))
	count := uint32(1) << uint(32-ones)
	self := binary.BigEndian.Uint32(ip4)
	var hosts []string
	for i := uint32(1); i < count-1; i++ { // .0(network)·마지막(broadcast) 제외
		v := base + i
		if v == self {
			continue // 자기 자신 제외
		}
		var h [4]byte
		binary.BigEndian.PutUint32(h[:], v)
		hosts = append(hosts, net.IP(h[:]).String())
	}
	return hosts
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
