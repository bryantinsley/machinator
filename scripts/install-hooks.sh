#!/bin/bash
mkdir -p .git/hooks
ln -sf ../../scripts/hooks/pre-commit .git/hooks/pre-commit
echo "Pre-commit hook installed to .git/hooks/pre-commit"
chmod +x .git/hooks/pre-commit
