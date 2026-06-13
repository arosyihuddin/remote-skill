# remote-skill

Bridge agar AI agent (atau CLI) bisa kontrol laptop/workstation secara realtime via WebSocket.

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

## Komponen

- `cmd/rsk` -> binary `rsk` (dual mode). Broker WS di `:7777` + monitoring HTTP di `:7800` + CLI mode.
- `cmd/node` -> binary `rsk-node` di laptop. Dial-out ke broker, eksekusi request.
- `internal/cli` -> CLI mode (connect WS, kirim request, print response).
- `internal/proto` -> wire format JSON.
- `internal/broker` -> device registry + request routing.
- `internal/handlers` -> exec / file / GUI implementation (Linux/Wayland).
- Systemd units di-embed dalam binary (`//go:embed`).

## Build

```bash
./build.sh
# atau manual:
CGO_ENABLED=0 go build -o bin/rsk ./cmd/rsk
CGO_ENABLED=0 go build -o bin/rsk-node ./cmd/node
```

Cross-compile dari laptop ke VPS (linux/amd64):

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/rsk-linux-amd64 ./cmd/rsk
```

## Setup daemon

```bash
# Install sebagai user service (auto-detect config path)
./bin/rsk setup

# Atau kustom:
# ./bin/rsk setup --agent 0.0.0.0:7777 --monitor 127.0.0.1:7800 --token mytoken
```

Config tersimpan di `~/.config/rsk/rsk.env`. Service: `systemctl --user rsk`.

`AGENT_LISTEN` sebaiknya bind ke IP mesh VPN (`100.116.138.90:7777`), jangan `0.0.0.0` kecuali fronted dengan TLS+auth.

## Setup node

```bash
# Install sebagai user service (auto-read token dari daemon config)
./bin/rsk-node setup

# Atau kustom:
# ./bin/rsk-node setup --server ws://vps:7777/agent --device my-laptop --token secret123
```

`rsk-node setup` otomatis:
- Drop udev rule + `chmod 0666 /dev/uinput` (akses keyboard/mouse langsung)
- Auto-read token dari `~/.config/rsk/rsk.env` (kalau satu mesin)
- Buat config `~/.config/rsk/node.env`
- Copas binary ke `~/.local/bin/rsk-node`
- Install + enable systemd user service

## CLI Usage

```bash
# Single device — auto-detect
rsk exec "ls -la"
rsk screenshot

# Multi-device — specify device-id
rsk my-laptop exec "echo hi"
rsk my-laptop screenshot

# Or via env var
export RSK_DEVICE=my-laptop
rsk exec "ls -la"
```

## Multi-device

`DEVICE_ID` di config tiap laptop harus unik (misal `laptop-pstar7`, `desktop-home`, `macbook-work`). CLI bisa specify device-id sebagai argumen pertama. Jika cuma 1 device terkoneksi, otomatis.

## Smoke test (lokal, tanpa VPS)

```bash
# terminal 1 — broker
RSK_TOKEN=test123 ./bin/rsk daemon

# terminal 2 — node
RSK_TOKEN=test123 ./bin/rsk-node

# terminal 3 — CLI
./bin/rsk exec "echo hi"
```

## Security model

- Single shared `TOKEN` between daemon and all nodes.
- Monitoring HTTP (`:7800`) sebaiknya bind ke `127.0.0.1` (hanya akses lokal) — ganti ke `0.0.0.0` hanya jika perlu akses dari browser remote.
- WS broker (`:7777`) sebaiknya bind ke mesh VPN IP saja (`100.116.138.90:7777`).
- CLI dan Node konek via WS — siapa pun yang punya token bisa akses laptop. Treat `TOKEN` sebagai root credential.


