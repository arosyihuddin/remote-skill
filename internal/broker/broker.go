// Package broker manages the device registry and routes requests/responses
// between the skill HTTP API and connected agent WebSocket sessions.
package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/pstar7/remote-skill/internal/proto"
)

// PendingResponse delivers either a final response frame or streaming chunks.
// Streaming consumers read from Stream until it closes; non-streaming consumers
// read once from Final.
type PendingResponse struct {
	id      string
	Stream  chan proto.Frame // chunks for streaming requests; closed when done
	Final   chan proto.Frame // final response frame
	stream  bool
	closed  bool
	closeMu sync.Mutex
}

func (p *PendingResponse) closeStream() {
	p.closeMu.Lock()
	defer p.closeMu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	close(p.Stream)
}

// Device is a connected node.
type Device struct {
	ID       string
	Hostname string
	OS       string
	Arch     string
	Version  string
	Conn     *websocket.Conn
	Connected time.Time

	writeMu sync.Mutex
	pending sync.Map // id -> *PendingResponse
}

func (d *Device) writeFrame(ctx context.Context, f proto.Frame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	return d.Conn.Write(ctx, websocket.MessageText, b)
}

// SendRequest sends a typed request and registers a pending response slot.
// Caller must drain Final (and Stream if streaming) and call cleanup().
func (d *Device) SendRequest(ctx context.Context, t proto.MessageType, payload any, streaming bool) (*PendingResponse, func(), error) {
	id := uuid.NewString()
	pl, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	pr := &PendingResponse{
		id:     id,
		Stream: make(chan proto.Frame, 64),
		Final:  make(chan proto.Frame, 1),
		stream: streaming,
	}
	d.pending.Store(id, pr)
	cleanup := func() {
		d.pending.Delete(id)
		pr.closeStream()
	}
	if err := d.writeFrame(ctx, proto.Frame{Type: t, ID: id, Payload: pl}); err != nil {
		cleanup()
		return nil, nil, err
	}
	return pr, cleanup, nil
}

// dispatchFrame is called by the read loop to deliver an incoming frame
// to the right pending response.
func (d *Device) dispatchFrame(f proto.Frame) {
	if f.ID == "" {
		return
	}
	v, ok := d.pending.Load(f.ID)
	if !ok {
		return
	}
	pr := v.(*PendingResponse)
	switch f.Type {
	case proto.TypeStream:
		select {
		case pr.Stream <- f:
		default:
			// drop if buffer full to avoid blocking the read loop
		}
	case proto.TypeResponse, proto.TypeError:
		pr.closeStream()
		select {
		case pr.Final <- f:
		default:
		}
	}
}

// Broker is the global registry of connected devices.
type Broker struct {
	mu      sync.RWMutex
	devices map[string]*Device // device_id -> Device
}

func New() *Broker {
	return &Broker{devices: map[string]*Device{}}
}

// Register stores a device, replacing any prior connection with the same ID.
func (b *Broker) Register(d *Device) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if old, ok := b.devices[d.ID]; ok {
		_ = old.Conn.Close(websocket.StatusNormalClosure, "replaced by new connection")
	}
	b.devices[d.ID] = d
}

// Unregister removes a device only if it still owns the slot.
func (b *Broker) Unregister(d *Device) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if cur, ok := b.devices[d.ID]; ok && cur == d {
		delete(b.devices, d.ID)
	}
}

// Get returns the device by id, or nil if not connected.
func (b *Broker) Get(id string) *Device {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.devices[id]
}

// List returns a snapshot of connected devices (metadata only).
func (b *Broker) List() []DeviceInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]DeviceInfo, 0, len(b.devices))
	for _, d := range b.devices {
		out = append(out, DeviceInfo{
			ID:        d.ID,
			Hostname:  d.Hostname,
			OS:        d.OS,
			Arch:      d.Arch,
			Version:   d.Version,
			Connected: d.Connected,
		})
	}
	return out
}

// HandleAgentFrames runs the read loop. Returns when the connection closes.
func (b *Broker) HandleAgentFrames(ctx context.Context, d *Device) error {
	for {
		_, data, err := d.Conn.Read(ctx)
		if err != nil {
			return err
		}
		var f proto.Frame
		if err := json.Unmarshal(data, &f); err != nil {
			continue
		}
		switch f.Type {
		case proto.TypePing:
			_ = d.writeFrame(ctx, proto.Frame{Type: proto.TypePong, ID: f.ID})
		case proto.TypeResponse, proto.TypeStream, proto.TypeError:
			d.dispatchFrame(f)
		default:
			// unknown / not for server-side handling
		}
	}
}

