#!/bin/sh
set -e

REPO="arosyihuddin/remote-skill"
VERSION="${RSK_VERSION:-latest}"

usage() {
  cat <<'EOF'
Usage: install.sh <server|node> [options]

Server:
  install.sh server [--agent ADDR] [--monitor ADDR] [--token SECRET] [--ui-password PASS]

Node:
  install.sh node --server URL [--device NAME] [--token SECRET] [--allow-gui]
EOF
  exit 1
}

detect_arch() {
  case "$(uname -s)" in
    Linux)  OS=linux ;;
    Darwin) OS=darwin ;;
    *)      echo "unsupported OS: $(uname -s)"; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *)      echo "unsupported arch: $(uname -m)"; exit 1 ;;
  esac
}

download() {
  local name=$1 suffix="-$OS-$ARCH"
  [ "$OS" = linux ] && suffix="-$OS-$ARCH"
  echo "==> downloading $name$suffix"
  if [ "$VERSION" = "latest" ]; then
    url="https://github.com/$REPO/releases/latest/download/$name$suffix"
  else
    url="https://github.com/$REPO/releases/download/$VERSION/$name$suffix"
  fi
  curl -fsSL "$url" -o "/tmp/$name"
  chmod +x "/tmp/$name"
}

cmd_server() {
  download rsk
  shift
  /tmp/rsk setup "$@"
}

cmd_node() {
  download rsk-node
  shift
  /tmp/rsk-node setup "$@"
}

detect_arch

case "${1:-}" in
  server) cmd_server "$@" ;;
  node)   cmd_node "$@" ;;
  *)      usage ;;
esac
