package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/pstar7/remote-skill/internal/config"
	"github.com/pstar7/remote-skill/internal/proto"
)

func resolveServerURL() string {
	if v := os.Getenv("RSK_SERVER"); v != "" {
		return v
	}
	return "ws://127.0.0.1:7777"
}

func resolveToken() string {
	if v := os.Getenv("RSK_TOKEN"); v != "" {
		return v
	}
	candidates := []string{
		os.Getenv("HOME") + "/.config/rsk/rsk.env",
		os.Getenv("HOME") + "/.config/rsk/config.env",
		"/etc/rsk/config.env",
	}
	for _, p := range candidates {
		m, err := config.LoadEnvFile(p)
		if err != nil {
			continue
		}
		if t, ok := m["TOKEN"]; ok && t != "" {
			return t
		}
	}
	return ""
}

var knownCommands = map[string]bool{
	"exec": true, "read": true, "write": true, "ls": true,
	"screenshot": true, "click": true, "type": true, "key": true,
	"mouse": true, "clip": true, "scroll": true, "devices": true,
	"-h": true, "--help": true, "help": true,
}

func Run(command string, args []string) {
	serverURL := resolveServerURL()
	token := resolveToken()
	device := os.Getenv("RSK_DEVICE")

	// First arg might be device-id instead of command
	if !knownCommands[command] {
		device = command
		if len(args) == 0 {
			printUsage("")
			os.Exit(2)
		}
		command = args[0]
		args = args[1:]
	}

	switch command {
	case "-h", "--help", "help":
		printUsage("")
		return
	case "devices":
		runDevices(serverURL, token)
		return
	}

	savePath := extractSaveFlag(command, &args)

	payload, err := buildPayload(command, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if payload == nil {
		printUsage(command)
		os.Exit(2)
	}

	streaming := false
	if command == "exec" {
		for _, a := range args {
			if a == "--stream" {
				streaming = true
			}
		}
	}

	subCmd := command
	if command == "clip" && len(args) > 0 && args[0] == "set" {
		subCmd = "clip-set"
	} else if command == "clip" {
		subCmd = "clip-get"
	}
	resp, err := sendRequest(serverURL, token, device, subCmd, payload, streaming)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if streaming {
		for _, chunk := range resp.Stream {
			fmt.Print(chunk)
		}
	}
	if resp.Final != nil {
		switch command {
		case "screenshot":
			printScreenshot(resp.Final.(json.RawMessage), savePath)
		case "read":
			printRead(resp.Final.(json.RawMessage), savePath)
		default:
			b, _ := json.MarshalIndent(resp.Final, "", "  ")
			fmt.Println(string(b))
		}
	}
}

type cliResponse struct {
	Stream []string
	Final  any
}

func sendRequest(serverURL, token, device, cmdType string, payload any, stream bool) (*cliResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	wsURL := serverURL
	if !strings.HasSuffix(wsURL, "/cli") {
		wsURL = strings.TrimRight(wsURL, "/") + "/cli"
	}
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: &http.Client{Timeout: 0},
	})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	c.SetReadLimit(64 << 20)

	hostname, _ := os.Hostname()
	hello := proto.Hello{
		Token:    token,
		DeviceID: device,
		Hostname: hostname,
		Role:     proto.RoleCLI,
	}
	plBytes, _ := json.Marshal(hello)
	helloFrame, _ := json.Marshal(proto.Frame{Type: proto.TypeHello, ID: "cli-hello", Payload: plBytes})
	if err := c.Write(ctx, websocket.MessageText, helloFrame); err != nil {
		return nil, fmt.Errorf("write hello: %w", err)
	}

	readCtx, cancelR := context.WithTimeout(ctx, 10*time.Second)
	_, ackBytes, err := c.Read(readCtx)
	cancelR()
	if err != nil {
		return nil, fmt.Errorf("read ack: %w", err)
	}
	var ackFrame proto.Frame
	if err := json.Unmarshal(ackBytes, &ackFrame); err != nil {
		return nil, fmt.Errorf("decode ack: %w", err)
	}
	if ackFrame.Type != proto.TypeAck {
		return nil, fmt.Errorf("expected ack, got %s", ackFrame.Type)
	}

	pl, _ := json.Marshal(payload)
	var msgType proto.MessageType
	switch cmdType {
	case "exec":
		msgType = proto.TypeExec
	case "read":
		msgType = proto.TypeReadFile
	case "write":
		msgType = proto.TypeWriteFile
	case "ls":
		msgType = proto.TypeListDir
	case "screenshot":
		msgType = proto.TypeScreenshot
	case "click":
		msgType = proto.TypeClick
	case "type":
		msgType = proto.TypeType
	case "key":
		msgType = proto.TypeKey
	case "mouse":
		msgType = proto.TypeMouse
	case "scroll":
		msgType = proto.TypeScroll
	case "clip", "clip-get":
		msgType = proto.TypeClipboardRead
	case "clip-set":
		msgType = proto.TypeClipboardWrite
	case "devices":
		msgType = proto.TypeDevices
	default:
		return nil, fmt.Errorf("unknown command: %s", cmdType)
	}
	reqFrame, _ := json.Marshal(proto.Frame{Type: msgType, ID: "req-1", Payload: pl})
	if err := c.Write(ctx, websocket.MessageText, reqFrame); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	resp := &cliResponse{}
	if stream {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return nil, fmt.Errorf("read stream: %w", err)
			}
			var f proto.Frame
			if err := json.Unmarshal(data, &f); err != nil {
				return nil, fmt.Errorf("bad frame: %w", err)
			}
			switch f.Type {
			case proto.TypeStream:
				var sc proto.StreamChunk
				json.Unmarshal(f.Payload, &sc)
				resp.Stream = append(resp.Stream, sc.Data)
			case proto.TypeResponse, proto.TypeError:
				resp.Final = json.RawMessage(f.Payload)
				return resp, nil
			}
		}
	}

	_, data, err := c.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	var f proto.Frame
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("bad frame: %w", err)
	}
	if f.Type == proto.TypeError {
		var ep proto.ErrorPayload
		json.Unmarshal(f.Payload, &ep)
		return nil, fmt.Errorf("agent error: %s: %s", ep.Code, ep.Message)
	}
	resp.Final = json.RawMessage(f.Payload)
	return resp, nil
}

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
	req := struct {
		DY int `json:"dy"`
	}{DY: -3}
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

