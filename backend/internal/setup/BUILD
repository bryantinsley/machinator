load("@rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "setup",
    srcs = ["setup.go"],
    importpath = "github.com/bryantinsley/machinator/backend/internal/setup",
    visibility = ["//backend:__subpackages__"],
)

go_test(
    name = "setup_test",
    srcs = ["setup_test.go"],
    embed = [":setup"],
)
