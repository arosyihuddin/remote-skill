#!/bin/sh
set -e
CGO_ENABLED=0
export CGO_ENABLED

VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS="-X 'main.Version=$VERSION' -X 'github.com/pstar7/remote-skill/internal/node.Version=$VERSION'"

echo "==> building rsk ($VERSION)"
go build -ldflags="$LDFLAGS" -o bin/rsk ./cmd/rsk

echo "==> building rsk-node ($VERSION)"
go build -ldflags="$LDFLAGS" -o bin/rsk-node ./cmd/node

echo "==> done. binary sizes:"
ls -lh bin/
