package configcodec_test

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/utils/configcodec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testValidatedString string

func (value testValidatedString) Validate() error {
	if value != "valid" {
		return errors.New("value must be valid")
	}
	return nil
}

type testBinaryValue string

func (value testBinaryValue) MarshalBinary() ([]byte, error) {
	if value == "marshal-error" {
		return nil, errors.New("binary marshal failed")
	}
	return []byte("encoded:" + value), nil
}

func (value *testBinaryValue) UnmarshalBinary(data []byte) error {
	if string(data) == "decode-error" {
		return errors.New("binary unmarshal failed")
	}
	*value = testBinaryValue(string(data))
	return nil
}

type testNestedConfig struct {
	Count int `json:"count"`
}

type testConfig struct {
	Name     string              `json:"name" required:"true"`
	Timeout  time.Duration       `json:"timeout" validate:"compareDuration(self, '1s') >= 0"`
	Endpoint url.URL             `json:"endpoint"`
	Labels   []string            `json:"labels"`
	Nested   testNestedConfig    `json:"nested"`
	Custom   testValidatedString `json:"custom"`
	Binary   testBinaryValue     `json:"binary"`
}

func TestDecodeYAMLExpandsMacrosAppliesOverridesAndCustomCodecs(t *testing.T) {
	t.Setenv("CONFIG_NAME", "from-macro")
	t.Setenv("RADIXCONFIG_NESTED_COUNT", "42")
	t.Setenv("RADIXCONFIG_LABELS", "one,two")

	data := []byte(`
name: $__env(CONFIG_NAME)
timeout: 2m30s
endpoint: https://example.com/path?q=value
labels: [original]
nested:
  count: 1
custom: valid
binary: decoded-value
`)
	var actual testConfig

	err := configcodec.Decode(data, &actual)

	require.NoError(t, err)
	assert.Equal(t, "from-macro", actual.Name)
	assert.Equal(t, 2*time.Minute+30*time.Second, actual.Timeout)
	assert.Equal(t, "https://example.com/path?q=value", actual.Endpoint.String())
	assert.Equal(t, []string{"one", "two"}, actual.Labels)
	assert.Equal(t, 42, actual.Nested.Count)
	assert.Equal(t, testValidatedString("valid"), actual.Custom)
	assert.Equal(t, testBinaryValue("decoded-value"), actual.Binary)
}

func TestDecodeAcceptsJSONAndNumericDuration(t *testing.T) {
	var actual struct {
		Timeout time.Duration `json:"timeout"`
	}

	err := configcodec.Decode([]byte(`{"timeout":1500000000}`), &actual)

	require.NoError(t, err)
	assert.Equal(t, 1500*time.Millisecond, actual.Timeout)
}

func TestDecodeReturnsErrorsWithoutChangingOutput(t *testing.T) {
	validOutput := testConfig{Name: "unchanged"}
	var nilOutput *testConfig

	testCases := map[string]struct {
		data          []byte
		out           any
		setEnv        func(*testing.T)
		errorContains string
	}{
		"nil output": {
			data:          []byte("name: valid"),
			out:           nil,
			errorContains: "out must be a pointer",
		},
		"nil output pointer": {
			data:          []byte("name: valid"),
			out:           nilOutput,
			errorContains: "out must be a pointer",
		},
		"non-pointer output": {
			data:          []byte("name: valid"),
			out:           testConfig{},
			errorContains: "out must be a pointer",
		},
		"invalid YAML": {
			data:          []byte("name: ["),
			out:           &validOutput,
			errorContains: "failed to convert YAML to JSON",
		},
		"invalid field value": {
			data:          []byte("name: valid\nnested:\n  count: no"),
			out:           &validOutput,
			errorContains: "failed to unmarshal JSON",
		},
		"invalid duration": {
			data:          []byte("name: valid\ntimeout: eventually"),
			out:           &validOutput,
			errorContains: "invalid duration string",
		},
		"invalid duration kind": {
			data:          []byte("name: valid\ntimeout: true"),
			out:           &validOutput,
			errorContains: "cannot unmarshal JSON kind",
		},
		"invalid env override": {
			data: []byte("name: valid"),
			out:  &validOutput,
			setEnv: func(t *testing.T) {
				t.Setenv("RADIXCONFIG_NESTED_COUNT", "not-an-int")
			},
			errorContains: "failed to process env overrides",
		},
		"container env override": {
			data: []byte("name: valid"),
			out:  &validOutput,
			setEnv: func(t *testing.T) {
				t.Setenv("RADIXCONFIG_NESTED", "not-allowed")
			},
			errorContains: "not allowed to use env-overrides",
		},
		"missing required field": {
			data:          []byte("{}"),
			out:           &validOutput,
			errorContains: `field "Name" is required but not set`,
		},
		"custom validation": {
			data:          []byte("name: valid\ncustom: invalid"),
			out:           &validOutput,
			errorContains: "value must be valid",
		},
		"CEL validation": {
			data:          []byte("name: valid\ntimeout: 500ms"),
			out:           &validOutput,
			errorContains: "did not pass validation expression",
		},
		"binary unmarshaler": {
			data:          []byte("name: valid\nbinary: decode-error"),
			out:           &validOutput,
			errorContains: "binary unmarshal failed",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if testCase.setEnv != nil {
				testCase.setEnv(t)
			}
			before := validOutput

			err := configcodec.Decode(testCase.data, testCase.out)

			require.ErrorContains(t, err, testCase.errorContains)
			if testCase.out == &validOutput {
				assert.Equal(t, before, validOutput)
			}
		})
	}
}

