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

### Default output — Toon flat list

```bash
rsk a11y
```

Output:
```
nodes[450]{id,role,name,x,y,w,h,parent,mon}:
  1,desktop frame,main,0,0,1024,768,0,-1
  27,window,ghostty,0,0,948,1023,6,1
  47,application,waybar,0,0,0,0,1,-1
  55,label,YouTube - LibreWolf,0,0,212,40,54,0
  79,button,11 ,680,8,47,24,78,0
  106,label,󰤨   1.2kB/s  824.0B/s,1423,0,204,40,105,0
```

**Format:**
| Field | Arti |
|-------|------|
| `id` | ID unik sequential |
| `role` | Tipe element AT-SPI2 |
| `name` | Nama/konten (icon, label, atau teks) |
| `x,y` | Posisi kiri-atas dalam pixel |
| `w,h` | Lebar dan tinggi |
| `parent` | ID induk di tree (0 = root) |
| `mon` | Monitor index (0=eDP-1, 1=HDMI-A-1, -1=background) |

### Role umum

| Role | Arti | Interactive |
|------|------|-------------|
| `push button` | Tombol yang bisa diklik | ✅ |
| `toggle button` | Tombol on/off | ✅ |
| `entry` | Input teks | ✅ |
| `password text` | Input password | ✅ |
| `combo box` | Dropdown | ✅ |
| `check box` | Checkbox | ✅ |
| `page tab` | Tab di browser | ✅ |
| `link` | Hyperlink | ✅ |
| `menu item` | Item di menu | ✅ |
| `slider` | Slider | ✅ |
| `spin button` | Up/down spinner | ✅ |
| `label` | Teks statis | ❌ |
| `window` | Window | ❌ |
| `application` | Aplikasi (bounds 0,0,0,0) | ❌ |
| `filler` | Container layout, ignore | ❌ |

### Detail node dengan character bounds

```bash
rsk a11y --id 106
```

Output:
```
label "󰤨  1.2kB/s" [1423,0,204,40]
  chars: 1441,11,8,17;1449,11,8,17;1457,11,8,17;...
```

Char bounds = `x,y,w,h` per karakter. Berguna untuk klik element yang mengandung icon (Nerd Font). Karakter pertama (`[0]`) biasanya adalah icon — klik di tengahnya:

```
click_x = char.x + char.w / 2
click_y = char.y + char.h / 2
```

### Filter role

```bash
rsk a11y --role button                    # cuma push/toggle button
rsk a11y --role button,input              # multiple role
rsk a11y --role button --id 23            # detail node yang cocok filter
```

Alias role: `button` → push button/toggle button, `input` → entry/password text, `checkbox` → check box, `dropdown` → combo box.

### Flags

| Flag | Fungsi |
|------|--------|
| *(none)* | Monitor 0 saja (default) |
| `--all` | Semua monitor |
| `--monitor N` | Filter spesifik monitor (0, 1, -1) |
| `--id N` | Detail node + char bounds per karakter |
| `--depth N` | Tree depth (default 8, maks 20) |
| `--role name` | Filter role (button, input, checkbox, dropdown) |
| `--show-all` | Tampilkan semua node termasuk yg tidak berguna |

### AI Agent usage pattern

```bash
# 1. Cari monitor target
rsk monitors

# 2. Ambil tree di monitor itu
rsk a11y                        # default monitor 0
rsk a11y --monitor 1            # monitor khusus
rsk a11y --all                  # semua monitor

# 3. Cari target — filter role yg interactive
rsk a11y --role button
rsk a11y --role button,input

# 4. Detail node untuk posisi presisi (apalagi ada icon)
rsk a11y --id 23

# 5. Klik di koordinat
rsk mouse <x + w/2> <y + h/2>
rsk click

# 6. Type di entry
rsk type "teks"

# 7. Verifikasi perubahan
rsk a11y
```

### Known limitations

- Monitor assignment via height heuristic + IoU dengan `hyprctl clients`. Waybar child elements tetap `mon=-1` (panel bukan client).
- Waybar element terekspos sebagai `label`, bukan `push button` — tapi koordinatnya tetap akurat untuk klik.
- Window/LibreWolf element role nya mungkin berbeda dari GTK native.
- Element dengan bounds `-1,-1,-1,-1` atau `0,0,0,0` — tidak punya posisi layar (menu belum terbuka, atau background app).
- Default depth 8; depth lebih tinggi = lebih banyak node tapi lebih lambat (banyak DBus calls).

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
