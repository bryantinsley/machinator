#!/bin/bash
set -e

if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]]; then
    echo "Error: This must be run via 'bazel run'"
    exit 1
fi

cd "$BUILD_WORKSPACE_DIRECTORY"

echo "Querying for all golden update targets..."
UPDATE_TARGETS=$(bazel query 'kind(sh_binary, //...) intersect attr(name, ".*\.update", //...)')

if [[ -z "$UPDATE_TARGETS" ]]; then
    echo "No golden update targets found."
    exit 0
fi

for target in $UPDATE_TARGETS; do
    echo "Running $target..."
    bazel run "$target"
done

echo "All goldens updated."
