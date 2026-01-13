load("@rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "dummy-gemini_lib",
    srcs = ["main.go"],
    importpath = "github.com/bryantinsley/machinator/orchestrator/tools/dummy-gemini",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "dummy-gemini",
    embed = [":dummy-gemini_lib"],
    visibility = ["//visibility:public"],
)

go_binary(
    name = "dummy-gemini-linux",
    embed = [":dummy-gemini_lib"],
    goarch = "amd64",
    goos = "linux",
    visibility = ["//visibility:public"],
)

go_test(
    name = "dummy-gemini_test",
    srcs = ["main_test.go"],
    embed = [":dummy-gemini_lib"],
)