package coal

import (
	"go.mongodb.org/mongo-driver/bson"
)

// Reconcile uses a stream to reconcile changes to a collection. It will
// automatically load existing models once the underlying stream has been opened.
// After that it will yield all changes to the collection until the returned
// stream has been closed.
func Reconcile(store *Store, model Model, loaded func(), created, updated func(Model), deleted func(ID), errored func(error)) *Stream {
	// prepare load
	load := func() error {
		// get cursor
		iter, err := store.C(model).Find(nil, bson.M{})
		if err != nil {
			return err
		}

		// iterate over all models
		defer iter.Close()
		for iter.Next() {
			// decode model
			model := GetMeta(model).Make()
			err := iter.Decode(model)
			if err != nil {
				return err
			}

			// call callback if available
			if created != nil {
				created(model)
			}
		}

		// check error
		err = iter.Error()
		if err != nil {
			return err
		}

		// call callback if available
		if loaded != nil {
			loaded()
		}

		return nil
	}

	// open stream
	stream := OpenStream(store, model, nil, func(event Event, id ID, model Model, err error, bytes []byte) error {
		// handle events
		switch event {
		case Opened:
			return load()
		case Created:
			// call callback if available
			if created != nil {
				created(model)
			}
		case Updated:
			// call callback if available
			if updated != nil {
				updated(model)
			}
		case Deleted:
			// call callback if available
			if deleted != nil {
				deleted(id)
			}
		case Errored:
			// call callback if available
			if errored != nil {
				errored(err)
			}
		}

		return nil
	})

	return stream
}
