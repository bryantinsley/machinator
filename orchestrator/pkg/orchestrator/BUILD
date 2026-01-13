load("@rules_go//go:def.bzl", "go_library", "go_test")

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
        "//orchestrator/pkg/ui/agentgrid",
        "//orchestrator/pkg/ui/components",
        "@com_github_charmbracelet_bubbles//viewport",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)

go_test(
    name = "orchestrator_test",
    srcs = [
        "quota_render_test.go",
        "tools_check_test.go",
        "tui_golden_test.go",
        "tui_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":orchestrator"],
    deps = [
        "//orchestrator/pkg/ui/agentgrid",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_x_exp_teatest//:teatest",
    ],
)
