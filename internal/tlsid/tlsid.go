// Package tlsid issues a self-signed TLS identity per process and exposes a
// pinned-verify client config.
//
// WHY: RedPhone은 중앙 CA가 없는 LAN P2P이므로 표준 PKI를 쓸 수 없다.
// 대신 각 인스턴스가 시작 시 자체 인증서를 만들고, UDP HELLO에 그 인증서의
// SHA-256 지문(fp)을 실어 광고한다. 발신 측은 광고된 지문과 핸드셰이크에서
// 제시된 leaf cert의 지문이 일치할 때만 연결을 수용 → MITM 차단.
package tlsid

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"
)

// Identity is one process's TLS material plus its pinning fingerprint.
type Identity struct {
	Cert        tls.Certificate
	Fingerprint string // 소문자 hex SHA-256 of DER leaf cert
}

// Generate makes an ephemeral self-signed P-256 cert valid for ~1 year.
// commonName은 사람이 읽기 쉬운 식별자(보통 사용자 이름)일 뿐, 신뢰 결정에는
// 쓰이지 않는다 — 신뢰는 지문 핀닝으로만 한다.
func Generate(commonName string) (Identity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Identity{}, fmt.Errorf("tlsid: keygen: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Identity{}, fmt.Errorf("tlsid: serial: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now.Add(-1 * time.Hour),
		NotAfter:     now.Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		// SAN은 ASCII 호스트명만 넣는다. 신뢰는 핀닝이 담당하므로
		// 표시 이름은 인증서에 실을 필요가 없다.
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return Identity{}, fmt.Errorf("tlsid: create cert: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return Identity{}, fmt.Errorf("tlsid: parse cert: %w", err)
	}
	return Identity{
		Cert: tls.Certificate{
			Certificate: [][]byte{der},
			PrivateKey:  key,
			Leaf:        leaf,
		},
		Fingerprint: FingerprintOf(leaf),
	}, nil
}

// ServerTLSConfig returns a TLS config suitable for http.Server / httptest.
func (id Identity) ServerTLSConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{id.Cert},
		MinVersion:   tls.VersionTLS12,
	}
}

// FingerprintOf computes the canonical SHA-256 hex fingerprint of a cert's DER.
func FingerprintOf(c *x509.Certificate) string {
	sum := sha256.Sum256(c.Raw)
	return hex.EncodeToString(sum[:])
}

// PinnedClientConfig returns a TLS config that ignores normal CA validation
// (InsecureSkipVerify=true) but enforces an exact fingerprint match on the
// peer's leaf cert. 표준 검증을 끄는 이유는 self-signed라 어떤 CA에도 묶이지
// 않기 때문이고, 그 대신 지문이 신뢰 앵커가 된다.
func PinnedClientConfig(expectedFP string) *tls.Config {
	want := expectedFP
	return &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("tlsid: no peer certificate")
			}
			got := FingerprintOf(cs.PeerCertificates[0])
			if got != want {
				return fmt.Errorf("tlsid: fingerprint mismatch: got %s want %s", got, want)
			}
			return nil
		},
	}
}
