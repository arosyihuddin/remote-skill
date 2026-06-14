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
fi

VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS="-X 'main.Version=$VERSION' -X 'github.com/pstar7/remote-skill/internal/node.Version=$VERSION'"

echo "==> building rsk ($VERSION)"
go build -ldflags="$LDFLAGS" -o "bin/rsk$SUFFIX$EXE" ./cmd/rsk

echo "==> building rsk-node ($VERSION)"
go build -ldflags="$LDFLAGS" -o "bin/rsk-node$SUFFIX$EXE" ./cmd/node

echo "==> done."
ls -lh bin/
