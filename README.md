# Summary

Cloud optimized back-end services following principles of green software enigneering for [Roll the Cloud Inc](https://github.com/rollthecloudinc).

# Architecture

This is a [bazel monorepo](https://bazel.build/) using [serverless framework](https://www.serverless.com/).

* api
  * Public APIs part of api gateway.
* func
  * Independent lambdas execuated manually or via events.
* lib
  * Internal libraries shared accross entire organization.

# Languages

* golang
* nodejs
