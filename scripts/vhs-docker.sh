#!/bin/bash
# Wrapper for VHS to run via Docker to avoid sandbox issues on macOS.
# Reference: https://github.com/charmbracelet/vhs

docker run --rm -v "$PWD:/vhs" ghcr.io/charmbracelet/vhs "$@"
