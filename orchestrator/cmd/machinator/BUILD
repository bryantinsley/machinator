load("@rules_go//go:def.bzl", "go_binary")

go_binary(
    name = "machinator",
    srcs = ["main.go"],
    data = [
        "//templates:directive_template.txt",
        "//templates:setup_go_env.sh",
        "//templates:unblocking_directive.txt",
    ],
    visibility = ["//visibility:public"],
    deps = [
        "//orchestrator/pkg/orchestrator",
        "//orchestrator/pkg/setup",
    ],
)
