package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pstar7/remote-skill/internal/node"
	"github.com/pstar7/remote-skill/internal/audit"
	"github.com/pstar7/remote-skill/internal/config"
	"github.com/pstar7/remote-skill/internal/handlers"
	"github.com/pstar7/remote-skill/internal/policy"
	"github.com/pstar7/remote-skill/internal/proto"
	"github.com/pstar7/remote-skill/internal/update"
)

//go:embed rsk-node.service
var serviceUnit []byte

//go:embed gnome-ext/*
var gnomeExt embed.FS

const udevRule = `KERNEL=="uinput", MODE="0666"`

type dep struct {
	Name        string
	Pkg         string
	Desc        string
	AutoInstall bool
}

var depsByEnv = map[string][]dep{
	"x11": {
		{Name: "wmctrl", Pkg: "wmctrl", Desc: "window list & management", AutoInstall: true},
	},
	"hyprland": {
		{Name: "hyprctl", Pkg: "", Desc: "window list (comes with Hyprland)", AutoInstall: false},
		{Name: "grim", Pkg: "grim", Desc: "screenshot capture", AutoInstall: true},
		{Name: "wlr-randr", Pkg: "wlr-randr", Desc: "display monitor info", AutoInstall: true},
	},
	"wayland": {
		{Name: "grim", Pkg: "grim", Desc: "screenshot capture", AutoInstall: true},
		{Name: "wlr-randr", Pkg: "wlr-randr", Desc: "display monitor info", AutoInstall: true},
	},
	"gnome": {
		{Name: "grim", Pkg: "grim", Desc: "screenshot capture", AutoInstall: true},
		{Name: "wlr-randr", Pkg: "wlr-randr", Desc: "display monitor info", AutoInstall: true},
		{Name: "wmctrl", Pkg: "wmctrl", Desc: "window list (XWayland fallback)", AutoInstall: true},
	},
}

func main() {
	config.LoadDotEnv()

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "daemon":
			runDaemon(os.Args[2:])
			return
		case "setup":
			runSetup(os.Args[2:])
			return
		case "uninstall":
			runUninstall()
			return
		case "version":
			printVersion()
			return
		case "update":
			runUpdate()
			return
		case "start":
			exec.Command("systemctl", "--user", "start", "rsk-node").Run()
			return
		case "stop":
			exec.Command("systemctl", "--user", "stop", "rsk-node").Run()
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
		}
	}
	printUsage()
	os.Exit(2)
}

func runDaemon(args []string) {
	cfgPath := flag.String("config", defaultConfigPath(), "path to config file")
	flag.Parse()

	cfg, err := config.LoadNodeConfig(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pol, err := policy.New(cfg.AllowCmd, cfg.DenyCmd, cfg.DenyPath, cfg.AllowGUI)
	if err != nil {
		log.Fatalf("policy: %v", err)
	}
	if len(cfg.AllowCmd) > 0 {
		log.Printf("policy: ALLOW_CMD active (%d patterns)", len(cfg.AllowCmd))
	}
	if len(cfg.DenyCmd) > 0 {
		log.Printf("policy: DENY_CMD active (%d patterns)", len(cfg.DenyCmd))
	}
	if len(cfg.DenyPath) > 0 {
		log.Printf("policy: DENY_PATH active (%d patterns)", len(cfg.DenyPath))
	}
	var auditLog *audit.Logger
	if cfg.AuditEnabled && cfg.AuditPath != "" {
		auditLog, err = audit.Open(cfg.AuditPath)
		if err != nil {
			log.Fatalf("audit: %v", err)
		}
		defer auditLog.Close()
		log.Printf("audit: writing to %s", cfg.AuditPath)
	}

	deps := &handlers.Deps{
		Policy:   pol,
		Audit:    auditLog,
		DeviceID: cfg.DeviceID,
	}

	n := node.New(cfg.ServerURL, cfg.DeviceID, cfg.Token)

	n.Register(proto.TypeExec, deps.WrapExec())
	n.Register(proto.TypeReadFile, deps.WrapReadFile())
	n.Register(proto.TypeWriteFile, deps.WrapWriteFile())
	n.Register(proto.TypeListDir, deps.WrapListDir())
	n.Register(proto.TypeScreenshot, deps.WrapScreenshot())
	n.Register(proto.TypeClick, deps.WrapClick())
	n.Register(proto.TypeType, deps.WrapType())
	n.Register(proto.TypeKey, deps.WrapKey())
	n.Register(proto.TypeMouse, deps.WrapMouse())
	n.Register(proto.TypeScroll, deps.WrapScroll())
	n.Register(proto.TypeClipboardRead, deps.WrapClipboardRead())
	n.Register(proto.TypeClipboardWrite, deps.WrapClipboardWrite())
	n.Register(proto.TypeDrag, deps.WrapDrag())
	n.Register(proto.TypeBoard, deps.WrapBoard())
	n.Register(proto.TypeWindows, deps.WrapWindows())
	n.Register(proto.TypeAccessibilityTree, deps.WrapAccessibilityTree())
	n.Register(proto.TypeMonitors, deps.WrapMonitors())
	n.Register(proto.TypeCursorPos, deps.WrapCursorPos())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down")
		cancel()
	}()

	if err := n.RunForever(ctx, time.Duration(cfg.ReconnectSec)*time.Second); err != nil && err != context.Canceled {
		log.Fatalf("node stopped: %v", err)
	}
}

