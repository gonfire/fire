package coal

import (
	"context"
	"errors"
	"fmt"

	"github.com/256dpi/lungo/bsonkit"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/cinder"
)

// ErrTransactionRequired is returned if an operation would be unsafe to perform
// without a transaction.
var ErrTransactionRequired = errors.New("operation requires a transaction")

var incrementLock = bson.M{
	"$inc": bson.M{
		"_lk": 1,
	},
}

var returnAfterUpdate = options.FindOneAndUpdate().SetReturnDocument(options.After)

// Manager manages operations on collection of documents. It will validate
// operations and ensure that they are safe under the MongoDB guarantees.
type Manager struct {
	meta  *Meta
	coll  *Collection
	trans *Translator
}

// Find will find the document with the specified id. It will return whether
// a document has been found. Lock can be set to true to force a write lock on
// the document and prevent a stale read during a transaction.
//
// A transaction is required for locking.
func (m *Manager) Find(ctx context.Context, model Model, id ID, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Find")
	span.Log("id", id.Hex())
	defer span.Finish()

	// check lock
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// prepare query
	query := bson.M{
		"_id": id,
	}

	// find document
	var err error
	if lock {
		err = m.coll.FindOneAndUpdate(ctx, query, incrementLock, returnAfterUpdate).Decode(model)
	} else {
		err = m.coll.FindOne(ctx, query).Decode(model)
	}
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// FindFirst will find the first document that matches the specified query. It
// will return whether a document has been found. Lock can be set to true to
// force a write lock on the document and prevent a stale read during a
// transaction.
//
// A transaction is required for locking. If the operation depends on
// interleaving writes to not include or exclude documents from the query it
// should be run during a transaction.
func (m *Manager) FindFirst(ctx context.Context, model Model, query bson.M, sort []string, skip int64, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindFirst")
	span.Log("query", query)
	span.Log("sort", sort)
	span.Log("skip", skip)
	defer span.Finish()

	// check lock
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// prepare options
	opts := options.FindOne()

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			return false, err
		}

		opts.SetSort(sortDoc)
	}

	// set skip
	if skip > 0 {
		opts.SetSkip(skip)
	}

	// find document
	if lock {
		err = m.coll.FindOneAndUpdate(ctx, queryDoc, incrementLock, returnAfterUpdate).Decode(model)
	} else {
		err = m.coll.FindOne(ctx, queryDoc).Decode(model)
	}
	if IsMissing(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// FindAll will find all documents that match the specified query. Lock can be
// set to true to force a write lock on the documents and prevent a stale read
// during a transaction.
//
// A transaction is always required to ensure isolation.
func (m *Manager) FindAll(ctx context.Context, slicePtr interface{}, query bson.M, sort []string, skip, limit int64, lock bool) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindAll")
	span.Log("query", query)
	span.Log("sort", sort)
	span.Log("skip", skip)
	span.Log("limit", limit)
	defer span.Finish()

	// require transaction
	if !getKey(ctx, hasTransaction) {
		return ErrTransactionRequired
	}

	// check lock
	if lock && (skip > 0 || limit > 0) {
		return fmt.Errorf("cannot lock with skip and limit")
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return err
	}

	// prepare options
	opts := options.Find()

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			return err
		}

		opts.SetSort(sortDoc)
	}

	// set skip
	if skip > 0 {
		opts.SetSkip(skip)
	}

	// set limit
	if limit > 0 {
		opts.SetLimit(limit)
	}

	// lock documents
	if lock {
		_, err = m.coll.UpdateMany(ctx, queryDoc, incrementLock)
		if err != nil {
			return err
		}
	}

	// find documents
	err = m.coll.FindAll(ctx, slicePtr, queryDoc, opts)
	if err != nil {
		return err
	}

	return nil
}

// FindEach will find all documents that match the specified query. Lock can be
// set to true to force a write lock on the documents and prevent a stale read
// during a transaction.
//
// A transaction is always required to ensure isolation.
func (m *Manager) FindEach(ctx context.Context, query bson.M, sort []string, skip, limit int64, lock bool) (*Iterator, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.FindEach")
	span.Log("query", query)
	span.Log("sort", sort)
	span.Log("skip", skip)
	span.Log("limit", limit)

	// require transaction
	if !getKey(ctx, hasTransaction) {
		return nil, ErrTransactionRequired
	}

	// check lock
	if lock && (skip > 0 || limit > 0) {
		return nil, fmt.Errorf("cannot lock with skip and limit")
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		span.Finish()
		return nil, err
	}

	// prepare options
	opts := options.Find()

	// set sort
	if len(sort) > 0 {
		sortDoc, err := m.trans.Sort(sort)
		if err != nil {
			span.Finish()
			return nil, err
		}

		opts.SetSort(sortDoc)
	}

	// set skip
	if skip > 0 {
		opts.SetSkip(skip)
	}

	// set limit
	if limit > 0 {
		opts.SetLimit(limit)
	}

	// lock documents
	if lock {
		_, err = m.coll.UpdateMany(ctx, queryDoc, incrementLock)
		if err != nil {
			return nil, err
		}
	}

	// find documents
	iter, err := m.coll.Find(ctx, queryDoc, opts)
	if err != nil {
		span.Finish()
		return nil, err
	}

	// attach span
	iter.spans = append(iter.spans, span)

	return iter, nil
}

