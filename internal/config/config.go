// Package config loads simple TOML-like configs without external deps.
// We use a minimal KEY=VALUE format (env-style) for simplicity.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ServerConfig is loaded by the VPS-side broker.
type ServerConfig struct {
	// AgentListen is the address WS agents connect to (e.g. "100.116.138.90:7777").
	AgentListen string
	// SkillListen is the local HTTP API address for the skill (e.g. "127.0.0.1:7800").
	SkillListen string
	// Token authenticates agents (must match agent's token).
	Token string
}

// NodeConfig is loaded by the laptop daemon.
type NodeConfig struct {
	// ServerURL is the WS URL of the broker, e.g. "ws://100.116.138.90:7777/agent".
	ServerURL string
	// DeviceID is a stable identifier like "laptop-pstar7".
	DeviceID string
	// Token must match the server token.
	Token string
	// ReconnectSec is the base reconnect delay in seconds.
	ReconnectSec int

	// --- Policy (opt-in: empty fields = current permissive behavior) -----

	// AllowCmd is a list of regex patterns (anchored with ^ $ recommended).
	// If non-empty, exec is rejected unless argv[0] matches one of these.
	// Comma-separated in env file. e.g. `^(ls|cat|git|go|make)$`
	AllowCmd []string
	// DenyCmd is a list of regex patterns. Always rejected when matched,
	// even if AllowCmd permits the command.
	DenyCmd []string
	// DenyPath is a list of glob patterns matched against absolute file paths
	// for read/write/list. Examples: "/etc/shadow", "/root/**", "**/.ssh/**".
	DenyPath []string
	// AllowGUI toggles screenshot/click/type/key/mouse. Default true.
	AllowGUI bool


	// --- Audit ----------------------------------------------------------

	// AuditPath is the JSONL file path. Empty = audit disabled.
	// Tilde-expansion supported. Default: ~/.local/state/remote-agent/audit.log
	AuditPath string
	// AuditEnabled flips audit on/off. If true and AuditPath empty, default is used.
	AuditEnabled bool
}

// LoadEnvFile parses simple KEY=VALUE files (comments with #, blank lines ok).
func LoadEnvFile(path string) (map[string]string, error) {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			return nil, fmt.Errorf("invalid line: %q", line)
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		v = strings.Trim(v, `"'`)
		out[k] = v
	}
	return out, scan.Err()
}

// LoadServerConfig reads from path, then overlays env vars with prefix RSK_.
func LoadServerConfig(path string) (ServerConfig, error) {
	c := ServerConfig{
		AgentListen: "0.0.0.0:7777",
		SkillListen: "127.0.0.1:7800",
	}
	if path != "" {
		m, err := LoadEnvFile(path)
		if err != nil && !os.IsNotExist(err) {
			return c, err
		}
		if v, ok := m["AGENT_LISTEN"]; ok {
			c.AgentListen = v
		}
		if v, ok := m["SKILL_LISTEN"]; ok {
			c.SkillListen = v
		}
		if v, ok := m["TOKEN"]; ok {
			c.Token = v
		}
	}
	if v := os.Getenv("RSK_AGENT_LISTEN"); v != "" {
		c.AgentListen = v
	}
	if v := os.Getenv("RSK_MONITOR"); v != "" {
		c.SkillListen = v
	}
	if v := os.Getenv("RSK_TOKEN"); v != "" {
		c.Token = v
	}
	if c.Token == "" {
		return c, fmt.Errorf("server token is empty (set TOKEN in config or RSK_TOKEN env)")
	}
	return c, nil
}

// LoadNodeConfig reads node config.
func LoadNodeConfig(path string) (NodeConfig, error) {
	c := NodeConfig{
		ReconnectSec: 3,
		AllowGUI:     true,
	}
	if path != "" {
		m, err := LoadEnvFile(path)
		if err != nil && !os.IsNotExist(err) {
			return c, err
		}
		if v, ok := m["SERVER_URL"]; ok {
			c.ServerURL = v
		}
		if v, ok := m["DEVICE_ID"]; ok {
			c.DeviceID = v
		}
		if v, ok := m["TOKEN"]; ok {
			c.Token = v
		}
		if v, ok := m["RECONNECT_SEC"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				c.ReconnectSec = n
			}
		}
		if v, ok := m["ALLOW_CMD"]; ok {
			c.AllowCmd = splitCSV(v)
		}
		if v, ok := m["DENY_CMD"]; ok {
			c.DenyCmd = splitCSV(v)
		}
		if v, ok := m["DENY_PATH"]; ok {
			c.DenyPath = splitCSV(v)
		}
		if v, ok := m["ALLOW_GUI"]; ok {
			c.AllowGUI = parseBool(v)
		}
		if v, ok := m["AUDIT_PATH"]; ok {
			c.AuditPath = v
			c.AuditEnabled = true
		}
		if v, ok := m["AUDIT"]; ok {
			c.AuditEnabled = parseBool(v)
		}
	}
	if v := os.Getenv("RSK_NODE_SERVER_URL"); v != "" {
		c.ServerURL = v
	}
	if v := os.Getenv("RSK_NODE_DEVICE_ID"); v != "" {
		c.DeviceID = v
	}
	if v := os.Getenv("RSK_TOKEN"); v != "" {
		c.Token = v
	}
	if v := os.Getenv("RSK_NODE_ALLOW_CMD"); v != "" {
		c.AllowCmd = splitCSV(v)
	}
	if v := os.Getenv("RSK_NODE_DENY_CMD"); v != "" {
		c.DenyCmd = splitCSV(v)
	}
	if v := os.Getenv("RSK_NODE_DENY_PATH"); v != "" {
		c.DenyPath = splitCSV(v)
	}
	if v := os.Getenv("RSK_NODE_ALLOW_GUI"); v != "" {
		c.AllowGUI = parseBool(v)
	}
	if v := os.Getenv("RSK_NODE_AUDIT_PATH"); v != "" {
		c.AuditPath = v
		c.AuditEnabled = true
	}
	if v := os.Getenv("RSK_NODE_AUDIT"); v != "" {
		c.AuditEnabled = parseBool(v)
	}
	if c.ServerURL == "" || c.Token == "" || c.DeviceID == "" {
		return c, fmt.Errorf("missing required config: SERVER_URL, DEVICE_ID, TOKEN")
	}
	if c.AuditEnabled && c.AuditPath == "" {
		// Default location: ~/.local/state/rsk-node/audit.log
		if h, err := os.UserHomeDir(); err == nil {
			c.AuditPath = h + "/.local/state/rsk-node/audit.log"
		}
	}
	return c, nil
}

// splitCSV splits "a, b , c" into ["a","b","c"], trimming and dropping empties.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// LoadDotEnv reads a .env file and sets any variables not already present
// in the environment. Searches CWD, executable dir, and its parent.
func LoadDotEnv() {
	candidates := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, ".env"))
		candidates = append(candidates, filepath.Join(dir, "..", ".env"))
	}
	for _, path := range candidates {
		if loadDotEnvFile(path) {
			return
		}
	}
}

func loadDotEnvFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		v = strings.Trim(v, `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
	return true
}

// parseBool accepts 1/0, true/false, yes/no, on/off (case-insensitive).
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
