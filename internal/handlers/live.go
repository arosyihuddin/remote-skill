package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/coder/websocket"
	"github.com/creack/pty"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

// LiveSession represents an active PTY session.
type LiveSession struct {
	ID         string
	PTY        *os.File
	InputChan  chan []byte
	ResizeChan chan proto.LiveResizeRequest
	Done       chan struct{}
	Cleanup    func()
}

var (
	liveSessions = map[string]*LiveSession{}
	liveMu       sync.RWMutex
)

// GetLiveSession returns the active session by ID.
func GetLiveSession(id string) (*LiveSession, bool) {
	liveMu.RLock()
	defer liveMu.RUnlock()
	s, ok := liveSessions[id]
	return s, ok
}

// RemoveLiveSession removes a session.
func RemoveLiveSession(id string) {
	liveMu.Lock()
	defer liveMu.Unlock()
	delete(liveSessions, id)
}

// RangeLiveSession iterates over all active sessions. The callback returns false to stop.
func RangeLiveSession(fn func(id string, s *LiveSession) bool) {
	liveMu.RLock()
	defer liveMu.RUnlock()
	for id, s := range liveSessions {
		if !fn(id, s) {
			break
		}
	}
}

// Live starts an interactive PTY session.
func Live(ctx context.Context, payload json.RawMessage, sw handler.StreamWriter) (any, error) {
	var req proto.LiveRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("bad payload: %w", err)
	}

	if req.Cols == 0 {
		req.Cols = 80
	}
	if req.Rows == 0 {
		req.Rows = 24
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.CommandContext(ctx, shell, "-l")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: req.Cols,
		Rows: req.Rows,
	})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	session := &LiveSession{
		ID:         sw.Id,
		PTY:        ptmx,
		InputChan:  make(chan []byte, 256),
		ResizeChan: make(chan proto.LiveResizeRequest, 64),
		Done:       make(chan struct{}),
	}

	liveMu.Lock()
	liveSessions[sw.Id] = session
	liveMu.Unlock()

	session.Cleanup = func() {
		liveMu.Lock()
		delete(liveSessions, sw.Id)
		liveMu.Unlock()
		ptmx.Close()
		close(session.Done)
	}

	// Stream PTY output to client as binary frames
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				frame := make([]byte, 1+n)
				frame[0] = proto.BinaryFrameStdout
				copy(frame[1:], buf[:n])
				sw.Mu.Lock()
				_ = sw.Conn.Write(context.Background(), websocket.MessageBinary, frame)
				sw.Mu.Unlock()
			}
			if err != nil {
				if err != io.EOF {
					msg := fmt.Sprintf("\r\n[pty error: %v]\r\n", err)
					frame := make([]byte, 1+len(msg))
					frame[0] = proto.BinaryFrameStdout
					copy(frame[1:], msg)
					sw.Mu.Lock()
					_ = sw.Conn.Write(context.Background(), websocket.MessageBinary, frame)
					sw.Mu.Unlock()
				}
				return
			}
		}
	}()

	// Read input from channel and write to PTY
	go func() {
		for {
			select {
			case data, ok := <-session.InputChan:
				if !ok {
					return
				}
				if len(data) > 0 && data[0] == proto.BinaryFrameStdin {
					data = data[1:]
				}
				if _, err := ptmx.Write(data); err != nil {
					return
				}
			case resize, ok := <-session.ResizeChan:
				if !ok {
					return
				}
				pty.Setsize(ptmx, &pty.Winsize{
					Cols: resize.Cols,
					Rows: resize.Rows,
				})
			case <-session.Done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for process to exit
	err = cmd.Wait()
	session.Cleanup()

	return proto.ExecResult{
		ExitCode: exitCode(err),
	}, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}
