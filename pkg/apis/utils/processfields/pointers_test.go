package processfields_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/utils/processfields"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkFieldsInitializesNilPointerUnmarshaler(t *testing.T) {
	t.Parallel()

	cfg := &struct{ Value *textValue }{}

	err := setAll(cfg, "radix")

	require.NoError(t, err)
	require.NotNil(t, cfg.Value)
	assert.Equal(t, textValue("RADIX"), *cfg.Value)
}

func TestWalkFieldsLeavesNilPointerUnmarshalerNilOnError(t *testing.T) {
	t.Parallel()

	cfg := &struct{ Value *textValue }{}

	err := setAll(cfg, "invalid")

	require.ErrorContains(t, err, "failed to unmarshal text: invalid text value")
	assert.Nil(t, cfg.Value)
}

func TestWalkFieldsSetsNonNilPointerUnmarshaler(t *testing.T) {
	t.Parallel()

	existing := textValue("initial")
	cfg := &struct{ Value *textValue }{Value: &existing}

	err := setAll(cfg, "radix")

	require.NoError(t, err)
	require.Same(t, &existing, cfg.Value)
	assert.Equal(t, textValue("RADIX"), existing)
}

// A value receiver is indistinguishable from one whose writes are lost, so it is
// rejected even when the mutation happens to escape through an inner pointer.
func TestWalkFieldsRejectsValueReceiverUnmarshaler(t *testing.T) {
	t.Parallel()

	received := ""
	cfg := &struct{ Value valueReceiverTextValue }{
		Value: valueReceiverTextValue{received: &received},
	}

	err := setAll(cfg, "radix")

	require.Error(t, err)
	assert.ErrorContains(t, err, "must implement the unmarshaler with a pointer receiver")
	assert.Empty(t, received)
}

// A value-receiver unmarshaler writes to a copy, so the result is discarded while
// the setter still reports success. It must either take effect or return an error.
func TestWalkFieldsDoesNotSilentlyDiscardValueReceiverWrites(t *testing.T) {
	t.Parallel()

	cfg := &struct{ Value selfMutatingTextValue }{}

	err := setAll(cfg, "radix")
	assert.ErrorContains(t, err, "Value")
}

// Pointers to primitives are the idiomatic way to model optional config values.
func TestWalkFieldsSetsPointerToPrimitive(t *testing.T) {
	t.Parallel()

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		cfg := &struct{ Value *string }{}
		require.NoError(t, setAll(cfg, "radix"))
		require.NotNil(t, cfg.Value)
		assert.Equal(t, "radix", *cfg.Value)
	})

	t.Run("int32", func(t *testing.T) {
		t.Parallel()
		cfg := &struct{ Value *int32 }{}
		require.NoError(t, setAll(cfg, "8080"))
		require.NotNil(t, cfg.Value)
		assert.Equal(t, int32(8080), *cfg.Value)
	})

	t.Run("bool", func(t *testing.T) {
		t.Parallel()
		cfg := &struct{ Value *bool }{}
		require.NoError(t, setAll(cfg, "true"))
		require.NotNil(t, cfg.Value)
		assert.True(t, *cfg.Value)
	})

	t.Run("duration", func(t *testing.T) {
		t.Parallel()
		cfg := &struct{ Value *time.Duration }{}
		require.NoError(t, setAll(cfg, "5m30s"))
		require.NotNil(t, cfg.Value)
		assert.Equal(t, 5*time.Minute+30*time.Second, *cfg.Value)
	})
}

func TestWalkFieldsLeavesPointerToPrimitiveNilOnError(t *testing.T) {
	t.Parallel()

	cfg := &struct{ Value *int }{}

	err := setAll(cfg, "not-an-int")

	require.Error(t, err)
	assert.Nil(t, cfg.Value)
}

func TestWalkFieldsSetsExistingPointerToPrimitiveInPlace(t *testing.T) {
	t.Parallel()

	existing := 1
	cfg := &struct{ Value *int }{Value: &existing}

	err := setAll(cfg, "42")

	require.NoError(t, err)
	require.Same(t, &existing, cfg.Value)
	assert.Equal(t, 42, existing)
}

