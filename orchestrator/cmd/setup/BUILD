load("@rules_go//go:def.bzl", "go_binary", "go_cross_binary")

go_binary(
    name = "setup",
    srcs = ["main.go"],
    visibility = ["//visibility:public"],
    deps = [
        "//orchestrator/pkg/setup",
    ],
)

go_cross_binary(
    name = "setup_linux",
    platform = "@rules_go//go/toolchain:linux_amd64",
    target = ":setup",
    visibility = ["//visibility:public"],
)
