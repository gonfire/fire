package stick

import (
	"fmt"
	"reflect"
	"sync"
)

// Field is dynamically accessible field.
type Field struct {
	Index int
	Type  reflect.Type
}

// Accessor provides dynamic access to a structs fields.
type Accessor struct {
	Name   string
	Fields map[string]*Field
}

// Accessible is a type that has dynamically accessible fields.
type Accessible interface {
	GetAccessor(interface{}) *Accessor
}

// GetAccessor is a short-hand to retrieve the accessor of an accessible.
func GetAccessor(acc Accessible) *Accessor {
	return acc.GetAccessor(acc)
}

var accessMutex sync.Mutex
var accessCache = map[reflect.Type]*Accessor{}

// BasicAccess may be embedded in a struct to provide basic accessibility.
type BasicAccess struct{}

// GetAccessor implements the Accessible interface.
func (a *BasicAccess) GetAccessor(v interface{}) *Accessor {
	// get type
	typ := reflect.TypeOf(v).Elem()

	// acquire mutex
	accessMutex.Lock()
	defer accessMutex.Unlock()

	// check if accessor has already been cached
	accessor, ok := accessCache[typ]
	if ok {
		return accessor
	}

	// build accessor
	accessor = BuildAccessor(v.(Accessible), "BasicAccess")

	// cache accessor
	accessCache[typ] = accessor

	return accessor
}

// BuildAccessor will build an accessor for the provided type.
func BuildAccessor(v Accessible, ignore ...string) *Accessor {
	// get type
	typ := reflect.TypeOf(v).Elem()

	// prepare accessor
	accessor := &Accessor{
		Name:   typ.String(),
		Fields: map[string]*Field{},
	}

	// iterate through all fields
	for i := 0; i < typ.NumField(); i++ {
		// get field
		field := typ.Field(i)

		// check field
		var skip bool
		for _, item := range ignore {
			if item == field.Name {
				skip = true
			}
		}
		if skip {
			continue
		}

		// add field
		accessor.Fields[field.Name] = &Field{
			Index: i,
			Type:  field.Type,
		}
	}

	return accessor
}

// Get will lookup the specified field on the accessible and return its value
// and whether the field was found at all.
func Get(acc Accessible, name string) (interface{}, bool) {
	// find field
	field := GetAccessor(acc).Fields[name]
	if field == nil {
		return nil, false
	}

	// get value
	value := reflect.ValueOf(acc).Elem().Field(field.Index).Interface()

	return value, true
}

// Set will set the specified field on the accessible with the provided value
// and return whether the field has been found and the value has been set.
func Set(acc Accessible, name string, value interface{}) bool {
	// find field
	field := GetAccessor(acc).Fields[name]
	if field == nil {
		return false
	}

	// get value
	fieldValue := reflect.ValueOf(acc).Elem().Field(field.Index)

	// get value value
	valueValue := reflect.ValueOf(value)

	// correct untyped nil values
	if value == nil && field.Type.Kind() == reflect.Ptr {
		valueValue = reflect.Zero(field.Type)
	}

	// check type
	if fieldValue.Type() != valueValue.Type() {
		return false
	}

	// set value
	fieldValue.Set(valueValue)

	return true
}

// MustGet will call Get and panic if the operation failed.
func MustGet(acc Accessible, name string) interface{} {
	// get value
	value, ok := Get(acc, name)
	if !ok {
		panic(fmt.Sprintf(`stick: could not get field "%s" on "%s"`, name, GetAccessor(acc).Name))
	}

	return value
}

// MustSet will call Set and panic if the operation failed.
func MustSet(acc Accessible, name string, value interface{}) {
	// get value
	ok := Set(acc, name, value)
	if !ok {
		panic(fmt.Sprintf(`stick: could not set "%s" on "%s"`, name, GetAccessor(acc).Name))
	}
}