// PickDeviceID returns the given id if non-empty, otherwise falls back to
// the only connected device. Returns error if none or multiple.
func (b *Broker) PickDeviceID(id string) (string, error) {
	if id != "" {
		if b.Get(id) == nil {
			return "", fmt.Errorf("%w: %s", ErrDeviceNotFound, id)
		}
		return id, nil
	}
	list := b.List()
	if len(list) == 0 {
		return "", fmt.Errorf("no devices connected")
	}
	if len(list) == 1 {
		return list[0].ID, nil
	}
	return "", fmt.Errorf("multiple devices connected, specify --device")
}

// DeviceInfo is the publicly-exposed metadata.
type DeviceInfo struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	Version   string    `json:"version"`
	Connected time.Time `json:"connected"`
}

// readOneFrame reads a single JSON frame from the WebSocket connection.
func readOneFrame(ctx context.Context, c *websocket.Conn) (proto.Frame, error) {
	_, data, err := c.Read(ctx)
	if err != nil {
		return proto.Frame{}, err
	}
	var f proto.Frame
	if err := json.Unmarshal(data, &f); err != nil {
		return proto.Frame{}, fmt.Errorf("bad frame: %w", err)
	}
	return f, nil
}

// writeOneFrame writes a single JSON frame to the WebSocket connection.
func writeOneFrame(ctx context.Context, c *websocket.Conn, f proto.Frame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, b)
}

// HandleCLI handles a transient CLI WebSocket connection.
// The client sends Hello → receives Ack → sends one request frame →
// receives the response → connection is done.
func (b *Broker) HandleCLI(ctx context.Context, c *websocket.Conn, serverToken string) error {
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	f, err := readOneFrame(readCtx, c)
	cancel()
	if err != nil {
		return fmt.Errorf("read hello: %w", err)
	}
	if f.Type != proto.TypeHello {
		return fmt.Errorf("expected hello, got %s", f.Type)
	}
	var hello proto.Hello
	if err := json.Unmarshal(f.Payload, &hello); err != nil {
		return fmt.Errorf("bad hello: %w", err)
	}
	if hello.Token != serverToken {
		return fmt.Errorf("bad token")
	}
	if hello.Role != proto.RoleCLI {
		return fmt.Errorf("expected role cli, got %s", hello.Role)
	}

	ackPL, _ := json.Marshal(proto.Ack{OK: true})
	ackFrame, _ := json.Marshal(proto.Frame{Type: proto.TypeAck, ID: f.ID, Payload: ackPL})
	if err := c.Write(ctx, websocket.MessageText, ackFrame); err != nil {
		return fmt.Errorf("ack: %w", err)
	}

	// Read request frame
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := readOneFrame(reqCtx, c)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	// Determine target device
	deviceID := ""
	// Pick the only connected device, or let the CLI specify
	list := b.List()
	if len(list) == 0 {
		return fmt.Errorf("no devices connected")
	}
	if len(list) == 1 {
		deviceID = list[0].ID
	} else {
		// First device by default; CLI should specify
		deviceID = list[0].ID
	}

	// Handle local-only request types
	switch req.Type {
	case proto.TypeDevices:
		devices := b.List()
		raw, _ := json.Marshal(devices)
		return writeOneFrame(ctx, c, proto.Frame{Type: proto.TypeResponse, ID: req.ID, Payload: raw})
	}

	// Forward request to device
	var raw json.RawMessage
	if err := b.Call(reqCtx, deviceID, req.Type, json.RawMessage(req.Payload), &raw); err != nil {
		ep, _ := json.Marshal(proto.ErrorPayload{Code: "proxy", Message: err.Error()})
		return writeOneFrame(ctx, c, proto.Frame{Type: proto.TypeError, ID: req.ID, Payload: ep})
	}

	return writeOneFrame(ctx, c, proto.Frame{Type: proto.TypeResponse, ID: req.ID, Payload: raw})
}

// ErrDeviceNotFound is returned when the requested device ID is not connected.
var ErrDeviceNotFound = errors.New("device not connected")

// Call sends a one-shot request and waits for the final response payload.
func (b *Broker) Call(ctx context.Context, deviceID string, t proto.MessageType, payload any, out any) error {
	d := b.Get(deviceID)
	if d == nil {
		return fmt.Errorf("%w: %s", ErrDeviceNotFound, deviceID)
	}
	pr, cleanup, err := d.SendRequest(ctx, t, payload, false)
	if err != nil {
		return err
	}
	defer cleanup()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case f := <-pr.Final:
		if f.Type == proto.TypeError {
			var ep proto.ErrorPayload
			_ = json.Unmarshal(f.Payload, &ep)
			return fmt.Errorf("agent error: %s: %s", ep.Code, ep.Message)
		}
		if out != nil {
			return json.Unmarshal(f.Payload, out)
		}
		return nil
	}
}
