load("@rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "components",
    srcs = [
        "button.go",
        "clickable.go",
        "dropdown.go",
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

go_test(
    name = "components_test",
    srcs = [
        "components_test.go",
        "dropdown_test.go",
    ],
    embed = [":components"],
    tags = ["fast"],
    deps = ["@com_github_charmbracelet_bubbletea//:bubbletea"],
)
