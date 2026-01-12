load("@rules_go//go:def.bzl", "go_binary", "go_cross_binary", "go_library")

go_binary(
    name = "machinator",
    data = [
        "//templates:directive_template.txt",
        "//templates:setup_go_env.sh",
        "//templates:unblocking_directive.txt",
    ],
    embed = [":machinator_lib"],
    visibility = ["//visibility:public"],
)

go_cross_binary(
    name = "machinator_linux",
    platform = "@rules_go//go/toolchain:linux_amd64",
    target = ":machinator",
    visibility = ["//visibility:public"],
)

go_library(
    name = "machinator_lib",
    srcs = ["main.go"],
    importpath = "github.com/bryantinsley/machinator/orchestrator/cmd/machinator",
    visibility = ["//visibility:private"],
    deps = [
        "//orchestrator/pkg/orchestrator",
        "//orchestrator/pkg/setup",
    ],
)
