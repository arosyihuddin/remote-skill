<p align="center">
  <img src="https://img.shields.io/badge/platform-linux-blue?style=flat-square">
  <img src="https://img.shields.io/badge/go-1.23+-00ADD8?style=flat-square&logo=go">
  <img src="https://img.shields.io/github/v/release/arosyihuddin/remote-skill?style=flat-square">
  <img src="https://img.shields.io/github/license/arosyihuddin/remote-skill?style=flat-square">
</p>

<h1 align="center">remote-skill</h1>
<p align="center">
  <em>Realtime remote control for Linux workstations — shell, files, GUI, clipboard.</em>
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> •
  <a href="#features">Features</a> •
  <a href="#usage">Usage</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#installation">Installation</a> •
  <a href="#configuration">Configuration</a>
</p>

---

## Quick Start

**Two machines, five commands.**

```bash
# ── server (VPS) ──────────────────────────────────────
wget https://github.com/arosyihuddin/remote-skill/releases/latest/download/rsk-linux-amd64 -O rsk
chmod +x rsk && ./rsk setup

# ── laptop (node) ─────────────────────────────────────
wget https://github.com/arosyihuddin/remote-skill/releases/latest/download/rsk-node-linux-amd64 -O rsk-node
chmod +x rsk-node && ./rsk-node setup
```

Both run as systemd user services. Done.

```bash
# Test it:
rsk exec "uname -a"
rsk screenshot
```

---

## Features

| Capability | Commands | What it does |
|---|---|---|
| **Shell execution** | `exec` | Run any command, stream stdout/stderr, timeout |
| **File operations** | `read`, `write`, `ls` | Transfer files, edit configs, browse directories |
| **Screenshots** | `screenshot` | Capture full screen or specific output |
| **Input automation** | `click`, `type`, `key`, `mouse`, `scroll`, `drag` | Click buttons, type text, send shortcuts, drag & drop |
| **Clipboard** | `clip`, `board` | Read/write clipboard, or write + paste in one shot |
| **System introspection** | `windows`, `a11y` | List open windows, dump accessibility tree |
| **Device management** | `devices` | List connected nodes, target specific device |
| **Service lifecycle** | `setup`, `uninstall`, `restart`, `status`, `info`, `log` | Install, manage, and monitor the daemon |
| **Self-update** | `update` | Download latest release and restart automatically |

<details>
<summary><b>Full command reference</b></summary>

```bash
rsk exec "<cmd>" [--shell] [--cwd PATH] [--timeout N] [--stream]
rsk read <path> [--binary] [--max N]
rsk write <path> [--file LOCAL] [--append]
rsk ls <path> [--hidden]
rsk screenshot [--region WxH+X+Y] [--output NAME]
rsk click [--x N] [--y N] [--button left|right|middle] [--double]
rsk type "<text>"
rsk key "<combo>"              # e.g. "ctrl+c", "Return", "Super+v"
rsk mouse <x> <y> [--relative]
rsk scroll [--dy N] [--up|--down]
rsk drag <x1> <y1> <x2> <y2> [--button left|right|middle]
rsk clip get|set "<text>"
rsk board "<text>"             # clipboard write + paste
rsk windows
rsk a11y
rsk devices
rsk wait <sec>
rsk env
```
</details>

---

## Usage

### Command syntax

```bash
rsk <command> [args...]              # auto-selects single device
rsk <device-id> <command> [args...]  # target specific node
```

### Service commands

```bash
rsk version           # print version
rsk status            # service status + config summary
rsk info              # detailed config (token masked)
rsk restart           # restart daemon
rsk log -n 100        # last 100 journal entries
rsk log -f            # follow logs
rsk update            # self-update from GitHub release
```

### Multi-device example

```bash
# Two laptops connected. Auto-detection won't work — specify target.
rsk desktop-home exec "echo hello"
rsk macbook-work exec "echo hello"
rsk desktop-home screenshot --save ~/shot.png
```

---

## Architecture