func TestMustDecode(t *testing.T) {
	var actual struct {
		Name string `json:"name" required:"true"`
	}
	require.NotPanics(t, func() { configcodec.MustDecode([]byte("name: valid"), &actual) })
	assert.Equal(t, "valid", actual.Name)

	assert.PanicsWithError(t, `failed to decode config: failed to validate config: field "Name" is required but not set`, func() {
		configcodec.MustDecode([]byte("{}"), &actual)
	})
}

func TestEncodeUsesDurationAndBinaryMarshalers(t *testing.T) {
	expectedURL, err := url.Parse("https://example.com/path")
	require.NoError(t, err)
	expected := testConfig{
		Name:     "config",
		Timeout:  90 * time.Second,
		Endpoint: *expectedURL,
		Labels:   []string{"one", "two"},
		Nested:   testNestedConfig{Count: 3},
		Custom:   "valid",
		Binary:   "value",
	}

	encoded, err := configcodec.Encode(expected)

	require.NoError(t, err)
	assert.Contains(t, string(encoded), "timeout: 1m30s")
	assert.Contains(t, string(encoded), "binary: encoded:value")

	var actual testConfig
	require.NoError(t, configcodec.Decode(encoded, &actual))
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Timeout, actual.Timeout)
	assert.Equal(t, expected.Endpoint, actual.Endpoint)
	assert.Equal(t, expected.Labels, actual.Labels)
	assert.Equal(t, expected.Nested, actual.Nested)
	assert.Equal(t, expected.Custom, actual.Custom)
	assert.Equal(t, testBinaryValue("encoded:value"), actual.Binary)
}

func TestEncodeReturnsMarshalerError(t *testing.T) {
	_, err := configcodec.Encode(struct {
		Value testBinaryValue `json:"value"`
	}{Value: "marshal-error"})

	require.ErrorContains(t, err, "failed to marshal config to JSON")
	assert.ErrorContains(t, err, "binary marshal failed")
}

func TestDecodeLeavesUnsetEnvironmentMacrosUnchanged(t *testing.T) {
	t.Setenv("PRESENT", "replacement")
	t.Setenv("EMPTY", "")
	var actual struct {
		Value string `json:"value"`
	}

	err := configcodec.Decode([]byte(`{"value":"$__env(PRESENT) $__env(MISSING) $__env(EMPTY)"}`), &actual)

	require.NoError(t, err)
	assert.Equal(t, "replacement $__env(MISSING) $__env(EMPTY)", actual.Value)
}

func TestDecodeValidatesCELExpressions(t *testing.T) {
	testCases := map[string]struct {
		data []byte
		out  any
	}{
		"self": {
			data: []byte(`{"value":3}`),
			out: &struct {
				Value int `json:"value" validate:"self > 2"`
			}{},
		},
		"config": {
			data: []byte(`{"maximum":5,"value":5}`),
			out: &struct {
				Maximum int `json:"maximum"`
				Value   int `json:"value" validate:"self <= config.maximum"`
			}{},
		},
		"quantity greater": {
			data: []byte(`{"value":"2Gi"}`),
			out: &struct {
				Value string `json:"value" validate:"compareQuantity(self, '1Gi') > 0"`
			}{},
		},
		"quantity equal": {
			data: []byte(`{"value":"1"}`),
			out: &struct {
				Value string `json:"value" validate:"compareQuantity(self, '1000m') == 0"`
			}{},
		},
		"duration less": {
			data: []byte(`{"value":"30s"}`),
			out: &struct {
				Value string `json:"value" validate:"compareDuration(self, '1m') < 0"`
			}{},
		},
		"duration equal": {
			data: []byte(`{"value":"1m"}`),
			out: &struct {
				Value string `json:"value" validate:"compareDuration(self, '60s') == 0"`
			}{},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, configcodec.Decode(testCase.data, testCase.out))
		})
	}
}

func TestDecodeReturnsCELValidationErrors(t *testing.T) {
	testCases := map[string]struct {
		data          []byte
		out           any
		errorContains string
	}{
		"false result": {
			data: []byte(`{"value":3}`),
			out: &struct {
				Value int `json:"value" validate:"self > 10"`
			}{},
			errorContains: "did not pass validation expression",
		},
		"parse": {
			data: []byte(`{"value":1}`),
			out: &struct {
				Value int `json:"value" validate:"self >"`
			}{},
			errorContains: "failed parsing validation",
		},
		"evaluation": {
			data: []byte(`{"value":{"present":1}}`),
			out: &struct {
				Value map[string]any `json:"value" validate:"self.missing == 1"`
			}{},
			errorContains: "failed evaluating validation",
		},
		"non-boolean": {
			data: []byte(`{"value":"value"}`),
			out: &struct {
				Value string `json:"value" validate:"self"`
			}{},
			errorContains: "did not return a bool",
		},
		"invalid quantity": {
			data: []byte(`{"value":"invalid"}`),
			out: &struct {
				Value string `json:"value" validate:"compareQuantity(self, '1Gi') > 0"`
			}{},
			errorContains: "invalid quantity",
		},
		"invalid duration": {
			data: []byte(`{"value":"invalid"}`),
			out: &struct {
				Value string `json:"value" validate:"compareDuration(self, '1m') > 0"`
			}{},
			errorContains: "invalid duration",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			err := configcodec.Decode(testCase.data, testCase.out)

			require.Error(t, err)
			assert.ErrorContains(t, err, testCase.errorContains)
		})
	}
}
