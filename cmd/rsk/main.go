package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/pstar7/remote-skill/internal/broker"
	"github.com/pstar7/remote-skill/internal/cli"
	"github.com/pstar7/remote-skill/internal/config"
	"github.com/pstar7/remote-skill/internal/db"
	"github.com/pstar7/remote-skill/internal/handlers"
	"github.com/pstar7/remote-skill/internal/proto"
	"github.com/pstar7/remote-skill/internal/update"
)

//go:embed ui/*
var uiFS embed.FS

//go:embed rsk.service
var serviceUnit []byte

var Version = "dev"

func main() {
	config.LoadDotEnv()

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "daemon":
			runDaemon(os.Args[2:])
			return
		case "token":
			printToken()
			return
		case "uninstall":
			runUninstall()
			return
		case "setup":
			runSetup(os.Args[2:])
			return
		case "version":
			printVersion()
			return
		case "update":
			runUpdate()
			return
		case "start":
			exec.Command("systemctl", "--user", "start", "rsk").Run()
			return
		case "stop":
			exec.Command("systemctl", "--user", "stop", "rsk").Run()
			return
		case "restart":
			runRestart()
			return
		case "status":
			runStatus()
			return
		case "info":
			runInfo()
			return
		case "logs":
			runLog(os.Args[2:])
			return
		case "-h", "--help", "help":
			printUsage()
			return
		default:
			cli.Run(os.Args[1], os.Args[2:])
			return
		}
	}
	printUsage()
}

func runSetup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	agentListen := fs.String("agent", "0.0.0.0:7777", "node WS listen address")
	monitor := fs.String("monitor", "127.0.0.1:7800", "monitoring HTTP address")
	token := fs.String("token", "", "auth token (auto-generate if empty)")
	uiPass := fs.String("ui-password", "", "dashboard login password (default: same as token)")
	uninstall := fs.Bool("uninstall", false, "remove installation")
	_ = fs.Parse(args)

	if *uninstall {
		runUninstall()
		return
	}

	home := os.Getenv("HOME")
	configPath := filepath.Join(home, ".config", "rsk", "rsk.env")

	// Check for existing config
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  ⚠ %s already exists\n", configPath)
		fmt.Print("  Use existing configuration? [Y/n]: ")
		var yn string
		fmt.Scanln(&yn)
		if yn != "n" && yn != "N" {
			if m, err := config.LoadEnvFile(configPath); err == nil {
				if v, ok := m["AGENT_LISTEN"]; ok && v != "" {
					*agentListen = v
				}
				if v, ok := m["SKILL_LISTEN"]; ok && v != "" {
					*monitor = v
				}
				if v, ok := m["TOKEN"]; ok && v != "" {
					*token = v
				}
				if v, ok := m["UI_PASSWORD"]; ok && v != "" {
					*uiPass = v
				}
			}
			fmt.Println("  ℹ using existing configuration")
		}
	}

	if *token == "" {
		t, _ := exec.Command("openssl", "rand", "-hex", "16").Output()
		if len(t) > 0 {
			*token = strings.TrimSpace(string(t))
		} else {
			*token = fmt.Sprintf("rsk-%d", time.Now().Unix())
		}
	}

	// 1. Copy binary to ~/.local/bin/rsk
	binDir := filepath.Join(home, ".local", "bin")
	os.MkdirAll(binDir, 0755)
	self, _ := os.Executable()
	binTarget := filepath.Join(binDir, "rsk")
	if self != binTarget {
		data, _ := os.ReadFile(self)
		if data != nil {
			os.WriteFile(binTarget, data, 0755)
		}
	}
	fmt.Printf("  ✔ installed: %s\n", binTarget)

	// 2. Config at ~/.config/rsk/rsk.env
	configDir := filepath.Join(home, ".config", "rsk")
	os.MkdirAll(configDir, 0755)
	content := fmt.Sprintf(`AGENT_LISTEN=%s
SKILL_LISTEN=%s
TOKEN=%s
UI_PASSWORD=%s
`, *agentListen, *monitor, *token, *uiPass)
	os.WriteFile(configPath, []byte(content), 0600)
	fmt.Printf("  ✔ config: %s\n", configPath)

	// 3. Udev rule for /dev/uinput (need sudo)
	udevRulePath := "/etc/udev/rules.d/99-rsk-uinput.rules"
	if _, err := os.Stat(udevRulePath); os.IsNotExist(err) {
		udevRule := `KERNEL=="uinput", MODE="0666"`
		exec.Command("sudo", "sh", "-c", fmt.Sprintf(
			"echo '%s' > %s && udevadm control --reload-rules && udevadm trigger && chmod 0666 /dev/uinput",
			udevRule, udevRulePath)).Run()
		fmt.Println("  ✔ /dev/uinput permission set")
	} else {
		fmt.Println("  ✔ /dev/uinput already configured")
	}

	// 4. Systemd user service
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	os.MkdirAll(unitDir, 0755)
	unitPath := filepath.Join(unitDir, "rsk.service")
	os.WriteFile(unitPath, serviceUnit, 0644)
	fmt.Printf("  ✔ service unit: %s\n", unitPath)
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	exec.Command("systemctl", "--user", "enable", "rsk").Run()
	exec.Command("systemctl", "--user", "restart", "rsk").Run()
	fmt.Println("  ✔ service started")

	fmt.Printf("\n✅ rsk installed\n")
	fmt.Printf("  Token: %s\n", *token)
	fmt.Printf("  Open http://%s/ in browser\n", *monitor)
}

