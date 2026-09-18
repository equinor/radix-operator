package configcodec

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/utils/processfields"
	"sigs.k8s.io/yaml"
)

var envMacroJSONRegexp = regexp.MustCompile(`\$__env\(([^)]+)\)`)

// Validator is an interface that types can implement to provide custom validation logic.
type Validator interface {
	Validate() error
}

// Decode decodes the given YAML or JSON data into the provided output structure, expanding environment macros and processing environment overrides. It also validates the resulting configuration.
func Decode(data []byte, out any) error {
	if reflect.TypeOf(out).Kind() != reflect.Pointer {
		return fmt.Errorf("out must be a pointer to a value of type T")
	}

	tmpOut := reflect.New(reflect.TypeOf(out).Elem()).Interface()
	expandedConfigYaml := expandEnvMacros(data)

	configJson, err := yaml.YAMLToJSON(expandedConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	if err := json.Unmarshal(configJson, tmpOut, unmarshalers); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Parse env overrides
	if err := processEnvOverrides(tmpOut, "RADIXCONFIG"); err != nil {
		return fmt.Errorf("failed to process env overrides: %w", err)
	}

	// Validate
	if err := validateConfig(tmpOut); err != nil {
		return fmt.Errorf("failed to validate config: %w", err)
	}

	reflect.ValueOf(out).Elem().Set(reflect.ValueOf(tmpOut).Elem())
	return nil
}

// MustDecode decodes the given YAML or JSON data into the provided output structure, panicking if an error occurs. It is a convenience wrapper around Decode.
func MustDecode(data []byte, out any) {
	if err := Decode(data, out); err != nil {
		panic(fmt.Errorf("failed to decode config: %w", err))
	}
}

// Encode encodes the given configuration structure into YAML format.
func Encode(cfg any) ([]byte, error) {
	jsonData, err := json.Marshal(cfg, marshalers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	yamlData, err := yaml.JSONToYAML(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert JSON to YAML: %w", err)
	}
	return yamlData, nil
}

func expandEnvMacros(configJson []byte) []byte {
	return envMacroJSONRegexp.ReplaceAllFunc(configJson, func(macro []byte) []byte {
		envName := string(macro[len(`$__env(`) : len(macro)-1])
		envValue := os.Getenv(envName)
		if envValue == "" {
			return macro
		}
		return []byte(envValue)
	})
}

func processEnvOverrides(out any, prefix string) error {
	return processfields.WalkFields(out, func(path string, field reflect.StructField, val reflect.Value, setter processfields.SetValFunc) error {
		env := strings.ReplaceAll(path, ".", "_")
		env = strings.ReplaceAll(env, "[", "_")
		env = strings.ReplaceAll(env, "]", "_")
		env = strings.ToUpper(prefix + "_" + env)
		env = strings.ReplaceAll(env, "__", "_")
		env = strings.TrimSuffix(env, "_")
		env = strings.TrimPrefix(env, "_")

		envValue := os.Getenv(env)
		if envValue == "" {
			return nil
		}

		if setter == nil {
			return fmt.Errorf("it's not allowed to use env-overrides (%s) on a struct on path %s", env, path)
		}

		values := []string{envValue}
		if field.Type.Kind() == reflect.Slice {
			values = strings.Split(envValue, ",")
		}

		if err := setter(values...); err != nil {
			return fmt.Errorf("failed to set field %q from env %q: %w", path, env, err)
		}
		return nil
	})
}

func validateConfig(cfg any) error {
	validator, err := newCelValidator()
	if err != nil {
		return fmt.Errorf("failed to create config validator: %w", err)
	}

	return processfields.WalkFields(cfg, func(path string, field reflect.StructField, value reflect.Value, _ processfields.SetValFunc) error {
		requiredTag := field.Tag.Get("required")
		required, _ := strconv.ParseBool(requiredTag)

		if value.IsZero() {
			if required {
				return fmt.Errorf("field %q is required but not set", path)
			}

			return nil
		}

		if val, ok := value.Interface().(Validator); ok {
			if err := val.Validate(); err != nil {
				return fmt.Errorf("field %q validation failed: %w", path, err)
			}
		}

		expression := field.Tag.Get("validate")
		if expression != "" {
			if valid, err := validator.ValidateField(expression, cfg, value); err != nil {
				return fmt.Errorf("field %q validation failed expression: %w", path, err)
			} else if !valid {
				return fmt.Errorf("field %q did not pass validation expression", path)
			}
		}

		return nil
	})
}
