package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/pstar7/remote-skill/internal/proto"
)

type hyprctlMonitor struct {
	Name    string  `json:"name"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Scale   float64 `json:"scale"`
	Focused bool    `json:"focused"`
}

func monitorsHyprctl(ctx context.Context) ([]proto.MonitorInfo, error) {
	cmd := exec.CommandContext(ctx, "hyprctl", "monitors", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var raw []hyprctlMonitor
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("hyprctl parse: %w", err)
	}
	mons := make([]proto.MonitorInfo, 0, len(raw))
	for i, m := range raw {
		mons = append(mons, proto.MonitorInfo{
			ID:      i,
			Name:    m.Name,
			X:       m.X,
			Y:       m.Y,
			Width:   m.Width,
			Height:  m.Height,
			Scale:   m.Scale,
			Focused: m.Focused,
		})
	}
	return mons, nil
}
