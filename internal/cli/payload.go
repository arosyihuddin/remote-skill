package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pstar7/remote-skill/internal/proto"
)

func buildPayload(cmd string, args []string) (any, error) {
	switch cmd {
	case "exec":
		return buildExecPayload(args)
	case "read":
		return buildReadPayload(args)
	case "write":
		return buildWritePayload(args)
	case "ls":
		return buildLsPayload(args)
	case "screenshot":
		return buildScreenshotPayload(args)
	case "click":
		return buildClickPayload(args)
	case "type":
		return buildTypePayload(args)
	case "key":
		return buildKeyPayload(args)
	case "mouse":
		return buildMousePayload(args)
	case "scroll":
		return buildScrollPayload(args)
	case "windows":
		return struct{}{}, nil
	case "a11y":
		return buildA11yPayload(args)
	case "monitors":
		return struct{}{}, nil
	case "cursorpos":
		return struct{}{}, nil
	case "drag":
		return buildDragPayload(args)
	case "board":
		return buildBoardPayload(args)
	case "apps":
		return buildAppsPayload(args)
	case "open":
		return buildOpenPayload(args)
	case "clip", "clip-get", "clip-set":
		return buildClipPayload(args)
	default:
		return nil, nil
	}
}

func buildExecPayload(args []string) (any, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("usage: rsk exec \"<cmd>\" [--shell] [--cwd PATH] [--timeout N] [--stream]")
	}
	req := &proto.ExecRequest{
		Cmd: strings.Fields(args[0]),
	}
	i := 1
	for i < len(args) {
		switch args[i] {
		case "--shell":
			req.Shell = true
			i++
		case "--cwd":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--cwd requires a path")
			}
			req.Cwd = args[i+1]
			i += 2
		case "--timeout":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--timeout requires seconds")
			}
			n, _ := strconv.Atoi(args[i+1])
			req.Timeout = n
			i += 2
		case "--stream":
			req.Stream = true
			i++
		default:
			req.Cmd = append(req.Cmd, args[i])
			i++
		}
	}
	return req, nil
}

func buildReadPayload(args []string) (any, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("usage: rsk read <path> [--binary] [--max N]")
	}
	req := &proto.ReadFileRequest{Path: args[0]}
	i := 1
	for i < len(args) {
		switch args[i] {
		case "--binary":
			req.Binary = true
			i++
		case "--max":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--max requires bytes")
			}
			n, _ := strconv.ParseInt(args[i+1], 10, 64)
			req.MaxBytes = n
			i += 2
		default:
			return nil, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	return req, nil
}

func buildWritePayload(args []string) (any, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("usage: rsk write <path> [--file LOCAL] [--append]")
	}
	path := args[0]
	req := &proto.WriteFileRequest{Path: path, MkdirP: true}
	i := 1
	for i < len(args) {
		switch args[i] {
		case "--file":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--file requires a path")
			}
			data, err := os.ReadFile(args[i+1])
			if err != nil {
				return nil, err
			}
			req.Content = string(data)
			i += 2
		case "--append":
			req.Append = true
			i++
		default:
			return nil, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	if req.Content == "" {
		// read from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, _ := os.ReadFile("/dev/stdin")
			req.Content = string(data)
		}
	}
	return req, nil
}

func buildLsPayload(args []string) (any, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("usage: rsk ls <path> [--hidden]")
	}
	req := &proto.ListDirRequest{Path: args[0]}
	for _, a := range args[1:] {
		if a == "--hidden" {
			req.Hidden = true
		}
	}
	return req, nil
}

func buildScreenshotPayload(args []string) (any, error) {
	req := &proto.ScreenshotRequest{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--region":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--region requires geometry")
			}
			req.Region = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--output requires name")
			}
			req.Output = args[i+1]
			i++
		default:
			return nil, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	return req, nil
}

