load("@rules_go//go:def.bzl", "go_binary", "go_cross_binary", "go_library")
load("//testing:golden.bzl", "golden_test")

go_binary(
    name = "setup",
    embed = [":setup_lib"],
    visibility = ["//visibility:public"],
)

golden_test(
    name = "main_screen_golden",
    args = ["--headless"],
    binary = ":setup",
    golden_file = "testdata/main_screen.golden",
)

go_cross_binary(
    name = "setup_linux",
    platform = "@rules_go//go/toolchain:linux_amd64",
    target = ":setup",
    visibility = ["//visibility:public"],
)

go_library(
    name = "setup_lib",
    srcs = ["main.go"],
    importpath = "github.com/bryantinsley/machinator/orchestrator/cmd/setup",
    visibility = ["//visibility:private"],
    deps = ["//orchestrator/pkg/setup"],
)
