package cli

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/term"

	"github.com/pstar7/remote-skill/internal/proto"
)

// runLive opens an interactive terminal session on a remote device.
func runLive(serverURL, token, device string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wsURL := serverURL
	if !strings.HasSuffix(wsURL, "/live") {
		wsURL = strings.TrimRight(wsURL, "/") + "/live"
	}

	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: &http.Client{Timeout: 0},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
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
	helloFrame, _ := json.Marshal(proto.Frame{Type: proto.TypeHello, ID: "live-hello", Payload: plBytes})
	if err := c.Write(ctx, websocket.MessageText, helloFrame); err != nil {
		fmt.Fprintf(os.Stderr, "write hello: %v\n", err)
		os.Exit(1)
	}

	readCtx, cancelR := context.WithTimeout(ctx, 10*time.Second)
	_, ackBytes, err := c.Read(readCtx)
	cancelR()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read ack: %v\n", err)
		os.Exit(1)
	}
	var ackFrame proto.Frame
	if err := json.Unmarshal(ackBytes, &ackFrame); err != nil || ackFrame.Type != proto.TypeAck {
		fmt.Fprintf(os.Stderr, "expected ack, got: %v\n", err)
		os.Exit(1)
	}

	fd := int(os.Stdin.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 24
	}

	liveReq := proto.LiveRequest{Cols: uint16(width), Rows: uint16(height)}
	reqPL, _ := json.Marshal(liveReq)
	reqFrame, _ := json.Marshal(proto.Frame{Type: proto.TypeLive, ID: "live-req", Payload: reqPL})
	if err := c.Write(ctx, websocket.MessageText, reqFrame); err != nil {
		fmt.Fprintf(os.Stderr, "write live request: %v\n", err)
		os.Exit(1)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "raw mode: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigCh
		term.Restore(fd, oldState)
		cancel()
	}()

	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-resizeCh:
				w, h, err := term.GetSize(fd)
				if err != nil {
					continue
				}
				frame := make([]byte, 5)
				frame[0] = proto.BinaryFrameResize
				binary.BigEndian.PutUint16(frame[1:3], uint16(w))
				binary.BigEndian.PutUint16(frame[3:5], uint16(h))
				_ = c.Write(ctx, websocket.MessageBinary, frame)
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			msgType, data, err := c.Read(ctx)
			if err != nil {
				cancel()
				return
			}
			if msgType == websocket.MessageBinary {
				if len(data) < 1 {
					continue
				}
				switch data[0] {
				case proto.BinaryFrameStdout:
					_, _ = os.Stdout.Write(data[1:])
				case proto.BinaryFrameExit:
					if len(data) > 1 {
						fmt.Fprintf(os.Stderr, "\r\n[session ended: %s]\r\n", string(data[1:]))
					}
					cancel()
					return
				}
			}
		}
	}()

	inputCh := make(chan []byte, 256)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				continue
			}
			data := make([]byte, 1+n)
			data[0] = proto.BinaryFrameStdin
			copy(data[1:], buf[:n])
			inputCh <- data
		}
	}()

	for {
		select {
		case <-ctx.Done():
			term.Restore(fd, oldState)
			return
		case data := <-inputCh:
			if err := c.Write(ctx, websocket.MessageBinary, data); err != nil {
				term.Restore(fd, oldState)
				return
			}
		}
	}
}