func runSetup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	serverURL := fs.String("server", "", "broker WS URL (e.g. ws://vps:7777/agent)")
	deviceID := fs.String("device", "", "device identifier")
	token := fs.String("token", "", "shared auth token")
	allowGUI := fs.Bool("allow-gui", true, "enable GUI actions")
	uninstall := fs.Bool("uninstall", false, "remove installation")
	_ = fs.Parse(args)

	if *uninstall {
		runUninstall()
		return
	}

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "rsk")
	configPath := filepath.Join(configDir, "node.env")
	os.MkdirAll(configDir, 0755)

	// Check for existing config
	existingConfig := map[string]string{}
	useExisting := false

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  ⚠ %s already exists\n", configPath)
		fmt.Print("  Use existing configuration? [Y/n]: ")
		var yn string
		fmt.Scanln(&yn)
		useExisting = yn != "n" && yn != "N"
		existingConfig, _ = config.LoadEnvFile(configPath)
		if useExisting {
			fmt.Println("  ℹ using existing configuration")
		}
	}

	// 1. UDEV rule for /dev/uinput
	fmt.Println("==> setting up /dev/uinput permission...")
	if err := setupUdev(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v (GUI input may not work)\n", err)
	} else {
		fmt.Println("  ✔ /dev/uinput accessible")
	}

	// 1.5 Dependencies
	env := detectDesktop()
	fmt.Printf("==> checking dependencies... [%s]\n", env)
	pm := detectPM()
	missing := checkDeps(env)
	var autoPkgs []string
	var warnMissing []dep
	for _, d := range missing {
		if d.AutoInstall {
			autoPkgs = append(autoPkgs, d.Pkg)
		} else {
			warnMissing = append(warnMissing, d)
		}
	}
	if len(autoPkgs) > 0 {
		fmt.Printf("  ℹ missing: %s\n", strings.Join(autoPkgs, ", "))
		fmt.Print("  Install automatically? [Y/n]: ")
		var yn string
		fmt.Scanln(&yn)
		if yn != "n" && yn != "N" {
			if err := installPkgs(pm, autoPkgs); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: auto-install failed: %v (install manually later)\n", err)
			} else {
				fmt.Println("  ✔ dependencies installed")
			}
		} else {
			for _, d := range missing {
				if d.AutoInstall {
					warnMissing = append(warnMissing, d)
				}
			}
		}
	}
	for _, d := range warnMissing {
		if d.Pkg != "" {
			fmt.Printf("  ⚠ %s not found (%s) — install: %s\n", d.Name, d.Desc, pmInstallCmd(pm, d.Pkg))
		} else {
			fmt.Printf("  ⚠ %s not found (%s) — install manually\n", d.Name, d.Desc)
		}
	}

	// 1.6 GNOME Shell extension
	if env == "gnome" {
		fmt.Println("==> installing GNOME Shell extension...")
		extDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "gnome-shell", "extensions", "rsk-windows@arosyihuddin.github.com")
		os.MkdirAll(extDir, 0755)

		extJS, err := gnomeExt.ReadFile("gnome-ext/extension.js")
		if err == nil {
			os.WriteFile(filepath.Join(extDir, "extension.js"), extJS, 0644)
		}
		meta, err := gnomeExt.ReadFile("gnome-ext/metadata.json")
		if err == nil {
			os.WriteFile(filepath.Join(extDir, "metadata.json"), meta, 0644)
		}
		exec.Command("gnome-extensions", "enable", "rsk-windows@arosyihuddin.github.com").Run()
		fmt.Println("  ✔ extension installed")
		fmt.Println("  ℹ restart GNOME Shell (Alt+F2, r) or logout/login to activate")
	}

	// 2. Config
	prompt := func(label, def string) string {
		fmt.Printf("  %s [%s]: ", label, def)
		var input string
		fmt.Scanln(&input)
		if input == "" {
			return def
		}
		return input
	}

	existingVal := func(key, fallback string) string {
		if v, ok := existingConfig[key]; ok && v != "" {
			return v
		}
		return fallback
	}

	sv := *serverURL
	if sv == "" {
		def := existingVal("SERVER_URL", "ws://127.0.0.1:7777/agent")
		if useExisting {
			sv = def
		} else {
			sv = prompt("Server URL", def)
		}
	}
	di := *deviceID
	if di == "" {
		host, _ := os.Hostname()
		def := existingVal("DEVICE_ID", host)
		if useExisting {
			di = def
		} else {
			di = prompt("Device ID", def)
		}
	}
	tk := *token
	if tk == "" {
		def := existingVal("TOKEN", "")
		if def != "" {
			tk = def
		} else if data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "rsk", "rsk.env")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "TOKEN=") {
					tk = strings.TrimSpace(line[6:])
					fmt.Printf("  ✔ token auto-read from daemon config\n")
				}
			}
		}
		if tk == "" {
			fmt.Println("  Token: get from `rsk token` on daemon machine, or check ~/.config/rsk/rsk.env")
			tk = prompt("  Token", "dev")
		}
	}

	if useExisting {
		if v, ok := existingConfig["ALLOW_GUI"]; ok && v != "" {
			*allowGUI = v == "true" || v == "1" || v == "yes"
		}
	}

	content := fmt.Sprintf(`SERVER_URL=%s
DEVICE_ID=%s
TOKEN=%s
ALLOW_GUI=%v
`, sv, di, tk, *allowGUI)

	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		log.Fatalf("error writing config: %v", err)
	}
	fmt.Printf("  ✔ config saved: %s\n", configPath)

	// 3. Systemd user unit
	unitDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	os.MkdirAll(unitDir, 0755)
	unitPath := filepath.Join(unitDir, "rsk-node.service")
	if err := os.WriteFile(unitPath, serviceUnit, 0644); err != nil {
		log.Fatalf("error writing service unit: %v", err)
	}
	fmt.Printf("  ✔ service unit: %s\n", unitPath)

	// 4. Copy binary to ~/.local/bin/rsk-node
	binDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	os.MkdirAll(binDir, 0755)
	self, _ := os.Executable()
	binTarget := filepath.Join(binDir, "rsk-node")
	if self != binTarget {
		data, err := os.ReadFile(self)
		if err == nil {
			os.WriteFile(binTarget, data, 0755)
			fmt.Printf("  ✔ binary copied: %s\n", binTarget)
		}
	} else {
		fmt.Printf("  ✔ binary already in place: %s\n", binTarget)
	}

	// 5. Enable & start
	fmt.Println("==> starting service...")
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	exec.Command("systemctl", "--user", "enable", "rsk-node").Run()
	if out, err := exec.Command("systemctl", "--user", "restart", "rsk-node").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: restart failed: %v\n%s\n", err, out)
	} else {
		fmt.Println("  ✔ rsk-node service started")
	}

	fmt.Println("\n✅ rsk-node installed and running")
}

