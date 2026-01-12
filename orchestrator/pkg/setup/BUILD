load("@rules_go//go:def.bzl", "go_library")

go_library(
    name = "setup",
    srcs = [
        "accounts.go",
        "config.go",
        "model.go",
        "setup.go",
    ],
    embedsrcs = ["nameplate.txt"],
    importpath = "github.com/bryantinsley/machinator/orchestrator/pkg/setup",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)
