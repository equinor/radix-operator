package processfields_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/utils/processfields"
)

type textValue string

func (value *textValue) UnmarshalText(text []byte) error {
	if string(text) == "invalid" {
		return errors.New("invalid text value")
	}
	*value = textValue(strings.ToUpper(string(text)))
	return nil
}

type valueReceiverTextValue struct {
	received *string
}

func (value valueReceiverTextValue) UnmarshalText(text []byte) error {
	*value.received = string(text)
	return nil
}

// selfMutatingTextValue can never observe a write, because the value receiver is a copy.
type selfMutatingTextValue struct {
	Received string
}

func (value selfMutatingTextValue) UnmarshalText(text []byte) error {
	value.Received = string(text) //nolint:staticcheck
	return nil
}

type binaryValue []byte

func (value *binaryValue) UnmarshalBinary(data []byte) error {
	if string(data) == "invalid" {
		return errors.New("invalid binary value")
	}
	*value = append((*value)[:0], data...)
	return nil
}

type dualUnmarshaler struct {
	Method string
}

func (value *dualUnmarshaler) UnmarshalText(_ []byte) error {
	value.Method = "text"
	return nil
}

func (value *dualUnmarshaler) UnmarshalBinary(_ []byte) error {
	value.Method = "binary"
	return nil
}

type jsonValue struct {
	Value string
}

func (value *jsonValue) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if text == "invalid" {
		return errors.New("invalid json value")
	}
	value.Value = strings.ToUpper(text)
	return nil
}

type binaryJSONUnmarshaler struct {
	Method string
}

func (value *binaryJSONUnmarshaler) UnmarshalBinary(_ []byte) error {
	value.Method = "binary"
	return nil
}

func (value *binaryJSONUnmarshaler) UnmarshalJSON(_ []byte) error {
	value.Method = "json"
	return nil
}

// setAll invokes the setter for every visited field using the same input value.
func setAll(cfg any, value string) error {
	return processfields.WalkFields(cfg, func(_ string, _ reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		if setter == nil {
			return nil
		}
		return setter(value)
	})
}

// setAllValues invokes the setter for every visited field using the same input values.
func setAllValues(cfg any, values ...string) error {
	return processfields.WalkFields(cfg, func(_ string, _ reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		if setter == nil {
			return nil
		}
		return setter(values...)
	})
}

// visitAll records the name of every field handed to the callback, in traversal order.
func visitAll(cfg any) ([]string, error) {
	var visited []string
	err := processfields.WalkFields(cfg, func(_ string, field reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		if setter == nil {
			return nil
		}
		visited = append(visited, field.Name)
		return nil
	})
	return visited, err
}

// visitAllPaths records the path of every field handed to the callback, in traversal order.
func visitAllPaths(cfg any) ([]string, error) {
	var visited []string
	err := processfields.WalkFields(cfg, func(path string, _ reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		if setter == nil {
			return nil
		}
		visited = append(visited, path)
		return nil
	})
	return visited, err
}