func runUninstall() {
	// Stop & disable service
	fmt.Println("==> stopping service...")
	exec.Command("systemctl", "--user", "stop", "rsk-node").Run()
	exec.Command("systemctl", "--user", "disable", "rsk-node").Run()

	// Remove service unit
	unitPath := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user", "rsk-node.service")
	os.Remove(unitPath)
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	fmt.Println("  ✔ service removed")

	// Remove udev rule
	rulePath := "/etc/udev/rules.d/99-rsk-uinput.rules"
	if _, err := os.Stat(rulePath); err == nil {
		if out, err := exec.Command("sudo", "rm", rulePath).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: remove udev rule: %v\n%s\n", err, out)
		} else {
			exec.Command("sudo", "udevadm", "control", "--reload-rules").Run()
			fmt.Println("  ✔ udev rule removed")
		}
	}

	// Remove GNOME Shell extension
	extDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "gnome-shell", "extensions", "rsk-windows@arosyihuddin.github.com")
	if _, err := os.Stat(extDir); err == nil {
		exec.Command("gnome-extensions", "disable", "rsk-windows@arosyihuddin.github.com").Run()
		os.RemoveAll(extDir)
		fmt.Println("  ✔ GNOME extension removed")
	}

	os.Remove(filepath.Join(os.Getenv("HOME"), ".local", "bin", "rsk-node"))
	fmt.Println("  ✔ binary removed")

	configPath := filepath.Join(os.Getenv("HOME"), ".config", "rsk", "node.env")
	os.Remove(configPath)
	fmt.Println("  ✔ config removed")

	fmt.Println("\n✅ rsk-node uninstalled")
}

