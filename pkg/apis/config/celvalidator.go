package config

import (
	"cmp"
	"encoding/json/v2"
	"fmt"
	"reflect"
	"time"

	"github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/api/resource"
)

type CelValidator struct {
	environment *cel.Env
}

func NewCelValidator() (*CelValidator, error) {
	environment, err := cel.NewEnv(
		cel.Variable("self", cel.DynType),
		cel.Variable("config", cel.DynType),
		cel.Function("compareQuantity",
			cel.Overload(
				"compare_quantity_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.IntType,
				cel.BinaryBinding(compareQuantity),
			),
		),
		cel.Function("compareDuration",
			cel.Overload(
				"compare_duration_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.IntType,
				cel.BinaryBinding(compareDuration),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create validation environment: %w", err)
	}
	return &CelValidator{environment: environment}, nil
}

func (v *CelValidator) ValidateField(expression string, config any, value reflect.Value) (valid bool, err error) {
	configValue, err := toJSONValue(config)
	if err != nil {
		return false, fmt.Errorf("failed to convert config for validation: %w", err)
	}

	self, err := toJSONValue(value.Interface())
	if err != nil {
		return false, fmt.Errorf("failed to convert value for validation: %w", err)
	}
	ast, issues := v.environment.Compile(expression)
	if issues.Err() != nil {
		return false, fmt.Errorf("failed parsing validation: %w", issues.Err())
	}
	program, err := v.environment.Program(ast)
	if err != nil {
		return false, fmt.Errorf("failed creating validation program: %w", err)
	}
	result, _, err := program.Eval(map[string]any{"self": self, "config": configValue})
	if err != nil {
		return false, fmt.Errorf("failed evaluating validation: %w", err)
	}
	valid, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("validation expression did not return a bool")
	}
	return valid, nil
}

func toJSONValue(value any) (any, error) {
	valueJSON, err := json.Marshal(value, DurationMarshaller)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(valueJSON, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func compareQuantity(lhs, rhs ref.Val) ref.Val {
	left, err := resource.ParseQuantity(string(lhs.(celtypes.String)))
	if err != nil {
		return celtypes.NewErr("invalid quantity %q: %v", lhs.Value(), err)
	}
	right, err := resource.ParseQuantity(string(rhs.(celtypes.String)))
	if err != nil {
		return celtypes.NewErr("invalid quantity %q: %v", rhs.Value(), err)
	}
	return celtypes.Int(left.Cmp(right))
}

func compareDuration(lhs, rhs ref.Val) ref.Val {
	left, err := time.ParseDuration(string(lhs.(celtypes.String)))
	if err != nil {
		return celtypes.NewErr("invalid duration %q: %v", lhs.Value(), err)
	}
	right, err := time.ParseDuration(string(rhs.(celtypes.String)))
	if err != nil {
		return celtypes.NewErr("invalid duration %q: %v", rhs.Value(), err)
	}

	return celtypes.Int(cmp.Compare(left, right))
}
