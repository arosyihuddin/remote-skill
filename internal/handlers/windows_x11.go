package handlers

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

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
			Title:    title,
			X: x, Y: y, Width: w, Height: h,
			WindowID: parts[0],
		})
	}
	return windows, nil
}

func closeWMCTRL(ctx context.Context, windowID string) error {
	cmd := exec.CommandContext(ctx, "wmctrl", "-i", "-c", windowID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wmctrl: %w %s", err, out)
	}
	return nil
}
