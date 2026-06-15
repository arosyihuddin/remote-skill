// Package proto defines the wire format between agent (laptop) and server (VPS).
//
// Frames are JSON over WebSocket. Every request has an id; every response
// carries the same id. Streaming responses use the same id and set
// Stream=true with chunk index, with the final chunk setting Done=true.
package proto

import "encoding/json"

// MessageType discriminates the payload.
type MessageType string

const (
	RoleNode = "node"
	RoleCLI  = "cli"
)

const (
	// Control
	TypeHello MessageType = "hello"   // agent -> server, first frame after connect
	TypeAck   MessageType = "ack"     // server -> agent, acknowledges hello
	TypeError MessageType = "error"   // either direction
	TypePing  MessageType = "ping"
	TypePong  MessageType = "pong"

	// Requests (server -> agent)
	TypeExec             MessageType = "exec"
	TypeReadFile         MessageType = "read_file"
	TypeWriteFile        MessageType = "write_file"
	TypeListDir          MessageType = "list_dir"
	TypeScreenshot       MessageType = "screenshot"
	TypeClick            MessageType = "click"
	TypeType             MessageType = "type"
	TypeKey              MessageType = "key"
	TypeMouse            MessageType = "mouse"
	TypeScroll           MessageType = "scroll"
	TypeClipboardRead    MessageType = "clipboard_read"
	TypeClipboardWrite   MessageType = "clipboard_write"
	TypeWindows          MessageType = "windows"
	TypeAccessibilityTree MessageType = "accessibility_tree"
	TypeDevices          MessageType = "devices"      // CLI internal: list connected devices
	TypeDrag             MessageType = "drag"         // mouse drag
	TypeBoard            MessageType = "board"        // clipboard write + paste
	TypeMonitors         MessageType = "monitors"     // monitor layout
	TypeCursorPos        MessageType = "cursorpos"    // current cursor position

	// Responses (agent -> server)
	TypeResponse MessageType = "response"
	TypeStream   MessageType = "stream" // streaming chunk for exec stdout/stderr
)

// Frame is the envelope for every WS message.
type Frame struct {
	Type    MessageType     `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Hello is the first frame from node or CLI client.
type Hello struct {
	Token    string `json:"token"`
	DeviceID string `json:"device_id"`  // stable name like "laptop-pstar7"
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Role     string `json:"role"`       // "node" or "cli"
}

// Ack is server's response to Hello.
type Ack struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// ErrorPayload is sent on protocol or runtime errors.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ExecRequest runs a shell command on the node.
// If Stream is true, stdout/stderr are sent as TypeStream frames as they arrive.
// The final frame is TypeResponse with ExecResult (ExitCode + truncated tails).
type ExecRequest struct {
	Cmd     []string          `json:"cmd"`
	Cwd     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Stdin   string            `json:"stdin,omitempty"`
	Timeout int               `json:"timeout_sec,omitempty"` // 0 = no timeout
	Shell   bool              `json:"shell,omitempty"`        // run via $SHELL -c
	Stream  bool              `json:"stream,omitempty"`
}

// StreamChunk is sent during streaming exec.
type StreamChunk struct {
	Channel string `json:"channel"` // "stdout" | "stderr"
	Data    string `json:"data"`
	Done    bool   `json:"done,omitempty"`
}

// ExecResult is the final response of an exec.
type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"` // full output if !Stream, else last tail
	Stderr   string `json:"stderr,omitempty"`
	TimedOut bool   `json:"timed_out,omitempty"`
}

// ReadFileRequest reads a file. Binary content is base64-encoded if Binary=true.
type ReadFileRequest struct {
	Path     string `json:"path"`
	Binary   bool   `json:"binary,omitempty"`
	MaxBytes int64  `json:"max_bytes,omitempty"` // 0 = no limit
}

type ReadFileResult struct {
	Content   string `json:"content"`              // utf8 or base64
	Base64    bool   `json:"base64,omitempty"`
	SizeBytes int64  `json:"size_bytes"`
	Truncated bool   `json:"truncated,omitempty"`
}

