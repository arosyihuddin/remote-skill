package handlers

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pstar7/remote-skill/internal/proto"
)

func monitorsXrandr(ctx context.Context) ([]proto.MonitorInfo, error) {
	cmd := exec.CommandContext(ctx, "xrandr", "--query")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var mons []proto.MonitorInfo
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
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
