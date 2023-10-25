package utils

import (
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/radix"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func Test_RadixEnvironment_Defaults(t *testing.T) {
	re := NewEnvironmentBuilder().BuildRE()

	parseMinimum, _ := time.Parse("YYYY-MM-DD hh:mm:ss", "0001-01-01 00:00:00")
	minTime := meta.Time{Time: parseMinimum}
	nilTime := (*meta.Time)(nil)

	// TypeMeta
	assert.Equal(t, radix.APIVersion, re.TypeMeta.APIVersion)
	assert.Equal(t, radix.KindRadixEnvironment, re.TypeMeta.Kind)

	// ObjectMeta
	assert.Len(t, re.ObjectMeta.Annotations, 0)
	assert.Equal(t, minTime, re.ObjectMeta.CreationTimestamp)
	assert.Equal(t, (*int64)(nil), re.ObjectMeta.DeletionGracePeriodSeconds)
	assert.Equal(t, nilTime, re.ObjectMeta.DeletionTimestamp)
	assert.Len(t, re.ObjectMeta.Finalizers, 0)
	assert.Equal(t, "", re.ObjectMeta.GenerateName)
	assert.Equal(t, int64(0), re.ObjectMeta.Generation)
	assert.Len(t, re.ObjectMeta.Labels, 0)
	assert.Len(t, re.ObjectMeta.ManagedFields, 0)
	assert.Equal(t, "", re.ObjectMeta.Name)
	assert.Equal(t, "", re.ObjectMeta.Namespace)
	assert.Len(t, re.ObjectMeta.OwnerReferences, 0)
	assert.Equal(t, "", re.ObjectMeta.ResourceVersion)
	assert.Equal(t, (types.UID)(""), re.ObjectMeta.UID)

	// Spec
	assert.Equal(t, "", re.Spec.AppName)
	assert.Equal(t, "", re.Spec.EnvName)

	// Status
	assert.True(t, re.Status.Orphaned)
	assert.Equal(t, minTime, re.Status.Reconciled)
}

// Test ObjectMeta manipulation

func Test_WithAppLabel(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName("myapp").
		BuildRE()

	assert.Len(t, re.ObjectMeta.Labels, 1)
	assert.Equal(t, "myapp", re.ObjectMeta.Labels["radix-app"])
}

func Test_WithLabel(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithLabel("key1", "value1").
		WithLabel("key2", "value2").
		BuildRE()

	assert.Len(t, re.ObjectMeta.Labels, 2)
	assert.Equal(t, "value1", re.ObjectMeta.Labels["key1"])
	assert.Equal(t, "value2", re.ObjectMeta.Labels["key2"])
}

func Test_WithAppName_WithEnvironmentName(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithAppName("prefix").
		WithEnvironmentName("suffix").
		BuildRE()

	assert.Equal(t, "prefix-suffix", re.ObjectMeta.Name)
}

func Test_WithCreatedTime(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithCreatedTime(time.Unix(12345678, 0).UTC()).
		BuildRE()

	assert.Equal(t, time.Unix(12345678, 0).UTC(), re.ObjectMeta.CreationTimestamp.Time)
}

func Test_WithOwner(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithOwner(meta.OwnerReference{
			Kind: "Test",
			Name: "TestOwner",
		}).
		BuildRE()

	assert.Len(t, re.ObjectMeta.OwnerReferences, 1)
	assert.Equal(t, "TestOwner", re.ObjectMeta.OwnerReferences[0].Name)
}

func Test_WithRegistrationOwner(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithRegistrationOwner(&v1.RadixRegistration{
			ObjectMeta: meta.ObjectMeta{
				Name: "RR",
			},
		}).
		BuildRE()

	assert.Len(t, re.ObjectMeta.OwnerReferences, 1)
	assert.Equal(t, "RR", re.ObjectMeta.OwnerReferences[0].Name)
	assert.Equal(t, radix.KindRadixRegistration, re.ObjectMeta.OwnerReferences[0].Kind)
}

func Test_WithRegistrationBuilder(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithRegistrationBuilder(NewRegistrationBuilder().
			WithName("RR")).
		BuildRE()

	assert.Len(t, re.ObjectMeta.OwnerReferences, 1)
	assert.Equal(t, "RR", re.ObjectMeta.OwnerReferences[0].Name)
	assert.Equal(t, radix.KindRadixRegistration, re.ObjectMeta.OwnerReferences[0].Kind)
}

func Test_WithResourceVersion(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithResourceVersion("v10.200.3000").
		BuildRE()

	assert.Equal(t, "v10.200.3000", re.ObjectMeta.ResourceVersion)
}

func Test_WithUID(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithUID("618334aa-1879-47c3-a4ea-e40a926b1bda").
		BuildRE()

	assert.Equal(t, (types.UID)("618334aa-1879-47c3-a4ea-e40a926b1bda"), re.ObjectMeta.UID)
}

// Test Status manipulation

func Test_WithReconciledTime(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithReconciledTime(time.Unix(12345678, 0).UTC()).
		BuildRE()

	assert.Equal(t, time.Unix(12345678, 0).UTC(), re.Status.Reconciled.Time)
}

func Test_WithOrphaned(t *testing.T) {
	re := NewEnvironmentBuilder().
		WithOrphaned(false).
		BuildRE()

	assert.False(t, re.Status.Orphaned)
}
