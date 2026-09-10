package processfields

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type logLevel string

type threadCount int

func TestSetSupportedBasicTypes(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		value    string
		config   any
		field    string
		expected any
	}{
		"string": {
			value:    "updated",
			config:   &struct{ Value string }{},
			field:    "Value",
			expected: "updated",
		},
		"named string": {
			value:    "debug",
			config:   &struct{ Value logLevel }{},
			field:    "Value",
			expected: logLevel("debug"),
		},
		"bool true": {
			value:    "true",
			config:   &struct{ Value bool }{},
			field:    "Value",
			expected: true,
		},
		"bool false": {
			value:    "false",
			config:   &struct{ Value bool }{Value: true},
			field:    "Value",
			expected: false,
		},
		"int": {
			value:    "42",
			config:   &struct{ Value int }{},
			field:    "Value",
			expected: 42,
		},
		"named int": {
			value:    "4",
			config:   &struct{ Value threadCount }{},
			field:    "Value",
			expected: threadCount(4),
		},
		"int8": {
			value:    "-8",
			config:   &struct{ Value int8 }{},
			field:    "Value",
			expected: int8(-8),
		},
		"int16": {
			value:    "1600",
			config:   &struct{ Value int16 }{},
			field:    "Value",
			expected: int16(1600),
		},
		"int32": {
			value:    "3200",
			config:   &struct{ Value int32 }{},
			field:    "Value",
			expected: int32(3200),
		},
		"int64": {
			value:    "6400",
			config:   &struct{ Value int64 }{},
			field:    "Value",
			expected: int64(6400),
		},
		"float32": {
			value:    "3.25",
			config:   &struct{ Value float32 }{},
			field:    "Value",
			expected: float32(3.25),
		},
		"float64": {
			value:    "6.5",
			config:   &struct{ Value float64 }{},
			field:    "Value",
			expected: 6.5,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := setAll(testCase.config, testCase.value)

			require.NoError(t, err)
			actual := reflect.ValueOf(testCase.config).Elem().FieldByName(testCase.field).Interface()
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

func TestSetUnsignedIntegerTypes(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		value    string
		config   any
		field    string
		expected any
	}{
		"uint": {
			value:    "42",
			config:   &struct{ Value uint }{},
			field:    "Value",
			expected: uint(42),
		},
		"uint8": {
			value:    "8",
			config:   &struct{ Value uint8 }{},
			field:    "Value",
			expected: uint8(8),
		},
		"uint16": {
			value:    "8080",
			config:   &struct{ Value uint16 }{},
			field:    "Value",
			expected: uint16(8080),
		},
		"uint32": {
			value:    "3200",
			config:   &struct{ Value uint32 }{},
			field:    "Value",
			expected: uint32(3200),
		},
		"uint64": {
			value:    "6400",
			config:   &struct{ Value uint64 }{},
			field:    "Value",
			expected: uint64(6400),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := setAll(testCase.config, testCase.value)

			require.NoError(t, err)
			actual := reflect.ValueOf(testCase.config).Elem().FieldByName(testCase.field).Interface()
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

func TestSetBasicTypeReturnsErrors(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		value       string
		config      any
		errorString string
	}{
		"invalid bool": {
			value:       "not-a-bool",
			config:      &struct{ Value bool }{},
			errorString: "failed to parse bool",
		},
		"invalid int": {
			value:       "not-an-int",
			config:      &struct{ Value int }{},
			errorString: "failed to parse int",
		},
		"int overflow": {
			value:       "128",
			config:      &struct{ Value int8 }{},
			errorString: "failed to parse int",
		},
		"negative uint": {
			value:       "-1",
			config:      &struct{ Value uint }{},
			errorString: "failed to parse uint",
		},
		"uint overflow": {
			value:       "256",
			config:      &struct{ Value uint8 }{},
			errorString: "failed to parse uint",
		},
		"invalid float": {
			value:       "not-a-float",
			config:      &struct{ Value float64 }{},
			errorString: "failed to parse float",
		},
		"empty int": {
			value:       "",
			config:      &struct{ Value int }{},
			errorString: "failed to parse int",
		},
		"unsupported slice element": {
			value:       "value",
			config:      &struct{ Value []any }{},
			errorString: "field \"Value[0]\": unsupported field type: interface",
		},
		"unsupported map": {
			value:       "value",
			config:      &struct{ Value map[string]string }{},
			errorString: "unsupported field type: map",
		},
		"unsupported interface": {
			value:       "value",
			config:      &struct{ Value any }{},
			errorString: "unsupported field type: interface",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := setAll(testCase.config, testCase.value)

			require.Error(t, err)
			assert.ErrorContains(t, err, testCase.errorString)
		})
	}
}
