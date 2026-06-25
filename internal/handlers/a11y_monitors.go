package handlers

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

func getWindowRects(root *A11yNode) []winRect {
	if wins := queryGNOMEExtWindows(); len(wins) > 0 {
		return wins
	}
	return queryHyprlandWindows()
}

func queryHyprlandWindows() []winRect {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hyprctl", "clients", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var clients []struct {
		At   [2]int `json:"at"`
		Size [2]int `json:"size"`
	}
	if err := json.Unmarshal(out, &clients); err != nil {
		return nil
	}

	wins := make([]winRect, 0, len(clients))
	for _, c := range clients {
		wins = append(wins, winRect{
			frameX: c.At[0],
			frameY: c.At[1],
			frameW: c.Size[0],
			frameH: c.Size[1],
			bufX:   c.At[0],
			bufY:   c.At[1],
			bufW:   c.Size[0],
			bufH:   c.Size[1],
		})
	}
	return wins
}

func queryGNOMEExtWindows() []winRect {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gdbus", "call", "--session",
		"--dest", "org.gnome.Shell",
		"--object-path", "/org/gnome/Shell/Extensions/Windows",
		"--method", "org.gnome.Shell.Extensions.Windows.List").Output()
	if err != nil {
		return nil
	}
	s := string(out)
	start := strings.IndexByte(s, '\'')
	end := strings.LastIndexByte(s, '\'')
	if start < 0 || end <= start {
		return nil
	}
	var wins []struct {
		X, Y           int
		BufX           int `json:"buf_x,omitempty"`
		BufY           int `json:"buf_y,omitempty"`
		Width, Height  int
		FrameW         int `json:"frame_w,omitempty"`
		FrameH         int `json:"frame_h,omitempty"`
	}
	if json.Unmarshal([]byte(s[start+1:end]), &wins) != nil {
		return nil
	}
	out2 := make([]winRect, 0, len(wins))
	for _, w := range wins {
		if w.Width > 0 && w.Height > 0 {
			fw, fh := w.FrameW, w.FrameH
			if fw <= 0 {
				fw = w.Width
			}
			if fh <= 0 {
				fh = w.Height
			}
			bx, by := w.BufX, w.BufY
			if bx == 0 && by == 0 && w.X != 0 {
				bx, by = w.X, w.Y
			}
			out2 = append(out2, winRect{
				bufX: bx, bufY: by, bufW: w.Width, bufH: w.Height,
				frameX: w.X, frameY: w.Y, frameW: fw, frameH: fh,
			})
		}
	}
	return out2
}

func monitorByGeometry(ctx context.Context, root *A11yNode) map[int]int {
	mons := hyprctlMonitors(ctx)
	if mons == nil {
		result := make(map[int]int)
		assignAllToMonitor(root, 0, result)
		return result
	}

	layouts := make([]monLayout, len(mons))
	for i, m := range mons {
		layouts[i] = monLayout{
			ID: m.ID, Width: m.Width, Height: m.Height,
			WorkHeight: m.Height - 40,
		}
	}

	panelMons := make([]int, 0)
	for _, p := range getPanelHeights(ctx) {
		for i := range layouts {
			if layouts[i].ID == p.monID {
				layouts[i].WorkHeight = layouts[i].Height - p.pixels
				panelMons = append(panelMons, p.monID)
			}
		}
	}

	clients := getClientRects(ctx)
	result := make(map[int]int)
	state := &walkState{}
	walkAssignMonitors(root, layouts, mons, clients, panelMons, state, result)
	return result
}

type walkState struct {
	waybarIdx int
}

type monLayout struct {
	ID         int
	Width      int
	Height     int
	WorkHeight int
}

type hyprctlMon struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

func hyprctlMonitors(ctx context.Context) []hyprctlMon {
	cmd := exec.CommandContext(ctx, "hyprctl", "monitors", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var raw []hyprctlMon
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}
	return raw
}

type layerRec struct {
	Namespace string `json:"namespace"`
	H         int    `json:"h"`
	Monitor   string `json:"monitor"`
}

type panelInfo struct {
	monID  int
	pixels int
}

