#!/bin/bash
export GOPATH="$(pwd)/.go-cache"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
export PATH="$(pwd)/.go-cache/bin:$PATH"
go test ./orchestrator/e2e/... -v
