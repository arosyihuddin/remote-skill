package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/pstar7/remote-skill/internal/proto"
)

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
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "") }()

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
	case "apps":
		msgType = proto.TypeAppList
	case "open":
		msgType = proto.TypeAppLaunch
	case "clip", "clip-get":
		msgType = proto.TypeClipboardRead
	case "clip-set":
		msgType = proto.TypeClipboardWrite
	case "windows":
		msgType = proto.TypeWindows
	case "a11y":
		msgType = proto.TypeAccessibilityTree
	case "drag":
		msgType = proto.TypeDrag
	case "board":
		msgType = proto.TypeBoard
	case "monitors":
		msgType = proto.TypeMonitors
	case "cursorpos":
		msgType = proto.TypeCursorPos
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
				_ = json.Unmarshal(f.Payload, &sc)
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
		_ = json.Unmarshal(f.Payload, &ep)
		return nil, fmt.Errorf("agent error: %s: %s", ep.Code, ep.Message)
	}
	resp.Final = json.RawMessage(f.Payload)
	return resp, nil
}
