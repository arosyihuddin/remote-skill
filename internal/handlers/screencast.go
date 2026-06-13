package handlers

import (
	"bytes"
	"context"
	"image/jpeg"
	"image/png"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	screenFPS         = 5
	screenPeriod      = 200 * time.Millisecond
	screenJPEGQuality = 70
)

type screenClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

var (
	screenClients = map[*screenClient]struct{}{}
	screenMu      sync.Mutex
	screenActive  bool
	screenStop    chan struct{}
)

// ServeScreenWS accepts WebSocket connections and streams live screen JPEG frames.
func ServeScreenWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	if _, err := exec.LookPath("grim"); err != nil {
		_ = c.Close(websocket.StatusUnsupportedData, "grim not found (sudo apt install grim)")
		return
	}

	client := &screenClient{conn: c}

	screenMu.Lock()
	screenClients[client] = struct{}{}
	if !screenActive {
		screenActive = true
		screenStop = make(chan struct{})
		go screenLoop(screenStop)
	}
	screenMu.Unlock()

	defer func() {
		screenMu.Lock()
		delete(screenClients, client)
		if len(screenClients) == 0 && screenActive {
			screenActive = false
			close(screenStop)
		}
		screenMu.Unlock()
	}()

	_, _, _ = c.Read(context.Background())
}

func screenLoop(stop chan struct{}) {
	ticker := time.NewTicker(screenPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			frame := captureJPEG()
			if frame == nil {
				continue
			}
			screenMu.Lock()
			for cl := range screenClients {
				go func(c *screenClient) {
					c.mu.Lock()
					defer c.mu.Unlock()
					_ = c.conn.Write(context.Background(), websocket.MessageBinary, frame)
				}(cl)
			}
			screenMu.Unlock()
		}
	}
}

func captureJPEG() []byte {
	cmd := exec.Command("grim", "-")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: screenJPEGQuality}); err != nil {
		return nil
	}
	return buf.Bytes()
}