// Only value structs are recursed into today, so an optional nested config section
// modelled as a pointer is treated as an unsupported leaf.
func TestWalkFieldsTraversesNonNilPointerToStruct(t *testing.T) {
	t.Parallel()

	type nested struct {
		Enabled bool
	}
	cfg := &struct{ Nested *nested }{Nested: &nested{}}

	visited, err := visitAll(cfg)

	require.NoError(t, err)
	assert.Equal(t, []string{"Enabled"}, visited)

	require.NoError(t, setAll(cfg, "true"))
	assert.True(t, cfg.Nested.Enabled)
}

func TestWalkFieldsTraversesNilPointerToStruct(t *testing.T) {
	t.Parallel()

	type nested struct {
		Enabled bool
	}
	cfg := &struct{ Nested *nested }{}

	visited, err := visitAll(cfg)

	require.NoError(t, err)
	assert.Equal(t, []string{"Enabled"}, visited)
}

// A nil section is allocated for the duration of the walk, so a chain of nil sections is
// materialised down to the leaf and kept once a value lands in it.
func TestWalkFieldsAllocatesNilStructPointerWhenFieldIsSet(t *testing.T) {
	t.Parallel()

	type inner struct {
		Level string
	}
	type outer struct {
		Inner *inner
	}
	cfg := &struct{ Outer *outer }{}

	err := setAll(cfg, "debug")

	require.NoError(t, err)
	require.NotNil(t, cfg.Outer)
	require.NotNil(t, cfg.Outer.Inner)
	assert.Equal(t, "debug", cfg.Outer.Inner.Level)
}

func TestWalkFieldsLeavesNilStructPointerNilWhenNothingIsSet(t *testing.T) {
	t.Parallel()

	type inner struct {
		Level string
	}
	type outer struct {
		Inner *inner
	}
	cfg := &struct{ Outer *outer }{}

	_, err := visitAll(cfg)

	require.NoError(t, err)
	assert.Nil(t, cfg.Outer)
}

// A section is kept or dropped by what it ends up holding, not by whether a setter ran, so
// writing a zero value is indistinguishable from writing nothing.
func TestWalkFieldsDropsNilStructPointerSetToZeroValue(t *testing.T) {
	t.Parallel()

	type nested struct {
		Enabled bool
	}
	cfg := &struct{ Nested *nested }{}

	err := setAll(cfg, "false")

	require.NoError(t, err)
	assert.Nil(t, cfg.Nested)
}

// A section that stayed empty is dropped when the walk ends, so a setter captured for one of
// its fields writes into a detached struct. Pinned so it is not rediscovered as a bug.
func TestWalkFieldsSetterForDroppedStructPointerDoesNotReachTheConfig(t *testing.T) {
	t.Parallel()

	type nested struct {
		Level string
	}
	cfg := &struct{ Nested *nested }{}
	var captured processfields.SetValFunc

	err := processfields.WalkFields(cfg, func(_ string, _ reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		captured = setter
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, captured)

	require.NoError(t, captured("debug"))
	assert.Nil(t, cfg.Nested)
}

// Leaf detection reads the method set from a reflect.Value, so it changes depending on
// whether the root was addressable. time.Time must be a leaf either way.
func TestWalkFieldsLeafDetectionIsIndependentOfAddressability(t *testing.T) {
	t.Parallel()

	type config struct {
		Timestamp time.Time
	}

	addressable, err := visitAll(&config{})
	require.NoError(t, err)

	nonAddressable, err := visitAll(config{})
	require.NoError(t, err)

	assert.Equal(t, []string{"Timestamp"}, addressable)
	assert.Equal(t, addressable, nonAddressable)
}

// The error names the field type instead of the field, which sends readers hunting
// for a field called "string".
func TestSetNonAddressableFieldNamesTheField(t *testing.T) {
	t.Parallel()

	err := setAll(struct{ Value string }{}, "updated")

	require.Error(t, err)
	assert.ErrorContains(t, err, `cannot set field "Value"`)
}

// Errors from nested fields carry no ancestry, so identically named fields in
// different sections are indistinguishable.
func TestSetErrorIdentifiesNestedFieldPath(t *testing.T) {
	t.Parallel()

	type operator struct {
		LogLevel int
	}
	type apiServer struct {
		LogLevel int
	}
	type config struct {
		Operator  operator
		ApiServer apiServer
	}

	err := processfields.WalkFields(&config{}, func(_ string, field reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		if field.Name != "LogLevel" {
			return nil
		}
		return setter("not-an-int")
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "Operator.LogLevel")
}
