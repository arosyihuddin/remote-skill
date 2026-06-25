package handlers

import (
	"log"
	"strings"
)

type winRect struct {
	bufX, bufY, bufW, bufH int
	frameX, frameY         int
	frameW, frameH         int
}

func fixAppWindowCoords(root *A11yNode) {
	wins := getWindowRects(root)
	log.Printf("a11y fix: %d window rects from extension", len(wins))
	for i, w := range wins {
		log.Printf("  win[%d]: buf=(%d,%d,%d,%d) frame=(%d,%d,%d,%d)", i, w.bufX, w.bufY, w.bufW, w.bufH, w.frameX, w.frameY, w.frameW, w.frameH)
	}
	if len(wins) == 0 {
		return
	}
	applyWindowOffset(root, wins)
}

func applyWindowOffset(node *A11yNode, wins []winRect) {
	var propagate func(n *A11yNode, offX, offY int)
	propagate = func(n *A11yNode, offX, offY int) {
		isShell := n.Role == "window" || n.Role == "desktop frame" ||
			(n.Role == "panel" && strings.HasPrefix(n.Name, "Wayland")) ||
			n.Role == "application"
		if isShell {
			for i := range n.Children {
				propagate(&n.Children[i], offX, offY)
			}
			return
		}

		n.Bounds[0] += offX
		n.Bounds[1] += offY

		fix := n.attemptWindowFix(wins)
		childOffX, childOffY := offX, offY
		if fix.applied {
			childOffX += fix.deltaX
			childOffY += fix.deltaY
		}

		for i := range n.Children {
			propagate(&n.Children[i], childOffX, childOffY)
		}
	}
	propagate(node, 0, 0)
}

type windowFix struct {
	applied  bool
	deltaX   int
	deltaY   int
}

func (node *A11yNode) attemptWindowFix(wins []winRect) windowFix {
	if len(wins) == 0 || node.Bounds[2] <= 0 || node.Bounds[3] <= 0 {
		return windowFix{}
	}

	nx, ny := node.Bounds[0], node.Bounds[1]

	for _, w := range wins {
		if nx >= w.frameX && nx < w.frameX+w.frameW && ny >= w.frameY && ny < w.frameY+w.frameH {
			return windowFix{}
		}
	}

	if nx == 0 && ny == 0 && node.Role != "frame" {
		return windowFix{}
	}

	bestIdx := -1
	bestDelta := int(^uint(0) >> 1)
	for i, w := range wins {
		ax, ay := w.bufX+nx, w.bufY+ny
		if ax >= w.frameX && ax < w.frameX+w.frameW && ay >= w.frameY && ay < w.frameY+w.frameH {
			dw := w.frameW - node.Bounds[2]
			dh := w.frameH - node.Bounds[3]
			delta := dw*dw + dh*dh
			if delta < bestDelta {
				bestDelta = delta
				bestIdx = i
			}
		}
	}

	if bestIdx < 0 {
		return windowFix{}
	}

	nodeArea := node.Bounds[2] * node.Bounds[3]
	if bestDelta > nodeArea {
		log.Printf("a11y fix: skip node %q role=%s atspi=(%d,%d) size=%dx%d [bestDelta=%d > nodeArea=%d]",
			node.Name, node.Role, nx, ny, node.Bounds[2], node.Bounds[3], bestDelta, nodeArea)
		return windowFix{}
	}

	w := wins[bestIdx]
	newX := w.bufX + nx
	newY := w.bufY + ny
	log.Printf("a11y fix: node %q role=%s atspi=(%d,%d) -> abs=(%d,%d) [buf=(%d,%d) frame=(%d,%d,%d,%d)]",
		node.Name, node.Role, nx, ny, newX, newY, w.bufX, w.bufY, w.frameX, w.frameY, w.frameW, w.frameH)
	node.Bounds[0] = newX
	node.Bounds[1] = newY
	return windowFix{applied: true, deltaX: newX - nx, deltaY: newY - ny}
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
