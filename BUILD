load("@gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/bryantinsley/machinator
# gazelle:exclude .go-cache
# gazelle:exclude .go-build-cache
# gazelle:exclude _archive
# gazelle:exclude testdata/vhs-fixture
gazelle(name = "gazelle")

exports_files([
    "AGENTS.md",
    "go.mod",
    "bd",
    "mock-bd.sh",
])

# Root BUILD file - unified machinator binary
#
# The machinator binary handles both setup and orchestration:
# - First run (no .beads/): launches setup wizard
# - Subsequent runs: launches orchestrator
# - --setup flag: forces setup mode

# Primary target - use this
alias(
    name = "machinator",
    actual = "//orchestrator/cmd/machinator",
    visibility = ["//visibility:public"],
)

# Legacy alias - deprecated, use :machinator instead
alias(
    name = "tui",
    actual = "//orchestrator/cmd/machinator",
)

# Setup-only mode: bazel run //:machinator -- --setup
# (Kept for backwards compatibility, but not recommended)
alias(
    name = "setup",
    actual = "//orchestrator/cmd/machinator",
)
