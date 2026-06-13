// Package node implements the laptop-side daemon: connects to broker,
// receives request frames, dispatches to handlers, and streams responses.
package node

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

const Version = "0.1.0"

// Node connects to a broker and runs handlers.
type Node struct {
	ServerURL string
	DeviceID  string
	Token     string
	Handlers  map[proto.MessageType]handler.Handler

	conn    *websocket.Conn
	writeMu sync.Mutex
}

func New(serverURL, deviceID, token string) *Node {
	return &Node{
		ServerURL: serverURL,
		DeviceID:  deviceID,
		Token:     token,
		Handlers:  map[proto.MessageType]handler.Handler{},
	}
}

// Register installs a handler for a message type.
func (n *Node) Register(t proto.MessageType, h handler.Handler) {
	n.Handlers[t] = h
}

// RunForever connects with reconnect backoff. Returns only on permanent error.
func (n *Node) RunForever(ctx context.Context, baseReconnect time.Duration) error {
	delay := baseReconnect
	for {
		err := n.runOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			log.Printf("connection ended: %v; retrying in %s", err, delay)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		// Exponential backoff capped at 30s
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		// Reset on successful long session is handled below in runOnce.
	}
}

func (n *Node) runOnce(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(dialCtx, n.ServerURL, &websocket.DialOptions{
		HTTPClient: &http.Client{Timeout: 0},
	})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.SetReadLimit(64 << 20)
	defer c.Close(websocket.StatusNormalClosure, "")

	n.conn = c

	// Send Hello
	hostname, _ := os.Hostname()
	hello := proto.Hello{
		Token:    n.Token,
		DeviceID: n.DeviceID,
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  Version,
		Role:     proto.RoleNode,
	}
	plBytes, _ := json.Marshal(hello)
	helloFrame, _ := json.Marshal(proto.Frame{Type: proto.TypeHello, ID: "hello", Payload: plBytes})
	if err := c.Write(ctx, websocket.MessageText, helloFrame); err != nil {
		return fmt.Errorf("write hello: %w", err)
	}

	// Wait for Ack
	readCtx, cancelR := context.WithTimeout(ctx, 10*time.Second)
	_, ackBytes, err := c.Read(readCtx)
	cancelR()
	if err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	var ackFrame proto.Frame
	if err := json.Unmarshal(ackBytes, &ackFrame); err != nil {
		return fmt.Errorf("decode ack: %w", err)
	}
	if ackFrame.Type != proto.TypeAck {
		return fmt.Errorf("expected ack, got %s", ackFrame.Type)
	}
	log.Printf("connected to %s as %s", n.ServerURL, n.DeviceID)

	// Read loop
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return err
		}
		var f proto.Frame
		if err := json.Unmarshal(data, &f); err != nil {
			continue
		}
		switch f.Type {
		case proto.TypePing:
			n.sendFrame(ctx, c, proto.Frame{Type: proto.TypePong, ID: f.ID})
		case proto.TypePong:
			// ignore
		default:
			go n.dispatch(ctx, f, c)
		}
	}
}

func (n *Node) sendFrame(ctx context.Context, conn *websocket.Conn, f proto.Frame) {
	b, _ := json.Marshal(f)
	n.writeMu.Lock()
	defer n.writeMu.Unlock()
	_ = conn.Write(ctx, websocket.MessageText, b)
}

func (n *Node) dispatch(ctx context.Context, f proto.Frame, conn *websocket.Conn) {
	h, ok := n.Handlers[f.Type]
	if !ok {
		ep, _ := json.Marshal(proto.ErrorPayload{Code: "unsupported", Message: string(f.Type)})
		n.sendFrame(ctx, conn, proto.Frame{Type: proto.TypeError, ID: f.ID, Payload: ep})
		return
	}
	// Detect streaming for exec
	streaming := false
	if f.Type == proto.TypeExec {
		var er proto.ExecRequest
		_ = json.Unmarshal(f.Payload, &er)
		streaming = er.Stream
	}
	sw := handler.StreamWriter{Id: f.ID, Conn: conn, Mu: &n.writeMu, Enable: streaming}
	res, err := h(ctx, f.Payload, sw)
	if err != nil {
		ep, _ := json.Marshal(proto.ErrorPayload{Code: "handler_error", Message: err.Error()})
		n.sendFrame(ctx, conn, proto.Frame{Type: proto.TypeError, ID: f.ID, Payload: ep})
		return
	}
	pl, err := json.Marshal(res)
	if err != nil {
		ep, _ := json.Marshal(proto.ErrorPayload{Code: "marshal", Message: err.Error()})
		n.sendFrame(ctx, conn, proto.Frame{Type: proto.TypeError, ID: f.ID, Payload: ep})
		return
	}
	n.sendFrame(ctx, conn, proto.Frame{Type: proto.TypeResponse, ID: f.ID, Payload: pl})
}
