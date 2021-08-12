# Verti-go

Vertigo is inspired by all the legacy projects out there that people still use. It is one persons vision to take everything that has been learned and create a modern API. It is to stop listening to people who say things can't be done or thigs are good enough the way they are. It is to innovate, inspire, and make people fall in love with programming again. It is to flip it all upside down and reevaluate. 

# Summary

Vertigo is a modern, back-end api for publishing content. The primary goal of vertigo is to provide a modern, fast, scalable API to publish content to the web or any other digital medium. This is achieved by abandoning long lived practices. Practices that work but require a whole a bunch of work-arounds to make efficient. Vertigo flips the traditional upside down. This project uses lesser known technology. It uses lesser known languages. Everything here works together to achieve common business goals within the digital realm.

## Go

Go is the fastest language on serverless infrastructure. Not to mention its clear, concise and doesn't take a whole lot to learn. Most of the apis, functions, and libraries here are built with go.

## Serverless

Severless is the backbone of this project. Everything serverless. Go APIs are deployed as lambda that are hooked up with aws gateways. Independent functions are also also hooked up with aws service actions. Everything is based on creating small, concrete, focused units that accomplish one goal.

## s3

s3 is a critical part of the infrastructure to manage data. No only binary data such as images. s3 is also used to store json documents. s3 acts as the master for entity storage. s3 owns most entities. Entities might be stored partially in other places but s3 maintains the complete entity as a json document. Instead of using a NoSQL solution like Mongo completre, master entities are mostly stored as flat files on s3.

## Elastic

Elastic is used as the primary search mechanism.

Instead of distrubuting data between multiple storages that could be used to store the full entity / document data and search / filter that data things are done different. s3 is used to store the full entity / document. Elastic is used for searching documents not s3. s3 is only used to pull down a specific document.

## Cassandra

Keyspaces is being used to store some data as well. Chat messages are stored in cassandra. Cassandra is a middle ground between s3/elastic. Its difficult to describe but there are some things that just don't seem right placing into s3 or elastic.

# Extra

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

## Entity API

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

## Loaders and Finders

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

## Saving

There are so many ways to store data these days. I mean back in the old days we basically just stuffed that
crap into some relational database like MySQL. However, in this modern age there are so many other alternative
storage solutions. We not only have relational databases but none relational, search systems, the list goes on.
That being said the concept of persisting and finding information remains similar. Especially saving/persisting
information to some type of storage facility. The idea of saving information hasn't changed very much. Before
that save occurs there are things that need to be done. We need to make sure that the information being sent
is valid, user is allowed to carry out the action. 

This is where we can begin to separate storage from creating and updating. Storage is the action of persisting
the information without any type of guards. Storage is raw data being sent and we just place 
wherever. Save and update are the wrapper around that which provide guards against inaccurate data and data that a user
is not allowed to change. Saving and updating are wrappers around the storage that are meant to maintain 
the integrity of the information being being stored. If an entity requires a title and the title is not supplied
the create/update should reject the entity before it ever gets to the storage persistence. If a user tries to update
an entity but is not allowed to do so the action should be rejected before getting to the storage persistence.

## Authorizers

Authorizers sit in the middle of that process. Authorizers are meant to ask the question whether someone is allowed
to carry out an action on an entity. There are many circumstances to restrict actions on access but lets discuss
the basic case. If you create a blog entry. You are the owner. Should other people be able to change that blog
entry which you created. I would say not. However, should you be able to change it – I would say so. So that
represents a fairly basic case of owners are allowed to change their blog entries but others are not allowed to modify other
individuals blog entries. Authorizers are applied to entity create, update, and delete operations. I mean… should
your blog entry be able to be deleted by John Smith. The answer is no. The authorizer is meant to enforce those kind of rules
and prevent that type of action by someone who should not be able to carry it out.

## Hooks

Hooks are used to change entities and alter collections. Standard hooks can alter an entity before and after it
is saved. Collection hooks can completely modify collections returned using find.

Collection hooks function similiar to rxjs functions. The example here demostrates building
a complete one on one chat stream. So what we end up with is messsages the current user sent to the target and those sent by the target to the current user. Inspired by rxjs the idea is to take the result set of the function and merge it with
the existing result set. These operators can all be combined together like rxjs operators hence the name of
the base function "PipeCollectionHooks".

```go
		CollectionHooks: map[string]entity.EntityCollectionHook{
			"default/chatmessages": entity.PipeCollectionHooks(
				entity.MergeEntities(func(m *entity.EntityManager) []map[string]interface{} {
					allAttributes := make([]entity.EntityAttribute, 0)
					data := entity.EntityFinderDataBag{
						Req:        req,
						Attributes: allAttributes,
					}
					return m.Find("default", "_chatmessages_inverse", &data)
				},
			)),
		},
```

Multiple collection hooks:

```go
				CollectionHooks: map[string]entity.EntityCollectionHook{
					"default/_chatconnections": entity.PipeCollectionHooks(
						entity.FilterEntities(func(ent map[string]interface{}) bool {
							return ent["createdAt"].(time.Time).After(time.Now().Add(-1 * time.Hour))
						}),
						entity.MergeEntities(func(m *entity.EntityManager) []map[string]interface{} {
							allAttributes := make([]entity.EntityAttribute, 0)
							data := entity.EntityFinderDataBag{
								Req:        req,
								Attributes: allAttributes,
								Metadata: map[string]interface{}{
									"recipientId": ent["recipientId"],
								},
							}
							return m.Find("default", "_chatconnections_inverse", &data)
						}),
					)
				},
			}
```

So in that case the idea is to filter the existing collection than merge it with a new collection.

