package pointers

import "reflect"

func Val[T any](i *T) T {
	var value T
	if i != nil {
		value = *i
	}

	return value
}

func IsNil(obj any) bool {
	return obj == nil || (reflect.ValueOf(obj).Kind() == reflect.Pointer && reflect.ValueOf(obj).IsNil())
}
