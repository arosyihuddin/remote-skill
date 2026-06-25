package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pstar7/remote-skill/internal/config"
	"github.com/pstar7/remote-skill/internal/node"
)

//go:embed rsk-node.service
var serviceUnit []byte

//go:embed gnome-ext/*
var gnomeExt embed.FS

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
