# Ensure a clean WORKSPACE file post-module migration

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Rules for Go (toolchain and Gazelle dependencies)
load("@rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")
go_rules_dependencies()
go_register_toolchains(version = "1.20.5")  # Adjust to your chosen Go version

load("@gazelle//:deps.bzl", "gazelle_dependencies")
gazelle_dependencies()

# Rules for Node.js (npm_install remains in WORKSPACE)
load("@rules_nodejs//:index.bzl", "node_repositories", "npm_install")
node_repositories(
    node_version = "16.6.2",
    package_json = ["//:package.json"],
)
npm_install(
    name = "npm",
    package_json = "//:package.json",
    package_lock_json = "//:package-lock.json",
)

# Commented rules for Docker/OCI containers (rules_oci was migrated to MODULE.bazel)
# Uncomment and use if required
#load("@rules_oci//oci:repositories.bzl", "oci_repository")
#oci_repository(
#    name = "ubuntu_20_04_slim",
#    tag = "20.04",
#    registry = "docker.io",
#    repository = "library/ubuntu",
#)