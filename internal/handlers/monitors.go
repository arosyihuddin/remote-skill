package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

func Monitors(ctx context.Context, _ json.RawMessage, _ handler.StreamWriter) (any, error) {
	mons, err := DetectMonitors(ctx)
	if err != nil {
		return nil, err
	}
	return mons, nil
}

func DetectMonitors(ctx context.Context) ([]proto.MonitorInfo, error) {
	if mons, err := monitorsHyprctl(ctx); err == nil {
		return mons, nil
	}

	if mons, err := monitorsSway(ctx); err == nil {
		return mons, nil
	}

	if mons, err := monitorsWlrRandr(ctx); err == nil {
		return mons, nil
	}

	if mons, err := monitorsXrandr(ctx); err == nil {
		return mons, nil
	}

	return nil, fmt.Errorf("monitors: no tool available (try installing hyprctl, swaymsg, wlr-randr, or xrandr)")
}
