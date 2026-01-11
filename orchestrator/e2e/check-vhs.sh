#!/bin/bash
if command -v vhs &> /dev/null
then
    echo "vhs is installed"
    vhs --version
    exit 0
else
    echo "Error: vhs is not installed. Please install it (e.g., brew install vhs)."
    exit 1
fi
