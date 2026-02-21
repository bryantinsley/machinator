load("@rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "executor",
    srcs = [
        "beads_integration.go",
        "directive.go",
        "events.go",
        "executor.go",
        "launch.go",
        "monitor.go",
        "retry.go",
        "run.go",
        "trace.go",
        "triage.go",
    ],
    importpath = "github.com/bryantinsley/machinator/backend/internal/executor",
    visibility = ["//backend:__subpackages__"],
)

go_test(
    name = "executor_test",
    srcs = [
        "beads_integration_test.go",
        "directive_test.go",
        "events_test.go",
        "monitor_test.go",
        "trace_test.go",
        "triage_test.go",
    ],
    embed = [":executor"],
    data = [
        "//:AGENTS.md",
        "//bootstrap:directive_template.txt",
    ],
)

go_test(
    name = "executor_integration_test",
    srcs = ["integration_test.go"],
    data = [
        "//:AGENTS.md",
        "//backend/cmd/dummy-gemini",
        "//bootstrap:directive_template.txt",
    ],
    embed = [":executor"],
    tags = ["integration"],
)
