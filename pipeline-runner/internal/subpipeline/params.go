package subpipeline

import (
	"errors"
	"fmt"
	"reflect"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// ObjectParamSpec builds a Tekton object ParamSpec from exported struct fields
// tagged with propname. Only string fields with a propname tag are included.
func ObjectParamSpec(paramName string, obj any) (pipelinev1.ParamSpec, error) {
	properties, err := propTaggedStringMap(obj)
	if err != nil {
		return pipelinev1.ParamSpec{}, err
	}
	keys := make([]string, 0, len(properties))
	for k := range properties {
		keys = append(keys, k)
	}
	return DynamicObjectParamSpec(paramName, keys), nil
}

// ObjectParamReference builds a Tekton object Param whose properties reference
// properties in another object parameter. Only exported string fields tagged
// with propname and present in the reference ParamSpec are included.
func ObjectParamReference(paramName string, obj any, reference pipelinev1.ParamSpec) (pipelinev1.Param, error) {
	if reference.Type != pipelinev1.ParamTypeObject {
		return pipelinev1.Param{}, errors.New("reference param must be of type object")
	}

	properties, err := propTaggedStringMap(obj)
	if err != nil {
		return pipelinev1.Param{}, err
	}

	var keys []string
	for propName := range properties {
		if _, exist := reference.Properties[propName]; exist {
			keys = append(keys, propName)
		}
	}

	return DynamicObjectParamReference(paramName, keys, reference.Name), nil
}

// ObjectParam builds a Tekton object Param from exported struct fields tagged
// with propname. Only string fields with a propname tag are included.
func ObjectParam(paramName string, obj any) (pipelinev1.Param, error) {
	properties, err := propTaggedStringMap(obj)
	if err != nil {
		return pipelinev1.Param{}, err
	}
	return DynamicObjectParam(paramName, properties), nil
}

// DynamicObjectParamSpec builds a Tekton object ParamSpec from a list of property names.
// All properties are typed as string.
func DynamicObjectParamSpec(paramName string, keys []string) pipelinev1.ParamSpec {
	properties := make(map[string]pipelinev1.PropertySpec, len(keys))
	for _, key := range keys {
		properties[key] = pipelinev1.PropertySpec{Type: pipelinev1.ParamTypeString}
	}
	return pipelinev1.ParamSpec{
		Name:       paramName,
		Type:       pipelinev1.ParamTypeObject,
		Properties: properties,
	}
}

// DynamicObjectParam builds a Tekton object Param from a map of key-value pairs.
func DynamicObjectParam(paramName string, values map[string]string) pipelinev1.Param {
	return pipelinev1.Param{
		Name:  paramName,
		Value: *pipelinev1.NewObject(values),
	}
}

// DynamicObjectParamReference builds a Tekton object Param whose properties reference
// properties in another object parameter by name.
func DynamicObjectParamReference(paramName string, keys []string, referenceParamName string) pipelinev1.Param {
	paramValues := make(map[string]string, len(keys))
	for _, key := range keys {
		paramValues[key] = fmt.Sprintf("$(params.%s.%s)", referenceParamName, key)
	}
	return pipelinev1.Param{
		Name:  paramName,
		Value: *pipelinev1.NewObject(paramValues),
	}
}

func propTaggedStringMap(obj any) (map[string]string, error) {
	if obj == nil {
		return nil, errors.New("input cannot be nil")
	}

	v := reflect.ValueOf(obj)
	t := v.Type()

	if t.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, errors.New("input cannot be nil pointer")
		}
		t = t.Elem()
		v = v.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, errors.New("input must be struct or pointer to struct")
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
