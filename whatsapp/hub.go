package whatsapp

import (
	"encoding/json"
	"time"

	"github.com/gofiber/contrib/websocket"
)

// ── Realtime WebSocket hub ──
//
// The hub keeps track of all active websocket connections grouped by user id.
// Whenever something interesting happens on a user's WhatsApp session (a new
// message, a status change, a receipt, a presence update, ...) the relevant
// handler calls Broadcast, which fans the event out to every socket the user
// currently has open.

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 25 * time.Second
	wsSendBuffer = 128
)

type wsClient struct {
	conn   *websocket.Conn
	userID string
	send   chan []byte
}

type wsHub struct {
	clients map[string]map[*wsClient]struct{}
	add     chan *wsClient
	remove  chan *wsClient
	emit    chan wsOutbound
	count   chan wsCountReq
}

type wsOutbound struct {
	userID  string
	payload []byte
}

type wsCountReq struct {
	userID string
	reply  chan int
}

var hub = newHub()

func newHub() *wsHub {
	h := &wsHub{
		clients: make(map[string]map[*wsClient]struct{}),
		add:     make(chan *wsClient),
		remove:  make(chan *wsClient),
		emit:    make(chan wsOutbound, 256),
		count:   make(chan wsCountReq),
	}
	go h.run()
	return h
}

// run owns the clients map; all mutation happens on this single goroutine so we
// never need a mutex and never risk sending on a closed channel.
func (h *wsHub) run() {
	for {
		select {
		case c := <-h.add:
			set := h.clients[c.userID]
			if set == nil {
				set = make(map[*wsClient]struct{})
				h.clients[c.userID] = set
			}
			set[c] = struct{}{}

		case c := <-h.remove:
			if set, ok := h.clients[c.userID]; ok {
				if _, ok := set[c]; ok {
					delete(set, c)
					close(c.send)
				}
				if len(set) == 0 {
					delete(h.clients, c.userID)
				}
			}

		case out := <-h.emit:
			for c := range h.clients[out.userID] {
				select {
				case c.send <- out.payload:
				default:
					// Slow consumer: drop this frame rather than block the hub.
				}
			}

		case req := <-h.count:
			req.reply <- len(h.clients[req.userID])
		}
	}
}

// Broadcast delivers an event envelope to every websocket connection a user has open.
func Broadcast(userID, event string, data interface{}) {
	payload, err := json.Marshal(map[string]interface{}{
		"event":     event,
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	select {
	case hub.emit <- wsOutbound{userID: userID, payload: payload}:
	default:
		// Hub emit buffer full — drop to keep callers (event handlers) non-blocking.
	}
}

// ConnectedSockets reports how many websocket clients a user currently has open.
func ConnectedSockets(userID string) int {
	reply := make(chan int, 1)
	hub.count <- wsCountReq{userID: userID, reply: reply}
	return <-reply
}

func sendToClient(c *wsClient, event string, data interface{}) {
	payload, err := json.Marshal(map[string]interface{}{
		"event":     event,
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	default:
	}
}

// ServeWS is the fiber websocket handler. The "userId" local must already be set
// by the upgrade middleware (see main.go). It runs one writer goroutine per
// connection and uses the calling goroutine as the reader / keepalive loop.
func ServeWS(conn *websocket.Conn) {
	userID, _ := conn.Locals("userId").(string)
	if userID == "" {
		_ = conn.Close()
		return
	}

	client := &wsClient{
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, wsSendBuffer),
	}
	hub.add <- client

	done := make(chan struct{})

	// Writer goroutine — the only place that writes to the socket.
	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case msg, ok := <-client.send:
				_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if !ok {
					_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Send an immediate status snapshot so a freshly connected client is in sync.
	sendToClient(client, "status", GetStatus(userID))

	// Reader loop — also drives the read deadline / pong keepalive.
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	}

	close(done)
	hub.remove <- client
}
