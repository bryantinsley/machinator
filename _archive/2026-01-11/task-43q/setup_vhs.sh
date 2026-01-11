#!/bin/bash
export GOPATH="$(pwd)/.go-cache"
export GOBIN="$(pwd)/.go-cache/bin"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
mkdir -p "$GOPATH" "$GOBIN" "$GOCACHE" "$GOMODCACHE"

echo "Installing VHS..."
go install github.com/charmbracelet/vhs@latest

echo "Verifying VHS installation..."
./.go-cache/bin/vhs --version
