package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type gnomeWindowRaw struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	WmClass string `json:"wm_class"`
	PID     int    `json:"pid"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Focused bool   `json:"focused"`
}

func windowsGNOME(ctx context.Context) ([]WindowInfo, error) {
	cmd := exec.CommandContext(ctx, "gdbus", "call", "--session",
		"--dest", "org.gnome.Shell",
		"--object-path", "/org/gnome/Shell/Extensions/Windows",
		"--method", "org.gnome.Shell.Extensions.Windows.List")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseGNOMEJSON(out)
}

func parseGNOMEJSON(out []byte) ([]WindowInfo, error) {
	s := string(out)
	start := strings.IndexByte(s, '\'')
	end := strings.LastIndexByte(s, '\'')
	if start == -1 || end <= start {
		return nil, fmt.Errorf("parse gnome list: unexpected gdbus output")
	}
	var raw []gnomeWindowRaw
	if err := json.Unmarshal([]byte(s[start+1:end]), &raw); err != nil {
		return nil, fmt.Errorf("parse gnome list: %w", err)
	}
	wins := make([]WindowInfo, len(raw))
	for i, w := range raw {
		wins[i] = WindowInfo{
			App:      w.WmClass,
			Title:    w.Title,
			X:        w.X,
			Y:        w.Y,
			Width:    w.Width,
			Height:   w.Height,
			Pid:      w.PID,
			Active:   w.Focused,
			WindowID: strconv.Itoa(w.ID),
		}
	}
	return wins, nil
}
