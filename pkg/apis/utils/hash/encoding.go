package hash

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"

	"sigs.k8s.io/yaml"
)

type encoder func(v any) ([]byte, error)

func boolEncoder(v any) ([]byte, error) {
	if reflect.ValueOf(v).Bool() {
		return []byte{1}, nil
	}
	return []byte{0}, nil
}

func intEncoder(v any) ([]byte, error) {
	vt := reflect.ValueOf(v)
	return binary.AppendVarint([]byte{}, vt.Int()), nil
}

func uintEncoder(v any) ([]byte, error) {
	vt := reflect.ValueOf(v)
	return binary.AppendUvarint([]byte{}, vt.Uint()), nil
}

func floatEncoder(v any) ([]byte, error) {
	vt := reflect.ValueOf(v)
	return uintEncoder(math.Float64bits(vt.Float()))
}

func stringEncoder(v any) ([]byte, error) {
	vt := reflect.ValueOf(v)
	return []byte(vt.String()), nil
}

func structEncoder(v any) ([]byte, error) {
	b, err := yaml.Marshal(v)
	return b, err
}

func getEncoder(v any) (encoder, error) {
	t := reflect.Indirect(reflect.ValueOf(v))

	switch t.Kind() {
	case reflect.Bool:
		return boolEncoder, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uintEncoder, nil
	case reflect.Float32, reflect.Float64:
		return floatEncoder, nil
	case reflect.String:
		return stringEncoder, nil
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		return structEncoder, nil
	}
	return nil, fmt.Errorf("encoder for type %s not supported", t.Kind().String())
}
