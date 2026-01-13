load("@rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "agentgrid",
    srcs = [
        "card.go",
        "grid.go",
    ],
    importpath = "github.com/bryantinsley/machinator/orchestrator/pkg/ui/agentgrid",
    visibility = ["//visibility:public"],
    deps = [
        "//orchestrator/pkg/ui/components",
        "//orchestrator/pkg/ui/styles",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)

go_test(
    name = "agentgrid_test",
    srcs = ["grid_test.go"],
    embed = [":agentgrid"],
    tags = ["fast"],
)
