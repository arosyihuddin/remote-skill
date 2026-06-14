package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atotto/clipboard"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

func ClipboardRead(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.ClipboardReadRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

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

func Board(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.BoardRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if err := clipboard.WriteAll(req.Text); err != nil {
		return nil, fmt.Errorf("clipboard write: %w", err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := platformKeyCombo([]string{"ctrl", "v"}); err != nil {
		if err2 := platformKeyCombo([]string{"super", "v"}); err2 != nil {
			return nil, fmt.Errorf("board paste failed: ctrl+v: %w, super+v: %w", err, err2)
		}
	}
	return proto.EmptyResult{OK: true}, nil
}
