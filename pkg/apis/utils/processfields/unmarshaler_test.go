package processfields

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

type timeout time.Duration

type byteSize int64

func TestSetUnmarshalerAndParsedTypes(t *testing.T) {
	t.Parallel()
	expectedURL, err := url.Parse("https://example.com/path?query=value#fragment")
	require.NoError(t, err)
	expectedTimestamp := time.Date(2026, time.August, 26, 14, 30, 45, 0, time.UTC)

	testCases := map[string]struct {
		value    string
		config   any
		field    string
		expected any
	}{
		"text unmarshaler with pointer receiver": {
			value:    "radix",
			config:   &struct{ Value textValue }{},
			field:    "Value",
			expected: textValue("RADIX"),
		},
		"binary unmarshaler": {
			value:    "radix",
			config:   &struct{ Value binaryValue }{},
			field:    "Value",
			expected: binaryValue("radix"),
		},
		"text unmarshaler takes precedence": {
			value:    "radix",
			config:   &struct{ Value dualUnmarshaler }{},
			field:    "Value",
			expected: dualUnmarshaler{Method: "text"},
		},
		"binary unmarshaler takes precedence over json": {
			value:    "radix",
			config:   &struct{ Value binaryJSONUnmarshaler }{},
			field:    "Value",
			expected: binaryJSONUnmarshaler{Method: "binary"},
		},
		"json unmarshaler": {
			value:    "radix",
			config:   &struct{ Value jsonValue }{},
			field:    "Value",
			expected: jsonValue{Value: "RADIX"},
		},
		"URL": {
			value:    expectedURL.String(),
			config:   &struct{ Value url.URL }{},
			field:    "Value",
			expected: *expectedURL,
		},
		"timestamp": {
			value:    expectedTimestamp.Format(time.RFC3339),
			config:   &struct{ Value time.Time }{},
			field:    "Value",
			expected: expectedTimestamp,
		},
		"duration": {
			value:    "5m30s",
			config:   &struct{ Value time.Duration }{},
			field:    "Value",
			expected: 5*time.Minute + 30*time.Second,
		},
		"quantity": {
			value:    "500m",
			config:   &struct{ Value resource.Quantity }{},
			field:    "Value",
			expected: resource.MustParse("500m"),
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

// Reflection cannot tell a type declared from time.Duration apart from any other defined
// int64, so neither gets duration syntax. Only time.Duration itself does.
func TestSetDefinedInt64RejectsDurationSyntax(t *testing.T) {
	t.Parallel()

	testCases := map[string]any{
		"declared from time.Duration": &struct{ Value timeout }{},
		"declared from int64":         &struct{ Value byteSize }{},
	}

	for name, cfg := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := setAll(cfg, "5m30s")

			require.Error(t, err)
			assert.ErrorContains(t, err, "failed to parse int")
		})
	}
}

// A bare number must not be accepted as a duration, otherwise "30" silently means 30ns.
func TestSetDurationRejectsUnitlessValue(t *testing.T) {
	t.Parallel()

	cfg := &struct{ Value time.Duration }{}

	err := setAll(cfg, "30")

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse duration")
}

func TestSetUnmarshalerReturnsErrors(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		value       string
		config      any
		errorString string
	}{
		"text unmarshaler error": {
			value:       "invalid",
			config:      &struct{ Value textValue }{},
			errorString: "failed to unmarshal text: invalid text value",
		},
		"binary unmarshaler error": {
			value:       "invalid",
			config:      &struct{ Value binaryValue }{},
			errorString: "failed to unmarshal binary: invalid binary value",
		},
		"json unmarshaler error": {
			value:       "invalid",
			config:      &struct{ Value jsonValue }{},
			errorString: "failed to unmarshal json: invalid json value",
		},
		"invalid quantity": {
			value:       "not-a-quantity",
			config:      &struct{ Value resource.Quantity }{},
			errorString: "failed to unmarshal json",
		},
		"invalid URL": {
			value:       "https://example.com/%zz",
			config:      &struct{ Value url.URL }{},
			errorString: "failed to unmarshal binary",
		},
		"invalid timestamp": {
			value:       "not-a-timestamp",
			config:      &struct{ Value time.Time }{},
			errorString: "failed to unmarshal text",
		},
		"invalid duration": {
			value:       "not-a-duration",
			config:      &struct{ Value time.Duration }{},
			errorString: "failed to parse duration",
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

func TestWalkFieldsTreatsUnmarshalerStructsAsLeaves(t *testing.T) {
	t.Parallel()

	type config struct {
		Timestamp time.Time
		URL       url.URL
		Dual      dualUnmarshaler
		JSON      jsonValue
		Quantity  resource.Quantity
	}

	visited, err := visitAll(&config{})

	require.NoError(t, err)
	assert.Equal(t, []string{"Timestamp", "URL", "Dual", "JSON", "Quantity"}, visited)
}
