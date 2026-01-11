#!/bin/bash
export GOPATH="$(pwd)/.go-cache"
export GOBIN="$GOPATH/bin"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
export PATH="$GOBIN:$PATH"
mkdir -p "$GOPATH" "$GOBIN" "$GOCACHE" "$GOMODCACHE"
