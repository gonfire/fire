package fire

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Application struct {
	Base      `bson:",inline" fire:"application:applications"`
	Name      string   `json:"name" valid:"required"`
	Key       string   `json:"key" valid:"required" fire:"identifiable"`
	Secret    []byte   `json:"secret" valid:"required" fire:"verifiable"`
	Scopes    []string `json:"scopes" valid:"required" fire:"grantable"`
	Callbacks []string `json:"callbacks" valid:"required" fire:"callable"`
}

type User struct {
	Base     `bson:",inline" fire:"user:users"`
	FullName string `json:"full_name" valid:"required"`
	Email    string `json:"email" valid:"required" fire:"identifiable"`
	Password []byte `json:"-" valid:"required" fire:"verifiable"`
}

type Post struct {
	Base     `bson:",inline" fire:"post:posts"`
	Title    string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	TextBody string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
}

type Comment struct {
	Base    `bson:",inline" fire:"comment:comments"`
	Message string         `json:"message" valid:"required"`
	Parent  *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

func Example() {
	// connect to database
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire")
	if err != nil {
		panic(err)
	}

	// defer close
	defer sess.Close()

	// get db
	db := sess.DB("")

	// create authenticator
	authenticator := NewAuthenticator(db, "a-very-long-secret", time.Hour)

	// set models
	authenticator.SetModels(&Application{}, &User{}, &AccessToken{})

	// create endpoint
	endpoint := NewEndpoint(db)

	// add post
	endpoint.AddResource(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer("admin"),
	})

	// add comment
	endpoint.AddResource(&Resource{
		Model:      &Comment{},
		Authorizer: authenticator.Authorizer("admin"),
	})

	// create router
	router := gin.New()

	// register authenticator
	authenticator.Register("auth", router)

	// register api
	endpoint.Register("api", router)

	fmt.Println("server ready to run")

	// run server
	//err = router.Run("localhost:8080")
	//if err != nil {
	//	panic(err)
	//}

	// Output:
	// server ready to run
}