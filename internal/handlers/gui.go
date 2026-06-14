package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"strings"
	"time"

	"github.com/kbinani/screenshot"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

func Screenshot(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.ScreenshotRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	// grim duluan — Wayland native, support -o outputName (multi-monitor)
	img, err := screenshotGrim(ctx, req)
	if err == nil {
		return encodePNG(img)
	}

	// fallback X11 (cuma display 0, ga support -o)
	img, err = screenshotX11(ctx, req)
	if err == nil {
		return encodePNG(img)
	}

	return nil, fmt.Errorf("screenshot: %w (try installing grim: sudo apt install grim)", err)
}

func encodePNG(img image.Image) (any, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("png: %w", err)
	}
	bounds := img.Bounds()
	return proto.ScreenshotResult{
		Base64: base64.StdEncoding.EncodeToString(buf.Bytes()),
		Format: "png",
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
	}, nil
}

func screenshotX11(ctx context.Context, req proto.ScreenshotRequest) (image.Image, error) {
	if req.Region != "" {
		var x, y, w, h int
		if _, err := fmt.Sscanf(req.Region, "%d,%d %dx%d", &x, &y, &w, &h); err != nil {
			return nil, err
		}
		return screenshot.Capture(x, y, w, h)
	}
	bounds := screenshot.GetDisplayBounds(0)
	return screenshot.CaptureRect(bounds)
}

func screenshotGrim(ctx context.Context, req proto.ScreenshotRequest) (image.Image, error) {
	if _, err := exec.LookPath("grim"); err != nil {
		return nil, fmt.Errorf("grim not found")
	}
	args := []string{}
	if req.Output != "" {
		args = append(args, "-o", req.Output)
	}
	if req.Region != "" {
		args = append(args, "-g", req.Region)
	}
	args = append(args, "-")
	cmd := exec.CommandContext(ctx, "grim", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func Click(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.ClickRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if req.X != nil || req.Y != nil {
		x, y := 0, 0
		if req.X != nil {
			x = *req.X
		}
		if req.Y != nil {
			y = *req.Y
		}
		if err := platformMoveMouse(x, y); err != nil {
			return nil, err
		}
	}
	btn := strings.ToLower(req.Button)
	if btn == "" {
		btn = "left"
	}
	if err := platformMouseClick(btn, req.Double); err != nil {
		return nil, err
	}
	return proto.EmptyResult{OK: true}, nil
}

func Type(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.TypeRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if req.Delay > 0 {
		time.Sleep(time.Duration(req.Delay) * time.Millisecond)
	}
	if err := platformTypeText(req.Text); err != nil {
		return nil, err
	}
	return proto.EmptyResult{OK: true}, nil
}

func Key(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.KeyRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	parts := strings.Split(req.Combo, "+")
	if err := platformKeyCombo(parts); err != nil {
		return nil, err
	}
	return proto.EmptyResult{OK: true}, nil
}

type wlrMode struct {
	Width   int  `json:"width"`
	Height  int  `json:"height"`
	Current bool `json:"current"`
}

type wlrPos struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type wlrOutput struct {
	Name     string    `json:"name"`
	Enabled  bool      `json:"enabled"`
	Modes    []wlrMode `json:"modes"`
	Position wlrPos    `json:"position"`
}

type MonitorInfo struct {
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func GetMonitors() ([]MonitorInfo, error) {
	cmd := exec.Command("wlr-randr", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wlr-randr: %w", err)
	}
	var outputs []wlrOutput
	if err := json.Unmarshal(out, &outputs); err != nil {
		return nil, fmt.Errorf("wlr-randr parse: %w", err)
	}
	var mons []MonitorInfo
	for _, o := range outputs {
		if !o.Enabled {
			continue
		}
		for _, m := range o.Modes {
			if m.Current {
				mons = append(mons, MonitorInfo{
					Name:   o.Name,
					X:      o.Position.X,
					Y:      o.Position.Y,
					Width:  m.Width,
					Height: m.Height,
				})
				break
			}
		}
	}
	return mons, nil
}

func Scroll(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.ScrollRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if err := platformMouseScroll(req.DY); err != nil {
		return nil, err
	}
	return proto.EmptyResult{OK: true}, nil
}

func MouseMove(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.MouseMoveRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	if req.Relative {
		return proto.EmptyResult{OK: true}, platformMoveMouseRel(req.X, req.Y)
	}

	x, y := req.X, req.Y

	mons, err := GetMonitors()
	if err == nil && len(mons) > 0 {
		minX, minY := 0, 0
		maxX, maxY := 0, 0
		for _, m := range mons {
			if m.X < minX {
				minX = m.X
			}
			if m.Y < minY {
				minY = m.Y
			}
			if mx := m.X + m.Width; mx > maxX {
				maxX = mx
			}
			if my := m.Y + m.Height; my > maxY {
				maxY = my
			}
		}

		canvasW := maxX - minX
		canvasH := maxY - minY
		if canvasW > 0 && canvasH > 0 {
			absX := x
			absY := y
			if req.Monitor != nil {
				idx := *req.Monitor
				if idx >= 0 && idx < len(mons) {
					m := mons[idx]
					absX = x + m.X
					absY = y + m.Y
				}
			}
			absX -= minX
			absY -= minY
			x = int(float64(absX) / float64(canvasW) * 65535)
			y = int(float64(absY) / float64(canvasH) * 65535)
		}
	}

	return proto.EmptyResult{OK: true}, platformMoveMouse(x, y)
}

func Drag(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.DragRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	btn := strings.ToLower(req.Button)
	if btn == "" {
		btn = "left"
	}
	if err := platformDragMouse(req.X1, req.Y1, req.X2, req.Y2, btn); err != nil {
		return nil, err
	}
	return proto.EmptyResult{OK: true}, nil
}
