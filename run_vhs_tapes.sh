#!/bin/bash
GOOS=linux GOARCH=amd64 go build -o setup-tui-linux ./orchestrator/cmd/setup/main.go
chmod +x setup-tui-linux
./scripts/vhs-docker.sh orchestrator/e2e/navigation.tape
./scripts/vhs-docker.sh orchestrator/e2e/crud.tape
./scripts/vhs-docker.sh orchestrator/e2e/crud.tape
