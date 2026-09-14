package processfields

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"time"
)

// SetValFunc sets a field from one or more string values. A slice field takes one value per
// element, any other field takes exactly one. It is nil for container fields.
type SetValFunc func(values ...string) error

type WalkFunc func(path string, field reflect.StructField, value reflect.Value, setter SetValFunc) error

func WalkFields(cfg any, fn WalkFunc) error {
	if fn == nil {
		return fmt.Errorf("field callback cannot be nil")
	}
	val := reflect.ValueOf(cfg)
	if !val.IsValid() {
		return fmt.Errorf("config must be a struct or pointer to struct, got nil")
	}
	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return fmt.Errorf("config must be a non-nil struct or pointer to struct")
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("config must be a struct or pointer to struct, got %s", val.Kind())
	}
	return (&walker{fn: fn}).walkStruct(val, "")
}

type walker struct {
	fn WalkFunc
	// ancestors holds the struct types on the current path, so a recursive type cannot loop forever.
	ancestors []reflect.Type
}

func (w *walker) walkStruct(val reflect.Value, path string) error {
	typ := val.Type()
	if slices.Contains(w.ancestors, typ) {
		return fmt.Errorf("field %q: recursive type %s is not supported", path, typ)
	}
	w.ancestors = append(w.ancestors, typ)
	defer func() { w.ancestors = w.ancestors[:len(w.ancestors)-1] }()

	for i := range typ.NumField() {
		fieldType := typ.Field(i)
		fieldValue := val.Field(i)

		if !fieldType.IsExported() && !fieldType.Anonymous {
			continue
		}

		nested := isNestedStruct(fieldType.Type)
		fieldPath := joinPath(path, fieldType.Name)
		if fieldType.Anonymous && nested {
			fieldPath = path
		}

		if nested {
			if fieldType.IsExported() {
				if err := w.fn(fieldPath, fieldType, fieldValue, nil); err != nil {
					return err
				}
			}
			if err := w.walkNested(fieldValue, fieldPath); err != nil {
				return err
			}
			continue
		}

		if fieldType.IsExported() && isNestedStructList(fieldType.Type) {
			if err := w.fn(fieldPath, fieldType, fieldValue, nil); err != nil {
				return err
			}
			if err := w.walkList(fieldValue, fieldPath); err != nil {
				return err
			}
			continue
		}

		// Embedded unexported non-struct fields cannot be set through reflection.
		if !fieldType.IsExported() {
			continue
		}

		setter := func(values ...string) error {
			return setFieldValue(fieldValue, values, fieldPath)
		}
		if err := w.fn(fieldPath, fieldType, fieldValue, setter); err != nil {
			return err
		}
	}

	return nil
}

func (w *walker) walkNested(field reflect.Value, path string) error {
	if field.Kind() != reflect.Pointer {
		return w.walkStruct(field, path)
	}
	if !field.IsNil() {
		return w.walkStruct(field.Elem(), path)
	}
	if !field.CanSet() {
		return fmt.Errorf("cannot set field %q", path)
	}

	// Allocate so the fields can be visited, then drop the section again if it stayed empty.
	field.Set(reflect.New(field.Type().Elem()))
	if err := w.walkStruct(field.Elem(), path); err != nil {
		return err
	}
	if field.Elem().IsZero() {
		field.SetZero()
	}
	return nil
}