func buildClickPayload(args []string) (any, error) {
	req := &proto.ClickRequest{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--x":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--x requires number")
			}
			n, _ := strconv.Atoi(args[i+1])
			req.X = &n
			i++
		case "--y":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--y requires number")
			}
			n, _ := strconv.Atoi(args[i+1])
			req.Y = &n
			i++
		case "--button":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--button requires left|right|middle")
			}
			req.Button = args[i+1]
			i++
		case "--double":
			req.Double = true
		default:
			return nil, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	return req, nil
}

func buildTypePayload(args []string) (any, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("usage: rsk type \"<text>\"")
	}
	return &proto.TypeRequest{Text: args[0]}, nil
}

func buildKeyPayload(args []string) (any, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("usage: rsk key \"<combo>\"")
	}
	return &proto.KeyRequest{Combo: args[0]}, nil
}

func buildMousePayload(args []string) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("usage: rsk mouse <x> <y> [--relative]")
	}
	x, _ := strconv.Atoi(args[0])
	y, _ := strconv.Atoi(args[1])
	req := &proto.MouseMoveRequest{X: x, Y: y}
	for _, a := range args[2:] {
		if a == "--relative" {
			req.Relative = true
		}
	}
	return req, nil
}

func buildScrollPayload(args []string) (any, error) {
	req := proto.ScrollRequest{DY: -3}
	for _, a := range args {
		if a == "--up" {
			req.DY = -absInt(req.DY)
		}
	}
	if len(args) > 0 && args[0] == "--down" {
		req.DY = 3
	}
	for i, a := range args {
		if a == "--dy" && i+1 < len(args) {
			n, _ := strconv.Atoi(args[i+1])
			req.DY = n
		}
	}
	return req, nil
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func buildClipPayload(args []string) (any, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: rsk clip get|set [text]")
	}
	switch args[0] {
	case "get":
		return &proto.ClipboardReadRequest{}, nil
	case "set":
		if len(args) < 2 {
			return nil, fmt.Errorf("usage: rsk clip set \"<text>\"")
		}
		return &proto.ClipboardWriteRequest{Content: args[1]}, nil
	default:
		return nil, fmt.Errorf("usage: rsk clip get|set")
	}
}

func buildDragPayload(args []string) (any, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("usage: rsk drag <x1> <y1> <x2> <y2> [--button left|right|middle]")
	}
	x1, _ := strconv.Atoi(args[0])
	y1, _ := strconv.Atoi(args[1])
	x2, _ := strconv.Atoi(args[2])
	y2, _ := strconv.Atoi(args[3])
	req := &proto.DragRequest{X1: x1, Y1: y1, X2: x2, Y2: y2}
	for _, a := range args[4:] {
		if a == "--button" {
			continue
		}
		switch a {
		case "left", "right", "middle":
			req.Button = a
		}
	}
	return req, nil
}

func buildA11yPayload(args []string) (any, error) {
	defaultMon := 0
	req := &proto.A11yRequest{Monitor: &defaultMon}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all":
			req.Monitor = nil
		case "--id":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--id requires a number")
			}
			n, _ := strconv.Atoi(args[i+1])
			req.ID = &n
			i++
		case "--depth":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--depth requires a number")
			}
			n, _ := strconv.Atoi(args[i+1])
			req.Depth = n
			i++
		case "--show-all":
			req.ShowAll = true
		case "--role":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--role requires a role name")
			}
			req.Roles = strings.Split(args[i+1], ",")
			i++
		case "--monitor":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--monitor requires a number")
			}
			n, _ := strconv.Atoi(args[i+1])
			req.Monitor = &n
			i++
		default:
			return nil, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	return req, nil
}

func buildAppsPayload(args []string) (any, error) {
	req := &proto.AppListRequest{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--filter" && i+1 < len(args) {
			req.Filter = args[i+1]
			i++
		}
	}
	return req, nil
}

func buildOpenPayload(args []string) (any, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("usage: rsk open \"<app-name>\"")
	}
	return &proto.AppLaunchRequest{Name: args[0], Args: args[1:]}, nil
}

func buildBoardPayload(args []string) (any, error) {
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("usage: rsk board \"<text>\"")
	}
	return &proto.BoardRequest{Text: args[0]}, nil
}
