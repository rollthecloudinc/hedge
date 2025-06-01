def setup_node_dependencies():
    # Load Node.js rules
    load("@build_bazel_rules_nodejs//:index.bzl", "node_repositories", "npm_install")

    # Declare Node.js repositories
    node_repositories(
        node_version = "16.6.2",
        package_json = ["//:package.json"],
    )

    # Install NPM packages
    npm_install(
        name = "npm",
        package_json = "//:package.json",
        package_lock_json = "//:package-lock.json",
    )