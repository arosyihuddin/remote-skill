package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

type A11yNode struct {
	ID       int            `json:"id,omitempty"`
	Name     string         `json:"name"`
	Role     string         `json:"role"`
	Bounds   [4]int         `json:"bounds"`
	Enabled  bool           `json:"enabled"`
	Focused  bool           `json:"focused"`
	Children []A11yNode     `json:"children,omitempty"`
	depth    int            `json:"-"`
	bus      string         `json:"-"`
	path     dbus.ObjectPath `json:"-"`
}

type FlatNode struct {
	ID      int    `json:"id"`
	Role   string `json:"role"`
	Name   string `json:"name"`
	Bounds [4]int `json:"bounds"`
	Parent int    `json:"parent,omitempty"`
	Monitor int   `json:"mon,omitempty"`
}

const sessionBusName = "org.a11y.Bus"
const sessionBusPath = "/org/a11y/bus"

var interactiveRoles = map[string]bool{
	"push button":   true,
	"toggle button": true,
	"entry":         true,
	"password text": true,
	"check box":     true,
	"combo box":     true,
	"radio button":  true,
	"slider":        true,
	"link":          true,
	"menu item":     true,
	"page tab":      true,
	"spin button":   true,
	"switch":        true,
	"tree item":     true,
}

func isInteractive(role string) bool {
	return interactiveRoles[role]
}

func validBounds(b [4]int) bool {
	return b[2] > 0 && b[3] > 0 && b[0] != -2147483648
}

func isUseful(node A11yNode, depth int) bool {
	if depth <= 1 {
		return true
	}
	if isInteractive(node.Role) {
		return true
	}
	if node.Name != "" {
		return true
	}
	if !validBounds(node.Bounds) {
		return false
	}
	for _, c := range node.Children {
		if isUseful(c, depth+1) {
			return true
		}
	}
	return false
}

func matchesAnyRole(role string, roles []string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
		if r == "button" && (role == "push button" || role == "toggle button") {
			return true
		}
		if r == "input" && (role == "entry" || role == "password text") {
			return true
		}
		if r == "checkbox" && role == "check box" {
			return true
		}
		if r == "dropdown" && role == "combo box" {
			return true
		}
	}
	return false
}