// WriteFileRequest writes a file. Content is base64 if Base64=true.
type WriteFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Base64  bool   `json:"base64,omitempty"`
	Mode    uint32 `json:"mode,omitempty"`   // 0 -> 0644
	Append  bool   `json:"append,omitempty"`
	MkdirP  bool   `json:"mkdir_p,omitempty"`
}

type WriteFileResult struct {
	BytesWritten int64 `json:"bytes_written"`
}

// ListDirRequest lists directory entries.
type ListDirRequest struct {
	Path     string `json:"path"`
	Hidden   bool   `json:"hidden,omitempty"`
}

type DirEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Mode    string `json:"mode,omitempty"`
	ModTime string `json:"mod_time,omitempty"`
}

type ListDirResult struct {
	Entries []DirEntry `json:"entries"`
}

// ScreenshotRequest captures the screen via grim (Wayland).
type ScreenshotRequest struct {
	Region string `json:"region,omitempty"` // grim geometry "x,y wxh", empty = full
	Output string `json:"output,omitempty"` // specific output name
}

type ScreenshotResult struct {
	Base64 string `json:"base64"`
	Format string `json:"format"` // "png"
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// ClickRequest performs a mouse click.
// Use pointer X/Y so (0,0) is distinguishable from "not set".
type ClickRequest struct {
	X      *int   `json:"x,omitempty"`
	Y      *int   `json:"y,omitempty"`
	Button string `json:"button,omitempty"` // "left"|"right"|"middle", default left
	Double bool   `json:"double,omitempty"`
}

// TypeRequest types text via ydotool/wtype.
type TypeRequest struct {
	Text  string `json:"text"`
	Delay int    `json:"delay_ms,omitempty"`
}

// KeyRequest sends a key combo, e.g. "ctrl+c", "Return".
type KeyRequest struct {
	Combo string `json:"combo"`
}

// MouseMoveRequest moves the cursor.
type MouseMoveRequest struct {
	X        int  `json:"x"`
	Y        int  `json:"y"`
	Relative bool `json:"relative,omitempty"`
	Monitor  *int `json:"monitor,omitempty"` // screen index (0-based), null = absolute in virtual canvas
}

// CursorPosResult returns the current cursor position.
type CursorPosResult struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ClipboardReadRequest reads the system clipboard.
type ClipboardReadRequest struct {
	Primary bool `json:"primary,omitempty"` // X11 primary selection (xclip only)
}

// ClipboardReadResult returns clipboard text.
type ClipboardReadResult struct {
	Content string `json:"content"`
}

// ClipboardWriteRequest sets the system clipboard.
type ClipboardWriteRequest struct {
	Content string `json:"content"`
	Primary bool   `json:"primary,omitempty"`
}

// ScrollRequest requests a mouse scroll.
type ScrollRequest struct {
	DY int `json:"dy"`
}

// DragRequest performs a mouse drag from (x1,y1) to (x2,y2).
type DragRequest struct {
	X1     int    `json:"x1"`
	Y1     int    `json:"y1"`
	X2     int    `json:"x2"`
	Y2     int    `json:"y2"`
	Button string `json:"button,omitempty"` // "left"|"right"|"middle"
}

// BoardRequest writes text to clipboard then pastes it.
type BoardRequest struct {
	Text string `json:"text"`
}

// A11yRequest requests an accessibility tree.
type A11yRequest struct {
	ID      *int     `json:"id,omitempty"`
	Depth   int      `json:"depth,omitempty"`
	ShowAll bool     `json:"show_all,omitempty"`
	Roles   []string `json:"roles,omitempty"`
	Monitor *int     `json:"monitor,omitempty"`
}

// A11yTextResponse is the text-format response for a11y.
type A11yTextResponse struct {
	Text string `json:"_text"`
}

// MonitorsRequest requests monitor layout info.
type MonitorsRequest struct{}

// MonitorInfo describes a single monitor.
type MonitorInfo struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Width   int     `json:"w"`
	Height  int     `json:"h"`
	Scale   float64 `json:"scale,omitempty"`
	Focused bool    `json:"focused,omitempty"`
}

// EmptyResult is used for actions with no return data beyond ok.
type EmptyResult struct {
	OK bool `json:"ok"`
}
