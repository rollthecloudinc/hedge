# Summary

Cloud optimized back-end services following [principles of green software engineering](https://principles.green/) for [Roll the Cloud Inc](https://github.com/rollthecloudinc).

# Organization

This is a [bazel monorepo](https://bazel.build/) using [serverless framework](https://www.serverless.com/).

* api
  * Public lambdas exposed as part of API gateway.
* func
  * Independent lambdas execuated manually or via events.
* lib
  * Internal libraries shared accross entire organization.

# Languages

* golang
* nodejs
