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
than a second using golang even on a cold start. Based on that this project was born.

So when I got into researching golang and wanting to redev a .net project I cam across some obstacles. One
of those being organzing code into an app to easily deploy. Many of the golang projects are isolated, separate
libraries. Golang has a module system but putting it all together, building always seemed like it would
be painful. So I began researching build systems that might better facilitate a golang project. I came accross bazel which I actually recognized based on my time with Angular. Although at that point I really didn't know what bazel was. I had only heard of it. After doing a bit more research learned bazel would work well for organizing a go api. So knowing nothing about bazel to begin with used it to create independent microservices which also share code between them.

Once I had that piece working I needed a means of deployment to AWS. I needed a way to easily deploy each separate lambda to aws. I had never used serverless before but had heard of it. So I installed serverless and figured out how to easily deploy each go program to aws as a separate lambda.

At that point I had things working. However, there was a bunch of repetative code for persistence operatations.
So I decided to refactor all that into a generic entity API able to facilitate nearly any data management need.

I ended up categorizing all entity management operations into an interface:

```go
type Manager interface {
	Create(entity map[string]interface{}) (map[string]interface{}, error)
	Update(entity map[string]interface{}) (map[string]interface{}, error)
	Purge(storage string, entities ...map[string]interface{})
	Save(entity map[string]interface{}, storage string)
	Load(id string, loader string) map[string]interface{}
	Find(finder string, query string, data *EntityFinderDataBag) []map[string]interface{}
	Allow(id string, op string, loader string) (bool, map[string]interface{})
	AddFinder(name string, finder Finder)
	ExecuteHook(hook Hooks, entity map[string]interface{}) (map[string]interface{}, error)
	ExecuteCollectionHook(hook string, entities []map[string]interface{}) ([]map[string]interface{}, error)
}
```

* create - create new entity
* update - update existing entity
* purge - remove entity
* save - create/update tntity
* load - get one entity by id
* find - find multiple entities
* allow - allow operation like write or delete
* hooks* - augment above processes with custom code

The configuration for any new entity being based on this framework.

```go
type EntityManager struct {
	Config          EntityConfig
	Creator         Creator
	Updator         Updator
	Loaders         map[string]Loader
	Finders         map[string]Finder
	Storages        map[string]Storage
	Authorizers     map[string]Authorization
	Hooks           map[Hooks]EntityHook
	CollectionHooks map[string]EntityCollectionHook
}
```

* config - base entity config things like the name and id field name.
* creator - the adaptor responsible for creating a new entity.
* updator - the adaptor responsible for updating an existing entity.
* loaders - loading strategies for the entity
* finders - finder strageies for the entity
* storages - storage mechanisms for the entity.
* authorizers - authorization of things like write and delete for an entity.
* hooks - alter the above processes using custom code.

Creators and updators are wrappers around saving an entity. They provide default operations
like checking validity, and access.

Loaders are resonsible for loading one single entity. 

On the other hand, finders are responsible for loading multiple entities or partial entities.

The reason loaders are separate from finders is because you might want to store the full entity
information separate from the aggregate. For example, placing an entity in elastic which is
a partial but loading the full entity from an s3 json file. We are breaking up the storage of the full entity
vs. the storage of the entity that will be used in lists. The full entity can be loaded by id but in a scenario
where you want to display a list there other criteria to consider and possibly the entity is not the full entity
that is stored in that storage mechanism.

Another difference is also that finders accept templates. Golang templates can be used used to define queries.

Like this…

```go
{{ define "chatconversations" }}
SELECT 
     recipientid AS recipientId, 
     recipientlabel AS recipientLabel, 
     userid AS userId
 FROM
     chatconversations 
WHERE 
     userid = {{ bindValue .Req.RequestContext.Authorizer.claims.sub }}
{{end}}
```

It doesn't have to be CQL. It can also be an elastic search query.

```json
{{ define "vocabularies" }}
{
    "query": {
        "bool": {
            "filter": [
                {
                    "bool": {
                        "must": {
                            "term": {
                                "userId.keyword": {
                                    "value": "{{ .Req.RequestContext.Authorizer.claims.sub }}"
                                }
                            }
                        }   
                    }
                }
            ]
        }
    },
    "size": 100
}
{{ end }}
```

Go templates are used to define queries.

Its even possible to execute a query within a template and pass the result to another lambda.

Use the result of the lambda to build another query.

```go
{{ define "profilenavitems" }}
{
    "query": {
        "bool": {
            "should": [
                {{ $profiles := (query "profile/_profiles" .) }}
                {{ $res := (lambda "profile/ReadableProfiles" .Req.RequestContext.Authorizer.claims.sub $profiles) }}
                {
                    "bool": {
                        "filter": [
                            {
                                "terms": {
                                    "id.keyword": [
                                        {{ range $index, $id := $res.Data }}
                                            {{ if ne $index 0 }},{{ end }}"{{ index $id "value" }}"
                                        {{ end }}
                                    ]
                                }
                            }
                        ]
                    }
                },
                {
                    "bool": {
                        "filter": [
                            {
                                "terms": {
                                    "parentId.keyword": [
                                        {{ range $index, $id := $res.Data }}
                                            {{ if ne $index 0 }},{{ end }}"{{ index $id "value" }}"
                                        {{ end }}
                                    ]
                                }
                            }
                        ]
                    }
                }
            ]
        }
    },
    "size": 1000
}
{{ end }}
```

You can also embed templates into others.

```go
{{ define "ads" }}
{
    "query": {{ template "_ads" . }}
}
{{end}}
```

Where _ads is another define that evaluates to a json object.