func runUninstall() {
	exec.Command("systemctl", "--user", "stop", "rsk").Run()
	exec.Command("systemctl", "--user", "disable", "rsk").Run()
	unitPath := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user", "rsk.service")
	os.Remove(unitPath)
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	fmt.Println("  ✔ service removed")

	rulePath := "/etc/udev/rules.d/99-rsk-uinput.rules"
	if _, err := os.Stat(rulePath); err == nil {
		exec.Command("sudo", "rm", rulePath).Run()
		exec.Command("sudo", "udevadm", "control", "--reload-rules").Run()
		fmt.Println("  ✔ udev rule removed")
	}

	os.Remove(filepath.Join(os.Getenv("HOME"), ".local", "bin", "rsk"))
	fmt.Println("  ✔ binary removed")

	configPath := filepath.Join(os.Getenv("HOME"), ".config", "rsk", "rsk.env")
	os.Remove(configPath)
	fmt.Println("  ✔ config removed")

	fmt.Println("\n✅ rsk uninstalled")
}

func printToken() {
	path := filepath.Join(os.Getenv("HOME"), ".config", "rsk", "rsk.env")
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: config not found at %s\n", path)
		os.Exit(1)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "TOKEN=") {
			fmt.Println(strings.TrimPrefix(line, "TOKEN="))
			return
		}
	}
	fmt.Fprintf(os.Stderr, "error: TOKEN not found in %s\n", path)
	os.Exit(1)
}

func printVersion() {
	fmt.Printf("rsk version %s\n", Version)
}

func runStatus() {
	fmt.Printf("rsk %s\n", Version)

	out, err := exec.Command("systemctl", "--user", "is-active", "rsk").Output()
	if err != nil {
		fmt.Println("  Service: not installed")
		return
	}
	fmt.Printf("  Service: %s\n", strings.TrimSpace(string(out)))

	configPath := filepath.Join(os.Getenv("HOME"), ".config", "rsk", "rsk.env")
	if m, err := config.LoadEnvFile(configPath); err == nil {
		if v, ok := m["AGENT_LISTEN"]; ok {
			fmt.Printf("  Agent:   ws://%s/agent\n", v)
		}
		if v, ok := m["SKILL_LISTEN"]; ok {
			fmt.Printf("  Monitor: http://%s/\n", v)
		}
	}
	fmt.Println("  Devices: run `rsk devices`")
}

func runInfo() {
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "rsk", "rsk.env")
	m, err := config.LoadEnvFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: config not found at %s\n", configPath)
		os.Exit(1)
	}

	fmt.Printf("Config: %s\n", configPath)
	if v, ok := m["AGENT_LISTEN"]; ok {
		fmt.Printf("  Agent:   ws://%s/agent\n", v)
	}
	if v, ok := m["SKILL_LISTEN"]; ok {
		fmt.Printf("  Monitor: http://%s/\n", v)
	}
	if v, ok := m["TOKEN"]; ok && v != "" {
		masked := v
		if len(v) > 8 {
			masked = v[:8] + "..." + v[len(v)-4:]
		}
		fmt.Printf("  Token:   %s\n", masked)
	}
}

func runRestart() {
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	if out, err := exec.Command("systemctl", "--user", "restart", "rsk").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "error: restart failed: %v\n%s\n", err, out)
		os.Exit(1)
	}
	fmt.Println("✔ rsk restarted")
}

