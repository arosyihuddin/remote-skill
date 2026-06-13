//go:build !linux

package handlers

import "fmt"

var errNoInput = fmt.Errorf("input automation not supported on this platform (build with linux)")

func platformMouseClick(btn string, double bool) error { return errNoInput }

func platformMoveMouse(x, y int) error { return errNoInput }

func platformMoveMouseRel(x, y int) error { return errNoInput }

func platformMouseScroll(dy int) error { return errNoInput }

func platformTypeText(text string) error { return errNoInput }

func platformKeyCombo(parts []string) error { return errNoInput }
