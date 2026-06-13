#!/bin/sh
set -e
CGO_ENABLED=0
export CGO_ENABLED

echo "==> building rsk"
go build -o bin/rsk ./cmd/rsk

echo "==> building rsk-node"
go build -o bin/rsk-node ./cmd/node

echo "==> done. binary sizes:"
ls -lh bin/
