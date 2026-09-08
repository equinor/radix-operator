package processfields_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/processfields"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkFieldsReturnsErrorForInvalidRoot(t *testing.T) {
	t.Parallel()

	var nilConfig *struct{ Value string }
	testCases := map[string]any{
		"nil interface":         nil,
		"nil pointer":           nilConfig,
		"non-struct":            "value",
		"pointer to non-struct": new(string),
	}

	for name, config := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.NotPanics(t, func() {
				err := processfields.WalkFields(config, func(_ string, _ reflect.StructField, _ reflect.Value, _ processfields.SetValFunc) error {
					return nil
				})
				require.Error(t, err)
			})
		})
	}
}

func TestWalkFieldsReturnsErrorForNilCallback(t *testing.T) {
	t.Parallel()

	err := processfields.WalkFields(&struct{ Value string }{}, nil)

	require.ErrorContains(t, err, "callback")
}

func TestWalkFieldsTraversesNestedStructsAndSkipsUnexportedFields(t *testing.T) {
	t.Parallel()

	type nestedConfig struct {
		Enabled bool
	}
	type config struct {
		Name          string
		Nested        nestedConfig
		hidden        string
		internalValue string
	}

	cfg := &config{}
	var visited []string
	err := processfields.WalkFields(cfg, func(_ string, field reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		visited = append(visited, field.Name)
		switch field.Name {
		case "Name":
			return setter("radix")
		case "Enabled":
			return setter("true")
		default:
			return nil
		}
	})

	require.NoError(t, err)
	assert.Equal(t, "radix", cfg.Name)
	assert.True(t, cfg.Nested.Enabled)
	assert.Empty(t, cfg.hidden)
	assert.Empty(t, cfg.internalValue)
	assert.ElementsMatch(t, []string{"Name", "Nested", "Enabled"}, visited)
}

func TestWalkFieldsVisitsNestedStructs(t *testing.T) {
	t.Parallel()

	type nestedConfig struct {
		Enabled bool
	}
	type config struct {
		Nested nestedConfig `required:"true"`
	}

	err := processfields.WalkFields(&config{}, func(path string, field reflect.StructField, value reflect.Value, _ processfields.SetValFunc) error {
		if field.Tag.Get("required") == "true" && value.IsZero() {
			return errors.New(path + " is required")
		}
		return nil
	})

	require.EqualError(t, err, "Nested is required")
}

func TestWalkFieldsVisitsFieldsInDeclarationOrder(t *testing.T) {
	t.Parallel()

	type inner struct {
		Second string
		Third  string
	}
	type config struct {
		First  string
		Inner  inner
		Fourth string
	}

	visited, err := visitAll(&config{})

	require.NoError(t, err)
	assert.Equal(t, []string{"First", "Second", "Third", "Fourth"}, visited)
}

func TestWalkFieldsPassesFieldPathToCallback(t *testing.T) {
	t.Parallel()

	type inner struct {
		Level string
	}
	type section struct {
		Threads int
	}
	type config struct {
		ExportedBase
		Name     string
		Operator inner
		Optional *inner
		Sections []section
	}

	paths, err := visitAllPaths(&config{Sections: []section{{}, {}}})

	require.NoError(t, err)
	assert.Equal(t, []string{
		"Level",
		"Name",
		"Operator.Level",
		"Optional.Level",
		"Sections[0].Threads",
		"Sections[1].Threads",
	}, paths)
}

func TestWalkFieldsTraversesDeeplyNestedStructs(t *testing.T) {
	t.Parallel()

	type level3 struct {
		Value string
	}
	type level2 struct {
		Level3 level3
	}
	type level1 struct {
		Level2 level2
	}
	type config struct {
		Level1 level1
	}

	cfg := &config{}
	err := setAll(cfg, "radix")

	require.NoError(t, err)
	assert.Equal(t, "radix", cfg.Level1.Level2.Level3.Value)
}

func TestWalkFieldsFlattensEmbeddedStructs(t *testing.T) {
	t.Parallel()

	visited, err := visitAll(&embeddedExportedConfig{})

	require.NoError(t, err)
	assert.Equal(t, []string{"Level", "Name"}, visited)
}

// Exported fields of an embedded unexported struct are settable through reflect,
// so they must be traversed like any other embedded struct.
func TestWalkFieldsFlattensEmbeddedUnexportedStructs(t *testing.T) {
	t.Parallel()

	cfg := &embeddedUnexportedConfig{}
	visited, err := visitAll(cfg)

	require.NoError(t, err)
	assert.Equal(t, []string{"Level", "Name"}, visited)

	require.NoError(t, setAll(cfg, "radix"))
	assert.Equal(t, "radix", cfg.Level)
	assert.Equal(t, "radix", cfg.Name)
}

func TestWalkFieldsFlattensEmbeddedPointerStructs(t *testing.T) {
	t.Parallel()

	cfg := &embeddedPointerConfig{ExportedBase: &ExportedBase{}}
	visited, err := visitAll(cfg)

	require.NoError(t, err)
	assert.Equal(t, []string{"Level", "Name"}, visited)
}

