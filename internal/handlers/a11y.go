package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

type A11yNode struct {
	ID       int             `json:"id,omitempty"`
	Name     string          `json:"name"`
	Role     string          `json:"role"`
	Bounds   [4]int          `json:"bounds"`
	Enabled  bool            `json:"enabled"`
	Focused  bool            `json:"focused"`
	Children []A11yNode      `json:"children,omitempty"`
	depth    int             `json:"-"`
	bus      string          `json:"-"`
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
		depth = 12
	}

	nextID := 1
	done := make(chan A11yNode, 1)
	go func() {
		done <- walkA11yTree(a11y, "org.a11y.atspi.Registry", dbus.ObjectPath("/org/a11y/atspi/accessible/root"), 0, depth, &nextID, false)
	}()

	var root A11yNode
	select {
	case root = <-done:
	case <-time.After(12 * time.Second):
		return nil, fmt.Errorf("a11y tree: timeout")
	}
	fixAppWindowCoords(&root)

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
