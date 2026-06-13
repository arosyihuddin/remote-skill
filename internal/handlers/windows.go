package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pstar7/remote-skill/internal/handler"
)

type WindowInfo struct {
	App    string `json:"app"`
	Title  string `json:"title"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Pid    int    `json:"pid,omitempty"`
	Active bool   `json:"active"`
}

type hyprctlWindow struct {
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

func Windows(ctx context.Context, _ json.RawMessage, _ handler.StreamWriter) (any, error) {
	return windowsHyprctl(ctx)
}

func windowsHyprctl(ctx context.Context) ([]WindowInfo, error) {
	cmd := exec.CommandContext(ctx, "hyprctl", "clients", "-j")
	out, err := cmd.Output()
	if err != nil {
		return windowsWMCTRL(ctx)
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
			App:    w.Class,
			Title:  w.Title,
			X:      x, Y: y,
			Width: width, Height: height,
			Pid:    w.Pid,
			Active: activeTitle != "" && (w.Class+":"+w.Title) == activeTitle,
		})
	}
	return windows, nil
}

func windowsWMCTRL(ctx context.Context) ([]WindowInfo, error) {
	cmd := exec.CommandContext(ctx, "wmctrl", "-lG")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wmctrl: %w", err)
	}
	var windows []WindowInfo
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 7 {
			continue
		}
		var id, x, y, w, h int
		fmt.Sscanf(parts[0], "%x", &id)
		fmt.Sscanf(parts[2], "%d", &x)
		fmt.Sscanf(parts[3], "%d", &y)
		fmt.Sscanf(parts[4], "%d", &w)
		fmt.Sscanf(parts[5], "%d", &h)
		title := strings.Join(parts[6:], " ")
		windows = append(windows, WindowInfo{
			Title:  title,
			X: x, Y: y, Width: w, Height: h,
		})
	}
	return windows, nil
}
