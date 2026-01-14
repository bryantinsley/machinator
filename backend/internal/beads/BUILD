load("@rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "beads",
    srcs = ["beads.go"],
    importpath = "github.com/bryantinsley/machinator/backend/internal/beads",
    visibility = ["//backend:__subpackages__"],
)

go_test(
    name = "beads_upstream_test",
    srcs = ["beads_upstream_test.go"],
    embed = [":beads"],
    # This test requires network access to fetch upstream types.go
    tags = ["requires-network"],
)