func AccessibilityTree(ctx context.Context, raw json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.A11yRequest
	if raw != nil {
		_ = json.Unmarshal(raw, &req)
	}

	session, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("session bus: %w", err)
	}

	var addr string
	if err := session.Object(sessionBusName, sessionBusPath).Call(sessionBusName+".GetAddress", 0).Store(&addr); err != nil {
		return nil, fmt.Errorf("a11y address: %w", err)
	}

	a11y, err := dbus.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("a11y dial: %w", err)
	}
	if err := a11y.Auth(nil); err != nil {
		_ = a11y.Close()
		return nil, fmt.Errorf("a11y auth: %w", err)
	}
	defer func() { _ = a11y.Close() }()

	if err := a11y.Hello(); err != nil {
		_ = a11y.Close()
		return nil, fmt.Errorf("a11y hello: %w", err)
	}

	depth := req.Depth
	if depth <= 0 {
		depth = 8
	}

	nextID := 1
	done := make(chan A11yNode, 1)
	go func() {
		done <- walkA11yTree(a11y, "org.a11y.atspi.Registry", dbus.ObjectPath("/org/a11y/atspi/accessible/root"), 0, depth, &nextID)
	}()

	var root A11yNode
	select {
	case root = <-done:
	case <-time.After(12 * time.Second):
		return nil, fmt.Errorf("a11y tree: timeout")
	}

	if req.ID != nil {
		node := findNodeByID(&root, *req.ID)
		if node == nil {
			return nil, fmt.Errorf("node %d not found", *req.ID)
		}
		text := formatNodeText(root, *req.ID, 0)
		if node.Name != "" && node.bus != "" {
			obj := a11y.Object(node.bus, node.path)
			if charText := getCharExtentsText(node.Name, obj); charText != "" {
				text += charText
			}
		}
		return proto.A11yTextResponse{Text: text}, nil
	}

	winMon := monitorByGeometry(ctx, &root)
	nodes := flattenTree(root, req.ShowAll, req.Roles, winMon, 0, -1, 0)
	if req.Monitor != nil {
		filtered := make([]FlatNode, 0, len(nodes))
		for _, n := range nodes {
			if n.Monitor == *req.Monitor {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}
	return proto.A11yTextResponse{Text: formatListText(nodes)}, nil
}

func a11yCall(obj dbus.BusObject, method string, args ...any) ([]any, error) {
	call := obj.Call(method, 0, args...)
	if call.Err != nil {
		return nil, call.Err
	}
	return call.Body, nil
}

func getString(v any) string {
	s, _ := v.(string)
	return s
}

func getPath(v any) dbus.ObjectPath {
	switch p := v.(type) {
	case dbus.ObjectPath:
		return p
	case string:
		return dbus.ObjectPath(p)
	}
	return ""
}

func walkA11yTree(conn *dbus.Conn, bus string, path dbus.ObjectPath, depth, maxDepth int, nextID *int) A11yNode {
	id := *nextID
	*nextID++

	node := A11yNode{ID: id, depth: depth, bus: bus, path: path}
	if depth > maxDepth {
		node.Role = "..."
		return node
	}

	obj := conn.Object(bus, path)

	if body, err := a11yCall(obj, "org.a11y.atspi.Accessible.GetRoleName"); err == nil && len(body) > 0 {
		node.Role = getString(body[0])
	}

	if v, err := obj.GetProperty("org.a11y.atspi.Accessible.Name"); err == nil {
		node.Name = getString(v.Value())
	}

	if body, err := a11yCall(obj, "org.a11y.atspi.Accessible.GetState"); err == nil && len(body) > 0 {
		if states, ok := body[0].([]uint32); ok {
			for _, s := range states {
				if s&(1<<1) != 0 {
					node.Enabled = true
				}
				if s&(1<<4) != 0 {
					node.Focused = true
				}
			}
		}
	}

	if body, err := a11yCall(obj, "org.a11y.atspi.Component.GetExtents", uint32(0)); err == nil && len(body) >= 1 {
		if tup, ok := body[0].([]any); ok && len(tup) >= 4 {
			if x, ok := tup[0].(int32); ok {
				node.Bounds[0] = int(x)
			}
			if y, ok := tup[1].(int32); ok {
				node.Bounds[1] = int(y)
			}
			if w, ok := tup[2].(int32); ok {
				node.Bounds[2] = int(w)
			}
			if h, ok := tup[3].(int32); ok {
				node.Bounds[3] = int(h)
			}
		}
	}

	// Skip children of invisible background apps (saves DBus calls)
	if depth >= 1 && !validBounds(node.Bounds) && !isInteractive(node.Role) && node.Name == "" {
		return node
	}

	if body, err := a11yCall(obj, "org.a11y.atspi.Accessible.GetChildren"); err == nil && len(body) > 0 {
		raw, ok := body[0].([][]any)
		if !ok {
			return node
		}
		for _, tup := range raw {
			if len(tup) < 2 {
				continue
			}
			childBus := getString(tup[0])
			childPath := getPath(tup[1])
			if childBus == "" {
				childBus = bus
			}
			if childPath == "" || childPath == "@" {
				continue
			}
			child := walkA11yTree(conn, childBus, childPath, depth+1, maxDepth, nextID)
			node.Children = append(node.Children, child)
		}
	}

	return node
}

func findNodeByID(node *A11yNode, id int) *A11yNode {
	if node.ID == id {
		return node
	}
	for i := range node.Children {
		if found := findNodeByID(&node.Children[i], id); found != nil {
			return found
		}
	}
	return nil
}

func flattenTree(node A11yNode, showAll bool, roles []string, winMon map[int]int, parentID int, parentMon int, depth int) []FlatNode {
	var result []FlatNode
	mon := parentMon
	if (node.Role == "window" || node.Role == "frame") && winMon != nil {
		if m, ok := winMon[node.ID]; ok {
			mon = m
		}
	}
	if (showAll || isUseful(node, depth)) && (len(roles) == 0 || matchesAnyRole(node.Role, roles)) {
		result = append(result, FlatNode{
			ID: node.ID, Role: node.Role, Name: node.Name,
			Bounds: node.Bounds, Parent: parentID, Monitor: mon,
		})
	}
	for _, child := range node.Children {
		result = append(result, flattenTree(child, showAll, roles, winMon, node.ID, mon, depth+1)...)
	}
	return result
}

// monitorByGeometry assigns monitor IDs to AT-SPI window/frame nodes using
// geometry-based matching: height heuristic first, then IoU fallback.
func monitorByGeometry(ctx context.Context, root *A11yNode) map[int]int {
	mons := hyprctlMonitors(ctx)
	if mons == nil {
		return nil
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}



func formatListText(nodes []FlatNode) string {
	var b strings.Builder
	if len(nodes) == 0 {
		b.WriteString("nodes[0]:\n")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("nodes[%d]{id,role,name,x,y,w,h,parent,mon}:\n", len(nodes)))
	for _, n := range nodes {
		name := n.Name
		if name == "" {
			name = "-"
		}
		// Escape commas and quotes in name for clean CSV
		name = strings.ReplaceAll(name, ",", "\\,")
		name = strings.ReplaceAll(name, "\"", "\\\"")
		b.WriteString(fmt.Sprintf("  %d,%s,%s,%d,%d,%d,%d,%d,%d\n",
			n.ID, n.Role, name, n.Bounds[0], n.Bounds[1], n.Bounds[2], n.Bounds[3], n.Parent, n.Monitor))
	}
	return b.String()
}

func formatNodeText(root A11yNode, id int, depth int) string {
	if root.ID == id {
		return formatNodeRecursive(root, depth)
	}
	for _, c := range root.Children {
		if s := formatNodeText(c, id, depth); s != "" {
			return s
		}
	}
	return ""
}

func formatNodeRecursive(node A11yNode, depth int) string {
	indent := strings.Repeat("  ", depth)
	name := node.Name
	if name == "" {
		name = "-"
	}
	en := ""
	if node.Enabled { en = " enabled" }
	fc := ""
	if node.Focused { fc = " focused" }
	line := fmt.Sprintf("%s%s \"%s\" [%d,%d,%d,%d]%s%s",
		indent, node.Role, name,
		node.Bounds[0], node.Bounds[1], node.Bounds[2], node.Bounds[3],
		en, fc)

	var b strings.Builder
	b.WriteString(line)
	b.WriteString("\n")
	for _, c := range node.Children {
		b.WriteString(formatNodeRecursive(c, depth+1))
	}
	return b.String()
}

func getCharExtentsText(name string, obj dbus.BusObject) string {
	runes := []rune(name)
	if len(runes) == 0 {
		return ""
	}
	var b strings.Builder
	gotAny := false
	for i := range runes {
		body, err := a11yCall(obj, "org.a11y.atspi.Text.GetCharacterExtents", int32(i), uint32(0))
		if err != nil || len(body) < 4 {
			continue
		}
		x, ok1 := body[0].(int32)
		y, ok2 := body[1].(int32)
		w, ok3 := body[2].(int32)
		h, ok4 := body[3].(int32)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			continue
		}
		if !gotAny {
			b.WriteString("  chars: ")
			gotAny = true
		} else {
			b.WriteString(";")
		}
		fmt.Fprintf(&b, "%d,%d,%d,%d", x, y, w, h)
	}
	if gotAny {
		b.WriteString("\n")
	}
	return b.String()
}
