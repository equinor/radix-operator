package subpipeline

import (
	"errors"
	"fmt"
	"reflect"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// ObjectParamSpec builds a Tekton object ParamSpec from exported struct fields
// tagged with propname. Only string fields with a propname tag are included.
//
// Example:
//
//	type MyParam struct {
//		GitSSHUrl string `propname:"gitSSHUrl"`
//		GitCommit string `propname:"gitCommit"`
//		UnTagged  string
//	}
//
//	spec, err := ObjectParamSpec("radix", MyParam{})
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
	properties, err := propTaggedStringMap(obj)
	if err != nil {
		return pipelinev1.ParamSpec{}, err
	}

	paramProperties := make(map[string]pipelinev1.PropertySpec, len(properties))
	for propName := range properties {
		paramProperties[propName] = pipelinev1.PropertySpec{Type: pipelinev1.ParamTypeString}
	}

	return pipelinev1.ParamSpec{
		Name:       paramName,
		Type:       pipelinev1.ParamTypeObject,
		Properties: paramProperties,
	}, nil
}

// ObjectParamReference builds a Tekton object Param whose properties reference
// properties in another object parameter. Only exported string fields tagged
// with propname and present in the reference ParamSpec are included.
//
// Example:
//
//	type MyParam struct {
//		GitSSHUrl string `propname:"gitSSHUrl"`
//		GitCommit string `propname:"gitCommit"`
//	}
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
//			"gitSSHUrl": "\"$(params.radix.gitSSHUrl)\"",
//			"gitCommit": "\"$(params.radix.gitCommit)\"",
//		}),
//	}
func ObjectParamReference(paramName string, obj any, reference pipelinev1.ParamSpec) (pipelinev1.Param, error) {
	if reference.Type != pipelinev1.ParamTypeObject {
		return pipelinev1.Param{}, errors.New("reference param must be of type object")
	}

	properties, err := propTaggedStringMap(obj)
	if err != nil {
		return pipelinev1.Param{}, err
	}

	paramValues := make(map[string]string, len(reference.Properties))
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

func ObjectParam(paramName string, obj any) (pipelinev1.Param, error) {
	properties, err := propTaggedStringMap(obj)
	if err != nil {
		return pipelinev1.Param{}, err
	}

	paramValues := make(map[string]string, len(properties))
	for propName, propValue := range properties {
		paramValues[propName] = propValue
	}

	return pipelinev1.Param{
		Name:  paramName,
		Value: *pipelinev1.NewObject(paramValues),
	}, nil
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
