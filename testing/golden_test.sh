#!/bin/bash
set -e

# Arguments:
# 1: Path to binary
# 2: Path to golden file
# 3...: Arguments to binary

BINARY_PATH="$1"
GOLDEN_PATH="$2"
shift 2

# In Bazel, we should look for files in the runfiles directory if they are not directly available.
# But for sh_test, they should be available relative to the current directory if using rootpath.

if [[ ! -f "$BINARY_PATH" ]]; then
    # Try finding it in runfiles
    if [[ -f "$TEST_SRCDIR/$TEST_WORKSPACE/$BINARY_PATH" ]]; then
        BINARY_PATH="$TEST_SRCDIR/$TEST_WORKSPACE/$BINARY_PATH"
    else
        echo "Error: Binary not found at $BINARY_PATH"
        exit 1
    fi
fi

if [[ ! -f "$GOLDEN_PATH" ]]; then
    # Try finding it in runfiles
    if [[ -f "$TEST_SRCDIR/$TEST_WORKSPACE/$GOLDEN_PATH" ]]; then
        GOLDEN_PATH="$TEST_SRCDIR/$TEST_WORKSPACE/$GOLDEN_PATH"
    else
        echo "Error: Golden file not found at $GOLDEN_PATH"
        exit 1
    fi
fi

# Prepare to capture output
ACTUAL_OUT=$(mktemp)

# Check if --update is in the arguments (passed via sh_binary)
UPDATE=0
REMAINING_ARGS=()
for arg in "$@"; do
    if [[ "$arg" == "--update" ]]; then
        UPDATE=1
    else
        REMAINING_ARGS+=("$arg")
    fi
done

# Run binary
"./$BINARY_PATH" "${REMAINING_ARGS[@]}" > "$ACTUAL_OUT"

if [[ "$UPDATE" == "1" ]]; then
    if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]]; then
        echo "Error: --update requires running with 'bazel run'"
        exit 1
    fi
    
    # GOLDEN_SOURCE_PATH should be set by the sh_binary env
    if [[ -z "$GOLDEN_SOURCE_PATH" ]]; then
        echo "Error: GOLDEN_SOURCE_PATH environment variable not set"
        exit 1
    fi
    
    cp "$ACTUAL_OUT" "$BUILD_WORKSPACE_DIRECTORY/$GOLDEN_SOURCE_PATH"
    echo "Updated $GOLDEN_SOURCE_PATH"
else
    if ! diff -u "$GOLDEN_PATH" "$ACTUAL_OUT"; then
        echo ""
        echo "FAIL: Golden file mismatch for $GOLDEN_PATH"
        echo "To update this golden file, run:"
        echo "  bazel run ${BAZEL_TARGET}.update"
        exit 1
    fi
    echo "PASS: Golden file matches for $GOLDEN_PATH"
fi

rm "$ACTUAL_OUT"
