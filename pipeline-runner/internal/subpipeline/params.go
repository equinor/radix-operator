package subpipeline

import (
	"errors"
	"fmt"
	"reflect"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// ObjectParamSpec builds a Tekton object ParamSpec from the input.
// The input can be:
//   - A map[string]string: keys are used as property names.
//   - A struct (or pointer to struct) with exported string fields tagged with propname.
//
// Example:
//
//	type MyParam struct {
//		GitSSHUrl string `propname:"gitSSHUrl"`
//		GitCommit string `propname:"gitCommit"`
//		UnTagged  string
//	}
//
// OR:
//
// map[string]string{"gitSSHUrl": "github.com/somewhere/womething", "gitCommit": "abc123"})
//
// This creates an object param named "radix" with properties "gitSSHUrl" and
// "gitCommit", both typed as string.
//
// Example output:
//
//	pipelinev1.ParamSpec{
//		Name: "radix",
//		Type: pipelinev1.ParamTypeObject,
//		Properties: map[string]pipelinev1.PropertySpec{
//			"gitSSHUrl": {Type: pipelinev1.ParamTypeString},
//			"gitCommit": {Type: pipelinev1.ParamTypeString},
//		},
//	}
func ObjectParamSpec(paramName string, obj any) (pipelinev1.ParamSpec, error) {
	properties, err := toStringMap(obj)
	if err != nil {
		return pipelinev1.ParamSpec{}, err
	}
	propSpecs := make(map[string]pipelinev1.PropertySpec, len(properties))
	for k := range properties {
		propSpecs[k] = pipelinev1.PropertySpec{Type: pipelinev1.ParamTypeString}
	}
	return pipelinev1.ParamSpec{
		Name:       paramName,
		Type:       pipelinev1.ParamTypeObject,
		Properties: propSpecs,
	}, nil
}

// ObjectParamReference builds a Tekton object Param whose properties reference
// properties in another object parameter. Only keys present in the reference
// ParamSpec are included.
//
// The input can be a map[string]string or a struct with propname tags.
//
// Example:
//
//	type MyParam struct {
//		GitSSHUrl string `propname:"gitSSHUrl"`
//		GitCommit string `propname:"gitCommit"`
//	}
//
// OR:
//
//	map[string]string{"gitSSHUrl": "git@github.com:equinor/radix-operator.git", "gitCommit": "abc123"})
//
//	reference, _ := ObjectParamSpec("radix", MyParam{})
//	param, _ := ObjectParamReference("source", MyParam{}, reference)
//
// This creates an object param named "source" with values referencing
// "$(params.radix.<property>)" entries from the "radix" object parameter.
//
// Example output:
//
//	pipelinev1.Param{
//		Name: "source",
//		Value: *pipelinev1.NewObject(map[string]string{
//			"gitSSHUrl": "$(params.radix.gitSSHUrl)",
//			"gitCommit": "$(params.radix.gitCommit)",
//		}),
//	}
func ObjectParamReference(paramName string, obj any, reference pipelinev1.ParamSpec) (pipelinev1.Param, error) {
	if reference.Type != pipelinev1.ParamTypeObject {
		return pipelinev1.Param{}, errors.New("reference param must be of type object")
	}

	properties, err := toStringMap(obj)
	if err != nil {
		return pipelinev1.Param{}, err
	}

	paramValues := make(map[string]string)
	for propName := range properties {
		if _, exist := reference.Properties[propName]; exist {
			paramValues[propName] = fmt.Sprintf("$(params.%s.%s)", reference.Name, propName)
		}
	}

	return pipelinev1.Param{
		Name:  paramName,
		Value: *pipelinev1.NewObject(paramValues),
	}, nil
}

// ObjectParam builds a Tekton object Param from the input.
// The input can be:
//   - A map[string]string: used directly as param values.
//   - A struct (or pointer to struct) with exported string fields tagged with propname.
//
// Example:
//
//	 type MyParam struct {
//		GitSSHUrl string `propname:"gitSSHUrl"`
//		GitCommit string `propname:"gitCommit"`
//	 	UnTagged  string
//	 }
//
// OR:
//
//	map[string]string{"gitSSHUrl": "git@github.com:equinor/radix-operator.git", "gitCommit": "abc123"})
//
// Example output:
//
//	pipelinev1.Param{
//		Name: "radix",
//		Value: *pipelinev1.NewObject(map[string]string{
//			"gitSSHUrl": "git@github.com:equinor/radix-operator.git",
//			"gitCommit": "abc123",
//		}),
//	}
func ObjectParam(paramName string, obj any) (pipelinev1.Param, error) {
	properties, err := toStringMap(obj)
	if err != nil {
		return pipelinev1.Param{}, err
	}
	return pipelinev1.Param{
		Name:  paramName,
		Value: *pipelinev1.NewObject(properties),
	}, nil
}

func toStringMap(obj any) (map[string]string, error) {
	if obj == nil {
		return nil, errors.New("input cannot be nil")
	}

	v := reflect.ValueOf(obj)
	t := v.Type()

	// Handle map[string]string and named types based on it (e.g. type ComponentImages map[string]string)
	if t.Kind() == reflect.Map && t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.String {
		result := make(map[string]string, v.Len())
		for _, key := range v.MapKeys() {
			result[key.String()] = v.MapIndex(key).String()
		}
		return result, nil
	}

	if t.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, errors.New("input cannot be nil pointer")
		}
		t = t.Elem()
		v = v.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, errors.New("input must be struct, pointer to struct, or map[string]string")
	}

	result := map[string]string{}
	for i := 0; i < t.NumField(); i++ {
		fieldType := t.Field(i)
		if !fieldType.IsExported() {
			continue
		}

		propName, hasPropTag := fieldType.Tag.Lookup("propname")
		if !hasPropTag || propName == "" {
			continue
		}

		if fieldType.Type.Kind() != reflect.String {
			return nil, fmt.Errorf("field %s has prop tag but is not string", fieldType.Name)
		}

		result[propName] = v.Field(i).String()
	}

	return result, nil
}
