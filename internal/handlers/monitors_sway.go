package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/pstar7/remote-skill/internal/proto"
)

type swayOutput struct {
	Name   string   `json:"name"`
	Make   string   `json:"make"`
	Model  string   `json:"model"`
	Active bool     `json:"active"`
	Rect   swayRect `json:"rect"`
	Scale  *float64 `json:"scale"`
}

func monitorsSway(ctx context.Context) ([]proto.MonitorInfo, error) {
	cmd := exec.CommandContext(ctx, "swaymsg", "-t", "get_outputs")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var raw []swayOutput
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("swaymsg parse: %w", err)
	}
	mons := make([]proto.MonitorInfo, 0, len(raw))
	for i, o := range raw {
		if !o.Active {
			continue
		}
		mons = append(mons, proto.MonitorInfo{
			ID:      i,
			Name:    o.Name,
			X:       o.Rect.X,
			Y:       o.Rect.Y,
			Width:   o.Rect.Width,
			Height:  o.Rect.Height,
			Focused: false,
		})
		if o.Scale != nil {
			mons[len(mons)-1].Scale = *o.Scale
		}
	}
	if len(mons) == 0 {
		return nil, fmt.Errorf("swaymsg: no active outputs")
	}
	return mons, nil
}