func getPanelHeights(ctx context.Context) []panelInfo {
	cmd := exec.CommandContext(ctx, "hyprctl", "layers", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var layers []layerRec
	if err := json.Unmarshal(out, &layers); err != nil {
		return nil
	}

	mons := hyprctlMonitors(ctx)
	if mons == nil {
		return nil
	}
	nameToID := make(map[string]int)
	for _, m := range mons {
		nameToID[m.Name] = m.ID
	}

	panelByMon := make(map[int]int)
	for _, l := range layers {
		if l.Namespace == "waybar" || l.Namespace == "ags" || l.Namespace == "gnome-panel" {
			if mid, ok := nameToID[l.Monitor]; ok {
				panelByMon[mid] += l.H
			}
		}
	}

	var result []panelInfo
	for id, h := range panelByMon {
		result = append(result, panelInfo{id, h})
	}
	return result
}

type hyprctlClientRect struct {
	At      [2]int `json:"at"`
	Size    [2]int `json:"size"`
	Monitor int    `json:"monitor"`
}

func getClientRects(ctx context.Context) []hyprctlClientRect {
	cmd := exec.CommandContext(ctx, "hyprctl", "clients", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var raw []hyprctlClientRect
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}
	return raw
}

func assignAllToMonitor(node *A11yNode, mon int, result map[int]int) {
	if node.Role == "window" || node.Role == "frame" {
		if node.Bounds[2] > 0 && node.Bounds[3] > 0 {
			result[node.ID] = mon
		}
	}
	for i := range node.Children {
		assignAllToMonitor(&node.Children[i], mon, result)
	}
}

func walkAssignMonitors(node *A11yNode, layouts []monLayout, mons []hyprctlMon, clients []hyprctlClientRect, panelMons []int, state *walkState, result map[int]int) {
	if node.Bounds[2] > 0 && node.Bounds[3] > 0 {
		if node.Role == "frame" && node.Name == "waybar" && len(panelMons) > 0 {
			mon := panelMons[state.waybarIdx%len(panelMons)]
			state.waybarIdx++
			result[node.ID] = mon
		} else if mon := matchByGeometry(node, layouts, mons, clients); mon >= 0 {
			result[node.ID] = mon
		}
	}
	for i := range node.Children {
		walkAssignMonitors(&node.Children[i], layouts, mons, clients, panelMons, state, result)
	}
}

func matchByGeometry(node *A11yNode, layouts []monLayout, mons []hyprctlMon, clients []hyprctlClientRect) int {
	h := node.Bounds[3]

	bestMon, bestDist := -1, 99999
	for _, m := range layouts {
		if d := abs(h - m.WorkHeight); d < bestDist {
			bestDist = d
			bestMon = m.ID
		}
	}

	if bestMon >= 0 && bestDist <= 50 {
		ambiguous := false
		for _, m := range layouts {
			if m.ID != bestMon && abs(h-m.WorkHeight) <= 50 {
				ambiguous = true
				break
			}
		}
		if !ambiguous {
			return bestMon
		}
	}

	return matchByIoU(node, mons, clients)
}

type rect struct{ X, Y, W, H int }

func matchByIoU(node *A11yNode, mons []hyprctlMon, clients []hyprctlClientRect) int {
	bestMon, bestArea := -1, 0

	for _, mo := range mons {
		global := rect{
			X: node.Bounds[0] + mo.X,
			Y: node.Bounds[1] + mo.Y,
			W: node.Bounds[2],
			H: node.Bounds[3],
		}
		for _, c := range clients {
			if c.Monitor != mo.ID {
				continue
			}
			cr := rect{X: c.At[0], Y: c.At[1], W: c.Size[0], H: c.Size[1]}
			if area := intersectArea(global, cr); area > bestArea {
				bestArea = area
				bestMon = mo.ID
			}
		}
	}
	if bestArea > 0 {
		return bestMon
	}
	return -1
}

func intersectArea(a, b rect) int {
	xL := max(a.X, b.X)
	yT := max(a.Y, b.Y)
	xR := min(a.X+a.W, b.X+b.W)
	yB := min(a.Y+a.H, b.Y+b.H)
	if xR <= xL || yB <= yT {
		return 0
	}
	return (xR - xL) * (yB - yT)
}
