package runtime_test

import (
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/runtime"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_GetArchitectureFromRuntime(t *testing.T) {
	nodeArch, ok := runtime.GetArchitectureFromRuntime(nil)
	assert.Equal(t, defaults.DefaultNodeSelectorArchitecture, nodeArch, "use default architecture when runtime is nil")
	assert.False(t, ok, "use default architecture when runtime is nil")
	nodeArch, ok = runtime.GetArchitectureFromRuntime(&v1.Runtime{Architecture: ""})
	assert.Equal(t, defaults.DefaultNodeSelectorArchitecture, nodeArch, "use default architecture when runtime.architecture is empty")
	assert.False(t, ok, "use default architecture when runtime.architecture is empty")
	nodeArch, ok = runtime.GetArchitectureFromRuntime(&v1.Runtime{Architecture: "customarch"})
	assert.Equal(t, "customarch", nodeArch, "use when runtime.architecture set")
	assert.Truef(t, ok, "use when runtime.architecture set")
	nodeArch, ok = runtime.GetArchitectureFromRuntime(&v1.Runtime{NodeType: pointers.Ptr("memory-optimized-v1")})
	assert.Equal(t, string(v1.RuntimeArchitectureAmd64), nodeArch, "use when runtime.nodeType set")
	assert.True(t, ok, "use when runtime.nodeType set")
	nodeArch, ok = runtime.GetArchitectureFromRuntime(&v1.Runtime{NodeType: pointers.Ptr("gpu-a100-v1")})
	assert.Equal(t, string(v1.RuntimeArchitectureAmd64), nodeArch, "use when runtime.nodeType set")
	assert.True(t, ok, "use when runtime.nodeType set")
}
