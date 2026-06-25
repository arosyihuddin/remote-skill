package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type swayNode struct {
	ID       int           `json:"id"`
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	AppID    *string       `json:"app_id"`
	PID      int           `json:"pid"`
	Focused  bool          `json:"focused"`
	Rect     swayRect      `json:"rect"`
	Nodes    []swayNode    `json:"nodes"`
	Floating []swayNode    `json:"floating_nodes"`
	WinProps *swayWinProps `json:"window_properties"`
}

type swayRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type swayWinProps struct {
	Class string `json:"class"`
}

type swayTree struct {
	Nodes []swayNode `json:"nodes"`
}

func windowsSway(ctx context.Context) ([]WindowInfo, error) {
	cmd := exec.CommandContext(ctx, "swaymsg", "-t", "get_tree")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var tree swayTree
	if err := json.Unmarshal(out, &tree); err != nil {
		return nil, fmt.Errorf("swaymsg parse: %w", err)
	}
	return swayCollect(tree.Nodes), nil
}

func swayCollect(nodes []swayNode) []WindowInfo {
	var windows []WindowInfo
	for _, n := range nodes {
		if n.Type == "con" || n.Type == "floating_con" {
			app := ""
			if n.AppID != nil {
				app = *n.AppID
			} else if n.WinProps != nil {
				app = n.WinProps.Class
			}
			if n.Name != "" {
				windows = append(windows, WindowInfo{
					App:      app,
					Title:    n.Name,
					X:        n.Rect.X,
					Y:        n.Rect.Y,
					Width:    n.Rect.Width,
					Height:   n.Rect.Height,
					Pid:      n.PID,
					Active:   n.Focused,
					WindowID: strconv.Itoa(n.ID),
				})
			}
		}
		windows = append(windows, swayCollect(n.Nodes)...)
		windows = append(windows, swayCollect(n.Floating)...)
	}
	return windows
}

func closeSway(ctx context.Context, windowID string) error {
	cmd := exec.CommandContext(ctx, "swaymsg", fmt.Sprintf("[con_id=%s] kill", windowID))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("swaymsg: %w %s", err, out)
	}
	return nil
}
