package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pstar7/remote-skill/internal/handler"
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

type swayOutput struct {
	Name   string      `json:"name"`
	Make   string      `json:"make"`
	Model  string      `json:"model"`
	Active bool        `json:"active"`
	Rect   swayRect    `json:"rect"`
	Scale  *float64    `json:"scale"`
}

type monWlrHead struct {
	Name       string  `json:"name"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Position   wlrPos  `json:"position"`
	Scale      float64 `json:"scale,omitempty"`
	Mode       wlrMode `json:"mode,omitempty"`
}

type monWlrOutput struct {
	Heads []monWlrHead `json:"heads"`
}

func Monitors(ctx context.Context, _ json.RawMessage, _ handler.StreamWriter) (any, error) {
	mons, err := DetectMonitors(ctx)
	if err != nil {
		return nil, err
	}
	return mons, nil
}

func DetectMonitors(ctx context.Context) ([]proto.MonitorInfo, error) {
	// 1. Hyprland — hyprctl monitors -j
	if mons, err := monitorsHyprctl(ctx); err == nil {
		return mons, nil
	}

	// 2. Sway — swaymsg -t get_outputs
	if mons, err := monitorsSway(ctx); err == nil {
		return mons, nil
	}

	// 3. wlroots universal — wlr-randr --json
	if mons, err := monitorsWlrRandr(ctx); err == nil {
		return mons, nil
	}

	// 4. X11 universal — xrandr --query
	if mons, err := monitorsXrandr(ctx); err == nil {
		return mons, nil
	}

	return nil, fmt.Errorf("monitors: no tool available (try installing hyprctl, swaymsg, wlr-randr, or xrandr)")
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

func monitorsXrandr(ctx context.Context) ([]proto.MonitorInfo, error) {
	cmd := exec.CommandContext(ctx, "xrandr", "--query")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var mons []proto.MonitorInfo
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// Match lines like: "HDMI-1 connected 1920x1080+0+0"
		if !strings.Contains(line, " connected") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		mode := fields[2]
		if mode == "primary" && len(fields) > 3 {
			mode = fields[3]
		}
		// mode format: 1920x1080+0+0 or 1920x1080 (if disconnected)
		if !strings.Contains(mode, "+") {
			continue
		}
		parts := strings.Split(mode, "+")
		if len(parts) != 3 {
			continue
		}
		res := strings.Split(parts[0], "x")
		if len(res) != 2 {
			continue
		}
		w, _ := strconv.Atoi(res[0])
		h, _ := strconv.Atoi(res[1])
		x, _ := strconv.Atoi(parts[1])
		y, _ := strconv.Atoi(parts[2])
		mons = append(mons, proto.MonitorInfo{
			ID:     len(mons),
			Name:   name,
			X:      x,
			Y:      y,
			Width:  w,
			Height: h,
			Scale:  1.0,
		})
	}
	if len(mons) == 0 {
		return nil, fmt.Errorf("xrandr: no active outputs")
	}
	return mons, nil
}
