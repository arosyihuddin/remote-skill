package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/pstar7/remote-skill/internal/broker"
	"github.com/pstar7/remote-skill/internal/db"
	"github.com/pstar7/remote-skill/internal/handlers"
	"github.com/pstar7/remote-skill/internal/proto"
)

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, apiError{Error: err.Error()})
}

func readReqJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if checkAuth(r, token) {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/login" || r.URL.Path == "/logout" || r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		writeErr(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
	})
}

func checkAuth(r *http.Request, token string) bool {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == token {
		return true
	}
	if r.URL.Query().Get("token") == token {
		return true
	}
	if c, err := r.Cookie("token"); err == nil && c.Value == token {
		return true
	}
	return false
}

func registerMonitoringRoutes(mux *http.ServeMux, br *broker.Broker, database *db.DB, token, uiPass string) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := fs.ReadFile(uiFS, "ui/index.html")
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	sub, _ := fs.Sub(uiFS, "ui")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, 405, fmt.Errorf("method not allowed"))
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		if err := readReqJSON(r, &body); err != nil {
			writeErr(w, 400, fmt.Errorf("bad json: %w", err))
			return
		}
		if body.Token != uiPass {
			writeErr(w, 401, fmt.Errorf("invalid token"))
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, 405, fmt.Errorf("method not allowed"))
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/devices", func(w http.ResponseWriter, r *http.Request) {
		monitors, _ := handlers.DetectMonitors(r.Context())
		writeJSON(w, 200, struct {
			Devices  []broker.DeviceInfo  `json:"devices"`
			Monitors []proto.MonitorInfo  `json:"monitors"`
		}{
			Devices:  br.List(),
			Monitors: monitors,
		})
	})

	mux.HandleFunc("/exec", handleCall(br, proto.TypeExec, func() any { return &proto.ExecRequest{} }))
	mux.HandleFunc("/read", handleCall(br, proto.TypeReadFile, func() any { return &proto.ReadFileRequest{} }))
	mux.HandleFunc("/write", handleCall(br, proto.TypeWriteFile, func() any { return &proto.WriteFileRequest{} }))
	mux.HandleFunc("/ls", handleCall(br, proto.TypeListDir, func() any { return &proto.ListDirRequest{} }))
	mux.HandleFunc("/screenshot", handleCall(br, proto.TypeScreenshot, func() any { return &proto.ScreenshotRequest{} }))
	mux.HandleFunc("/click", handleCall(br, proto.TypeClick, func() any { return &proto.ClickRequest{} }))
	mux.HandleFunc("/type", handleCall(br, proto.TypeType, func() any { return &proto.TypeRequest{} }))
	mux.HandleFunc("/key", handleCall(br, proto.TypeKey, func() any { return &proto.KeyRequest{} }))
	mux.HandleFunc("/mouse", handleCall(br, proto.TypeMouse, func() any { return &proto.MouseMoveRequest{} }))
	mux.HandleFunc("/clipboard/read", handleCall(br, proto.TypeClipboardRead, func() any { return &proto.ClipboardReadRequest{} }))
	mux.HandleFunc("/clipboard/write", handleCall(br, proto.TypeClipboardWrite, func() any { return &proto.ClipboardWriteRequest{} }))
	mux.HandleFunc("/scroll", handleCall(br, proto.TypeScroll, func() any { return &proto.ScrollRequest{} }))
	mux.HandleFunc("/windows", handleCall(br, proto.TypeWindows, func() any { return &struct{}{} }))
	mux.HandleFunc("/close-window", handleCall(br, proto.TypeCloseWindow, func() any { return &proto.CloseWindowRequest{} }))
	mux.HandleFunc("/a11y/tree", handleCall(br, proto.TypeAccessibilityTree, func() any { return &struct{}{} }))
	mux.HandleFunc("/monitors", handleCall(br, proto.TypeMonitors, func() any { return &proto.MonitorsRequest{} }))
	mux.HandleFunc("/cursorpos", handleCall(br, proto.TypeCursorPos, func() any { return &struct{}{} }))
	mux.HandleFunc("/apps", handleCall(br, proto.TypeAppList, func() any { return &proto.AppListRequest{} }))
	mux.HandleFunc("/apps/launch", handleCall(br, proto.TypeAppLaunch, func() any { return &proto.AppLaunchRequest{} }))
	mux.HandleFunc("/screen.ws", handlers.ServeScreenWS)
	mux.HandleFunc("/exec/stream", handleExecStream(br))
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		c.SetReadLimit(64 << 20)
		ctx := r.Context()
		if err := br.HandleLiveSession(ctx, c); err != nil {
			log.Printf("live session: %v", err)
		}
		_ = c.Close(websocket.StatusNormalClosure, "")
	})

	mux.HandleFunc("/shortcuts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			list, err := database.List()
			if err != nil {
				writeErr(w, 500, err)
				return
			}
			writeJSON(w, 200, list)
		case http.MethodPost:
			var body struct {
				Name  string `json:"name"`
				Combo string `json:"combo"`
			}
			if err := readReqJSON(r, &body); err != nil {
				writeErr(w, 400, fmt.Errorf("bad json: %w", err))
				return
			}
			if body.Name == "" || body.Combo == "" {
				writeErr(w, 400, fmt.Errorf("name and combo required"))
				return
			}
			s, err := database.Add(body.Name, body.Combo)
			if err != nil {
				writeErr(w, 500, err)
				return
			}
			writeJSON(w, 200, s)
		default:
			writeErr(w, 405, fmt.Errorf("method not allowed"))
		}
	})
	mux.HandleFunc("/shortcuts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeErr(w, 405, fmt.Errorf("DELETE only"))
			return
		}
		parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[2] == "" {
			writeErr(w, 400, fmt.Errorf("missing id"))
			return
		}
		var id int
		if _, err := fmt.Sscanf(parts[2], "%d", &id); err != nil {
			writeErr(w, 400, fmt.Errorf("bad id"))
			return
		}
		if err := database.Delete(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})
}

