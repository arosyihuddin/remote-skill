package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pstar7/remote-skill/internal/cli"
	"github.com/pstar7/remote-skill/internal/config"
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
	fmt.Fprintf(os.Stderr, "  apps [--filter name]  List installed GUI applications\n")
	fmt.Fprintf(os.Stderr, "  open \"<name>\"         Launch an application by name\n")
	fmt.Fprintf(os.Stderr, "  wait <sec>          Sleep N seconds\n")
	fmt.Fprintf(os.Stderr, "  env                 Show env vars\n")
	fmt.Fprintf(os.Stderr, "  clip get|set        Clipboard operations\n")
	fmt.Fprintf(os.Stderr, "  live                Open interactive terminal session\n")
}
