package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

type WindowInfo struct {
	App      string `json:"app"`
	Title    string `json:"title"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Pid      int    `json:"pid,omitempty"`
	Active   bool   `json:"active"`
	WindowID string `json:"window_id,omitempty"`
}

func Windows(ctx context.Context, _ json.RawMessage, _ handler.StreamWriter) (any, error) {
	wins, err := windowsHyprctl(ctx)
	if err == nil {
		return wins, nil
	}

	wins, err = windowsSway(ctx)
	if err == nil {
		return wins, nil
	}

	wins, err = windowsGNOME(ctx)
	if err == nil {
		return wins, nil
	}

	wins, err = windowsWMCTRL(ctx)
	if err == nil {
		return wins, nil
	}

	return nil, fmt.Errorf("windows: no compositor tool available (try installing hyprctl, swaymsg, or wmctrl)")
}

func CloseWindow(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.CloseWindowRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if req.WindowID == "" {
		return nil, fmt.Errorf("close_window: window_id is required")
	}

	if err := closeHyprland(ctx, req.WindowID); err == nil {
		return proto.EmptyResult{OK: true}, nil
	}

	if err := closeSway(ctx, req.WindowID); err == nil {
		return proto.EmptyResult{OK: true}, nil
	}

	if err := closeWMCTRL(ctx, req.WindowID); err == nil {
		return proto.EmptyResult{OK: true}, nil
	}

	return nil, fmt.Errorf("close_window: no compositor tool available")
}
