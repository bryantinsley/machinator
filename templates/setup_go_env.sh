#!/bin/bash
# Set Go environment to use local caches within the project

export GOPATH="$(pwd)/.go-cache"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
export PATH="$(pwd)/.go-cache/bin:$PATH"

# Ensure directories exist
mkdir -p "$GOPATH" "$GOCACHE" "$GOMODCACHE"

echo "✅ Go environment configured for local caching:"
echo "   GOPATH=$GOPATH"
echo "   GOCACHE=$GOCACHE"
echo "   GOMODCACHE=$GOMODCACHE"
