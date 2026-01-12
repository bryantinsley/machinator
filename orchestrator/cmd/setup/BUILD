load("@rules_go//go:def.bzl", "go_binary", "go_cross_binary", "go_library")

go_binary(
    name = "setup",
    embed = [":setup_lib"],
    visibility = ["//visibility:public"],
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
