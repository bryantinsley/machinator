#!/bin/bash
set -e

# This script runs VHS validation using Bazel-built Linux binaries.
# It is intended to run on a Linux host with Docker installed (e.g., CI).

# Locate runfiles
if [[ -z "$RUNFILES_DIR" ]]; then
    if [[ -d "$0.runfiles" ]]; then
        RUNFILES_DIR="$0.runfiles"
    else
        # Try to find it relative to script
        RUNFILES_DIR="$(cd "$(dirname "$0")" && pwd).runfiles"
    fi
fi

# Find files in runfiles (Bazel structure can vary)
# Usually: $RUNFILES_DIR/_main/path/to/file or $RUNFILES_DIR/workspace_name/path/to/file
find_in_runfiles() {
    local target="$1"
    # Use -L to follow symlinks in runfiles
    find -L "$RUNFILES_DIR" -name "$target" -type f | head -n 1
}

find_dir_in_runfiles() {
    local target="$1"
    find -L "$RUNFILES_DIR" -name "$target" -type d | head -n 1
}

MACHINATOR_LINUX=$(find_in_runfiles "machinator-linux")
DUMMY_GEMINI_LINUX=$(find_in_runfiles "dummy-gemini-linux")
CRUD_TAPE=$(find_in_runfiles "crud.tape")
VHS_FIXTURE_DIR=$(find_dir_in_runfiles "vhs-fixture")

echo "--- VHS Validation Setup ---"
echo "MACHINATOR_LINUX: $MACHINATOR_LINUX"
echo "DUMMY_GEMINI_LINUX: $DUMMY_GEMINI_LINUX"
echo "CRUD_TAPE: $CRUD_TAPE"
echo "VHS_FIXTURE_DIR: $VHS_FIXTURE_DIR"

if [[ -z "$MACHINATOR_LINUX" || -z "$DUMMY_GEMINI_LINUX" || -z "$CRUD_TAPE" || -z "$VHS_FIXTURE_DIR" ]]; then
    echo "❌ Error: Missing required files in runfiles."
    exit 1
fi

# Check for Docker
if ! command -v docker &> /dev/null; then
    echo "⚠️ Docker not found. Skipping VHS validation."
    exit 0
fi

# Create temporary workspace
if [[ "$OSTYPE" == "darwin"* ]]; then
    # On macOS with Colima/Docker, we MUST use a path under /Users/
    # as other paths like /tmp are often not mounted by default.
    # We try to find a writable path under /Users.
    USER_NAME=$(id -un)
    for p in "/Users/$USER_NAME" "$HOME" "/tmp"; do
        if [[ -d "$p" && -w "$p" ]]; then
            mkdir -p "$p/.tmp/machinator" 2>/dev/null
            TMP_WS=$(mktemp -d "$p/.tmp/machinator/vhs-XXXXXX" 2>/dev/null)
            if [[ -n "$TMP_WS" ]]; then break; fi
        fi
    done
    if [[ -z "$TMP_WS" ]]; then
        TMP_WS=$(mktemp -d)
    fi
else
    TMP_WS=$(mktemp -d)
fi
echo "Created temporary workspace: $TMP_WS"

# Cleanup on exit
cleanup() {
    echo "Cleaning up $TMP_WS..."
    rm -rf "$TMP_WS"
}
trap cleanup EXIT

# Copy tape and fixture
cp "$CRUD_TAPE" "$TMP_WS/crud.tape"
mkdir -p "$TMP_WS/testdata/vhs-fixture"
cp -r "$VHS_FIXTURE_DIR/." "$TMP_WS/testdata/vhs-fixture/"

# Replace placeholder binaries with the ones built by Bazel
cp "$MACHINATOR_LINUX" "$TMP_WS/machinator-linux"
chmod +x "$TMP_WS/machinator-linux"

cp "$DUMMY_GEMINI_LINUX" "$TMP_WS/testdata/vhs-fixture/.machinator/gemini"
chmod +x "$TMP_WS/testdata/vhs-fixture/.machinator/gemini"

# Ensure the output directory exists so VHS can write to it
mkdir -p "$TMP_WS/docs/ui-history"

# Run VHS via Docker
echo "Running VHS..."

# Use the same logic as scripts/vhs-docker.sh but inline
IMAGE="ghcr.io/charmbracelet/vhs"
if [[ "$(docker images -q machinator-vhs 2> /dev/null)" != "" ]]; then
    IMAGE="machinator-vhs"
fi

# We mount TMP_WS to /vhs and tell VHS to run /vhs/crud.tape
docker run --rm -v "$TMP_WS:/vhs" -w /vhs "$IMAGE" crud.tape

echo "✅ VHS validation completed successfully!"