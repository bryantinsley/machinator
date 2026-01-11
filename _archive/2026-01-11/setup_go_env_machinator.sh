#!/bin/bash
export GOPATH="$(pwd)/.go-cache"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
mkdir -p "$GOPATH" "$GOCACHE" "$GOMODCACHE"
