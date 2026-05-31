package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// freeUDPPort picks an unused UDP port so discovery doesn't clash with a real
// instance on 17000 or with a sibling test.
func freeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).Port
}

// testConfig returns a config bound to ephemeral ports with the browser off.
func testConfig(t *testing.T, stdin io.Reader, onReady func(string)) Config {
	t.Helper()
	return Config{
		Name:           "test",
		HTTPPort:       0, // OS 자동
		DiscoveryPort:  0, // freeUDPPort로 채움
		OpenBrowser:    false,
		InstanceIDPath: t.TempDir() + "/redphone.id",
		Stdin:          stdin,
		OnReady:        onReady,
	}
}

// WHY: 종료 경로(ctx 취소)에서 Run이 graceful 정리 후 신속히 반환해야 한다.
func TestRunReturnsPromptlyOnCancel(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()

	cfg := testConfig(t, pr, nil)
	cfg.DiscoveryPort = freeUDPPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	time.Sleep(40 * time.Millisecond) // 기동 여유
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return promptly after cancel")
	}
}

// 콘솔 "exit" 입력으로 종료되는지.
func TestRunExitsOnStdinExit(t *testing.T) {
	pr, pw := io.Pipe()
	cfg := testConfig(t, pr, nil)
	cfg.DiscoveryPort = freeUDPPort(t)

	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), cfg) }()

	go func() { _, _ = io.WriteString(pw, "exit\n") }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit on 'exit'")
	}
}

// 통합: HTTP가 실제로 뜨고 /api/peers가 응답하며, /api/shutdown으로 종료된다.
func TestRunServesAndShutsDownViaHTTP(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()

	urlCh := make(chan string, 1)
	cfg := testConfig(t, pr, func(u string) { urlCh <- u })
	cfg.DiscoveryPort = freeUDPPort(t)

	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), cfg) }()

	var base string
	select {
	case base = <-urlCh:
	case <-time.After(2 * time.Second):
		t.Fatal("app never became ready")
	}

	resp, err := http.Get(base + "/api/peers")
	if err != nil {
		t.Fatalf("GET /api/peers: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/peers status = %d", resp.StatusCode)
	}

	// 종료 버튼 경로.
	resp, err = http.Post(base+"/api/shutdown", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/shutdown: %v", err)
	}
	resp.Body.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after /api/shutdown")
	}
}

func TestLoadOrCreateStableIDPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "redphone.id")

	id1, err := loadOrCreateStableID(path)
	if err != nil {
		t.Fatalf("first loadOrCreateStableID: %v", err)
	}
	id2, err := loadOrCreateStableID(path)
	if err != nil {
		t.Fatalf("second loadOrCreateStableID: %v", err)
	}
	if id1 == "" || id1 != id2 {
		t.Fatalf("stable id mismatch: %q vs %q", id1, id2)
	}
}
