# Installation

## Download

Get the latest binary from GitHub Releases:

| Binary | Download |
|---|---|
| **rsk** (server daemon + CLI) | `https://github.com/arosyihuddin/remote-skill/releases/latest/download/rsk-linux-amd64` |
| **rsk-node** (client node) | `https://github.com/arosyihuddin/remote-skill/releases/latest/download/rsk-node-linux-amd64` |

## Setup

### rsk (server daemon + CLI)

```
Usage: rsk setup [--agent ADDR] [--monitor ADDR] [--token SECRET]
```

Installs as systemd user service (`rsk.service`).

- `--agent` — WS broker address (default: `0.0.0.0:7777`)
- `--monitor` — HTTP monitor address (default: `127.0.0.1:7800`)
- `--token` — Auth token. Auto-generated if empty.

Files created:
- Binary: `~/.local/bin/rsk`
- Config: `~/.config/rsk/rsk.env`
- Service: `~/.config/systemd/user/rsk.service`
- Udev: `/etc/udev/rules.d/99-rsk-uinput.rules` (via sudo, one-time)

### rsk-node (client node)

```
Usage: rsk-node setup --server URL [--device NAME] [--token SECRET] [--allow-gui]
```

Installs as systemd user service (`rsk-node.service`).

- `--server` — Broker WS URL, e.g. `ws://192.168.1.100:7777/agent` **(required)**
- `--device` — Unique device identifier (default: hostname)
- `--token` — Must match daemon token. Auto-read from `~/.config/rsk/rsk.env` if available.
- `--allow-gui` — Enable GUI actions (default: true)

Files created:
- Binary: `~/.local/bin/rsk-node`
- Config: `~/.config/rsk/node.env`
- Service: `~/.config/systemd/user/rsk-node.service`

## Token

```bash
# Print token from server config
rsk token
```

## Update

```bash
rsk update         # update server binary + restart service
rsk-node update    # update node binary + restart service
```

Downloads the latest release from GitHub, replaces the binary atomically, and restarts the service.

## Uninstall

```bash
rsk uninstall        # remove server binary, config, service
rsk-node uninstall   # remove node binary, config, service
```

## AI Agent Integration

Untuk memberikan konteks penggunaan remote-skill ke AI agent, tambahkan file skill berikut:

```
https://raw.githubusercontent.com/arosyihuddin/remote-skill/main/skill/SKILL.md
```

**Claude Code** — SKILL.md auto-terdeteksi saat berada di direktori project.  
**AI Chat lainnya** — tempel raw URL di atas ke system prompt atau custom instructions.
