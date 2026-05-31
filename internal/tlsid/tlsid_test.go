package tlsid

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 생성된 self-signed 인증서는 즉시 TLS 서버에 쓸 수 있고
// 지문은 결정론적(같은 cert → 같은 fp) 이어야 한다.
func TestGenerateProducesUsableCert(t *testing.T) {
	id, err := Generate("alpha")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if id.Cert.Certificate == nil || id.Cert.PrivateKey == nil {
		t.Fatal("incomplete tls cert")
	}
	if len(id.Fingerprint) != 64 { // sha256 hex
		t.Fatalf("fingerprint len = %d, want 64 hex chars", len(id.Fingerprint))
	}
	if _, err := hex.DecodeString(id.Fingerprint); err != nil {
		t.Fatalf("fingerprint not hex: %v", err)
	}

	// 서버에 붙여 실제 TLS handshake가 되는지 확인.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	}))
	srv.TLS = id.ServerTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: PinnedClientConfig(id.Fingerprint)},
		Timeout:   2 * time.Second,
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("pinned client get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
}

// 지문이 다른 인증서를 제시하는 서버는 거부되어야 한다(MITM 방어).
func TestPinnedConfigRejectsMismatch(t *testing.T) {
	server, _ := Generate("server")
	attacker, _ := Generate("attacker")

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.TLS = server.ServerTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	// attacker.Fingerprint로 핀닝 → server가 제시하는 cert와 불일치.
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: PinnedClientConfig(attacker.Fingerprint)},
		Timeout:   2 * time.Second,
	}
	if _, err := client.Get(srv.URL); err == nil {
		t.Fatal("expected handshake failure on fingerprint mismatch")
	}
}

// 같은 입력으로 두 번 만들어도 키쌍은 매번 다르므로 지문도 달라야 한다
// (인증서 캐시 등으로 우연히 같아지는 회귀 방지).
func TestGenerateProducesUniqueCerts(t *testing.T) {
	a, _ := Generate("x")
	b, _ := Generate("x")
	if a.Fingerprint == b.Fingerprint {
		t.Fatal("two fresh certs collided on fingerprint")
	}
}

// PinnedClientConfig는 cert chain이 비어 있어도 안전하게 거부해야 한다.
// (정상 핸드셰이크 흐름에선 발생하지 않지만 방어적으로 확인)
func TestPinnedConfigRejectsEmptyChain(t *testing.T) {
	cfg := PinnedClientConfig("deadbeef")
	if cfg.VerifyConnection == nil {
		t.Fatal("VerifyConnection not set")
	}
	err := cfg.VerifyConnection(tls.ConnectionState{PeerCertificates: nil})
	if err == nil || !strings.Contains(err.Error(), "no peer certificate") {
		t.Fatalf("expected no-cert error, got %v", err)
	}
}

// 핀닝 클라이언트가 일반 TCP 끝점에 닿아도 클린하게 실패해야 한다(panic 없음).
func TestPinnedClientFailsOnNonTLSServer(t *testing.T) {
	id, _ := Generate("x")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			c.Close()
		}
	}()

	d := tls.Dialer{Config: PinnedClientConfig(id.Fingerprint)}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := d.DialContext(ctx, "tcp", ln.Addr().String()); err == nil {
		t.Fatal("expected dial error against non-TLS server")
	}
}

// FingerprintOf는 PinnedClientConfig.VerifyPeerCertificate와 동일한 산식이어야 한다.
func TestFingerprintOfMatchesPinning(t *testing.T) {
	id, _ := Generate("x")
	leaf, err := x509.ParseCertificate(id.Cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := FingerprintOf(leaf); got != id.Fingerprint {
		t.Fatalf("FingerprintOf=%s, want %s", got, id.Fingerprint)
	}
}
