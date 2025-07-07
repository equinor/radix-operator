package generic

import (
	"reflect"
)

func InstantiateGenericStruct[TObj any]() TObj {
	var obj TObj
	value := reflect.ValueOf(obj)
	valueType := value.Type()

	switch valueType.Kind() {
	case reflect.Ptr:
		// If TObj is a pointer type, we need to get the element type
		valueType = valueType.Elem()
		return reflect.New(valueType).Interface().(TObj)
	case reflect.Struct, reflect.String, reflect.Int, reflect.Float64, reflect.Bool:
		// If TObj is a basic type, return its zero value
		return reflect.Zero(valueType).Interface().(TObj)
	default:
		panic("InstantiateGenericStruct: unsupported type " + valueType.String())
	}
}
