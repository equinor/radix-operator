package predicate

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_IsActiveRadixDeployment(t *testing.T) {
	assert.True(t, IsActiveRadixDeployment(radixv1.RadixDeployment{Status: radixv1.RadixDeployStatus{Condition: radixv1.DeploymentActive}}))
	assert.False(t, IsActiveRadixDeployment(radixv1.RadixDeployment{Status: radixv1.RadixDeployStatus{Condition: radixv1.DeploymentInactive}}))
	assert.False(t, IsActiveRadixDeployment(radixv1.RadixDeployment{}))
}

func Test_IsNotOrphanEnvironment(t *testing.T) {
	assert.True(t, IsNotOrphanEnvironment(radixv1.RadixEnvironment{}))
	assert.True(t, IsNotOrphanEnvironment(radixv1.RadixEnvironment{Status: radixv1.RadixEnvironmentStatus{Orphaned: false, OrphanedTimestamp: nil}}))
	assert.False(t, IsNotOrphanEnvironment(radixv1.RadixEnvironment{Status: radixv1.RadixEnvironmentStatus{Orphaned: true, OrphanedTimestamp: pointers.Ptr(metav1.Now())}}))
}

func Test_IsOrphanEnvironment(t *testing.T) {
	assert.True(t, IsOrphanEnvironment(radixv1.RadixEnvironment{Status: radixv1.RadixEnvironmentStatus{Orphaned: true, OrphanedTimestamp: pointers.Ptr(metav1.Now())}}))
	assert.False(t, IsOrphanEnvironment(radixv1.RadixEnvironment{}))
	assert.False(t, IsOrphanEnvironment(radixv1.RadixEnvironment{Status: radixv1.RadixEnvironmentStatus{Orphaned: false, OrphanedTimestamp: nil}}))
}

func Test_IsBatchJobStatusForBatchJob(t *testing.T) {
	sut := IsBatchJobStatusForBatchJob(radixv1.RadixBatchJob{Name: "jobname"})
	assert.True(t, sut(radixv1.RadixBatchJobStatus{Name: "jobname"}))
	assert.False(t, sut(radixv1.RadixBatchJobStatus{Name: "otherjobname"}))
}

func Test_IsBatchJobWithName(t *testing.T) {
	sut := IsBatchJobWithName("jobname")
	assert.True(t, sut(radixv1.RadixBatchJob{Name: "jobname"}))
	assert.False(t, sut(radixv1.RadixBatchJob{Name: "otherjobname"}))
}

func Test_IsRadixDeploymentForRadixBatch(t *testing.T) {
	batch := &radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{Namespace: "batchns"},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: "deployname"},
			},
		},
	}
	sut := IsRadixDeploymentForRadixBatch(batch)
	assert.True(t, sut(radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{
		Name:      "deployname",
		Namespace: "batchns",
	}}))
	assert.False(t, sut(radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{
		Name:      "otherdeployname",
		Namespace: "batchns",
	}}))
	assert.False(t, sut(radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{
		Name:      "deployname",
		Namespace: "otherbatchns",
	}}))

	sut = IsRadixDeploymentForRadixBatch(nil)
	assert.False(t, sut(radixv1.RadixDeployment{ObjectMeta: metav1.ObjectMeta{
		Name:      "anydeployname",
		Namespace: "anybatchns",
	}}))
}
