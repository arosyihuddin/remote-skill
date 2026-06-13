// Package audit writes per-request JSONL audit entries to a file.
//
// Designed to be safe for concurrent use, append-only, and zero-impact when
// disabled (nil *Logger). Errors writing to the audit file are not fatal —
// they're written to stderr so handlers keep working even if the disk is full.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger is the audit sink. nil is a no-op.
type Logger struct {
	path string

	mu sync.Mutex
	f  *os.File
}

// Open creates parent dirs if needed and opens the file in append mode.
// Returns nil, nil if path is empty (audit disabled).
func Open(path string) (*Logger, error) {
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	return &Logger{path: path, f: f}, nil
}

// Close flushes & closes the file.
func (l *Logger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// Entry is the JSONL record. Optional fields are omitted when zero.
type Entry struct {
	Timestamp  string         `json:"ts"`
	DeviceID   string         `json:"device,omitempty"`
	Type       string         `json:"type"`

	Cmd        []string       `json:"cmd,omitempty"`
	Cwd        string         `json:"cwd,omitempty"`
	Shell      bool           `json:"shell,omitempty"`
	ExitCode   *int           `json:"exit_code,omitempty"`
	DurationMs int64          `json:"duration_ms,omitempty"`
	Path       string         `json:"path,omitempty"`
	SizeBytes  int64          `json:"size_bytes,omitempty"`
	Action     string         `json:"action,omitempty"`
	X          int            `json:"x,omitempty"`
	Y          int            `json:"y,omitempty"`
	Button     string         `json:"button,omitempty"`
	Combo      string         `json:"combo,omitempty"`
	Text       string         `json:"text,omitempty"`
	Allowed    *bool          `json:"allowed,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// Log writes one JSONL line. Safe to call on nil *Logger.
func (l *Logger) Log(e Entry) {
	if l == nil || l.f == nil {
		return
	}
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	// truncate text fields to 1KB to bound log size
	e.Text = truncate(e.Text, 1024)
	e.Path = truncate(e.Path, 1024)
	for i, s := range e.Cmd {
		e.Cmd[i] = truncate(s, 1024)
	}

	b, err := json.Marshal(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit: marshal failed: %v\n", err)
		return
	}
	b = append(b, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.f.Write(b); err != nil {
		fmt.Fprintf(os.Stderr, "audit: write failed: %v\n", err)
	}
}

// Allowed/Denied are convenience pointer helpers (so omitempty works on bool).
func Allowed() *bool { v := true; return &v }
func Denied() *bool  { v := false; return &v }

// IntPtr is a helper for ExitCode.
func IntPtr(i int) *int { return &i }

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
