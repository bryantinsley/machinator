load("@rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "fixture-gen_lib",
    srcs = ["main.go"],
    importpath = "github.com/bryantinsley/machinator/tools/fixture-gen",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "fixture-gen",
    embed = [":fixture-gen_lib"],
    visibility = ["//visibility:public"],
)
