package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/pstar7/remote-skill/internal/handler"
)

type A11yNode struct {
	Name     string     `json:"name"`
	Role     string     `json:"role"`
	Bounds   [4]int     `json:"bounds"`
	Enabled  bool       `json:"enabled"`
	Focused  bool       `json:"focused"`
	Children []A11yNode `json:"children,omitempty"`

	depth int `json:"-"`
}

const sessionBusName = "org.a11y.Bus"
const sessionBusPath = "/org/a11y/bus"

func AccessibilityTree(ctx context.Context, _ json.RawMessage, _ handler.StreamWriter) (any, error) {
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
		a11y.Close()
		return nil, fmt.Errorf("a11y auth: %w", err)
	}
	defer a11y.Close()

	if err := a11y.Hello(); err != nil {
		a11y.Close()
		return nil, fmt.Errorf("a11y hello: %w", err)
	}

	done := make(chan A11yNode, 1)
	go func() {
		done <- walkA11yTree(a11y, "org.a11y.atspi.Registry", dbus.ObjectPath("/org/a11y/atspi/accessible/root"), 0, 3)
	}()

	select {
	case root := <-done:
		return root, nil
	case <-time.After(8 * time.Second):
		return nil, fmt.Errorf("a11y tree: timeout")
	}
}

func a11yCall(obj dbus.BusObject, method string, args ...interface{}) ([]interface{}, error) {
	call := obj.Call(method, 0, args...)
	if call.Err != nil {
		return nil, call.Err
	}
	return call.Body, nil
}

func getString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func getPath(v interface{}) dbus.ObjectPath {
	switch p := v.(type) {
	case dbus.ObjectPath:
		return p
	case string:
		return dbus.ObjectPath(p)
	}
	return ""
}

func walkA11yTree(conn *dbus.Conn, bus string, path dbus.ObjectPath, depth, maxDepth int) A11yNode {
	node := A11yNode{depth: depth}

	if depth > maxDepth {
		node.Role = "..."
		return node
	}

	obj := conn.Object(bus, path)

	// Role
	if body, err := a11yCall(obj, "org.a11y.atspi.Accessible.GetRoleName"); err == nil && len(body) > 0 {
		node.Role = getString(body[0])
	}

	// Name via property
	if v, err := obj.GetProperty("org.a11y.atspi.Accessible.Name"); err == nil {
		node.Name = getString(v.Value())
	}

	// State
	if body, err := a11yCall(obj, "org.a11y.atspi.Accessible.GetState"); err == nil && len(body) > 0 {
		if states, ok := body[0].([]uint32); ok {
			for _, s := range states {
				if s == 1<<1 {
					node.Enabled = true
				}
				if s == 1<<4 {
					node.Focused = true
				}
			}
		}
	}

	// Bounds — GetExtents returns (x,y,w,h) as a single struct
	if body, err := a11yCall(obj, "org.a11y.atspi.Component.GetExtents", uint32(0)); err == nil && len(body) >= 1 {
		if tup, ok := body[0].([]interface{}); ok && len(tup) >= 4 {
			if x, ok := tup[0].(int32); ok { node.Bounds[0] = int(x) }
			if y, ok := tup[1].(int32); ok { node.Bounds[1] = int(y) }
			if w, ok := tup[2].(int32); ok { node.Bounds[2] = int(w) }
			if h, ok := tup[3].(int32); ok { node.Bounds[3] = int(h) }
		}
	}

	// Children
	if body, err := a11yCall(obj, "org.a11y.atspi.Accessible.GetChildren"); err == nil && len(body) > 0 {
		raw, ok := body[0].([][]interface{})
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
			child := walkA11yTree(conn, childBus, childPath, depth+1, maxDepth)
			node.Children = append(node.Children, child)
		}
	}

	return node
}
