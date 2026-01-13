load("@rules_go//go:def.bzl", "go_library", "go_test")

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
        "//orchestrator/pkg/accountpool",
        "//orchestrator/pkg/ui/components",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_lipgloss//:lipgloss",
    ],
)

go_test(
    name = "setup_test",
    srcs = [
        "accounts_test.go",
        "config_test.go",
        "rename_repro_test.go",
        "setup_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":setup"],
    tags = ["fast"],
    deps = [
        "//orchestrator/pkg/ui/components",
        "@com_github_charmbracelet_bubbletea//:bubbletea",
        "@com_github_charmbracelet_x_exp_teatest//:teatest",
    ],
)
