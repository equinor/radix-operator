package utils_test

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func Test_GetArchitectureFromRuntime(t *testing.T) {
	assert.Equal(t, defaults.DefaultNodeSelectorArchitecture, utils.GetArchitectureFromRuntime(nil), "use default architecture when runtime is nil")
	assert.Equal(t, defaults.DefaultNodeSelectorArchitecture, utils.GetArchitectureFromRuntime(&v1.Runtime{Architecture: ""}), "use default architecture when runtime.architecture is empty")
	assert.Equal(t, "customarch", utils.GetArchitectureFromRuntime(&v1.Runtime{Architecture: "customarch"}), "use when runtime.architecture set")
}
