load("@rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "mock-bd_lib",
    srcs = ["main.go"],
    importpath = "github.com/bryantinsley/machinator/tools/mock-bd",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "mock-bd",
    embed = [":mock-bd_lib"],
    visibility = ["//visibility:public"],
)
