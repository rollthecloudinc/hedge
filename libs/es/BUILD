load("@io_bazel_rules_go//go:def.bzl", "go_library")
go_library(
     name = "es",
     srcs = ["main.go"],
     importpath = "goclassifieds/libs/es",
     visibility = ["//visibility:public"],
     deps = [
         "@com_github_elastic_go_elasticsearch_v7//:go_default_library"
     ]
)