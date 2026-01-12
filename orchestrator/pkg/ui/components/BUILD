load("@rules_go//go:def.bzl", "go_library")

go_library(
    name = "components",
    srcs = [
        "button.go",
        "clickable.go",
        "list_item.go",
    ],
    importpath = "github.com/bryantinsley/machinator/orchestrator/pkg/ui/components",
    visibility = ["//visibility:public"],
    deps = [
        "//orchestrator/pkg/ui/styles",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)
