package jsonapi

import (
	"testing"

	"github.com/gonfire/fire/model"
	"github.com/stretchr/testify/assert"
)

func TestAction(t *testing.T) {
	table := []struct {
		a Action
		r bool
		w bool
	}{
		{List, true, false},
		{Find, true, false},
		{Create, false, true},
		{Update, false, true},
		{Delete, false, true},
	}

	for _, entry := range table {
		assert.Equal(t, entry.r, entry.a.Read())
		assert.Equal(t, entry.w, entry.a.Write())
	}
}

func TestContextOriginal(t *testing.T) {
	store := getCleanStore()

	savedPost := model.Init(&Post{
		Title: "foo",
	}).(*Post)

	saveModel(savedPost)

	post := model.Init(&Post{
		Title: "bar",
	}).(*Post)

	post.DocID = savedPost.DocID

	ctx := &Context{
		Action: Update,
		Model:  post,
		Store:  store,
	}

	m, err := ctx.Original()
	assert.NoError(t, err)
	assert.Equal(t, savedPost.ID(), m.ID())
	assert.Equal(t, savedPost.Get("Title"), m.Get("Title"))
}

func TestContextOriginalWrongAction(t *testing.T) {
	ctx := &Context{
		Action: Find,
	}

	assert.Panics(t, func() {
		ctx.Original()
	})
}

func TestContextOriginalNonExisting(t *testing.T) {
	store := getCleanStore()

	post := model.Init(&Post{
		Title: "foo",
	}).(*Post)

	ctx := &Context{
		Action: Update,
		Model:  post,
		Store:  store,
	}

	m, err := ctx.Original()
	assert.Error(t, err)
	assert.Nil(t, m)
}