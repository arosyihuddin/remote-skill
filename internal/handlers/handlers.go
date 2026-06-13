// Package handlers implements the per-request handlers for the node.
package handlers

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

// Exec runs a command. Streams stdout/stderr if requested, else returns full
// output (capped). Honors timeout and optional shell wrapping.
func Exec(ctx context.Context, payload json.RawMessage, sw handler.StreamWriter) (any, error) {
	var req proto.ExecRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if len(req.Cmd) == 0 {
		return nil, fmt.Errorf("empty cmd")
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Timeout)*time.Second)
		defer cancel()
	}

	var c *exec.Cmd
	if req.Shell {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		joined := strings.Join(req.Cmd, " ")
		c = exec.CommandContext(ctx, shell, "-c", joined)
	} else {
		c = exec.CommandContext(ctx, req.Cmd[0], req.Cmd[1:]...)
	}
	if runtime.GOOS == "linux" {
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if req.Cwd != "" {
		c.Dir = expandPath(req.Cwd)
	}
	if len(req.Env) > 0 {
		env := os.Environ()
		for k, v := range req.Env {
			env = append(env, k+"="+v)
		}
		c.Env = env
	}
	if req.Stdin != "" {
		c.Stdin = strings.NewReader(req.Stdin)
	}

	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}

	const maxCap = 256 * 1024 // tail buffer if not streaming
	stdoutBuf := newRingBuf(maxCap)
	stderrBuf := newRingBuf(maxCap)

	done := make(chan struct{}, 2)
	go pumpStream(stdoutPipe, "stdout", req.Stream, sw, stdoutBuf, done)
	go pumpStream(stderrPipe, "stderr", req.Stream, sw, stderrBuf, done)
	<-done
	<-done

	exitCode := 0
	timedOut := false
	if err := c.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			timedOut = true
			if runtime.GOOS == "linux" && c.Process != nil {
				_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
			}
		}
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return proto.ExecResult{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.string(),
		Stderr:   stderrBuf.string(),
		TimedOut: timedOut,
	}, nil
}

func pumpStream(r io.Reader, channel string, stream bool, sw handler.StreamWriter, buf *ringBuf, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	br := bufio.NewReaderSize(r, 4096)
	chunk := make([]byte, 4096)
	for {
		n, err := br.Read(chunk)
		if n > 0 {
			buf.write(chunk[:n])
			if stream {
				sw.Send(channel, string(chunk[:n]))
			}
		}
		if err != nil {
			return
		}
	}
}

// ringBuf keeps the last N bytes of a stream.
type ringBuf struct {
	cap  int
	data []byte
}

func newRingBuf(cap int) *ringBuf { return &ringBuf{cap: cap} }

func (b *ringBuf) write(p []byte) {
	b.data = append(b.data, p...)
	if len(b.data) > b.cap {
		b.data = b.data[len(b.data)-b.cap:]
	}
}

func (b *ringBuf) string() string {
	if !utf8.Valid(b.data) {
		return string(b.data) // best effort
	}
	return string(b.data)
}

// ReadFile reads a file from disk.
func ReadFile(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.ReadFileRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	path := expandPath(req.Path)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	size := info.Size()
	limit := req.MaxBytes
	truncated := false
	if limit > 0 && size > limit {
		size = limit
		truncated = true
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, size)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	buf = buf[:n]

	res := proto.ReadFileResult{SizeBytes: int64(n), Truncated: truncated}
	if req.Binary || !utf8.Valid(buf) {
		res.Content = base64.StdEncoding.EncodeToString(buf)
		res.Base64 = true
	} else {
		res.Content = string(buf)
	}
	return res, nil
}

// WriteFile writes a file (creates dirs, modes, optional append/binary).
func WriteFile(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.WriteFileRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	path := expandPath(req.Path)
	mode := os.FileMode(req.Mode)
	if mode == 0 {
		mode = 0644
	}
	if req.MkdirP {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
	}
	var data []byte
	if req.Base64 {
		var err error
		data, err = base64.StdEncoding.DecodeString(req.Content)
		if err != nil {
			return nil, err
		}
	} else {
		data = []byte(req.Content)
	}
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if req.Append {
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	f, err := os.OpenFile(path, flag, mode)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	n, err := f.Write(data)
	if err != nil {
		return nil, err
	}
	return proto.WriteFileResult{BytesWritten: int64(n)}, nil
}

// ListDir lists directory contents.
func ListDir(ctx context.Context, payload json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.ListDirRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	path := expandPath(req.Path)
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := make([]proto.DirEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !req.Hidden && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, proto.DirEntry{
			Name:    name,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}
	return proto.ListDirResult{Entries: out}, nil
}

// expandPath expands a leading "~" to the user home directory.
func expandPath(p string) string {
	if p == "" {
		return p
	}
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
	}
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}
