load("@rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

def setup_go_dependencies():

    # Load Go rules dependencies
    go_rules_dependencies()
    go_register_toolchains(version = "1.20.5")  # Set Go toolchain version

    # Define external Go repositories (examples shown for dependencies)
    go_repository(
        name = "com_github_dgrijalva_jwt_go",
        importpath = "github.com/dgrijalva/jwt-go",
        sum = "h1:7qlOGliEKZXTDg6OTjfoBKDXWrumCAMpl/TFQ4/5kLM=",
        version = "v3.2.0+incompatible",
    )

    go_repository(
        name = "com_github_golang_jwt_jwt_v4",
        importpath = "github.com/golang-jwt/jwt/v4",
        sum = "h1:rcc4lwaZgFMCZ5jxF9ABolDcIHdBytAFgqFPbSJQAYs=",
        version = "v4.4.2",
    )

    gazelle_dependencies()  # Required for adjusting Go dependencies for Gazelle