func setupUdev() error {
	rulePath := "/etc/udev/rules.d/99-rsk-uinput.rules"
	if _, err := os.Stat(rulePath); os.IsNotExist(err) {
		cmd := exec.Command("sudo", "sh", "-c",
			fmt.Sprintf("echo '%s' > %s && udevadm control --reload-rules && udevadm trigger", udevRule, rulePath))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("sudo udev setup failed: %w\n%s", err, out)
		}
	}
	if out, err := exec.Command("sudo", "chmod", "0666", "/dev/uinput").CombinedOutput(); err != nil {
		return fmt.Errorf("sudo chmod /dev/uinput: %w\n%s", err, out)
	}
	return nil
}

func processRunning(name string) bool {
	out, _ := exec.Command("pgrep", "-x", name).Output()
	return len(out) > 0
}

func detectDesktop() string {
	switch {
	case processRunning("Hyprland"):
		return "hyprland"
	case processRunning("sway"), processRunning("river"), processRunning("wayfire"):
		return "wayland"
	case processRunning("gnome-shell"):
		return "gnome"
	case processRunning("mutter"), processRunning("kwin_wayland"):
		return "wayland"
	case processRunning("Xorg"), processRunning("X"):
		return "x11"
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return "wayland"
	case os.Getenv("DISPLAY") != "":
		return "x11"
	default:
		return "x11"
	}
}

func detectPM() string {
	for _, pm := range []string{"apt", "dnf", "yum", "pacman", "zypper", "apk"} {
		if _, err := exec.LookPath(pm); err == nil {
			return pm
		}
	}
	return "apt"
}

func checkDeps(env string) []dep {
	var missing []dep
	for _, d := range depsByEnv[env] {
		if _, err := exec.LookPath(d.Name); err != nil {
			missing = append(missing, d)
		}
	}
	return missing
}

func pmInstallCmd(pm, pkg string) string {
	switch pm {
	case "pacman":
		return fmt.Sprintf("sudo pacman -S %s", pkg)
	case "apk":
		return fmt.Sprintf("sudo apk add %s", pkg)
	case "apt", "dnf", "yum", "zypper":
		return fmt.Sprintf("sudo %s install -y %s", pm, pkg)
	default:
		return fmt.Sprintf("sudo %s install -y %s", pm, pkg)
	}
}

