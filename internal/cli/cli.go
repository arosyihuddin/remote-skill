// Package cli implements the rsk command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pstar7/remote-skill/internal/config"
)

func init() {
	cleanOldTemp()
}

func cleanOldTemp() {
	dir := os.TempDir()
	entries, _ := os.ReadDir(dir)
	now := time.Now()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "rsk-screenshot-") && !strings.HasPrefix(name, "rsk-read-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > time.Hour {
			_ = os.Remove(dir + "/" + name)
		}
	}
}

func resolveServerURL() string {
	if v := os.Getenv("RSK_SERVER"); v != "" {
		return v
	}
	return "ws://127.0.0.1:7777"
}

func resolveToken() string {
	if v := os.Getenv("RSK_TOKEN"); v != "" {
		return v
	}
	candidates := []string{
		os.Getenv("HOME") + "/.config/rsk/rsk.env",
		os.Getenv("HOME") + "/.config/rsk/config.env",
		"/etc/rsk/config.env",
	}
	for _, p := range candidates {
		m, err := config.LoadEnvFile(p)
		if err != nil {
			continue
		}
		if t, ok := m["TOKEN"]; ok && t != "" {
			return t
		}
	}
	return ""
}

var knownCommands = map[string]bool{
	"exec": true, "read": true, "write": true, "ls": true,
	"screenshot": true, "click": true, "type": true, "key": true,
	"mouse": true, "clip": true, "scroll": true, "devices": true,
	"windows": true, "a11y": true, "monitors": true, "cursorpos": true, "drag": true, "board": true,
	"apps": true, "open": true, "wait": true, "env": true,
	"-h": true, "--help": true, "help": true,
}

func Run(command string, args []string) {
	serverURL := resolveServerURL()
	token := resolveToken()
	device := os.Getenv("RSK_DEVICE")

	// First arg might be device-id instead of command
	if !knownCommands[command] {
		device = command
		if len(args) == 0 {
			printUsage("")
			os.Exit(2)
		}
		command = args[0]
		args = args[1:]
	}

	switch command {
	case "-h", "--help", "help":
		printUsage("")
		return
	case "devices":
		runDevices(serverURL, token)
		return
	case "wait":
		runWait(args)
		return
	case "env":
		runEnv()
		return
	}

	savePath := extractSaveFlag(command, &args)

	payload, err := buildPayload(command, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if payload == nil {
		printUsage(command)
		os.Exit(2)
	}

	streaming := false
	if command == "exec" {
		for _, a := range args {
			if a == "--stream" {
				streaming = true
			}
		}
	}

	subCmd := command
	if command == "clip" && len(args) > 0 && args[0] == "set" {
		subCmd = "clip-set"
	} else if command == "clip" {
		subCmd = "clip-get"
	}
	resp, err := sendRequest(serverURL, token, device, subCmd, payload, streaming)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if streaming {
		for _, chunk := range resp.Stream {
			fmt.Print(chunk)
		}
	}
	if resp.Final != nil {
		switch command {
		case "screenshot":
			printScreenshot(resp.Final.(json.RawMessage), savePath)
		case "read":
			printRead(resp.Final.(json.RawMessage), savePath)
		default:
			b, _ := json.MarshalIndent(resp.Final, "", "  ")
			var textResp struct{ Text string `json:"_text"` }
			if json.Unmarshal(b, &textResp) == nil && textResp.Text != "" {
				fmt.Println(textResp.Text)
			} else {
				fmt.Println(string(b))
			}
		}
	}
}

func extractSaveFlag(cmd string, args *[]string) string {
	if cmd != "screenshot" && cmd != "read" {
		return ""
	}
	filtered := make([]string, 0, len(*args))
	savePath := ""
	for i := 0; i < len(*args); i++ {
		if (*args)[i] == "--save" {
			if i+1 < len(*args) {
				savePath = (*args)[i+1]
				i++
			}
			continue
		}
		filtered = append(filtered, (*args)[i])
	}
	*args = filtered
	return savePath
}

func printUsage(_ string) {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  rsk <command> [args...]\n")
	fmt.Fprintf(os.Stderr, "  rsk <device-id> <command> [args...]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  devices                 List connected devices\n")
	fmt.Fprintf(os.Stderr, "  exec \"<cmd>\"            Run a command on remote node\n")
	fmt.Fprintf(os.Stderr, "  read <path>             Read a file\n")
	fmt.Fprintf(os.Stderr, "  write <path>            Write a file (stdin or --file)\n")
	fmt.Fprintf(os.Stderr, "  ls <path>               List directory\n")
	fmt.Fprintf(os.Stderr, "  screenshot              Capture screenshot\n")
	fmt.Fprintf(os.Stderr, "  click                   Mouse click\n")
	fmt.Fprintf(os.Stderr, "  type \"<text>\"           Type text\n")
	fmt.Fprintf(os.Stderr, "  key \"<combo>\"           Send key combo\n")
	fmt.Fprintf(os.Stderr, "  mouse <x> <y>           Move mouse\n")
	fmt.Fprintf(os.Stderr, "  scroll [--dy N]         Scroll (default -3)\n")
	fmt.Fprintf(os.Stderr, "  drag <x1> <y1> <x2> <y2>  Mouse drag\n")
	fmt.Fprintf(os.Stderr, "  board \"<text>\"          Clipboard write + paste\n")
	fmt.Fprintf(os.Stderr, "  windows                 List windows\n")
	fmt.Fprintf(os.Stderr, "  a11y [--id N] [--depth N] [--role name] [--show-all] [--monitor N] [--all]\n")
	fmt.Fprintf(os.Stderr, "                        Accessibility tree in Toon CSV (--monitor: filter by monitor, --all: all monitors)\n")
	fmt.Fprintf(os.Stderr, "  monitors                List monitors\n")
	fmt.Fprintf(os.Stderr, "  cursorpos               Get current cursor position\n")
	fmt.Fprintf(os.Stderr, "  apps [--filter name]    List installed GUI applications\n")
	fmt.Fprintf(os.Stderr, "  open \"<name>\"           Launch an application by name\n")
	fmt.Fprintf(os.Stderr, "  wait <sec>              Sleep N seconds\n")
	fmt.Fprintf(os.Stderr, "  env                     Show env vars\n")
	fmt.Fprintf(os.Stderr, "  clip get|set            Clipboard operations\n")
}
