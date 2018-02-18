package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestC(t *testing.T) {
	assert.Equal(t, "posts", C(&postModel{}))
}

func TestF(t *testing.T) {
	assert.Equal(t, "text_body", F(&postModel{}, "TextBody"))

	assert.PanicsWithValue(t, `coal: field "Foo" not found on "coal.postModel"`, func() {
		F(&postModel{}, "Foo")
	})
}

func TestA(t *testing.T) {
	assert.Equal(t, "text-body", A(&postModel{}, "TextBody"))

	assert.PanicsWithValue(t, `coal: field "Foo" not found on "coal.postModel"`, func() {
		A(&postModel{}, "Foo")
	})
}

func TestR(t *testing.T) {
	assert.Equal(t, "post", R(&commentModel{}, "Post"))

	assert.PanicsWithValue(t, `coal: field "Foo" not found on "coal.postModel"`, func() {
		R(&postModel{}, "Foo")
	})
}

func TestP(t *testing.T) {
	id := bson.NewObjectId()
	assert.Equal(t, &id, P(id))
}

func TestN(t *testing.T) {
	var id *bson.ObjectId
	assert.Equal(t, id, N())
	assert.NotEqual(t, nil, N())
}

func TestUnique(t *testing.T) {
	id1 := bson.NewObjectId()
	id2 := bson.NewObjectId()

	assert.Equal(t, []bson.ObjectId{id1}, Unique([]bson.ObjectId{id1}))
	assert.Equal(t, []bson.ObjectId{id1}, Unique([]bson.ObjectId{id1, id1}))
	assert.Equal(t, []bson.ObjectId{id1, id2}, Unique([]bson.ObjectId{id1, id2, id1}))
	assert.Equal(t, []bson.ObjectId{id1, id2}, Unique([]bson.ObjectId{id1, id2, id1, id2}))
}

func TestContains(t *testing.T) {
	a := bson.NewObjectId()
	b := bson.NewObjectId()
	c := bson.NewObjectId()
	d := bson.NewObjectId()

	assert.True(t, Contains([]bson.ObjectId{a, b, c}, a))
	assert.True(t, Contains([]bson.ObjectId{a, b, c}, b))
	assert.True(t, Contains([]bson.ObjectId{a, b, c}, c))
	assert.False(t, Contains([]bson.ObjectId{a, b, c}, d))
}
