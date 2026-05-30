package web

import "context"

// Client is one connected browser. send는 쓰기 펌프가 소비하는 아웃바운드 큐다.
type Client struct {
	send chan []byte
}

// NewClient creates a client with a small buffered outbound queue.
func NewClient() *Client {
	return &Client{send: make(chan []byte, 16)}
}

// Send exposes the read side of the outbound queue for the write pump (and tests).
func (c *Client) Send() <-chan []byte { return c.send }

// Hub fans WebSocket messages out to every connected browser.
//
// WHY: 클라이언트 맵을 mutex로 공유하는 대신, 맵을 Run 고루틴 단 하나가 소유하고
// register/unregister/broadcast를 채널로 보낸다. "통신해서 메모리를 공유하라" —
// 데드락·경합 표면을 줄인다.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	stopped    chan struct{} // Run 종료 시 close → 송신 메서드가 블로킹되지 않게
}

// NewHub allocates the hub channels. 실행은 Run(ctx)로 시작한다.
func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 64),
		stopped:    make(chan struct{}),
	}
}

// Run owns the clients map until ctx is cancelled. 단일 고루틴 전용.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.stopped)
	clients := make(map[*Client]struct{})
	for {
		select {
		case <-ctx.Done():
			for c := range clients {
				close(c.send)
			}
			return
		case c := <-h.register:
			clients[c] = struct{}{}
		case c := <-h.unregister:
			if _, ok := clients[c]; ok {
				delete(clients, c)
				close(c.send)
			}
		case msg := <-h.broadcast:
			for c := range clients {
				select {
				case c.send <- msg:
				default:
					// 느린/멈춘 클라이언트는 드랍해 hub가 막히지 않게 한다.
					delete(clients, c)
					close(c.send)
				}
			}
		}
	}
}

// Register adds a client. 반환 false면 hub가 이미 정지된 것.
func (h *Hub) Register(c *Client) bool {
	select {
	case h.register <- c:
		return true
	case <-h.stopped:
		return false
	}
}

// Unregister removes a client; no-op if the hub has stopped.
func (h *Hub) Unregister(c *Client) {
	select {
	case h.unregister <- c:
	case <-h.stopped:
	}
}

// Broadcast queues a message for all clients; dropped if the hub has stopped.
func (h *Hub) Broadcast(b []byte) {
	select {
	case h.broadcast <- b:
	case <-h.stopped:
	}
}