// Count will count the documents that match the specified query. Lock can be
// set to true to force a write lock on the documents and prevent a stale read
// during a transaction.
//
// A transaction is always required to ensure isolation.
func (m *Manager) Count(ctx context.Context, query bson.M, skip, limit int64, lock bool) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Count")
	span.Log("query", query)
	span.Log("skip", skip)
	span.Log("limit", limit)
	defer span.Finish()

	// require transaction
	if !getKey(ctx, hasTransaction) {
		return 0, ErrTransactionRequired
	}

	// check lock
	if lock && (skip > 0 || limit > 0) {
		return 0, fmt.Errorf("cannot lock with skip and limit")
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return 0, err
	}

	// prepare options
	opts := options.Count()

	// set skip
	if skip > 0 {
		opts.SetSkip(skip)
	}

	// set limit
	if limit > 0 {
		opts.SetLimit(limit)
	}

	// update if locked
	if lock {
		res, err := m.coll.UpdateMany(ctx, queryDoc, incrementLock)
		if err != nil {
			return 0, err
		}

		return res.ModifiedCount, nil
	}

	// count documents
	count, err := m.coll.CountDocuments(ctx, queryDoc, opts)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Insert will insert the provided document. If the document has a zero id a new
// id will be generated and assigned. It will return whether a document has been
// inserted.
//
// The operation is always atomic. Use a unique index to prevent unwanted
// duplicates when inserting in parallel.
func (m *Manager) Insert(ctx context.Context, model Model) error {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Insert")
	defer span.Finish()

	// ensure id
	if model.ID().IsZero() {
		model.GetBase().DocID = New()
	}

	// insert document
	_, err := m.coll.InsertOne(ctx, model)
	if err != nil {
		return err
	}

	return nil
}

// InsertIfMissing will insert the provided document if no document matched the
// provided query. If the document has a zero id a new id will be generated and
// assigned. It will return whether a document has been inserted. The underlying
// upsert operation will merge the query with the model fields.
//
// A transaction is always required to ensure atomicity. Use a unique index to
// prevent unwanted duplicates when inserting in parallel.
func (m *Manager) InsertIfMissing(ctx context.Context, query bson.M, model Model) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.InsertIfMissing")
	span.Log("query", query)
	defer span.Finish()

	// require transaction
	if !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// ensure id
	if model.ID().IsZero() {
		model.GetBase().DocID = New()
	}

	// prepare options
	opts := options.Update().SetUpsert(true)

	// upsert document
	res, err := m.coll.UpdateOne(ctx, queryDoc, bson.M{
		"$setOnInsert": model,
	}, opts)
	if err != nil {
		return false, err
	}

	return res.UpsertedCount == 1, nil
}

// Replace will replace the existing document with the provided one. It will
// return whether a document has been found. Lock can be set to true to force a
// write lock on the document and prevent a stale read during a transaction in
// case the replace did not change the document.
//
// A transaction is required for locking, but the operation is always atomic.
func (m *Manager) Replace(ctx context.Context, model Model, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Replace")
	defer span.Finish()

	// check id
	if model.ID().IsZero() {
		return false, fmt.Errorf("model has a zero id")
	}

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// increment lock manually
	if lock {
		model.GetBase().Lock += 1000
	}

	// replace document
	res, err := m.coll.ReplaceOne(ctx, bson.M{
		"_id": model.ID(),
	}, model)
	if err != nil {
		return false, err
	}

	return res.MatchedCount == 1, nil
}

