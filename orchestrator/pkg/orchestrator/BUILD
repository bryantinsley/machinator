load("@rules_go//go:def.bzl", "go_library")

go_library(
    name = "orchestrator",
    srcs = [
        "acp_events.go",
        "quota_check.go",
        "tools_check.go",
        "tui.go",
        "tui_helper.go",
    ],
    importpath = "github.com/bryantinsley/machinator/orchestrator/pkg/orchestrator",
    visibility = ["//visibility:public"],
    deps = [
        "//orchestrator/pkg/accountpool",
        "//orchestrator/pkg/setup",
        "@com_github_charmbracelet_bubbles//viewport",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)
