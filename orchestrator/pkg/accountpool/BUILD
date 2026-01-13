load("@rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "accountpool",
    srcs = [
        "pool.go",
        "types.go",
    ],
    importpath = "github.com/bryantinsley/machinator/orchestrator/pkg/accountpool",
    visibility = ["//visibility:public"],
)

go_test(
    name = "accountpool_test",
    srcs = [
        "loader_test.go",
        "pool_test.go",
    ],
    embed = [":accountpool"],
    tags = ["fast"],
)
