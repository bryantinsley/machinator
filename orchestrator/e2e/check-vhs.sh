#!/bin/bash
# Check if docker is available for VHS
if command -v docker &> /dev/null
then
    echo "✅ docker is installed"
    echo "Checking VHS via Docker..."
    ./scripts/vhs-docker.sh --version
    exit 0
else
    echo "❌ Error: docker is not installed. Please install Docker Desktop (https://docker.io)."
    exit 1
fi