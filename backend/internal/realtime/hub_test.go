package realtime

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"

	"golang.org/x/net/websocket"
)

func TestHubSendsBrowserCompatibleTextFrames(t *testing.T) {
	hub := NewHub()
	wsServer := websocket.Server{
		Handler: func(socket *websocket.Conn) {
			hub.Serve(7, socket, func(_ context.Context, _ uint, incoming IncomingEvent) *Event {
				reply := NewEvent("test.ack", map[string]any{"ok": true})
				reply.RequestID = incoming.RequestID
				return &reply
			})
		},
	}
	serverConn, clientConn := net.Pipe()
	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		request, err := http.ReadRequest(reader)
		if err != nil {
			serverDone <- err
			return
		}
		writer := &pipeResponseWriter{
			conn: serverConn,
			buf:  bufio.NewReadWriter(reader, bufio.NewWriter(serverConn)),
		}
		wsServer.ServeHTTP(writer, request)
		serverDone <- nil
	}()

	config, err := websocket.NewConfig("ws://videohub.test/ws", "http://videohub.test")
	if err != nil {
		t.Fatalf("create websocket config: %v", err)
	}
	client, err := websocket.NewClient(config, clientConn)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	assertTextEvent(t, client, "connected", "")
	if err := websocket.Message.Send(client, `{"type":"test","request_id":"request-1"}`); err != nil {
		t.Fatalf("send request: %v", err)
	}
	assertTextEvent(t, client, "test.ack", "request-1")
	if err := client.Close(); err != nil {
		t.Fatalf("close websocket: %v", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("websocket server: %v", err)
	}
}

type pipeResponseWriter struct {
	conn net.Conn
	buf  *bufio.ReadWriter
}

func (w *pipeResponseWriter) Header() http.Header         { return make(http.Header) }
func (w *pipeResponseWriter) WriteHeader(_ int)           {}
func (w *pipeResponseWriter) Write(p []byte) (int, error) { return w.conn.Write(p) }
func (w *pipeResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.buf, nil
}

func assertTextEvent(t *testing.T, client *websocket.Conn, eventType, requestID string) {
	t.Helper()
	reader, err := client.NewFrameReader()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if reader.PayloadType() != websocket.TextFrame {
		t.Fatalf("frame type = %d, want TextFrame", reader.PayloadType())
	}
	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if event.Type != eventType || event.RequestID != requestID {
		t.Fatalf("event = type %q request_id %q, want type %q request_id %q", event.Type, event.RequestID, eventType, requestID)
	}
}
