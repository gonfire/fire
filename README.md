<img src="http://joel-github-static.s3.amazonaws.com/gonfire/logo.png" alt="Logo" width="256"/>

# Go on Fire

[![Build Status](https://travis-ci.org/256dpi/fire.svg?branch=master)](https://travis-ci.org/256dpi/fire)
[![Coverage Status](https://coveralls.io/repos/github/256dpi/fire/badge.svg?branch=master)](https://coveralls.io/github/256dpi/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/256dpi/fire)](http://goreportcard.com/report/256dpi/fire)

**An idiomatic micro-framework for building Ember.js compatible APIs with Go.**

[Go on Fire](https://gonfire.org) is built on top of the wonderful built-in [http](https://golang.org/pkg/net/http) package, implements the [JSON API](http://jsonapi.org) specification through the dedicated [jsonapi](https://github.com/256dpi/jsonapi) library, uses the official [mongo](https://github.com/mongodb/mongo-go-driver) driver for persisting resources with [MongoDB](https://www.mongodb.com) and leverages the dedicated [oauth2](https://github.com/256dpi/oauth2) library to provide out of the box support for [OAuth2](https://oauth.net/2/) authentication using [JWT](https://jwt.io) tokens.

The deliberate and tight integration of these components provides a very simple and extensible set of abstractions for rapidly building backend services for websites that use [Ember.js](http://emberjs.com) as their frontend framework. Of course it can also be used in conjunction with any other single page application framework or as a backend for native mobile applications.

To quickly get started with building an API with Go on Fire follow the [quickstart guide](https://github.com/256dpi/fire#quickstart), read the detailed sections in this documentation and refer to the [package documentation](https://godoc.org/github.com/256dpi/fire) for more detailed information on the used types and methods.

## Features

Go on Fire ships with builtin support for various features to also provide a complete toolkit for advanced projects:

- Declarative definition of models and resource controllers.
- Custom group, collection and resource actions.
- Builtin validators incl. automatic relationship validation.
- Callback based plugin system for easy extendability.
- Integrated asynchronous and distributed job queuing system.
- Event sourcing via SSE and WebSockets.
- Declarative authentication and authorization framework.
- Integrated OAuth2 authenticator and authorizer.
- Support for tracing via [opentracing](https://opentracing.io).

## Example

The [example](https://github.com/256dpi/fire/tree/master/example) application implements an API will all Go on Fire features.

## Quickstart

First of all, install the package using the go tool:

```bash
$ go get -u github.com/256dpi/fire
```

Then import the `fire` package in your Go project:

```go
import "github.com/256dpi/fire"
```

### Declare Models

A basic declaration of models looks like the following example for a blog system:

```go
type Post struct {
	coal.Base  `json:"-" bson:",inline" coal:"posts"`
	Title      string        `json:"title" bson:"title"`
	TextBody   string        `json:"text-body" bson:"text_body"`
	Comments   coal.HasMany  `json:"-" bson:"-" coal:"comments:comments:post"`
}

type Comment struct {
	coal.Base  `json:"-" bson:",inline" coal:"comments"`
	Message    string         `json:"message"`
	Parent     *bson.ObjectId `json:"-" coal:"parent:comments"`
	PostID     bson.ObjectId  `json:"-" bson:"post_id" coal:"post:posts"`
}
```

Following that we need to create a store that is responsible for managing the database connection: 

```go
store := coal.MustCreateStore("mongodb://localhost/my-app")
```

### Create Controllers

Controllers make the previously declared models available from the JSON API:

```go
group := fire.NewGroup()

group.Add(&fire.Controller{
    Model: &Post{},
    Store: store,
})

group.Add(&fire.Controller{
    Model: &Comment{},
    Store: store,
})
```

### Run Application

Finally, the controller group can be served using the built-in http package:

```go
http.Handle("/api/", group.Endpoint("/api/"))

http.ListenAndServe(":4000", nil)
```

The JSON API is now available at `http://0.0.0.0:4000/api` and ready to be integrated in an Ember project.

Go on Fire provides various advanced features to hook into the request processing flow and adds for example authentication or more complex validation of models. Please read the following documentation carefully to get an overview of all available features.

## Models

Go on Fire implements a small introspection library that is able to infer all necessary meta information about your models from the already available `json` and `bson` struct tags. Additionally it introduces the `coal` struct tag that is used to declare to-one, to-many and has-many relationships.

### Basics

The [`Base`](https://godoc.org/github.com/256dpi/fire/coal#Base) struct has to be embedded in every Go on Fire model as it holds the document id and defines the models plural name and collection via the `coal:"plural-name[:collection]"` struct tag:

```go
type Post struct {
    coal.Base `json:"-" bson:",inline" coal:"posts"`
    // ...
}
```

- If the collection is not explicitly set the plural name is used instead.
- The plural name of the model is also the type for to-one, to-many and has-many relationships.

Note: Ember Data requires you to use dashed names for multi-word model names like `blog-posts`.

All other fields of a struct are treated as attributes except for relationships (more on that later):

```go
type Post struct {
    // ...
    Title    string `json:"title" bson:"title"`
    TextBody string `json:"text-body" bson:"text_body"`
    // ...
}
```

- Fire will use the `bson` struct tag to infer the database field or fallback to the lowercase version of the field name.
- The `json` struct tag is used for marshaling and unmarshaling the models attributes from or to a JSON API resource object. Hidden fields can be marked with the tag `json:"-"`. Fields that may only be present while creating the resource (e.g. a plain password field) can be made optional using `json:"password,omitempty" bson:"-"`.
- The `coal` tag may used on fields to tag them with custom and builtin tags.

Note: Ember Data requires you to use dashed names for multi-word attribute names like `text-body`.

### Helpers

The [`ID`](https://godoc.org/github.com/256dpi/fire/coal#Base.ID) method can be used to get the document id:

```go
post.ID()
```

The [`MustGet`](https://godoc.org/github.com/256dpi/fire/coal#Base.MustGet) and [`MustSet`](https://godoc.org/github.com/256dpi/fire/coal#Base.MustSet) method can be used to get and set any field on the model:

```go
title := post.MustGet("title")
post.MustSet("title", "New Title")
```

- Both methods use the field name e.g. `TextBody` to find the value and panic if no matching field is found.
- Calling [`MustSet`](https://godoc.org/github.com/256dpi/fire#Base.MustSet) with a different type than the field causes a panic.

### Meta

All parsed information from the model struct and its tags is saved to the [`Meta`](https://godoc.org/github.com/256dpi/fire/coal#Meta) struct that can be accessed using the [`Meta`](https://godoc.org/github.com/256dpi/fire/coal#Base.Meta) method:

```go
post.Meta().Name
post.Meta().PluralName
post.Meta().Collection
post.Meta().Fields
post.Meta().OrderedFields
post.Meta().DatabaseFields
post.Meta().Attributes
post.Meta().Relationships
post.Meta().FlaggedFields
```

### To-One Relationships

Fields of the type `primitive.ObjectID` or `*primitive.ObjectID` can be marked as to-one relationships using the `coal:"name:type"` struct tag:

```go
type Comment struct {
	// ...
	Post primitive.ObjectID `json:"-" bson:"post_id" coal:"post:posts"`
    // ...
}
```

- Fields of the type `*primitive.ObjectID` are treated as optional relationships

Note: To-one relationship fields should be excluded from the attributes object by using the `json:"-"` struct tag.

Note: Ember Data requires you to use dashed names for multi-word relationship names like `last-posts`.

### To-Many Relationships

Fields of the type `[]primitive.ObjectID` can be marked as to-many relationships using the `coal:"name:type"` struct tag:

```go
type Selection struct {
    // ...
	Posts []primitive.ObjectID `json:"-" bson:"post_ids" coal:"posts:posts"`
	// ...
}
```

Note: To-many relationship fields should be excluded from the attributes object by using the `json:"-"` struct tag.

Note: Ember Data requires you to use dashed names for multi-word relationship names like `favorited-posts`.

### Has-Many Relationships

Fields that have a `HasMany` as their type define the inverse of a to-one relationship and require the `coal:"name:type:inverse"` struct tag:

```go
type Post struct {
    // ...
	Comments coal.HasMany `json:"-" bson:"-" coal:"comments:comments:post"`
	// ...
}
```

Note: Ember Data requires you to use dashed names for multi-word relationship names like `authored-posts`.

Note: These fields should have the `json:"-" bson"-"` tag set, as they are only syntactic sugar and hold no other information.

## Controllers

Go on Fire implements the JSON API specification and provides the management of the previously declared models via a set of controllers that are combined to a group which provides the necessary interconnection between resources.

Controllers are declared by creating a [`Controller`](https://godoc.org/github.com/256dpi/fire#Controller) and providing a reference to the model and a store:

```go
postsController := &fire.Controller{
    Model: &Post{},
    Store: store,
    // ...
}
```

### Groups

Controller groups provide the necessary interconnection and integration between controllers. A [`Group`](https://godoc.org/github.com/256dpi/fire#Group) can be created by calling [`NewGroup`](https://godoc.org/github.com/256dpi/fire#NewGroup) while controllers are added using [`Add`](https://godoc.org/github.com/256dpi/fire#Group.Add):

```go
group := fire.NewGroup()

group.Add(postsController)
group.Add(commentsController)
````

### Filtering & Sorting

To enable the built-in support for filtering and sorting via the JSON API specification you need to specify the allowed fields for each feature:

```go
postsController := &fire.Controller{
    // ...
    Filters: []string{"Title", "Published"},
    Sorters: []string{"Title"},
    // ...
}
```

Filters can be activated using the `/posts?filter[published]=true` query parameter while the sorting can be specified with the `/posts?sort=created-at` (ascending) or `/posts?sort=-created-at` (descending) query parameter.

Note: `true` and `false` are automatically converted to boolean values if the field has the `bool` type.

More information about filtering and sorting can be found in the [JSON API Spec](http://jsonapi.org/format/#fetching-sorting).

### Sparse Fieldsets

Sparse Fieldsets are automatically supported on all responses an can be activated using the `/posts?fields[posts]=bar` query parameter.

More information about sparse fieldsets can be found in the [JSON API Spec](http://jsonapi.org/format/#fetching-sparse-fieldsets).

### Callbacks

Controllers support the definition of multiple callbacks that are called while processing the requests:

```go
postsController := &fire.Controller{
    // ...
    Authorizers: []fire.Callback{
        fire.C("MyAuthorizer", fire.All(), func(ctx *fire.Context) error {
            // ...
        }),
        // ...
    },
    Validators: []fire.Callback{
        fire.C("MyValidator", fire.All(), func(ctx *fire.Context) error {
            // ...
        }),
        // ...
    },
}
```

The [`Authorizer`](https://godoc.org/github.com/256dpi/fire#Callback) callbacks are run after inferring all available data from the request and are therefore perfectly suited to do a general user authentication. The [`Validator`](https://godoc.org/github.com/256dpi/fire#Callback) callbacks are only run before creating, updating or deleting a model and are ideal to protect resources from certain actions.

Errors returned by the callbacks are serialize to an JSON API compliant error object and yield an "unauthorized" status code from an authorizer and a "bad request" status code from a validator.

### Safe Errors

If errors are marked as [`Safe`](https://godoc.org/github.com/256dpi/fire#Safe) or constructed using the [`E`](https://godoc.org/github.com/256dpi/fire#E) helper, the error message is serialized and returned in the JSON-API error response:

### Built-in Callbacks

Fire ships with several built-in callbacks that implement common concerns:

- [Basic Authorizer](https://godoc.org/github.com/256dpi/fire#BasicAuthorizer)
- [Model Validator](https://godoc.org/github.com/256dpi/fire#ModelValidator)
- [Protected Fields Validator](https://godoc.org/github.com/256dpi/fire#ProtectedFieldsValidator)
- [Dependent Resources Validator](https://godoc.org/github.com/256dpi/fire#DependentResourcesValidator)
- [Referenced Resources Validator](https://godoc.org/github.com/256dpi/fire#ReferencedResourcesValidator)
- [Matching References Validator](https://godoc.org/github.com/256dpi/fire#MatchingReferencesValidator)
- [Unique Field Validator](https://godoc.org/github.com/256dpi/fire#UniqueFieldValidator)
- [Relationship Validator](https://godoc.org/github.com/256dpi/fire#RelationshipValidator)
- [Timestamp Validator](https://godoc.org/github.com/256dpi/fire#TimestampValidator)

## Authenticator

The [`flame`](https://godoc.org/github.com/256dpi/fire/flame) sub package implements the OAuth2 specification and provides the Resource Owner Password, Client Credentials and Implicit grant. The issued access and refresh tokens are [JWT](https://jwt.io) tokens and are thus able to transport custom data.

Every authenticator needs a [`Policy`](https://godoc.org/github.com/256dpi/fire/flame#Policy) that describes how the authentication is enforced. A basic policy can be created and extended using [`DefaultPolicy`](https://godoc.org/github.com/256dpi/fire/flame#DefaultPolicy):

```go
policy := flame.DefaultPolicy("a-very-long-secret")
policy.PasswordGrant = true
```

- The default policy uses the built-in [`Token`](https://godoc.org/github.com/256dpi/fire/flame#Token), [`User`](https://godoc.org/github.com/256dpi/fire/flame#User) and [`Application`](https://godoc.org/github.com/256dpi/fire/flame#Application) model and the [`DefaultGrantStrategy`](https://godoc.org/github.com/256dpi/fire/flame#DefaultGrantStrategy).

An [`Authenticator`](https://godoc.org/github.com/256dpi/fire/flame#Authenticator) is created by specifying the policy and store. After that, it can be mounted and served using for example the built-in http package:

```go
authenticator := flame.NewAuthenticator(store, policy)

http.Handle("/auth/", authenticator.Endpoint("/auth/"))
```

More information about OAuth2 flows can be found [here](https://www.digitalocean.com/community/tutorials/an-introduction-to-oauth-2).

### Scope

The default grant strategy grants the requested scope if the client satisfies the scope. However, most applications want to grant the scope based on client types and owner roles. A custom grant strategy can be implemented by setting a different `GrantStrategy`.

The following example callback grants the `default` scope and additionally the `admin` scope if the user has the admin flag set:
 
```go
policy.GrantStrategy = func(scope oauth2.Scope, client flame.Client, ro flame.ResourceOwner) (oauth2.Scope, error) {
    list := oauth2.Scope{"default"}
    
    if ro != nil && ro.(*User).Admin {
        list = append(list, "admin")
    }

    return list, nil
}
```

### Authorization

The authenticator can be used to authorize access to JSON API resources by using the  [`Callback`](https://godoc.org/github.com/256dpi/fire/flame#Callback) with a scope that must have been granted:

```go
postsController := &fire.Controller{
    // ...
    Authorizers: []fire.Callback{
        flame.Callback(true, "admin"),
        // ...
    },
    // ...
}
```

- The authorizer will assign the authorized [`Token`](https://godoc.org/github.com/256dpi/fire/flame#Token) to the context using the [`AccessTokenContextKey`](https://godoc.org/github.com/256dpi/fire/flame#AccessTokenContextKey) key.

## Tools

Go on Fire ships with a selection of built-in tools that provide common functionality used by many applications:

- [Asset Server](https://godoc.org/github.com/256dpi/fire#AssetServer)
- [Error Reporter](https://godoc.org/github.com/256dpi/fire#ErrorReporter)

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