func handleCall(br *broker.Broker, t proto.MessageType, mkPayload func() any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, 405, fmt.Errorf("POST only"))
			return
		}
		dev, err := pickDevice(br, r)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		payload := mkPayload()
		if r.ContentLength > 0 {
			if err := readReqJSON(r, payload); err != nil {
				writeErr(w, 400, fmt.Errorf("bad json: %w", err))
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		var raw json.RawMessage
		if err := br.Call(ctx, dev, t, payload, &raw); err != nil {
			writeErr(w, 502, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(raw)
	}
}

func handleExecStream(br *broker.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, 405, fmt.Errorf("POST only"))
			return
		}
		devID, err := pickDevice(br, r)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		var req proto.ExecRequest
		if err := readReqJSON(r, &req); err != nil {
			writeErr(w, 400, err)
			return
		}
		req.Stream = true
		dev := br.Get(devID)
		if dev == nil {
			writeErr(w, 502, broker.ErrDeviceNotFound)
			return
		}
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(200)

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		defer cancel()
		pr, cleanup, err := dev.SendRequest(ctx, proto.TypeExec, &req)
		if err != nil {
			fmt.Fprintf(w, `{"error":%q}`+"\n", err.Error())
			return
		}
		defer cleanup()

		emit := func(obj any) {
			b, _ := json.Marshal(obj)
			_, _ = w.Write(b)
			_, _ = io.WriteString(w, "\n")
			if flusher != nil {
				flusher.Flush()
			}
		}

		for {
			select {
			case <-ctx.Done():
				emit(map[string]any{"error": "timeout"})
				return
			case chunk, ok := <-pr.Stream:
				if !ok {
					select {
					case f := <-pr.Final:
						emit(map[string]any{"final": json.RawMessage(f.Payload), "type": string(f.Type)})
					case <-ctx.Done():
						emit(map[string]any{"error": "timeout"})
					}
					return
				}
				emit(map[string]any{"chunk": json.RawMessage(chunk.Payload)})
			case f := <-pr.Final:
				for {
					select {
					case chunk, ok := <-pr.Stream:
						if !ok {
							goto done
						}
						emit(map[string]any{"chunk": json.RawMessage(chunk.Payload)})
					default:
						goto done
					}
				}
			done:
				emit(map[string]any{"final": json.RawMessage(f.Payload), "type": string(f.Type)})
				return
			}
		}
	}
}

func pickDevice(br *broker.Broker, r *http.Request) (string, error) {
	id := r.URL.Query().Get("device")
	if id == "" {
		id = r.Header.Get("X-Device-ID")
	}
	return br.PickDeviceID(id)
}
