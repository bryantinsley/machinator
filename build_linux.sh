#!/bin/bash
export GOPATH="$(pwd)/.go-cache"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
export PATH="$(pwd)/.go-cache/bin:$PATH"
mkdir -p "$GOPATH" "$(pwd)/.go-cache/bin" "$GOCACHE" "$GOMODCACHE"
GOOS=linux GOARCH=amd64 go build -o setup-tui-linux ./orchestrator/cmd/setup
GOOS=linux GOARCH=amd64 go build -o machinator-linux ./orchestrator/cmd/machinator
