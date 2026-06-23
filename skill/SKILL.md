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

# Key combo — nama case-insensitive
rsk key "ctrl+alt+t"     # buka terminal
rsk key "ctrl+l"          # focus address bar
rsk key "Return"           # Enter (bisa "return", "ENTER", "Return")
rsk key "alt+f4"          # tutup window
rsk key "Escape"           # tutup dialog
rsk key "pagedown"        # scroll halaman

# Scroll
rsk scroll                 # scroll down 3
rsk scroll --dy -10       # scroll down 10
rsk scroll --dy 5         # scroll up 5

# Clipboard
rsk clip get
rsk clip set "text to copy"

# Drag
rsk drag 100 100 300 300 [--button left|right|middle]

# Board (clipboard write + paste)
rsk board "Hello World"

# List connected devices
rsk devices

# Windows
rsk windows

# Accessibility tree
rsk a11y
rsk a11y --id 106
rsk a11y --role button
rsk a11y --role button,input
rsk a11y --depth 12
rsk a11y --show-all
rsk a11y --all
rsk a11y --monitor 1
rsk a11y --monitor -1
```

## Accessibility (a11y)

Accessibility tree via AT-SPI2 — lihat semua UI element di layar dalam format **Toon CSV**.

Semua koordinat (x,y) adalah **absolute desktop pixel** — hasil auto-fix + propagate offset dari parent frame ke child. Bisa langsung dipake klik.

### Output format

```bash
rsk a11y
```

Output:
```
nodes[182]{id,role,name,x,y,w,h,parent,mon}:
  1,frame,Browser,0,0,1920,1080,0,0
  100,button,Close,0,0,37,40,50,0
  200,combo box,Search or enter address,492,56,901,28,150,0
```

**Format:**
| Field | Arti |
|-------|------|
| `id` | ID unik sequential |
| `role` | Tipe element AT-SPI2 |
| `name` | Nama/konten (icon, label, atau teks) |
| `x,y` | Posisi kiri-atas (absolute desktop) |
| `w,h` | Lebar dan tinggi |
| `parent` | ID induk di tree (0 = root) |
| `mon` | Monitor index |

### Role umum

| Role | Arti | Interactive |
|------|------|-------------|
| `push button` | Tombol yang bisa diklik | ✅ |
| `toggle button` | Tombol on/off | ✅ |
| `entry` | Input teks | ✅ |
| `password text` | Input password | ✅ |
| `combo box` | Dropdown / URL bar | ✅ |
| `check box` | Checkbox | ✅ |
| `page tab` | Tab di browser | ✅ |
| `link` | Hyperlink | ✅ |
| `menu item` | Item di menu | ✅ |
| `slider` | Slider | ✅ |
| `spin button` | Up/down spinner | ✅ |
| `label` | Teks statis | ❌ |
| `window` | Window | ❌ |
| `frame` | App window frame | ❌ |
| `tool bar` | Toolbar container | ❌ |
| `filler` | Container layout, ignore | ❌ |
| `application` | Root app node (bounds 0,0,0,0) | ❌ |

### Filter role

```bash
rsk a11y --role button                    # cuma push/toggle button
rsk a11y --role button,input              # multiple role
rsk a11y --role button --id 23            # detail node yang cocok filter
```

Alias role: `button` → `push button`/`toggle button`, `input` → `entry`/`password text`, `checkbox` → `check box`, `dropdown` → `combo box`.

### Flags

| Flag | Fungsi |
|------|--------|
| *(none)* | Monitor 0 saja (default) |
| `--all` | Semua monitor |
| `--monitor N` | Filter spesifik monitor |
| `--id N` | Detail node + char bounds per karakter |
| `--depth N` | Tree depth (default 8, max 20) |
| `--role name` | Filter role |
| `--show-all` | Tampilkan semua node termasuk yg tidak berguna |

### Detail node + character bounds

```bash
rsk a11y --id 23
```

Output:
```
label "icon text" [100,50,200,40]
  chars: 110,55,8,17;118,55,8,17;...
```

Char bounds = `x,y,w,h` per karakter. Berguna untuk klik element yang mengandung icon. Karakter pertama biasanya icon — klik di tengahnya:
```
click_x = char.x + char.w / 2
click_y = char.y + char.h / 2
```

### AI Agent flow

```bash
# 1. Cek device & monitor
rsk monitors
rsk windows

# 2. Ambil a11y tree, filter interactive element
rsk a11y --role button,input,dropdown,checkbox,slider,link

# 3. Klik di center element
rsk mouse <x + w/2> <y + h/2>
rsk click

# 4. Type teks + Enter
rsk type "teks"
rsk key "Return"

# 5. Scroll
rsk scroll --dy -10         # down
rsk scroll --dy 10          # up

# 6. Cursor position
rsk cursorpos

# 7. Verifikasi
rsk windows
rsk a11y
```

### Known limitations

- **JS web apps (YouTube, dll)** — AT-SPI gak nembus shadow DOM. Fallback: `rsk screenshot`.
- **Waybar** — role `label` bukan `push button`, tapi koordinat akurat & bisa diklik.
- **Bounds `-1,-1,-1,-1` / `0,0,0,0`** — element gak visible di layar.
- **Depth > 8** — lebih lambat (banyak DBus calls). Naikin kalo perlu (max 20).
- **Key combo case-insensitive** — `"Return"`, `"return"`, `"ENTER"` semua work.

## Management

```bash
# Install
rsk setup
rsk setup --agent 0.0.0.0:7777 --monitor 127.0.0.1:7800 --token mytoken
rsk-node setup
rsk-node setup --server ws://vps:7777/agent --device my-laptop

# Uninstall
rsk uninstall
rsk-node uninstall

# Start/stop/logs
systemctl --user start rsk
systemctl --user stop rsk-node
journalctl --user -u rsk -f
```

## Config

Auto-detect. Gak perlu `--config`:

```
~/.config/rsk/rsk.env    — daemon
~/.config/rsk/node.env   — node
```

## Env overrides

| Var | Untuk | Default |
|-----|-------|---------|
| `RSK_SERVER` | CLI — WS URL | `ws://127.0.0.1:7777` |
| `RSK_TOKEN` | CLI + node — shared token | auto-detect |
| `RSK_DEVICE` | CLI — target device | auto-detect |

## Multi-device

Device ID unik per laptop. CLI pilih via arg pertama:
```bash
rsk my-laptop exec "echo hi"
rsk laptop-2 type "test"
```
Kalo cuma 1 device, otomatis.

## Monitoring UI

Browser buka `http://<vps-ip>:7800/` — devices, live screen, remote shell.

## Security

- Shared token antara daemon dan semua node
- Monitoring port (`:7800`) sebaiknya bind `127.0.0.1`
- Token adalah root credential
- Udev rule `/etc/udev/rules.d/99-rsk-uinput.rules` buat akses `/dev/uinput`
