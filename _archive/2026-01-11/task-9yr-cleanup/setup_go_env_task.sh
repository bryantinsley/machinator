#!/bin/bash
export GOPATH="$(pwd)/.go-cache"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
mkdir -p "$GOPATH" "$GOCACHE" "$GOMODCACHE"
export PATH="$(pwd)/vhs_0.10.0_Darwin_arm64:$(pwd)/.go-cache/bin:$PATH"
