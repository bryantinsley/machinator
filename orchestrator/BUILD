load("@rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "orchestrator_lib",
    srcs = [
        "acp_events.go",
        "quota_check.go",
        "tui.go",
        "tui_helper.go",
    ],
    importpath = "github.com/bryantinsley/machinator/orchestrator",
    visibility = ["//visibility:private"],
    deps = [
        "@com_github_charmbracelet_bubbles//viewport",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)

go_binary(
    name = "tui",
    data = [
        "//templates:directive_template.txt",
        "//templates:setup_go_env.sh",
        "//templates:unblocking_directive.txt",
    ],
    embed = [":orchestrator_lib"],
    visibility = ["//visibility:public"],
)