func runLog(args []string) {
	cmdArgs := append([]string{"--user", "-u", "rsk", "-n", "50"}, args...)
	cmd := exec.Command("journalctl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runUpdate() {
	asset := update.AssetName("rsk")
	fmt.Printf("Downloading latest %s ...\n", asset)

	tmp, err := update.DownloadLatest("arosyihuddin", "remote-skill", asset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmp)

	if ver, err := update.VersionOf(tmp); err == nil {
		if ver == Version {
			if Version != "dev" {
				fmt.Printf("Already up to date (%s)\n", ver)
				return
			}
			fmt.Printf("Version %s (dev), updating anyway...\n", ver)
		} else {
			fmt.Printf("Updating %s -> %s ...\n", Version, ver)
		}
	} else {
		fmt.Printf("Warning: cannot check version (%v), proceeding...\n", err)
	}

	if err := update.ReplaceSelf(tmp); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Binary updated, restarting service...")

	if err := update.RestartService("rsk"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		fmt.Println("Restart manually: systemctl --user restart rsk")
		os.Exit(1)
	}
	fmt.Println("rsk restarted")
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  rsk <command> [args...]\n")
	fmt.Fprintf(os.Stderr, "  rsk <device-id> <command> [args...]\n")
	fmt.Fprintf(os.Stderr, "  rsk daemon [--config PATH] [--db PATH]\n")
	fmt.Fprintf(os.Stderr, "  rsk setup [--agent ADDR] [--monitor ADDR] [--token SECRET] [--ui-password PASS]\n")
	fmt.Fprintf(os.Stderr, "  rsk uninstall\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  daemon              Start the broker daemon\n")
	fmt.Fprintf(os.Stderr, "  setup               Install as user service\n")
	fmt.Fprintf(os.Stderr, "  uninstall           Remove installation\n")
	fmt.Fprintf(os.Stderr, "  version             Print version\n")
	fmt.Fprintf(os.Stderr, "  update              Self-update from GitHub\n")
	fmt.Fprintf(os.Stderr, "  start               Start systemd service\n")
	fmt.Fprintf(os.Stderr, "  stop                Stop systemd service\n")
	fmt.Fprintf(os.Stderr, "  restart             Restart systemd service\n")
	fmt.Fprintf(os.Stderr, "  status              Show service status\n")
	fmt.Fprintf(os.Stderr, "  info                Show config summary\n")
	fmt.Fprintf(os.Stderr, "  logs [args]         Tail journal logs\n")
	fmt.Fprintf(os.Stderr, "  token               Print auth token\n")
	fmt.Fprintf(os.Stderr, "  devices             List connected devices\n")
	fmt.Fprintf(os.Stderr, "  exec \"<cmd>\"        Run a command on remote node\n")
	fmt.Fprintf(os.Stderr, "  read <path>         Read a file\n")
	fmt.Fprintf(os.Stderr, "  write <path>        Write a file (stdin or --file)\n")
	fmt.Fprintf(os.Stderr, "  ls <path>           List directory\n")
	fmt.Fprintf(os.Stderr, "  screenshot          Capture screenshot\n")
	fmt.Fprintf(os.Stderr, "  click               Mouse click\n")
	fmt.Fprintf(os.Stderr, "  type \"<text>\"       Type text\n")
	fmt.Fprintf(os.Stderr, "  key \"<combo>\"       Send key combo\n")
	fmt.Fprintf(os.Stderr, "  mouse <x> <y>       Move mouse\n")
	fmt.Fprintf(os.Stderr, "  scroll [--dy N]     Scroll (default -3)\n")
	fmt.Fprintf(os.Stderr, "  drag <x1> <y1> <x2> <y2>  Mouse drag\n")
	fmt.Fprintf(os.Stderr, "  board \"<text>\"      Clipboard write + paste\n")
	fmt.Fprintf(os.Stderr, "  windows             List windows\n")
	fmt.Fprintf(os.Stderr, "  a11y [--id N] [--depth N] [--role name] [--show-all] [--monitor N] [--all]\n")
	fmt.Fprintf(os.Stderr, "                        Accessibility tree in Toon CSV (--monitor: filter by monitor, --all: all monitors)\n")
	fmt.Fprintf(os.Stderr, "  monitors            List monitors\n")
	fmt.Fprintf(os.Stderr, "  cursorpos           Get current cursor position\n")
	fmt.Fprintf(os.Stderr, "  wait <sec>          Sleep N seconds\n")
	fmt.Fprintf(os.Stderr, "  env                 Show env vars\n")
	fmt.Fprintf(os.Stderr, "  clip get|set        Clipboard operations\n")
}

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

// ---- Monitoring HTTP -------------------------------------------------------

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
		monitors, _ := handlers.GetMonitors()
		writeJSON(w, 200, struct {
			Devices  []broker.DeviceInfo    `json:"devices"`
			Monitors []handlers.MonitorInfo `json:"monitors"`
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
	mux.HandleFunc("/a11y/tree", handleCall(br, proto.TypeAccessibilityTree, func() any { return &struct{}{} }))
	mux.HandleFunc("/monitors", handleCall(br, proto.TypeMonitors, func() any { return &proto.MonitorsRequest{} }))
	mux.HandleFunc("/cursorpos", handleCall(br, proto.TypeCursorPos, func() any { return &struct{}{} }))
	mux.HandleFunc("/screen.ws", handlers.ServeScreenWS)
	mux.HandleFunc("/exec/stream", handleExecStream(br))

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
		pr, cleanup, err := dev.SendRequest(ctx, proto.TypeExec, &req, true)
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


