package handlers

import (
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

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

func walkA11yTree(conn *dbus.Conn, bus string, path dbus.ObjectPath, depth, maxDepth int, nextID *int, insideApp bool) A11yNode {
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
	// But keep all nodes inside app windows for coordinate fix
	childInsideApp := insideApp
	if node.Role == "panel" && node.Name == "Wayland window" && node.Bounds[2] > 0 && node.Bounds[3] > 0 {
		childInsideApp = true
	}

	// Performance: don't descend into empty non-interactive nodes with no valid bounds.
	// These contribute nothing to the output or the window-offset heuristic.
	if !childInsideApp && !validBounds(node.Bounds) && !isInteractive(node.Role) && node.Name == "" && depth > 1 {
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
			child := walkA11yTree(conn, childBus, childPath, depth+1, maxDepth, nextID, childInsideApp)
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
