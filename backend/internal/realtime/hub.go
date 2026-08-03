package realtime

import (
	"context"
	"encoding/json"
	"time"

	"golang.org/x/net/websocket"
)

const (
	maxFrameBytes  = 16 * 1024
	clientQueue    = 128
	heartbeatEvery = 25 * time.Second
	readTimeout    = 70 * time.Second
	writeTimeout   = 10 * time.Second
)

type IncomingHandler func(context.Context, uint, IncomingEvent) *Event

type Hub struct {
	register   chan *connection
	unregister chan *connection
	deliver    chan delivery
}

type delivery struct {
	accountID uint
	payload   []byte
}

type connection struct {
	accountID uint
	socket    *websocket.Conn
	send      chan []byte
}

func NewHub() *Hub {
	hub := &Hub{
		register:   make(chan *connection),
		unregister: make(chan *connection),
		deliver:    make(chan delivery, 256),
	}
	go hub.run()
	return hub
}

func (h *Hub) run() {
	connections := make(map[uint]map[*connection]struct{})
	for {
		select {
		case client := <-h.register:
			if connections[client.accountID] == nil {
				connections[client.accountID] = make(map[*connection]struct{})
			}
			connections[client.accountID][client] = struct{}{}

		case client := <-h.unregister:
			accountConnections := connections[client.accountID]
			if _, ok := accountConnections[client]; !ok {
				continue
			}
			delete(accountConnections, client)
			close(client.send)
			if len(accountConnections) == 0 {
				delete(connections, client.accountID)
			}

		case item := <-h.deliver:
			for client := range connections[item.accountID] {
				select {
				case client.send <- item.payload:
				default:
					delete(connections[item.accountID], client)
					close(client.send)
					_ = client.socket.Close()
				}
			}
			if len(connections[item.accountID]) == 0 {
				delete(connections, item.accountID)
			}
		}
	}
}

func (h *Hub) Publish(accountID uint, event Event) {
	if accountID == 0 {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.PublishRaw(accountID, payload)
}

func (h *Hub) PublishRaw(accountID uint, payload []byte) {
	if accountID == 0 || len(payload) == 0 {
		return
	}
	cloned := append([]byte(nil), payload...)
	select {
	case h.deliver <- delivery{accountID: accountID, payload: cloned}:
	default:
		// 实时队列拥堵时允许丢提示；消息和通知已经持久化，可通过 REST 补拉。
	}
}

func (h *Hub) Serve(accountID uint, socket *websocket.Conn, handler IncomingHandler) {
	client := &connection{
		accountID: accountID,
		socket:    socket,
		send:      make(chan []byte, clientQueue),
	}
	socket.MaxPayloadBytes = maxFrameBytes
	socket.PayloadType = websocket.TextFrame
	_ = socket.SetReadDeadline(time.Now().Add(readTimeout))

	h.register <- client
	client.sendEvent(NewEvent("connected", map[string]any{"account_id": accountID}))

	writerDone := make(chan struct{})
	go func() {
		client.writeLoop()
		close(writerDone)
	}()

	for {
		var payload []byte
		if err := websocket.Message.Receive(socket, &payload); err != nil {
			break
		}
		_ = socket.SetReadDeadline(time.Now().Add(readTimeout))

		var incoming IncomingEvent
		if err := json.Unmarshal(payload, &incoming); err != nil {
			client.sendEvent(ErrorEvent("", "INVALID_EVENT", "invalid websocket event"))
			continue
		}
		if incoming.Type == "pong" {
			continue
		}
		if incoming.Type == "ping" {
			reply := NewEvent("pong", nil)
			reply.RequestID = incoming.RequestID
			client.sendEvent(reply)
			continue
		}
		if handler == nil {
			client.sendEvent(ErrorEvent(incoming.RequestID, "UNSUPPORTED_EVENT", "unsupported websocket event"))
			continue
		}
		if reply := handler(context.Background(), accountID, incoming); reply != nil {
			client.sendEvent(*reply)
		}
	}

	h.unregister <- client
	_ = socket.Close()
	<-writerDone
}

func (c *connection) sendEvent(event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	default:
		_ = c.socket.Close()
	}
}

func (c *connection) writeLoop() {
	ticker := time.NewTicker(heartbeatEvery)
	defer ticker.Stop()

	for {
		select {
		case payload, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.socket.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := websocket.Message.Send(c.socket, payload); err != nil {
				return
			}
		case <-ticker.C:
			payload, _ := json.Marshal(NewEvent("ping", nil))
			_ = c.socket.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := websocket.Message.Send(c.socket, payload); err != nil {
				return
			}
		}
	}
}
