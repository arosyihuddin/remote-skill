package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pstar7/remote-skill/internal/config"
	"github.com/pstar7/remote-skill/internal/update"
)

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
