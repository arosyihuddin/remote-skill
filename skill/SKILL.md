# remote-skill

Bridge AI agent kontrol remote laptop/workstation via WebSocket.

## Prerequisites

- `rsk` — broker daemon + CLI (dual mode), install: `rsk setup`
- `rsk-node` — daemon di laptop target, install: `rsk-node setup`
- Kedua daemon pake shared token (sama)

## CLI Usage

```bash
# Device — single otomatis, multi pake device-id
rsk exec "ls -la /home"
rsk my-laptop exec "uname -a"
rsk my-laptop screenshot

# Token — auto-detect dari config, atau manual
rsk token

# Exec — shell command
rsk exec "ls -la" --shell --cwd /tmp
rsk exec "apt update" --timeout 120 --shell

# Stream exec
rsk exec "ping -c 5 google.com" --stream

# Read file
rsk read /etc/hostname
rsk read /path/to/binary --binary
rsk read /path/to/file --max 1024

# Write file
rsk write /path/to/file.txt --file local.txt
echo "content" | rsk write /path/to/file.txt

# List directory
rsk ls /home
rsk ls /home --hidden

# Screenshot — auto-save ke /tmp/
rsk screenshot
# Output: /tmp/rsk-screenshot-xxx.png (1920x1080)
rsk screenshot --output DP-1 --region "0,0 1920x1080"
rsk screenshot --save custom.png

# Mouse
rsk mouse 500 500
rsk mouse 100 100 --relative

# Click
rsk click
rsk click --x 500 --y 500 --button left
rsk click --button right
rsk click --double

# Type text
rsk type "Hello World"

# Key combo
rsk key "ctrl+alt+t"     # buka terminal
rsk key "ctrl+l"          # focus address bar
rsk key "alt+f4"          # tutup window
rsk key "pagedown"        # scroll halaman

# Scroll
rsk scroll                 # scroll down 3
rsk scroll --dy -10       # scroll down 10
rsk scroll --dy 5         # scroll up 5

# Clipboard
rsk clip get
rsk clip set "text to copy"

# List connected devices
rsk devices
```

## Management

```bash
# Install daemon (user service + udev)
rsk setup
rsk setup --agent 0.0.0.0:7777 --monitor 127.0.0.1:7800 --token mytoken

# Install node (user service + udev + auto-read token)
rsk-node setup
rsk-node setup --server ws://vps:7777/agent --device my-laptop

# Uninstall
rsk uninstall
rsk-node uninstall

# Start/stop
systemctl --user start rsk
systemctl --user stop rsk-node
journalctl --user -u rsk -f
```

## Config

Semua config auto-detect. Gak perlu `--config`:

```
~/.config/rsk/rsk.env    — daemon
~/.config/rsk/node.env   — node
```

## Env var overrides

| Var | Untuk | Default |
|-----|-------|---------|
| `RSK_SERVER` | CLI — WS URL | `ws://127.0.0.1:7777` |
| `RSK_TOKEN` | CLI + node — shared token | auto-detect dari config |
| `RSK_DEVICE` | CLI — target device | auto-detect single device |

## Multi-device

Device ID di config tiap laptop unik. CLI pilih device via arg pertama:

```bash
rsk my-laptop exec "echo hi"
rsk laptop-2 type "test"
```

Kalo cuma 1 device connect, otomatis.

## Monitoring UI

Browser buka `http://<vps-ip>:7800/` — liat devices, live screen, remote shell.

## Security

- Shared token antara daemon dan semua node
- Monitoring port (`:7800`) sebaiknya bind ke `127.0.0.1`
- Token adalah root credential
- Udev rule `/etc/udev/rules.d/99-rsk-uinput.rules` buat akses `/dev/uinput`