func installPkgs(pm string, pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("sudo not found, install manually")
	}
	args := []string{}
	switch pm {
	case "apt":
		args = []string{"install", "-y"}
	case "dnf", "yum", "zypper":
		args = []string{"install", "-y"}
	case "pacman":
		args = []string{"-S", "--noconfirm"}
	case "apk":
		args = []string{"add"}
	}
	args = append(args, pkgs...)
	cmd := exec.Command("sudo", append([]string{pm}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func defaultConfigPath() string {
	if h, err := os.UserConfigDir(); err == nil {
		return h + "/rsk/node.env"
	}
	return ""
}

func printVersion() {
	fmt.Printf("rsk-node version %s\n", node.Version)
}

func runStatus() {
	fmt.Printf("rsk-node %s\n", node.Version)

	out, err := exec.Command("systemctl", "--user", "is-active", "rsk-node").Output()
	if err != nil {
		fmt.Println("  Service: not installed")
		return
	}
	fmt.Printf("  Service: %s\n", strings.TrimSpace(string(out)))

	configPath := filepath.Join(os.Getenv("HOME"), ".config", "rsk", "node.env")
	if m, err := config.LoadEnvFile(configPath); err == nil {
		if v, ok := m["SERVER_URL"]; ok {
			fmt.Printf("  Server:  %s\n", v)
		}
		if v, ok := m["DEVICE_ID"]; ok {
			fmt.Printf("  Device:  %s\n", v)
		}
	}
}

func runInfo() {
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "rsk", "node.env")
	m, err := config.LoadEnvFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: config not found at %s\n", configPath)
		os.Exit(1)
	}

	fmt.Printf("Config: %s\n", configPath)
	if v, ok := m["SERVER_URL"]; ok {
		fmt.Printf("  Server:   %s\n", v)
	}
	if v, ok := m["DEVICE_ID"]; ok {
		fmt.Printf("  Device:   %s\n", v)
	}
	if v, ok := m["TOKEN"]; ok && v != "" {
		masked := v
		if len(v) > 8 {
			masked = v[:8] + "..." + v[len(v)-4:]
		}
		fmt.Printf("  Token:    %s\n", masked)
	}
	if v, ok := m["ALLOW_GUI"]; ok {
		fmt.Printf("  AllowGUI: %s\n", v)
	}
}

func runRestart() {
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	if out, err := exec.Command("systemctl", "--user", "restart", "rsk-node").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "error: restart failed: %v\n%s\n", err, out)
		os.Exit(1)
	}
	fmt.Println("✔ rsk-node restarted")
}

func runLog(args []string) {
	cmdArgs := append([]string{"--user", "-u", "rsk-node", "-n", "50"}, args...)
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
	asset := update.AssetName("rsk-node")
	fmt.Printf("Downloading latest %s ...\n", asset)

	tmp, err := update.DownloadLatest("arosyihuddin", "remote-skill", asset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmp)

	if ver, err := update.VersionOf(tmp); err == nil {
		if ver == node.Version {
			if node.Version != "dev" {
				fmt.Printf("Already up to date (%s)\n", ver)
				return
			}
			fmt.Printf("Version %s (dev), updating anyway...\n", ver)
		} else {
			fmt.Printf("Updating %s -> %s ...\n", node.Version, ver)
		}
	} else {
		fmt.Printf("Warning: cannot check version (%v), proceeding...\n", err)
	}

	if err := update.ReplaceSelf(tmp); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Binary updated, restarting service...")

	if err := update.RestartService("rsk-node"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		fmt.Println("Restart manually: systemctl --user restart rsk-node")
		os.Exit(1)
	}
	fmt.Println("rsk-node restarted")
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: rsk-node <command> [args...]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  daemon          Start the node daemon\n")
	fmt.Fprintf(os.Stderr, "  setup           Install and configure as systemd user service\n")
	fmt.Fprintf(os.Stderr, "  uninstall       Remove installation\n")
	fmt.Fprintf(os.Stderr, "  version         Print version\n")
	fmt.Fprintf(os.Stderr, "  update          Self-update from GitHub\n")
	fmt.Fprintf(os.Stderr, "  start           Start systemd service\n")
	fmt.Fprintf(os.Stderr, "  stop            Stop systemd service\n")
	fmt.Fprintf(os.Stderr, "  restart         Restart systemd service\n")
	fmt.Fprintf(os.Stderr, "  status          Show service status\n")
	fmt.Fprintf(os.Stderr, "  info            Show config summary\n")
	fmt.Fprintf(os.Stderr, "  logs [args]      Tail journal logs\n")
	fmt.Fprintf(os.Stderr, "\nFlags for setup:\n")
	fmt.Fprintf(os.Stderr, "  --server URL     Broker WS address\n")
	fmt.Fprintf(os.Stderr, "  --device NAME    Device identifier\n")
	fmt.Fprintf(os.Stderr, "  --token SECRET   Auth token (default: dev)\n")
	fmt.Fprintf(os.Stderr, "  --allow-gui      Enable GUI actions (default: true)\n")
	fmt.Fprintf(os.Stderr, "  --uninstall       Remove installation\n")
}
