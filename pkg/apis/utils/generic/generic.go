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
	case reflect.Struct:
		// If TObj is a struct type, we can return a zero value of that type
		return reflect.Zero(valueType).Interface().(TObj)
	case reflect.String, reflect.Int, reflect.Float64, reflect.Bool:
		// If TObj is a basic type, return its zero value
		return reflect.Zero(valueType).Interface().(TObj)
	default:
		// If TObj is neither a struct nor a pointer to struct, panic
		panic("TObj must be a struct or pointer to struct type")
	}
}
