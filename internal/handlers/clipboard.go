package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/atotto/clipboard"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

func ClipboardRead(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.ClipboardReadRequest
	json.Unmarshal(payload, &req)

	text, err := clipboard.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("clipboard read: %w", err)
	}
	return proto.ClipboardReadResult{Content: text}, nil
}

func ClipboardWrite(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.ClipboardWriteRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if err := clipboard.WriteAll(req.Content); err != nil {
		return nil, fmt.Errorf("clipboard write: %w", err)
	}
	return proto.EmptyResult{OK: true}, nil
}
