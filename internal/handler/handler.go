// Package handler defines the shared handler interface for remote-skill operations.
package handler

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
	"github.com/pstar7/remote-skill/internal/proto"
)

// Handler executes a request payload and returns a result payload.
// For streaming requests, the handler writes chunks via stream and returns
// the final result.
type Handler func(ctx context.Context, payload json.RawMessage, stream StreamWriter) (any, error)

// StreamWriter sends streaming chunks during a request. Safe for concurrent use.
type StreamWriter struct {
	Id     string
	Conn   *websocket.Conn
	Mu     *sync.Mutex
	Enable bool
}

func (s StreamWriter) Send(channel, data string) {
	if !s.Enable {
		return
	}
	pl, _ := json.Marshal(proto.StreamChunk{Channel: channel, Data: data})
	frame, _ := json.Marshal(proto.Frame{Type: proto.TypeStream, ID: s.Id, Payload: pl})
	s.Mu.Lock()
	defer s.Mu.Unlock()
	_ = s.Conn.Write(context.Background(), websocket.MessageText, frame)
}