func runDevices(serverURL, token string) {
	resp, err := sendRequest(serverURL, token, "", "devices", nil, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if resp.Final != nil {
		b, _ := json.MarshalIndent(resp.Final, "", "  ")
		fmt.Println(string(b))
	}
}

func extractSaveFlag(cmd string, args *[]string) string {
	if cmd != "screenshot" && cmd != "read" {
		return ""
	}
	filtered := make([]string, 0, len(*args))
	savePath := ""
	for i := 0; i < len(*args); i++ {
		if (*args)[i] == "--save" {
			if i+1 < len(*args) {
				savePath = (*args)[i+1]
				i++
			}
			continue
		}
		filtered = append(filtered, (*args)[i])
	}
	*args = filtered
	return savePath
}

func saveTemp(prefix string, data []byte) string {
	f, err := os.CreateTemp("", prefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating temp file: %v\n", err)
		os.Exit(1)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		fmt.Fprintf(os.Stderr, "error writing temp file: %v\n", err)
		os.Exit(1)
	}
	f.Close()
	return f.Name()
}

func printScreenshot(raw json.RawMessage, savePath string) {
	var r proto.ScreenshotResult
	if err := json.Unmarshal(raw, &r); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding screenshot: %v\n", err)
		os.Exit(1)
	}
	data, err := base64.StdEncoding.DecodeString(r.Base64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding base64: %v\n", err)
		os.Exit(1)
	}
	path := savePath
	if path == "" {
		path = saveTemp("rsk-screenshot-*.png", data)
	} else {
		if err := os.WriteFile(path, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error saving: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("%s (%dx%d)\n", path, r.Width, r.Height)
}

func printRead(raw json.RawMessage, savePath string) {
	var r proto.ReadFileResult
	if err := json.Unmarshal(raw, &r); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding read result: %v\n", err)
		os.Exit(1)
	}
	if !r.Base64 && savePath == "" {
		fmt.Println(r.Content)
		return
	}
	var data []byte
	if r.Base64 {
		var err error
		data, err = base64.StdEncoding.DecodeString(r.Content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error decoding base64: %v\n", err)
			os.Exit(1)
		}
	} else {
		data = []byte(r.Content)
	}
	path := savePath
	if path == "" {
		path = saveTemp("rsk-read-*.bin", data)
	} else {
		if err := os.WriteFile(path, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error saving: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("%s (%d bytes)\n", path, r.SizeBytes)
}

func printUsage(_ string) {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  rsk <command> [args...]\n")
	fmt.Fprintf(os.Stderr, "  rsk <device-id> <command> [args...]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  devices                 List connected devices\n")
	fmt.Fprintf(os.Stderr, "  exec \"<cmd>\"            Run a command on remote node\n")
	fmt.Fprintf(os.Stderr, "  read <path>             Read a file\n")
	fmt.Fprintf(os.Stderr, "  write <path>            Write a file (stdin or --file)\n")
	fmt.Fprintf(os.Stderr, "  ls <path>               List directory\n")
	fmt.Fprintf(os.Stderr, "  screenshot              Capture screenshot\n")
	fmt.Fprintf(os.Stderr, "  click                   Mouse click\n")
	fmt.Fprintf(os.Stderr, "  type \"<text>\"           Type text\n")
	fmt.Fprintf(os.Stderr, "  key \"<combo>\"           Send key combo\n")
	fmt.Fprintf(os.Stderr, "  mouse <x> <y>           Move mouse\n")
	fmt.Fprintf(os.Stderr, "  scroll [--dy N]         Scroll (default -3)\n")
	fmt.Fprintf(os.Stderr, "  clip get|set            Clipboard operations\n")
}
