package generic_test

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/generic"
	"github.com/stretchr/testify/assert"
)

func TestInstantiateGenericPointerType(t *testing.T) {
	actual := generic.InstantiateGenericStruct[*v1.RadixRegistration]()
	expected := v1.RadixRegistration{}
	assert.Equal(t, *actual, expected)
}

func TestInstantiateGenericType(t *testing.T) {
	actual := generic.InstantiateGenericStruct[v1.RadixRegistration]()
	expected := v1.RadixRegistration{}
	assert.Equal(t, actual, expected)
}

func TestInstantiaPlain(t *testing.T) {
	actual := generic.InstantiateGenericStruct[string]()
	expected := ""
	assert.Equal(t, actual, expected)

	actualInt := generic.InstantiateGenericStruct[int]()
	expectedInt := 0
	assert.Equal(t, actualInt, expectedInt)

	actualFloat := generic.InstantiateGenericStruct[float64]()
	expectedFloat := 0.0
	assert.Equal(t, actualFloat, expectedFloat)

	actualBool := generic.InstantiateGenericStruct[bool]()
	expectedBool := false
	assert.Equal(t, actualBool, expectedBool)
}

func TestInstantiaPlainPointer(t *testing.T) {
	actual := generic.InstantiateGenericStruct[*string]()
	expected := ""
	assert.Equal(t, *actual, expected)

	actualInt := generic.InstantiateGenericStruct[*int]()
	expectedInt := 0
	assert.Equal(t, *actualInt, expectedInt)

	actualFloat := generic.InstantiateGenericStruct[*float64]()
	expectedFloat := 0.0
	assert.Equal(t, *actualFloat, expectedFloat)

	actualBool := generic.InstantiateGenericStruct[*bool]()
	expectedBool := false
	assert.Equal(t, *actualBool, expectedBool)
}

func TestInstantiatSlicePanics(t *testing.T) {
	assert.Panics(t, func() {
		generic.InstantiateGenericStruct[[]v1.RadixRegistration]()
	})
}
