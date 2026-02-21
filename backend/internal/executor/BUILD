load("@rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "executor",
    srcs = [
        "directive.go",
        "executor.go",
        "launch.go",
    ],
    importpath = "github.com/bryantinsley/machinator/backend/internal/executor",
    visibility = ["//backend:__subpackages__"],
)

go_test(
    name = "executor_test",
    srcs = ["directive_test.go"],
    embed = [":executor"],
    data = [
        "//:AGENTS.md",
        "//bootstrap:directive_template.txt",
    ],
)
