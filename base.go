package fire

import (
	"errors"
	"reflect"

	"github.com/asaskevich/govalidator"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

// Base is the base for every fire model.
type Base struct {
	DocID bson.ObjectId `json:"-" bson:"_id,omitempty"`

	model interface{}
	meta  *Meta
}

// ID returns the models id.
func (b *Base) ID() bson.ObjectId {
	return b.DocID
}

// Get returns the value of the given field.
//
// Note: Get will return the value of the first field that has a matching Name,
// JSONName, or BSONName and will panic if no field can be found.
func (b *Base) Get(name string) interface{} {
	for _, field := range b.meta.Fields {
		if field.JSONName == name || field.BSONName == name || field.Name == name {
			// read value from model struct
			field := reflect.ValueOf(b.model).Elem().Field(field.index)
			return field.Interface()
		}
	}

	panic("Missing field " + name + " on " + b.meta.SingularName)
}

// Set will set given field to the the passed valued.
//
// Note: Set will set the value of the first field that has a matching Name,
// JSONName, or BSONName and will panic if no field can been found. The method
// will also panic if the type of the field and the passed value do not match.
func (b *Base) Set(name string, value interface{}) {
	for _, field := range b.meta.Fields {
		if field.JSONName == name || field.BSONName == name || field.Name == name {
			// set the value on model struct
			reflect.ValueOf(b.model).Elem().Field(field.index).Set(reflect.ValueOf(value))
			return
		}
	}

	panic("Missing field " + name + " on " + b.meta.SingularName)
}

// Validate validates the model based on the `valid:""` struct tags.
func (b *Base) Validate(fresh bool) error {
	// validate id
	if !b.DocID.Valid() {
		return errors.New("Invalid id")
	}

	// validate parent model
	_, err := govalidator.ValidateStruct(b.model)
	if err != nil {
		return err
	}

	return nil
}

// Meta returns the models Meta structure.
func (b *Base) Meta() *Meta {
	return b.meta
}

func (b *Base) initialize(model Model) {
	b.model = model

	// set id if missing
	if !b.DocID.Valid() {
		b.DocID = bson.NewObjectId()
	}

	// assign meta
	b.meta = NewMeta(model)
}

/* api2go.jsonapi interface */

// GetName returns the plural name of the Model.
//
// This methods is required by https://godoc.org/github.com/manyminds/api2go/jsonapi#EntityNamer.
func (b *Base) GetName() string {
	return b.meta.PluralName
}

// GetID returns the id of the Model.
//
// This methods is required by https://godoc.org/github.com/manyminds/api2go/jsonapi#MarshalIdentifier.
func (b *Base) GetID() string {
	return b.DocID.Hex()
}

// SetID sets the id of the Model.
//
// This methods is required by https://godoc.org/github.com/manyminds/api2go/jsonapi#UnmarshalIdentifier.
func (b *Base) SetID(id string) error {
	if len(id) == 0 {
		b.DocID = bson.NewObjectId()
		return nil
	}

	if !bson.IsObjectIdHex(id) {
		return errors.New("Invalid id")
	}

	b.DocID = bson.ObjectIdHex(id)
	return nil
}

// GetReferences returns a list of the available references.
//
// This methods is required by https://godoc.org/github.com/manyminds/api2go/jsonapi#MarshalReferences.
func (b *Base) GetReferences() []jsonapi.Reference {
	// prepare result
	var refs []jsonapi.Reference

	// add to one, to many and has many relationships
	for _, field := range b.meta.Fields {
		if field.ToOne || field.ToMany {
			refs = append(refs, jsonapi.Reference{
				Type: field.RelType,
				Name: field.RelName,
			})
		} else if field.HasMany {
			refs = append(refs, jsonapi.Reference{
				Type:        field.RelType,
				Name:        field.RelName,
				IsNotLoaded: true,
			})
		}
	}

	return refs
}

// GetReferencedIDs returns list of references ids.
//
// This methods is required by https://godoc.org/github.com/manyminds/api2go/jsonapi#MarshalLinkedRelations.
func (b *Base) GetReferencedIDs() []jsonapi.ReferenceID {
	// prepare result
	var ids []jsonapi.ReferenceID

	// add to one relationships
	for _, field := range b.meta.Fields {
		if field.ToOne {
			// get struct field
			structField := reflect.ValueOf(b.model).Elem().Field(field.index)

			// prepare id
			var id string

			// check if field is optional
			if field.Optional {
				// continue if id is not set
				if structField.IsNil() {
					continue
				}

				// get id
				id = structField.Interface().(*bson.ObjectId).Hex()
			} else {
				// get id
				id = structField.Interface().(bson.ObjectId).Hex()
			}

			// append reference id
			ids = append(ids, jsonapi.ReferenceID{
				ID:   id,
				Type: field.RelType,
				Name: field.RelName,
			})
		}

		if field.ToMany {
			// get struct field
			structField := reflect.ValueOf(b.model).Elem().Field(field.index)

			// get ids
			for i := 0; i < structField.Len(); i++ {
				// read slice value
				id := structField.Index(i).Interface().(bson.ObjectId).Hex()

				// append reference id
				ids = append(ids, jsonapi.ReferenceID{
					ID:   id,
					Type: field.RelType,
					Name: field.RelName,
				})
			}
		}
	}

	return ids
}

// SetToOneReferenceID sets a reference to the passed id.
//
// This methods is required by https://godoc.org/github.com/manyminds/api2go/jsonapi#UnmarshalToOneRelations.
func (b *Base) SetToOneReferenceID(name, id string) error {
	// check object id
	if !bson.IsObjectIdHex(id) {
		return errors.New("Invalid id")
	}

	for _, field := range b.meta.Fields {
		if field.ToOne && field.RelName == name {
			// get struct field
			structField := reflect.ValueOf(b.model).Elem().Field(field.index)

			// create id
			oid := bson.ObjectIdHex(id)

			// check if optional
			if field.Optional {
				structField.Set(reflect.ValueOf(&oid))
			} else {
				structField.Set(reflect.ValueOf(oid))
			}

			return nil
		}
	}

	return errors.New("Missing relationship " + name)
}

// SetToManyReferenceIDs sets references to the passed ids.
//
// This methods is required by https://godoc.org/github.com/manyminds/api2go/jsonapi#UnmarshalToOneRelations.
func (b *Base) SetToManyReferenceIDs(name string, ids []string) error {
	// check object ids
	for _, id := range ids {
		if !bson.IsObjectIdHex(id) {
			return errors.New("Invalid id")
		}
	}

	for _, field := range b.meta.Fields {
		if field.ToMany && field.RelName == name {
			// get struct field
			structField := reflect.ValueOf(b.model).Elem().Field(field.index)

			// append ids
			for _, id := range ids {
				// create id
				oid := bson.ObjectIdHex(id)

				// append id
				structField.Set(reflect.Append(structField, reflect.ValueOf(oid)))
			}

			return nil
		}
	}

	return errors.New("Missing relationship " + name)
}