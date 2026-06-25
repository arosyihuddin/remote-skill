package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type hyprctlWindow struct {
	Address string `json:"address"`
	Class   string `json:"class"`
	Title   string `json:"title"`
	At      []int  `json:"at"`
	Size    []int  `json:"size"`
	Pid     int    `json:"pid"`
	Mapped  bool   `json:"mapped"`
	Hidden  bool   `json:"hidden"`
}

type hyprctlActive struct {
	Class string `json:"class"`
	Title string `json:"title"`
}

func windowsHyprctl(ctx context.Context) ([]WindowInfo, error) {
	cmd := exec.CommandContext(ctx, "hyprctl", "clients", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var all []hyprctlWindow
	if err := json.Unmarshal(out, &all); err != nil {
		return nil, fmt.Errorf("hyprctl parse: %w", err)
	}

	activeTitle := ""
	activeCmd := exec.CommandContext(ctx, "hyprctl", "activewindow", "-j")
	if activeOut, err := activeCmd.Output(); err == nil {
		var act hyprctlActive
		if json.Unmarshal(activeOut, &act) == nil {
			activeTitle = act.Class + ":" + act.Title
		}
	}

	var windows []WindowInfo
	for _, w := range all {
		if !w.Mapped || w.Hidden {
			continue
		}
		x, y := 0, 0
		if len(w.At) >= 2 {
			x, y = w.At[0], w.At[1]
		}
		width, height := 0, 0
		if len(w.Size) >= 2 {
			width, height = w.Size[0], w.Size[1]
		}
		windows = append(windows, WindowInfo{
			App:      w.Class,
			Title:    w.Title,
			X:        x, Y: y,
			Width:    width, Height: height,
			Pid:      w.Pid,
			Active:   activeTitle != "" && (w.Class+":"+w.Title) == activeTitle,
			WindowID: w.Address,
		})
	}
	return windows, nil
}

func closeHyprland(ctx context.Context, windowID string) error {
	cmd := exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "address:"+windowID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hyprctl: %w %s", err, out)
	}
	return nil
}
