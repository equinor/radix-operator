package mergo

import (
	"testing"

	"dario.cat/mergo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_BoolPtrTransformer(t *testing.T) {

	type testBool struct {
		V *bool
	}
	boolPtr := func(v bool) *bool { return &v }

	tests := map[string]struct {
		dst      testBool
		src      testBool
		expected testBool
	}{
		"use value from src #1": {dst: testBool{V: boolPtr(true)}, src: testBool{V: boolPtr(false)}, expected: testBool{V: boolPtr(false)}},
		"use value from src #2": {dst: testBool{V: boolPtr(false)}, src: testBool{V: boolPtr(true)}, expected: testBool{V: boolPtr(true)}},
		"use value from src #3": {dst: testBool{}, src: testBool{V: boolPtr(false)}, expected: testBool{V: boolPtr(false)}},
		"use value from src #4": {dst: testBool{}, src: testBool{V: boolPtr(true)}, expected: testBool{V: boolPtr(true)}},
		"use value from dst #1": {dst: testBool{V: boolPtr(true)}, src: testBool{}, expected: testBool{V: boolPtr(true)}},
		"use value from dst #2": {dst: testBool{V: boolPtr(false)}, src: testBool{}, expected: testBool{V: boolPtr(false)}},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			err := mergo.Merge(&test.dst, &test.src, mergo.WithOverride, mergo.WithTransformers(BoolPtrTransformer{}))
			require.NoError(t, err)
			assert.Equal(t, test.expected, test.dst)
		})
	}
}

func Test_ResourceQuantityTransformer_ValueType(t *testing.T) {
	type testObj struct {
		V resource.Quantity
	}

	tests := map[string]struct {
		dst      testObj
		src      testObj
		expected testObj
	}{
		"dst empty, src set, expect src": {dst: testObj{V: resource.Quantity{}}, src: testObj{V: resource.MustParse("2M")}, expected: testObj{V: resource.MustParse("2M")}},
		"dst set, src set, expect src":   {dst: testObj{V: resource.MustParse("1M")}, src: testObj{V: resource.MustParse("2M")}, expected: testObj{V: resource.MustParse("2M")}},
		"dst set, src empty, expect src": {dst: testObj{V: resource.MustParse("1M")}, src: testObj{V: resource.Quantity{}}, expected: testObj{V: resource.MustParse("1M")}},
		"all empty, expect empty":        {dst: testObj{V: resource.Quantity{}}, src: testObj{V: resource.Quantity{}}, expected: testObj{V: resource.Quantity{}}},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			err := mergo.Merge(&test.dst, &test.src, mergo.WithOverride, mergo.WithTransformers(ResourceQuantityTransformer{}))
			require.NoError(t, err)
			assert.Equal(t, test.expected, test.dst)
		})
	}
}

func Test_ResourceQuantityTransformer_PointerType(t *testing.T) {
	type testObj struct {
		V *resource.Quantity
	}

	tests := map[string]struct {
		dst      testObj
		src      testObj
		expected testObj
	}{
		"dst empty, src set, expect src": {
			dst:      testObj{V: &resource.Quantity{}},
			src:      testObj{V: resource.NewMilliQuantity(2, resource.DecimalSI)},
			expected: testObj{V: resource.NewMilliQuantity(2, resource.DecimalSI)},
		},
		"dst set, src empty, expect dst": {
			dst:      testObj{V: resource.NewMilliQuantity(1, resource.DecimalSI)},
			src:      testObj{V: &resource.Quantity{}},
			expected: testObj{V: resource.NewMilliQuantity(1, resource.DecimalSI)},
		},
		"dst set, src set, expect src": {
			dst:      testObj{V: resource.NewMilliQuantity(1, resource.DecimalSI)},
			src:      testObj{V: resource.NewMilliQuantity(2, resource.DecimalSI)},
			expected: testObj{V: resource.NewMilliQuantity(2, resource.DecimalSI)},
		},
		"dst nil, src set, expect src": {
			dst:      testObj{V: nil},
			src:      testObj{V: resource.NewMilliQuantity(2, resource.DecimalSI)},
			expected: testObj{V: resource.NewMilliQuantity(2, resource.DecimalSI)},
		},
		"dst set, src nil, expect dst": {
			dst:      testObj{V: resource.NewMilliQuantity(1, resource.DecimalSI)},
			src:      testObj{V: nil},
			expected: testObj{V: resource.NewMilliQuantity(1, resource.DecimalSI)},
		},
		"dst nil, src empty, expect empty": {
			dst:      testObj{V: nil},
			src:      testObj{V: &resource.Quantity{}},
			expected: testObj{V: &resource.Quantity{}},
		},
		"dst empty, src nil, expect empty": {
			dst:      testObj{V: &resource.Quantity{}},
			src:      testObj{V: nil},
			expected: testObj{V: &resource.Quantity{}},
		},
		"all empty, expect empty": {
			dst:      testObj{V: &resource.Quantity{}},
			src:      testObj{V: &resource.Quantity{}},
			expected: testObj{V: &resource.Quantity{}},
		},
		"all nil, expect nil": {
			dst:      testObj{V: nil},
			src:      testObj{V: nil},
			expected: testObj{V: nil},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			err := mergo.Merge(&test.dst, &test.src, mergo.WithOverride, mergo.WithTransformers(ResourceQuantityTransformer{}))
			require.NoError(t, err)
			assert.Equal(t, test.expected, test.dst)
		})
	}
}
