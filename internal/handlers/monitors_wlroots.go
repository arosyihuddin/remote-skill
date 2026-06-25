package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/pstar7/remote-skill/internal/proto"
)

type monWlrHead struct {
	Name     string  `json:"name"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Position wlrPos  `json:"position"`
	Scale    float64 `json:"scale,omitempty"`
	Mode     wlrMode `json:"mode,omitempty"`
}

type monWlrOutput struct {
	Heads []monWlrHead `json:"heads"`
}

func monitorsWlrRandr(ctx context.Context) ([]proto.MonitorInfo, error) {
	cmd := exec.CommandContext(ctx, "wlr-randr", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var raw []monWlrOutput
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("wlr-randr parse: %w", err)
	}
	mons := make([]proto.MonitorInfo, 0, len(raw))
	for _, o := range raw {
		for _, h := range o.Heads {
			w := h.Width
			if h.Mode.Width > 0 {
				w = h.Mode.Width
			}
			mons = append(mons, proto.MonitorInfo{
				ID:     len(mons),
				Name:   h.Name,
				X:      h.Position.X,
				Y:      h.Position.Y,
				Width:  w,
				Height: h.Height,
				Scale:  h.Scale,
			})
		}
	}
	if len(mons) == 0 {
		return nil, fmt.Errorf("wlr-randr: no outputs")
	}
	return mons, nil
}
