package processfields

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetSliceOfLeafTypes(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		values   []string
		config   any
		expected any
	}{
		"strings": {
			values:   []string{"a", "b", "c"},
			config:   &struct{ Value []string }{},
			expected: []string{"a", "b", "c"},
		},
		"single value": {
			values:   []string{"a"},
			config:   &struct{ Value []string }{},
			expected: []string{"a"},
		},
		"no values yields overrides existing and sets an empty slice": {
			values:   nil,
			config:   &struct{ Value []string }{Value: []string{"a", "b"}},
			expected: []string{},
		},
		"replaces the existing content": {
			values:   []string{"c"},
			config:   &struct{ Value []string }{Value: []string{"a", "b"}},
			expected: []string{"c"},
		},
		"named element type": {
			values:   []string{"debug", "info"},
			config:   &struct{ Value []logLevel }{},
			expected: []logLevel{"debug", "info"},
		},
		"ints": {
			values:   []string{"1", "2"},
			config:   &struct{ Value []int }{},
			expected: []int{1, 2},
		},
		"durations": {
			values:   []string{"5m", "30s"},
			config:   &struct{ Value []time.Duration }{},
			expected: []time.Duration{5 * time.Minute, 30 * time.Second},
		},
		"unmarshaler elements": {
			values:   []string{"radix", "operator"},
			config:   &struct{ Value []textValue }{},
			expected: []textValue{"RADIX", "OPERATOR"},
		},
		"pointer to slice": {
			values:   []string{"a", "b"},
			config:   &struct{ Value *[]string }{},
			expected: &[]string{"a", "b"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := setAllValues(testCase.config, testCase.values...)

			require.NoError(t, err)
			actual := reflect.ValueOf(testCase.config).Elem().FieldByName("Value").Interface()
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

// A slice type with its own unmarshaler parses the whole value itself.
func TestSetSliceUnmarshalerTakesPrecedence(t *testing.T) {
	t.Parallel()

	cfg := &struct{ Value binaryValue }{}

	err := setAllValues(cfg, "radix")

	require.NoError(t, err)
	assert.Equal(t, binaryValue("radix"), cfg.Value)
}

func TestSetSliceElementErrorIdentifiesIndex(t *testing.T) {
	t.Parallel()

	cfg := &struct{ Value []int }{Value: []int{7}}

	err := setAllValues(cfg, "1", "not-an-int")

	require.Error(t, err)
	assert.ErrorContains(t, err, `field "Value[1]": failed to parse int`)
	assert.Equal(t, []int{7}, cfg.Value, "the field must be left untouched")
}

func TestSetNonSliceRejectsAnythingButOneValue(t *testing.T) {
	t.Parallel()

	testCases := map[string][]string{
		"no values":       nil,
		"multiple values": {"a", "b"},
	}

	for name, values := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := setAllValues(&struct{ Value string }{}, values...)

			require.Error(t, err)
			assert.ErrorContains(t, err, "expected a single value")
		})
	}
}
