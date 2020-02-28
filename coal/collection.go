package coal

import (
	"context"

	"github.com/256dpi/lungo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/cinder"
)

// Collection wraps a collection to automatically push tracing spans for
// run queries.
type Collection struct {
	coll  lungo.ICollection
	trace *cinder.Trace
}

// AggregateAll wraps the native Aggregate collection method and decodes all
// documents to the provided slice.
func (c *Collection) AggregateAll(ctx context.Context, slicePtr interface{}, pipeline interface{}, opts ...*options.AggregateOptions) error {
	// push span
	c.trace.Push("coal/Collection.Aggregate")
	c.trace.Tag("pipeline", pipeline)
	defer c.trace.Pop()

	// run query
	csr, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return err
	}

	// decode all documents
	err = csr.All(ctx, slicePtr)
	if err != nil {
		return err
	}

	return nil
}

// AggregateIter wraps the native Aggregate collection method and calls the
// provided callback with the decode method until an error is returned or the
// cursor has been exhausted.
func (c *Collection) AggregateIter(ctx context.Context, pipeline interface{}, fn func(func(interface{}) error) error, opts ...*options.AggregateOptions) error {
	// push span
	c.trace.Push("coal/Collection.Aggregate")
	c.trace.Tag("pipeline", pipeline)
	defer c.trace.Pop()

	// run query
	csr, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return err
	}

	// ensure cursor is closed
	defer csr.Close(ctx)

	// iterate over all documents
	for csr.Next(ctx) {
		err = fn(csr.Decode)
		if err != nil {
			return err
		}
	}

	// close cursor
	err = csr.Close(nil)
	if err != nil {
		return err
	}

	return nil
}

// BulkWrite wraps the native BulkWrite collection method.
func (c *Collection) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	// push span
	c.trace.Push("coal/Collection.BulkWrite")
	defer c.trace.Pop()

	// run query
	return c.coll.BulkWrite(ctx, models, opts...)
}

// CountDocuments wraps the native CountDocuments collection method.
func (c *Collection) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	// push span
	c.trace.Push("coal/Collection.CountDocuments")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.CountDocuments(ctx, filter, opts...)
}

// DeleteMany wraps the native DeleteMany collection method.
func (c *Collection) DeleteMany(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// push span
	c.trace.Push("coal/Collection.DeleteMany")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.DeleteMany(ctx, filter, opts...)
}

// DeleteOne wraps the native DeleteOne collection method.
func (c *Collection) DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	// push span
	c.trace.Push("coal/Collection.DeleteOne")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.DeleteOne(ctx, filter, opts...)
}

// Distinct wraps the native Distinct collection method.
func (c *Collection) Distinct(ctx context.Context, fieldName string, filter interface{}, opts ...*options.DistinctOptions) ([]interface{}, error) {
	// push span
	c.trace.Push("coal/Collection.Distinct")
	c.trace.Tag("fieldName", fieldName)
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.Distinct(ctx, fieldName, filter, opts...)
}

// EstimatedDocumentCount wraps the native EstimatedDocumentCount collection method.
func (c *Collection) EstimatedDocumentCount(ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	// push span
	c.trace.Push("coal/Collection.EstimatedDocumentCount")
	defer c.trace.Pop()

	// run query
	return c.coll.EstimatedDocumentCount(ctx, opts...)
}

// FindAll wraps the native Find collection method and decodes all documents to
// the provided slice.
func (c *Collection) FindAll(ctx context.Context, slicePtr interface{}, filter interface{}, opts ...*options.FindOptions) error {
	// push span
	c.trace.Push("coal/Collection.Find")
	c.trace.Tag("filter", filter)
	defer c.trace.Pop()

	// run query
	csr, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		return err
	}

	// decode all documents
	err = csr.All(ctx, slicePtr)
	if err != nil {
		return err
	}

	return nil
}

// FindIter wraps the native Find collection method and calls the provided
// callback with the decode method until an error is returned or the cursor has
// been exhausted.
func (c *Collection) FindIter(ctx context.Context, filter interface{}, fn func(func(interface{}) error) error, opts ...*options.FindOptions) error {
	// push span
	c.trace.Push("coal/Collection.Find")
	c.trace.Tag("filter", filter)
	defer c.trace.Pop()

	// run query
	csr, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		return err
	}

	// ensure cursor is closed
	defer csr.Close(ctx)

	// iterate over all documents
	for csr.Next(ctx) {
		err = fn(csr.Decode)
		if err != nil {
			return err
		}
	}

	// close cursor
	err = csr.Close(nil)
	if err != nil {
		return err
	}

	return nil
}

// FindOne wraps the native FindOne collection method.
func (c *Collection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) lungo.ISingleResult {
	// push span
	c.trace.Push("coal/Collection.FindOne")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.FindOne(ctx, filter, opts...)
}

// FindOneAndDelete wraps the native FindOneAndDelete collection method.
func (c *Collection) FindOneAndDelete(ctx context.Context, filter interface{}, opts ...*options.FindOneAndDeleteOptions) lungo.ISingleResult {
	// push span
	c.trace.Push("coal/Collection.FindOneAndDelete")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.FindOneAndDelete(ctx, filter, opts...)
}

// FindOneAndReplace wraps the native FindOneAndReplace collection method.
func (c *Collection) FindOneAndReplace(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.FindOneAndReplaceOptions) lungo.ISingleResult {
	// push span
	c.trace.Push("coal/Collection.FindOneAndReplace")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.FindOneAndReplace(ctx, filter, replacement, opts...)
}

// FindOneAndUpdate wraps the native FindOneAndUpdate collection method.
func (c *Collection) FindOneAndUpdate(ctx context.Context, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) lungo.ISingleResult {
	// push span
	c.trace.Push("coal/Collection.FindOneAndUpdate")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.FindOneAndUpdate(ctx, filter, update, opts...)
}

// InsertMany wraps the native InsertMany collection method.
func (c *Collection) InsertMany(ctx context.Context, documents []interface{}, opts ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	// push span
	c.trace.Push("coal/Collection.InsertMany")
	defer c.trace.Pop()

	// run query
	return c.coll.InsertMany(ctx, documents, opts...)
}

// InsertOne wraps the native InsertOne collection method.
func (c *Collection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	// push span
	c.trace.Push("coal/Collection.InsertOne")
	defer c.trace.Pop()

	// run query
	return c.coll.InsertOne(ctx, document, opts...)
}

// ReplaceOne wraps the native ReplaceOne collection method.
func (c *Collection) ReplaceOne(ctx context.Context, filter interface{}, replacement interface{}, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {
	// push span
	c.trace.Push("coal/Collection.ReplaceOne")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.ReplaceOne(ctx, filter, replacement, opts...)
}

// UpdateMany wraps the native UpdateMany collection method.
func (c *Collection) UpdateMany(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// push span
	c.trace.Push("coal/Collection.UpdateMany")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.UpdateMany(ctx, filter, update, opts...)
}

// UpdateOne wraps the native UpdateOne collection method.
func (c *Collection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	// push span
	c.trace.Push("coal/Collection.UpdateOne")
	c.trace.Log("filter", filter)
	defer c.trace.Pop()

	// run query
	return c.coll.UpdateOne(ctx, filter, update, opts...)
}
