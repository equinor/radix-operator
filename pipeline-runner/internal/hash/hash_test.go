package hash_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ToHashString(t *testing.T) {
	type dataStruct struct {
		S string
	}

	type algorithm struct {
		alg     hash.Algorithm
		hashLen int
	}
	algorithms := []algorithm{{alg: hash.SHA256, hashLen: 32}}

	type testSpec struct {
		test     string
		val      any
		valOther any
		match    bool
	}
	tests := []testSpec{
		{test: "bool match", val: true, valOther: true, match: true},
		{test: "bool no match", val: true, valOther: false, match: false},
		{test: "int match", val: int(10), valOther: int(10), match: true},
		{test: "int no match", val: int(20), valOther: int(21), match: false},
		{test: "int8 match", val: int8(10), valOther: int8(10), match: true},
		{test: "int8 no match", val: int8(20), valOther: int8(21), match: false},
		{test: "int16 match", val: int16(10), valOther: int16(10), match: true},
		{test: "int16 no match", val: int16(20), valOther: int16(21), match: false},
		{test: "int32 match", val: int32(10), valOther: int32(10), match: true},
		{test: "int32 no match", val: int32(20), valOther: int32(21), match: false},
		{test: "int64 match", val: int64(10), valOther: int64(10), match: true},
		{test: "int64 no match", val: int64(20), valOther: int64(21), match: false},
		{test: "uint match", val: uint(10), valOther: uint(10), match: true},
		{test: "uint no match", val: uint(20), valOther: uint(21), match: false},
		{test: "uint8 match", val: uint8(10), valOther: uint8(10), match: true},
		{test: "uint8 no match", val: uint8(20), valOther: uint8(21), match: false},
		{test: "uint16 match", val: uint16(10), valOther: uint16(10), match: true},
		{test: "uint16 no match", val: uint16(20), valOther: uint16(21), match: false},
		{test: "uint32 match", val: uint32(10), valOther: uint32(10), match: true},
		{test: "uint32 no match", val: uint32(20), valOther: uint32(21), match: false},
		{test: "uint64 match", val: uint64(10), valOther: uint64(10), match: true},
		{test: "uint64 no match", val: uint64(20), valOther: uint64(21), match: false},
		{test: "float32 match", val: float32(10.5), valOther: float32(10.5), match: true},
		{test: "float32 no match", val: float32(20.5), valOther: float32(21.5), match: false},
		{test: "float64 match", val: float64(10.5), valOther: float64(10.5), match: true},
		{test: "float64 no match", val: float64(20.5), valOther: float64(21.5), match: false},
		{test: "string match", val: "foo", valOther: "foo", match: true},
		{test: "string no match", val: "foo", valOther: "Foo", match: false},
		{test: "struct match", val: dataStruct{S: "foo"}, valOther: dataStruct{S: "foo"}, match: true},
		{test: "struct no match", val: dataStruct{S: "foo"}, valOther: dataStruct{S: "Foo"}, match: false},
		{test: "struct pointer match", val: dataStruct{S: "foo"}, valOther: &dataStruct{S: "foo"}, match: true},
	}

	for _, test := range tests {
		for _, alg := range algorithms {
			t.Run(test.test, func(t *testing.T) {
				valHash, err := hash.ToHashString(alg.alg, test.val)
				require.NoError(t, err)
				hashParts := strings.Split(valHash, "=")
				assert.Equal(t, string(alg.alg), hashParts[0])
				valHashBytes, err := hex.DecodeString(hashParts[1])
				require.NoError(t, err)
				assert.Len(t, valHashBytes, alg.hashLen)
				match, err := hash.CompareWithHashString(test.valOther, valHash)
				require.NoError(t, err)
				assert.Equal(t, test.match, match)
				t.Parallel()
			})
		}
	}
}
