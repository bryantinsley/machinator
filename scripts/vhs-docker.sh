#!/bin/bash
# Wrapper for VHS to run via Docker to avoid sandbox issues on macOS.
# Reference: https://github.com/charmbracelet/vhs

# Ensure custom image is built
if [[ "$(docker images -q machinator-vhs 2> /dev/null)" == "" ]]; then
  echo "Building machinator-vhs docker image..."
  docker build -t machinator-vhs tools/vhs
fi

docker run --rm -v "$PWD:/vhs" machinator-vhs "$@"
