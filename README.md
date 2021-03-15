# Verti-go

Serverless backend api built with golang using bazel.

Associated front-end:

https://github.com/ng-druid

This was all originally programmed using gin. I than realized I didn't need that all and ripped it all out.

No MVC framework needed when using serveless development.

Not to mention the cold start time with golang surpasses any other language.

This is the fastest service layer in existence using serverless technology.

Before I built this with golang it was a .NET project. I began with intention of running it on kubernetes.
I than realized kubernetes is very expensive. So I looked at different options. One I came accross was
lambda. So I modified my .NET project to "function" on lambda. When I did so I was sometimes seeing 6 second
response times for end-points. So I did a bit of research. Learning c# has one of the worst cold start
times on lambda. At that point I decided to look into other tech better suited to a serverless envrionment.
The main language I came accross was golang. So I decided just rewrite everything in go. The result in response
times and cold start astronomical compared to .net. Requests that might take up to 6 seconds take less
than a second using golang even on a cold start.

