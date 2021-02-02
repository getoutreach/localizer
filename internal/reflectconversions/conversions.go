package reflectconversions

import (
	"fmt"
	"reflect"
)

// EnforceStruct ensures that a given reflect.Value is a struct type
func EnforceStruct(v reflect.Value) error {
	t := v.Type()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, but got %v: %v (%#v)", v.Kind(), t, v.Interface())
	}

	return nil
}
