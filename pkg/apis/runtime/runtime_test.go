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
	assert.Equal(t, defaults.DefaultNodeSelectorArchitecture, runtime.GetArchitectureFromRuntime(nil), "use default architecture when runtime is nil")
	assert.Equal(t, defaults.DefaultNodeSelectorArchitecture, runtime.GetArchitectureFromRuntime(&v1.Runtime{Architecture: ""}), "use default architecture when runtime.architecture is empty")
	assert.Equal(t, "customarch", runtime.GetArchitectureFromRuntime(&v1.Runtime{Architecture: "customarch"}), "use when runtime.architecture set")
	assert.Equal(t, string(v1.RuntimeArchitectureAmd64), runtime.GetArchitectureFromRuntime(&v1.Runtime{NodeType: pointers.Ptr(string(runtime.HighMemoryV1Node))}), "use when runtime.nodeType set")
	assert.Equal(t, string(v1.RuntimeArchitectureAmd64), runtime.GetArchitectureFromRuntime(&v1.Runtime{NodeType: pointers.Ptr(string(runtime.GpuV1))}), "use when runtime.nodeType set")
}
