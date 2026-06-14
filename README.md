# remote-skill 🔌

[![Linux](https://img.shields.io/badge/platform-linux-blue)]()

> Bridge for AI agents (or CLI) to control a Linux workstation in real-time over WebSocket.

---

## Quick Start ⚡

```bash
# server (VPS)
wget https://github.com/arosyihuddin/remote-skill/releases/latest/download/rsk-linux-amd64 -O rsk
chmod +x rsk
./rsk setup

# node (laptop)
wget https://github.com/arosyihuddin/remote-skill/releases/latest/download/rsk-node-linux-amd64 -O rsk-node
chmod +x rsk-node
./rsk-node setup --server ws://vps:7777/agent
```

Done. Service runs automatically — `systemctl --user rsk` / `rsk-node`.

---

## Features

| Category | Commands |
|---|---|
| **Shell** | `exec`, `read`, `write`, `ls` |
| **GUI** | `screenshot`, `click`, `type`, `key`, `mouse`, `scroll`, `drag` |
| **Clipboard** | `clip get`, `clip set`, `board` (write + paste) |
| **System** | `windows` (list windows), `a11y` (accessibility tree) |
| **Device** | `devices` (list connected), multi-device via ID prefix |
| **Service** | `setup`, `uninstall`, `version`, `status`, `info`, `restart`, `log` |
| **Update** | `update` (self-update from GitHub), `env`, `wait` |

---

## Architecture

```
┌──────────────────────────┐         ┌──────────────────────────┐
│  VPS                     │         │  Laptop / device         │
│                          │  WS     │                          │
│  ┌────────────────────┐  │ outbound│  ┌────────────────────┐  │
│  │ rsk (daemon)       │<─┼─────────┼──│ rsk-node           │  │
│  │ :7777 (broker)     │  │         │  │ (daemon)           │  │
│  │ :7800 (monitor)    │  │         │  └────────────────────┘  │
│  └────────────────────┘  │         │   exec / file / GUI      │
│         │                 │         │                          │
│    rsk exec "ls"         │         └──────────────────────────┘
│    (CLI mode, WS)        │
└──────────────────────────┘
```

**Components:**

| Component | Role |
|---|---|
| `cmd/rsk` | Broker daemon (`:7777` WS + `:7800` HTTP monitor) + CLI mode |
| `cmd/node` | Client daemon on laptop — dials out to broker, executes requests |
| `internal/cli` | WS-based CLI — connect, send request, print response |
| `internal/broker` | Device registry + request routing |
| `internal/handlers` | exec / file / GUI handlers (Linux/Wayland) |
| `internal/proto` | JSON wire protocol |

---

## Usage

### CLI

```bash
# Single device — auto-detect
rsk exec "ls -la"
rsk screenshot

# Multi-device — specify device-id
rsk my-laptop exec "echo hi"

# Or via env
export RSK_DEVICE=my-laptop
rsk exec "ls -la"
```

### Service management

```bash
rsk status              # service status + config
rsk info                # config summary (token masked)
rsk restart             # restart service
rsk log                 # tail 50 journal entries
rsk log -f              # follow logs
```

### Self-update

```bash
rsk update              # update daemon from GitHub release
rsk-node update         # update node
```

Binary is atomically replaced and service restarted automatically.

---

## Setup guides

### Daemon

```bash
rsk setup
# or:  rsk setup --agent 0.0.0.0:7777 --monitor 127.0.0.1:7800 --token mytoken
```

Config: `~/.config/rsk/rsk.env`. Service: `systemctl --user rsk`.

### Node

```bash
rsk-node setup --server ws://vps:7777/agent
# or:  rsk-node setup --server ws://vps:7777/agent --device my-laptop --token secret123
```

Auto-configures:
- `/dev/uinput` udev rule (keyboard/mouse)
- Token auto-read from daemon config (same machine)
- Config `~/.config/rsk/node.env`
- Binary copy to `~/.local/bin/rsk-node`
- systemd user service install + enable

---

## Development

```bash
git clone https://github.com/arosyihuddin/remote-skill.git
cd remote-skill
./build.sh

# Setup from local build
./bin/rsk setup
./bin/rsk-node setup
```

Build release binaries (cross-compiled for `linux/amd64`):

```bash
./build.sh release
# output: bin/rsk-linux-amd64 + bin/rsk-node-linux-amd64
```

### Smoke test (local, no VPS)

```bash
# terminal 1 — broker
RSK_TOKEN=test123 rsk daemon

# terminal 2 — node
RSK_TOKEN=test123 rsk-node

# terminal 3 — CLI
rsk exec "echo hi"
```

---

## Multi-device

Each device needs a unique `DEVICE_ID` in its config (e.g. `laptop-pstar7`, `desktop-home`). CLI can specify the target as the first argument. Auto-selected if only one device is connected.

---

## Security

- Single shared `TOKEN` authenticates both nodes and CLI clients.
- Monitoring HTTP (`:7800`) should bind to `127.0.0.1` — change to `0.0.0.0` only if remote browser access is needed with auth.
- WS broker (`:7777`) should bind to mesh VPN IP only (e.g. `100.116.138.90:7777`).
- Anyone with the token can execute commands. Treat `TOKEN` as a root credential.

---

## License

MIT
