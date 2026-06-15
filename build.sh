#!/bin/sh
set -e
CGO_ENABLED=0
export CGO_ENABLED

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) EXE=".exe" ;;
  *)                     EXE="" ;;
esac

if [ "$1" = "release" ]; then
  GOOS=linux GOARCH=amd64; export GOOS GOARCH
  SUFFIX="-linux-amd64"
  EXE=""
else
  SUFFIX=""
  if [ ! -f .env ]; then
    TOKEN=$(openssl rand -hex 16 2>/dev/null || echo "dev-$(date +%s)")
    cat > .env <<EOF
RSK_TOKEN=$TOKEN
RSK_AGENT_LISTEN=0.0.0.0:7777
RSK_MONITOR=127.0.0.1:7800
RSK_NODE_SERVER_URL=ws://127.0.0.1:7777/agent
RSK_NODE_DEVICE_ID=dev-$(hostname)
EOF
    echo "==> generated .env (token: $(printf '%.8s' "$TOKEN")...)"
  fi
fi

VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS="-X 'main.Version=$VERSION' -X 'github.com/pstar7/remote-skill/internal/node.Version=$VERSION'"

echo "==> building rsk ($VERSION)"
go build -ldflags="$LDFLAGS" -o "bin/rsk$SUFFIX$EXE" ./cmd/rsk

echo "==> building rsk-node ($VERSION)"
go build -ldflags="$LDFLAGS" -o "bin/rsk-node$SUFFIX$EXE" ./cmd/node

echo "==> done."
ls -lh bin/
