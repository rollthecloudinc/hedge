load("@rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "proxy",
    embed = [":proxy_lib"],
    goarch = "amd64",
    goos = "linux",
    importpath = "goclassifieds/api/proxy",
    visibility = ["//visibility:public"],
    out = "bootstrap"
)

go_library(
    name = "proxy_lib",
    srcs = ["main.go"],
    importpath = "goclassifieds/api/proxy",
    visibility = ["//visibility:private"],
    deps = [
        "//lib/utils",
    ],
)