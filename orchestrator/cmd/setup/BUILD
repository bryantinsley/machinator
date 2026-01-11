load("@rules_go//go:def.bzl", "go_binary")

go_binary(
    name = "setup",
    srcs = ["main.go"],
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)