func (w *walker) walkList(list reflect.Value, path string) error {
	for i := range list.Len() {
		if err := w.walkNested(list.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func setFieldValue(field reflect.Value, values []string, path string) error {
	if !field.CanSet() {
		return fmt.Errorf("cannot set field %q", path)
	}
	if field.Kind() != reflect.Pointer {
		return setValues(field, values, path)
	}
	if !field.IsNil() {
		return setValues(field.Elem(), values, path)
	}

	// Allocate into a temporary so the field stays nil when the value is invalid.
	temp := reflect.New(field.Type().Elem())
	if err := setValues(temp.Elem(), values, path); err != nil {
		return err
	}
	field.Set(temp)
	return nil
}

func setValues(field reflect.Value, values []string, path string) error {
	// An unmarshaler owns its own parsing, even when the underlying kind is a slice.
	if field.Kind() == reflect.Slice && !implementsUnmarshaler(field.Type()) {
		return setSlice(field, values, path)
	}
	if len(values) != 1 {
		return fmt.Errorf("field %q: expected a single value, got %d", path, len(values))
	}
	return setValue(field, values[0], path)
}

func setSlice(field reflect.Value, values []string, path string) error {
	// Filled in full before assignment, so a bad element leaves the field untouched.
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))
	for i, value := range values {
		if err := setValue(slice.Index(i), value, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	field.Set(slice)
	return nil
}

func setValue(field reflect.Value, value, path string) error {
	// Checked before the unmarshalers so a future Duration.UnmarshalText cannot change behaviour.
	if field.Type() == reflect.TypeFor[time.Duration]() {
		return setDuration(field, value, path)
	}
	if handled, err := unmarshalValue(field, value, path); handled {
		return err
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("field %q: failed to parse bool: %w", path, err)
		}
		field.SetBool(boolValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("field %q: failed to parse int: %w", path, err)
		}
		field.SetInt(intValue)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintValue, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("field %q: failed to parse uint: %w", path, err)
		}
		field.SetUint(uintValue)
	case reflect.Float32, reflect.Float64:
		floatValue, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("field %q: failed to parse float: %w", path, err)
		}
		field.SetFloat(floatValue)
	default:
		return fmt.Errorf("field %q: unsupported field type: %s", path, field.Kind())
	}
	return nil
}

func setDuration(field reflect.Value, value, path string) error {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("field %q: failed to parse duration: %w", path, err)
	}
	field.SetInt(int64(duration))
	return nil
}

func unmarshalValue(field reflect.Value, value, path string) (bool, error) {
	typ := field.Type()
	if !implementsUnmarshaler(typ) {
		return false, nil
	}
	// A value receiver unmarshals into a copy, so the result would be discarded silently.
	if implementsAnyUnmarshaler(typ) {
		return true, fmt.Errorf("field %q: %s must implement the unmarshaler with a pointer receiver", path, typ)
	}
	if !field.CanAddr() {
		return true, fmt.Errorf("cannot set field %q", path)
	}

	target := field.Addr().Interface()
	if unmarshaler, ok := target.(encoding.TextUnmarshaler); ok {
		if err := unmarshaler.UnmarshalText([]byte(value)); err != nil {
			return true, fmt.Errorf("field %q: failed to unmarshal text: %w", path, err)
		}
		return true, nil
	}
	if unmarshaler, ok := target.(encoding.BinaryUnmarshaler); ok {
		if err := unmarshaler.UnmarshalBinary([]byte(value)); err != nil {
			return true, fmt.Errorf("field %q: failed to unmarshal binary: %w", path, err)
		}
		return true, nil
	}
	if unmarshaler, ok := target.(json.Unmarshaler); ok {
		// Config values are plain strings, so they are handed over as a JSON string literal.
		encoded, _ := json.Marshal(value)
		if err := unmarshaler.UnmarshalJSON(encoded); err != nil {
			return true, fmt.Errorf("field %q: failed to unmarshal json: %w", path, err)
		}
		return true, nil
	}
	return false, nil
}

func isNestedStruct(typ reflect.Type) bool {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.Kind() == reflect.Struct && !implementsUnmarshaler(typ)
}

func isNestedStructList(typ reflect.Type) bool {
	if typ.Kind() != reflect.Slice && typ.Kind() != reflect.Array {
		return false
	}
	return !implementsUnmarshaler(typ) && isNestedStruct(typ.Elem())
}

func implementsUnmarshaler(typ reflect.Type) bool {
	return implementsAnyUnmarshaler(typ) || implementsAnyUnmarshaler(reflect.PointerTo(typ))
}

func implementsAnyUnmarshaler(typ reflect.Type) bool {
	return typ.Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) ||
		typ.Implements(reflect.TypeFor[encoding.BinaryUnmarshaler]()) ||
		typ.Implements(reflect.TypeFor[json.Unmarshaler]())
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}
