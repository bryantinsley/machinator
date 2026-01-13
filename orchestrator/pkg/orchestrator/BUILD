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
        "//orchestrator/pkg/ui/styles",
        "@com_github_charmbracelet_bubbles//viewport",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)

go_test(
    name = "orchestrator_test",
    srcs = [
        "acp_events_test.go",
        "branch_protection_test.go",
        "claiming_test.go",
        "done_routing_test.go",
        "find_ready_task_test.go",
        "git_prep_test.go",
        "multi_agent_test.go",
        "pull_test.go",
        "quota_render_test.go",
        "quota_visual_test.go",
        "recovery_test.go",
        "repro_test.go",
        "staleness_test.go",
        "tools_check_test.go",
        "tui_golden_test.go",
        "tui_test.go",
        "worktree_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":orchestrator"],
    tags = ["fast"],
    deps = [
        "//orchestrator/pkg/accountpool",
        "//orchestrator/pkg/ui/agentgrid",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_x_ansi//:ansi",
        "@com_github_charmbracelet_x_exp_teatest//:teatest",
    ],
)
