package discovery

import (
	"net"
	"testing"
)

// TestHostsInNet는 서브넷 호스트 열거의 핵심 수학을 검증한다 — 실제 NIC에
// 의존하지 않도록 합성 IPNet으로 본다.
func TestHostsInNet(t *testing.T) {
	contains := func(hs []string, ip string) bool {
		for _, h := range hs {
			if h == ip {
				return true
			}
		}
		return false
	}

	// /24: .1~.254 중 self(.10) 제외 → 253개. .0/.255는 제외.
	_, n24, _ := net.ParseCIDR("192.168.0.10/24")
	n24.IP = net.ParseIP("192.168.0.10") // ParseCIDR은 IP를 네트워크로 마스킹하므로 복원
	h24 := hostsInNet(n24)
	if len(h24) != 253 {
		t.Errorf("/24 host count = %d, want 253", len(h24))
	}
	if contains(h24, "192.168.0.10") {
		t.Error("self(.10)가 포함됐다")
	}
	if contains(h24, "192.168.0.0") || contains(h24, "192.168.0.255") {
		t.Error("network/broadcast가 포함됐다")
	}
	if !contains(h24, "192.168.0.1") || !contains(h24, "192.168.0.254") {
		t.Error(".1 또는 .254가 누락됐다")
	}

	// /16처럼 넓은 마스크는 self의 /24로 좁혀야 한다(폭주 방지).
	wide := &net.IPNet{IP: net.ParseIP("10.1.2.3"), Mask: net.CIDRMask(16, 32)}
	hWide := hostsInNet(wide)
	if len(hWide) != 253 {
		t.Errorf("/16 narrowed host count = %d, want 253", len(hWide))
	}
	if !contains(hWide, "10.1.2.1") || contains(hWide, "10.1.3.1") {
		t.Error("/16을 self의 /24(10.1.2.0/24)로 좁히지 못했다")
	}

	// 루프백·IPv6는 스캔 대상이 아니다.
	lo := &net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}
	if h := hostsInNet(lo); h != nil {
		t.Errorf("loopback should yield nil, got %d", len(h))
	}
	v6 := &net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)}
	if h := hostsInNet(v6); h != nil {
		t.Errorf("IPv6 should yield nil, got %d", len(h))
	}
}
