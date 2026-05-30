package app

import (
	"context"
	"io"
	"testing"
	"time"
)

// WHY: 종료 경로 중 하나(ctx 취소)에서 Run이 즉시 반환해야 graceful
// shutdown 전체가 제때 수렴한다. DoD = cancel 후 ≤100ms 내 반환.
func TestRunReturnsWithinDeadlineOnCancel(t *testing.T) {
	// stdin은 막혀 있는 파이프로 둬서 "exit" 경로가 아닌 ctx 취소만 검증한다.
	pr, pw := io.Pipe()
	defer pw.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, Config{Stdin: pr}) }()

	// 고루틴이 기동할 짧은 여유.
	time.Sleep(10 * time.Millisecond)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Run did not return within 100ms of cancel")
	}
}

// WHY: 콘솔 "exit" 입력은 사용자가 가장 먼저 만나는 종료 경로다.
func TestRunExitsOnStdinExit(t *testing.T) {
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), Config{Stdin: pr}) }()

	go func() { _, _ = io.WriteString(pw, "exit\n") }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not exit on 'exit' command")
	}
}

// WHY: "exit"가 아닌 잡음 입력은 무시하고 앱이 계속 살아 있어야 한다.
func TestRunIgnoresNonExitInput(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, Config{Stdin: pr}) }()

	go func() { _, _ = io.WriteString(pw, "hello\nworld\n") }()

	select {
	case <-done:
		t.Fatal("Run exited on non-exit input")
	case <-time.After(80 * time.Millisecond):
		// 기대대로 아직 살아 있음 → 정리.
	}
	cancel()
	<-done
}
