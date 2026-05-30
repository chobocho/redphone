package web

import (
	"context"
	"sync"
	"testing"
	"time"
)

func recv(t *testing.T, c *Client) []byte {
	t.Helper()
	select {
	case b, ok := <-c.Send():
		if !ok {
			t.Fatal("client send channel closed unexpectedly")
		}
		return b
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast")
		return nil
	}
}

func TestHubBroadcastReachesAllClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := NewHub()
	go h.Run(ctx)

	a, b := NewClient(), NewClient()
	h.Register(a)
	h.Register(b)

	h.Broadcast([]byte("ping"))

	if string(recv(t, a)) != "ping" || string(recv(t, b)) != "ping" {
		t.Fatal("both clients should receive the broadcast")
	}
}

func TestHubUnregisterClosesAndStopsDelivery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := NewHub()
	go h.Run(ctx)

	a := NewClient()
	h.Register(a)
	h.Unregister(a)

	// 등록 해제 후 send 채널은 닫혀야 한다(쓰기 펌프가 종료를 감지).
	select {
	case _, ok := <-a.Send():
		if ok {
			t.Fatal("expected closed channel after unregister")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("channel not closed after unregister")
	}
}

// WHY: ctx 취소(graceful shutdown) 시 모든 클라이언트 채널을 닫아
// 쓰기 펌프들이 빠짐없이 종료되게 한다(좀비 goroutine 방지).
func TestHubClosesClientsOnCtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h := NewHub()
	go h.Run(ctx)

	a := NewClient()
	h.Register(a)
	cancel()

	select {
	case _, ok := <-a.Send():
		if ok {
			t.Fatal("expected closed channel on ctx cancel")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("channel not closed on ctx cancel")
	}
}

func TestHubMethodsSafeAfterStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h := NewHub()
	go h.Run(ctx)
	cancel()
	time.Sleep(20 * time.Millisecond) // Run 종료 대기

	// 정지 후 호출이 영원히 블로킹되면 안 된다.
	done := make(chan struct{})
	go func() {
		h.Register(NewClient())
		h.Unregister(NewClient())
		h.Broadcast([]byte("late"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("hub methods blocked after stop")
	}
}

// 동시 register/broadcast가 패닉 없이 동작하는지(논리 안전성).
func TestHubConcurrentRegisterBroadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := NewHub()
	go h.Run(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); h.Register(NewClient()) }()
		go func() { defer wg.Done(); h.Broadcast([]byte("x")) }()
	}
	wg.Wait()
}
