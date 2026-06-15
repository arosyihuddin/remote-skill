// Package handlers — shared dependencies (policy + audit) and constructors
// that wrap each handler with enforcement and logging.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/audit"
	"github.com/pstar7/remote-skill/internal/policy"
	"github.com/pstar7/remote-skill/internal/proto"
)

// Deps carries cross-cutting concerns to handler constructors.
type Deps struct {
	Policy   *policy.Policy
	Audit    *audit.Logger
	DeviceID string
}

// WrapExec adds policy check + audit logging around the raw Exec handler.
func (d *Deps) WrapExec() handler.Handler {
	return func(ctx context.Context, payload json.RawMessage, sw handler.StreamWriter) (any, error) {
		var req proto.ExecRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: "exec",
				Allowed: audit.Denied(), Error: "bad payload: " + err.Error(),
			})
			return nil, fmt.Errorf("bad payload: %w", err)
		}

		if err := d.Policy.CheckCmd(req.Cmd, req.Shell); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: "exec",
				Cmd: req.Cmd, Cwd: req.Cwd, Shell: req.Shell,
				Allowed: audit.Denied(), Error: err.Error(),
			})
			return nil, err
		}

		start := time.Now()
		res, err := Exec(ctx, payload, sw)
		dur := time.Since(start).Milliseconds()

		entry := audit.Entry{
			DeviceID: d.DeviceID, Type: "exec",
			Cmd: req.Cmd, Cwd: req.Cwd, Shell: req.Shell,
			DurationMs: dur, Allowed: audit.Allowed(),
		}
		if err != nil {
			entry.Error = err.Error()
		}
		if r, ok := res.(proto.ExecResult); ok {
			entry.ExitCode = audit.IntPtr(r.ExitCode)
		}
		d.Audit.Log(entry)
		return res, err
	}
}

// WrapReadFile checks DENY_PATH and audits.
func (d *Deps) WrapReadFile() handler.Handler {
	return func(ctx context.Context, payload json.RawMessage, sw handler.StreamWriter) (any, error) {
		var req proto.ReadFileRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: "read_file",
				Allowed: audit.Denied(), Error: "bad payload: " + err.Error(),
			})
			return nil, fmt.Errorf("bad payload: %w", err)
		}
		if err := d.Policy.CheckPath(expandPath(req.Path)); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: "read_file",
				Path: req.Path, Allowed: audit.Denied(), Error: err.Error(),
			})
			return nil, err
		}
		res, err := ReadFile(ctx, payload, sw)
		entry := audit.Entry{
			DeviceID: d.DeviceID, Type: "read_file",
			Path: req.Path, Allowed: audit.Allowed(),
		}
		if err != nil {
			entry.Error = err.Error()
		}
		if r, ok := res.(proto.ReadFileResult); ok {
			entry.SizeBytes = r.SizeBytes
		}
		d.Audit.Log(entry)
		return res, err
	}
}

// WrapWriteFile checks DENY_PATH and audits.
func (d *Deps) WrapWriteFile() handler.Handler {
	return func(ctx context.Context, payload json.RawMessage, sw handler.StreamWriter) (any, error) {
		var req proto.WriteFileRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: "write_file",
				Allowed: audit.Denied(), Error: "bad payload: " + err.Error(),
			})
			return nil, fmt.Errorf("bad payload: %w", err)
		}
		if err := d.Policy.CheckPath(expandPath(req.Path)); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: "write_file",
				Path: req.Path, Allowed: audit.Denied(), Error: err.Error(),
			})
			return nil, err
		}
		res, err := WriteFile(ctx, payload, sw)
		entry := audit.Entry{
			DeviceID: d.DeviceID, Type: "write_file",
			Path: req.Path, Allowed: audit.Allowed(),
		}
		if err != nil {
			entry.Error = err.Error()
		}
		if r, ok := res.(proto.WriteFileResult); ok {
			entry.SizeBytes = r.BytesWritten
		}
		d.Audit.Log(entry)
		return res, err
	}
}

// WrapListDir audits and checks path.
func (d *Deps) WrapListDir() handler.Handler {
	return func(ctx context.Context, payload json.RawMessage, sw handler.StreamWriter) (any, error) {
		var req proto.ListDirRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: "list_dir",
				Allowed: audit.Denied(), Error: "bad payload: " + err.Error(),
			})
			return nil, fmt.Errorf("bad payload: %w", err)
		}
		if err := d.Policy.CheckPath(expandPath(req.Path)); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: "list_dir",
				Path: req.Path, Allowed: audit.Denied(), Error: err.Error(),
			})
			return nil, err
		}
		res, err := ListDir(ctx, payload, sw)
		entry := audit.Entry{
			DeviceID: d.DeviceID, Type: "list_dir",
			Path: req.Path, Allowed: audit.Allowed(),
		}
		if err != nil {
			entry.Error = err.Error()
		}
		d.Audit.Log(entry)
		return res, err
	}
}

// wrapGUI is a helper that checks AllowGUI policy + audits, then calls the raw handler.
func (d *Deps) wrapGUI(name string, raw func(ctx context.Context, payload json.RawMessage, sw handler.StreamWriter) (any, error)) handler.Handler {
	return func(ctx context.Context, payload json.RawMessage, sw handler.StreamWriter) (any, error) {
		if err := d.Policy.CheckGUI(); err != nil {
			d.Audit.Log(audit.Entry{
				DeviceID: d.DeviceID, Type: name,
				Allowed: audit.Denied(), Error: err.Error(),
			})
			return nil, err
		}
		start := time.Now()
		res, err := raw(ctx, payload, sw)
		dur := time.Since(start).Milliseconds()
		entry := audit.Entry{
			DeviceID: d.DeviceID, Type: name,
			DurationMs: dur, Allowed: audit.Allowed(),
		}
		if err != nil {
			entry.Error = err.Error()
		}
		d.Audit.Log(entry)
		return res, err
	}
}

func (d *Deps) WrapScreenshot() handler.Handler       { return d.wrapGUI("screenshot", Screenshot) }
func (d *Deps) WrapClick() handler.Handler            { return d.wrapGUI("click", Click) }
func (d *Deps) WrapType() handler.Handler             { return d.wrapGUI("type", Type) }
func (d *Deps) WrapKey() handler.Handler              { return d.wrapGUI("key", Key) }
func (d *Deps) WrapMouse() handler.Handler            { return d.wrapGUI("mouse", MouseMove) }
func (d *Deps) WrapScroll() handler.Handler           { return d.wrapGUI("scroll", Scroll) }
func (d *Deps) WrapClipboardRead() handler.Handler    { return d.wrapGUI("clipboard_read", ClipboardRead) }
func (d *Deps) WrapClipboardWrite() handler.Handler   { return d.wrapGUI("clipboard_write", ClipboardWrite) }
func (d *Deps) WrapDrag() handler.Handler             { return d.wrapGUI("drag", Drag) }
func (d *Deps) WrapBoard() handler.Handler            { return d.wrapGUI("board", Board) }
func (d *Deps) WrapWindows() handler.Handler          { return d.wrapGUI("windows", Windows) }
func (d *Deps) WrapAccessibilityTree() handler.Handler { return d.wrapGUI("accessibility_tree", AccessibilityTree) }
func (d *Deps) WrapMonitors() handler.Handler           { return d.wrapGUI("monitors", Monitors) }
func (d *Deps) WrapCursorPos() handler.Handler          { return d.wrapGUI("cursorpos", CursorPos) }

