<img width="146" alt="Screen Shot 2022-06-13 at 1 43 16 AM" src="https://user-images.githubusercontent.com/73197190/173287441-8ce440b1-2833-4950-8a75-c75f28304c3c.png">


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

# Cloud

* AWS
  * Cognito
  * API Gateway
    * HTTP
    * Websocket
  * Lambda
  * Open Search
  * s3
  * Key Spaces (cassandra)

# Purpose

Intended to be used internally for satisfying specific domain requirements of Roll the Cloud initiatives. These APIs fill gaps when direct communication with AWS is not possible in the browser for [druids](https://github.com/ng-druid/platform).

* security vulnerability
* sdk incompatibility
* event bridge handler
* secure communication w/ vendors outside of AWS

# Considerations

New and existing APIs should be created and repurporsed / replaced with maximum reusability in mind across the corporation. An example of this is the [internal entity API](https://github.com/verti-go/main/wiki/Entity-API). The entity API is intended to manage persistence and search of entities accross any number of source destinations.