func TestWalkFieldsDoesNotInterpretFieldTags(t *testing.T) {
	t.Parallel()

	cfg := &struct {
		Required string `required:"true"`
	}{}
	callbackCalled := false

	err := processfields.WalkFields(cfg, func(_ string, _ reflect.StructField, _ reflect.Value, _ processfields.SetValFunc) error {
		callbackCalled = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, callbackCalled)
}

func TestWalkFieldsReturnsCallbackError(t *testing.T) {
	t.Parallel()

	expectedError := errors.New("callback failed")
	cfg := &struct{ Value string }{}

	err := processfields.WalkFields(cfg, func(_ string, _ reflect.StructField, _ reflect.Value, _ processfields.SetValFunc) error {
		return expectedError
	})

	require.ErrorIs(t, err, expectedError)
}

func TestWalkFieldsStopsOnFirstCallbackError(t *testing.T) {
	t.Parallel()

	type config struct {
		First  string
		Second string
	}

	var visited []string
	err := processfields.WalkFields(&config{}, func(_ string, field reflect.StructField, _ reflect.Value, _ processfields.SetValFunc) error {
		visited = append(visited, field.Name)
		return errors.New("callback failed")
	})

	require.Error(t, err)
	assert.Equal(t, []string{"First"}, visited)
}

// The setter closes over the field, so it stays usable after the walk completes.
func TestWalkFieldsSetterRemainsValidAfterWalk(t *testing.T) {
	t.Parallel()

	cfg := &struct{ Value string }{}
	var captured processfields.SetValFunc

	err := processfields.WalkFields(cfg, func(_ string, _ reflect.StructField, _ reflect.Value, setter processfields.SetValFunc) error {
		captured = setter
		return nil
	})

	require.NoError(t, err)
	require.NotNil(t, captured)
	require.NoError(t, captured("radix"))
	assert.Equal(t, "radix", cfg.Value)
}

// A self-referential type has no finite field set, so it must be rejected instead of
// recursing until the process runs out of memory.
func TestWalkFieldsRejectsRecursiveTypes(t *testing.T) {
	t.Parallel()

	type node struct {
		Name string
		Next *node
	}

	testCases := map[string]*node{
		"nil self reference":       {},
		"populated self reference": {Next: &node{}},
	}

	for name, cfg := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := visitAll(cfg)

			require.ErrorContains(t, err, "recursive type")
		})
	}
}

// Two fields of the same type are not a cycle, so both must still be traversed.
func TestWalkFieldsTraversesRepeatedStructTypes(t *testing.T) {
	t.Parallel()

	type section struct {
		Level string
	}
	type config struct {
		Operator  section
		ApiServer section
	}

	visited, err := visitAll(&config{})

	require.NoError(t, err)
	assert.Equal(t, []string{"Level", "Level"}, visited)
}

// A list of sections is an ordinary config shape, so every element must be traversed
// like any other nested struct.
func TestWalkFieldsTraversesListOfStructs(t *testing.T) {
	t.Parallel()

	type section struct {
		Name  string
		Level string
	}
	type config struct {
		Sections []section
	}

	cfg := &config{Sections: []section{{}, {}}}
	visited, err := visitAll(cfg)

	require.NoError(t, err)
	assert.Equal(t, []string{"Name", "Level", "Name", "Level"}, visited)

	require.NoError(t, setAll(cfg, "radix"))
	assert.Equal(t, []section{
		{Name: "radix", Level: "radix"},
		{Name: "radix", Level: "radix"},
	}, cfg.Sections)
}

func TestWalkFieldsTraversesListOfStructPointers(t *testing.T) {
	t.Parallel()

	type section struct {
		Level string
	}
	type config struct {
		Sections []*section
	}

	cfg := &config{Sections: []*section{{}, nil}}

	require.NoError(t, setAll(cfg, "radix"))
	require.Len(t, cfg.Sections, 2)
	assert.Equal(t, "radix", cfg.Sections[0].Level)
	require.NotNil(t, cfg.Sections[1])
	assert.Equal(t, "radix", cfg.Sections[1].Level)
}

// Elements have no field name of their own, so the index is the only thing that tells
// two failing sections apart.
func TestWalkFieldsListErrorIdentifiesElementIndex(t *testing.T) {
	t.Parallel()

	type section struct {
		Threads int
	}
	type config struct {
		Sections []section
	}

	err := setAll(&config{Sections: []section{{}, {}}}, "not-an-int")

	require.Error(t, err)
	assert.ErrorContains(t, err, "Sections[0].Threads")
}

func TestWalkFieldsVisitsNothingForEmptyListOfStructs(t *testing.T) {
	t.Parallel()

	type section struct {
		Level string
	}
	type config struct {
		Empty []section
		Nil   []section
	}

	visited, err := visitAll(&config{Empty: []section{}})

	require.NoError(t, err)
	assert.Empty(t, visited)
}

type ExportedBase struct {
	Level string
}

type unexportedBase struct {
	Level string
}

type embeddedExportedConfig struct {
	ExportedBase
	Name string
}

type embeddedUnexportedConfig struct {
	unexportedBase
	Name string
}

type embeddedPointerConfig struct {
	*ExportedBase
	Name string
}
