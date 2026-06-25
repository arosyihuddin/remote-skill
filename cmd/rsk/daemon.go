package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/pstar7/remote-skill/internal/broker"
	"github.com/pstar7/remote-skill/internal/config"
	"github.com/pstar7/remote-skill/internal/db"
	"github.com/pstar7/remote-skill/internal/proto"
)

func defaultDaemonConfigPath() string {
	if h, err := os.UserConfigDir(); err == nil {
		return h + "/rsk/rsk.env"
	}
	return ""
}

func defaultDBPath() string {
	candidates := []string{"."}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, dir)
		candidates = append(candidates, filepath.Join(dir, ".."))
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
			return filepath.Join(dir, "rsk.db")
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		dir := h + "/.local/share/rsk"
		os.MkdirAll(dir, 0755)
		return dir + "/rsk.db"
	}
	return "rsk.db"
}

func runDaemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	defCfg := defaultDaemonConfigPath()
	cfgPath := fs.String("config", defCfg, "path to config file")
	dbPath := fs.String("db", defaultDBPath(), "path to shortcuts database")
	_ = fs.Parse(args)

	cfg, err := config.LoadServerConfig(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	br := broker.New()

	// Agent listener (WS :7777)
	agentMux := http.NewServeMux()
	agentMux.HandleFunc("/agent", agentHandler(br, cfg.Token))
	agentMux.HandleFunc("/cli", cliWSHandler(br, cfg.Token))
	agentSrv := &http.Server{Addr: cfg.AgentListen, Handler: agentMux}

	// Database for shortcuts
	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	// Monitoring HTTP (:7800)
	monMux := http.NewServeMux()
	registerMonitoringRoutes(monMux, br, database, cfg.Token, cfg.UIPassword)
	monSrv := &http.Server{
		Addr:    cfg.SkillListen,
		Handler: authMiddleware(cfg.Token, monMux),
	}

	go func() {
		log.Printf("agent WS listening on %s", cfg.AgentListen)
		if err := agentSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("agent server: %v", err)
		}
	}()
	go func() {
		log.Printf("monitoring HTTP listening on %s", cfg.SkillListen)
		if err := monSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("monitoring server: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = agentSrv.Shutdown(ctx)
	_ = monSrv.Shutdown(ctx)
}

func agentHandler(br *broker.Broker, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		c.SetReadLimit(64 << 20)

		ctx := r.Context()
		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, data, err := c.Read(readCtx)
		cancel()
		if err != nil {
			_ = c.Close(websocket.StatusPolicyViolation, "no hello")
			return
		}
		var f proto.Frame
		if err := json.Unmarshal(data, &f); err != nil || f.Type != proto.TypeHello {
			_ = c.Close(websocket.StatusPolicyViolation, "expected hello")
			return
		}
		var hello proto.Hello
		if err := json.Unmarshal(f.Payload, &hello); err != nil {
			_ = c.Close(websocket.StatusPolicyViolation, "bad hello")
			return
		}
		if hello.Token != token {
			_ = c.Close(websocket.StatusPolicyViolation, "bad token")
			return
		}
		if hello.Role != proto.RoleNode {
			_ = c.Close(websocket.StatusPolicyViolation, "expected role node")
			return
		}
		if hello.DeviceID == "" {
			_ = c.Close(websocket.StatusPolicyViolation, "missing device_id")
			return
		}

		dev := &broker.Device{
			ID:        hello.DeviceID,
			Hostname:  hello.Hostname,
			OS:        hello.OS,
			Arch:      hello.Arch,
			Version:   hello.Version,
			Conn:      c,
			Connected: time.Now(),
		}

		ackPL, _ := json.Marshal(proto.Ack{OK: true})
		ackBytes, _ := json.Marshal(proto.Frame{Type: proto.TypeAck, ID: f.ID, Payload: ackPL})
		if err := c.Write(ctx, websocket.MessageText, ackBytes); err != nil {
			_ = c.Close(websocket.StatusInternalError, "ack failed")
			return
		}

		br.Register(dev)
		log.Printf("device connected: %s (%s)", dev.ID, dev.Hostname)
		defer func() {
			br.Unregister(dev)
			log.Printf("device disconnected: %s", dev.ID)
		}()

		_ = br.HandleAgentFrames(ctx, dev)
		_ = c.Close(websocket.StatusNormalClosure, "")
	}
}

func cliWSHandler(br *broker.Broker, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		c.SetReadLimit(64 << 20)
		ctx := r.Context()
		if err := br.HandleCLI(ctx, c, token); err != nil {
			log.Printf("cli ws: %v", err)
		}
		_ = c.Close(websocket.StatusNormalClosure, "")
	}
}
