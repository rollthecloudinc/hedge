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

create - create new entity
update - update existing entity
purge - remove entity
save - create/update tntity
load - get one entity by id
find - find multiple entities
allow - allow operation like write or delete
hooks* - augment above processes with custom code

The configuration for any new entity being based on this framework.

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

config - base entity config things like the name and id field name.
creator - the adaptor responsible for creating a new entity.
updator - the adaptor responsible for updating an existing entity.
loaders - loading strategies for the entity
finders - finder strageies for the entity
storages - storage mechanisms for the entity.
authorizers - authorization of things like write and delete for an entity.
hooks - alter the above processes using custom code.