```
  ┌────────────────────────────────────────────────────────────────┐
  │                        VPS / Server                            │
  │                                                                │
  │  ┌─────────────────────────────────────────┐                   │
  │  │   rsk daemon                            │                   │
  │  │                                         │                   │
  │  │  ┌─────────┐   ┌──────────────┐        │                   │
  │  │  │ WS      │   │ HTTP monitor  │        │                   │
  │  │  │ broker  │   │ :7800         │        │                   │
  │  │  │ :7777   │   │ (auth req)    │        │                   │
  │  │  └────┬────┘   └──────────────┘        │                   │
  │  └───────┼─────────────────────────────────┘                   │
  └──────────┼─────────────────────────────────────────────────────┘
             │ WebSocket
  ┌──────────┼─────────────────────────────────────────────────────┐
  │  ┌───────┴──────┐                                             │
  │  │   rsk-node   │  ┌──────────────────────────────────────┐   │
  │  │  (daemon)    │  │  Handlers:                            │   │
  │  │              │  │  exec → read → write → ls             │   │
  │  │  Dial-out    │  │  screenshot → click → type → key      │   │
  │  │  connects to │  │  mouse → scroll → drag → clipboard    │   │
  │  │  broker      │  │  windows → a11y                       │   │
  │  └──────────────┘  └──────────────────────────────────────┘   │
  │                        Laptop / Workstation                    │
  └────────────────────────────────────────────────────────────────┘
```

**How it works:**

1. `rsk` daemon listens on `:7777` (WebSocket broker) and `:7800` (HTTP monitor).
2. `rsk-node` connects outbound to the broker — no open ports needed on the laptop.
3. CLI connects to `:7777/cli`, sends a command frame, broker forwards to the right node.
4. Node executes, streams back response, CLI prints it.

---

## Installation

### Option 1: Binary download (recommended)

Download the latest release binary. No Go toolchain needed.

```bash
# Find the latest version:
# https://github.com/arosyihuddin/remote-skill/releases

wget https://github.com/arosyihuddin/remote-skill/releases/latest/download/rsk-linux-amd64 -O rsk
wget https://github.com/arosyihuddin/remote-skill/releases/latest/download/rsk-node-linux-amd64 -O rsk-node
chmod +x rsk rsk-node
```

### Option 2: Build from source

```bash
git clone https://github.com/arosyihuddin/remote-skill.git
cd remote-skill
./build.sh

# Build release binaries (cross-compiled linux/amd64):
./build.sh release
```

### Smoke test

```bash
# terminal 1 — broker (VPS)
RSK_TOKEN=test123 ./bin/rsk daemon

# terminal 2 — node (laptop)
RSK_TOKEN=test123 ./bin/rsk-node

# terminal 3 — CLI
./bin/rsk exec "echo hello from remote"
```

---

## Configuration

Both daemon and node use simple `KEY=VALUE` config files.

### Daemon (`~/.config/rsk/rsk.env`)

```
AGENT_LISTEN=0.0.0.0:7777     # WS broker address
SKILL_LISTEN=127.0.0.1:7800   # HTTP monitor address
TOKEN=<auto-generated>         # shared secret
```

Set via `rsk setup` or edit manually. Env overrides: `RSK_AGENT_LISTEN`, `RSK_MONITOR`, `RSK_TOKEN`.

### Node (`~/.config/rsk/node.env`)

```
SERVER_URL=ws://vps:7777/agent  # broker URL
DEVICE_ID=my-laptop              # unique identifier
TOKEN=<same-as-daemon>           # must match daemon
ALLOW_GUI=true                   # enable screenshot, click, etc.
```

Set via `rsk-node setup` or edit manually. Env overrides: `RSK_NODE_SERVER_URL`, `RSK_NODE_DEVICE_ID`, `RSK_TOKEN`.

---

## Security

- **Single shared token** authenticates nodes, CLI clients, and HTTP monitor.
- **WS broker** (`:7777`) should bind to a mesh VPN IP, not `0.0.0.0`.
- **HTTP monitor** (`:7800`) requires `Authorization: Bearer <token>` or `?token=` query param.
- Treat the **TOKEN** as a root credential — anyone with it can execute arbitrary commands on connected nodes.

---

## License

[MIT](LICENSE)