// ReplaceFirst will replace the first document that matches the specified query.
// It will return whether a document has been found. Lock can be set to true to
// force a write lock on the document and prevent a stale read during a
// transaction if the replace did not cause an update.
//
// A transaction is required for locking. If the operation depends on
// interleaving writes to not include or exclude documents from the query it
// should be run as part of a transaction.
func (m *Manager) ReplaceFirst(ctx context.Context, query bson.M, model Model, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.ReplaceFirst")
	span.Log("query", query)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// increment lock manually
	if lock {
		model.GetBase().Lock += 1000
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// replace document
	res, err := m.coll.ReplaceOne(ctx, queryDoc, model)
	if err != nil {
		return false, err
	}

	return res.MatchedCount == 1, nil
}

// Update will update the document with the specified id. It will return whether
// a document has been found. Lock can be set to true to force a write lock on
// the document and prevent a stale read during a transaction in case the
// update did not chang the document.
//
// A transaction is required for locking, but the operation is always atomic.
func (m *Manager) Update(ctx context.Context, id ID, update bson.M, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Update")
	span.Log("id", id.Hex())
	span.Log("update", update)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// increment lock
	if lock {
		_, err := bsonkit.Put(&updateDoc, "$inc._lk", 1, false)
		if err != nil {
			return false, fmt.Errorf("unable to add lock: %w", err)
		}
	}

	// update document
	res, err := m.coll.UpdateOne(ctx, bson.M{
		"_id": id,
	}, updateDoc)
	if err != nil {
		return false, err
	}

	return res.MatchedCount == 1, nil
}

// UpdateFirst will update the first document that matches the specified query.
// It will return whether a document has been found. Lock can be set to true to
// force a write lock on the document and prevent a stale read during a
// transaction in case the update did not change the document.
//
// A transaction is required for locking. If the operation depends on
// interleaving writes to not include or exclude documents from the query it
// should be run as part of a transaction.
func (m *Manager) UpdateFirst(ctx context.Context, query, update bson.M, lock bool) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.UpdateFirst")
	span.Log("query", query)
	span.Log("update", update)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// increment lock
	if lock {
		_, err := bsonkit.Put(&updateDoc, "$inc._lk", 1, false)
		if err != nil {
			return false, fmt.Errorf("unable to add lock: %w", err)
		}
	}

	// update document
	res, err := m.coll.UpdateOne(ctx, queryDoc, updateDoc)
	if err != nil {
		return false, err
	}

	return res.MatchedCount == 1, nil
}

// UpdateAll will update the documents that match the specified query. It will
// return the number of matched documents. Lock can be set to true to force a
// write lock on the documents and prevent a stale read during a transaction in
// case the operation did not change all documents.
//
// A transaction is required for locking. If the operation depends on
// interleaving writes to not include or exclude documents from the query it
// should be run as part of a transaction.
func (m *Manager) UpdateAll(ctx context.Context, query, update bson.M, lock bool) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.UpdateAll")
	span.Log("query", query)
	span.Log("update", update)
	defer span.Finish()

	// require transaction
	if lock && !getKey(ctx, hasTransaction) {
		return 0, ErrTransactionRequired
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return 0, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return 0, err
	}

	// increment lock
	if lock {
		_, err := bsonkit.Put(&updateDoc, "$inc._lk", 1, false)
		if err != nil {
			return 0, fmt.Errorf("unable to add lock: %w", err)
		}
	}

	// update documents
	res, err := m.coll.UpdateMany(ctx, queryDoc, updateDoc)
	if err != nil {
		return 0, err
	}

	return res.MatchedCount, nil
}

// Upsert will update the first document that matches the specified query. If
// no document has been found, the update document is applied to the query and
// inserted. It will return whether a document has been inserted.
//
// A transaction is always required to ensure atomicity.
func (m *Manager) Upsert(ctx context.Context, query, update bson.M) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Upsert")
	span.Log("query", query)
	span.Log("update", update)
	defer span.Finish()

	// require transaction
	if !getKey(ctx, hasTransaction) {
		return false, ErrTransactionRequired
	}

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// translate update
	updateDoc, err := m.trans.Document(update)
	if err != nil {
		return false, err
	}

	// prepare options
	opts := options.Update().SetUpsert(true)

	// update document
	res, err := m.coll.UpdateOne(ctx, queryDoc, updateDoc, opts)
	if err != nil {
		return false, err
	}

	return res.UpsertedCount == 1, nil
}

// Delete will remove the document with the specified id. It will return
// whether a document has been found and deleted.
//
// The operation is always atomic.
func (m *Manager) Delete(ctx context.Context, id ID) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.Delete")
	span.Log("id", id.Hex())
	defer span.Finish()

	// delete document
	res, err := m.coll.DeleteOne(ctx, bson.M{
		"_id": id,
	})
	if err != nil {
		return false, err
	}

	return res.DeletedCount == 1, nil
}

// DeleteAll will delete the documents that match the specified query. It will
// return the number of deleted documents.
//
// If the operation depends on interleaving writes to not include or exclude
// documents from the query it should be run as part of a transaction.
func (m *Manager) DeleteAll(ctx context.Context, query bson.M) (int64, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.DeleteAll")
	span.Log("query", query)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return 0, err
	}

	// update documents
	res, err := m.coll.DeleteMany(ctx, queryDoc)
	if err != nil {
		return 0, err
	}

	return res.DeletedCount, nil
}

// DeleteFirst will delete the first document that matches the specified query.
// It will return whether a document has been deleted.
//
// If the operation depends on interleaving writes to not include or exclude
// documents from the query it should be run as part of a transaction.
func (m *Manager) DeleteFirst(ctx context.Context, query bson.M) (bool, error) {
	// track
	ctx, span := cinder.Track(ctx, "coal/Manager.DeleteFirst")
	span.Log("query", query)
	defer span.Finish()

	// translate query
	queryDoc, err := m.trans.Document(query)
	if err != nil {
		return false, err
	}

	// update document
	res, err := m.coll.DeleteOne(ctx, queryDoc)
	if err != nil {
		return false, err
	}

	return res.DeletedCount == 1, nil
}