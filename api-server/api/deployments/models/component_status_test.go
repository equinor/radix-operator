package models_test

import (
	"testing"
	"time"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/api-server/api/deployments/models"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNoKubeDeployments_IsReconciling(t *testing.T) {
	status := models.ComponentStatusFromDeployment(&radixv1.RadixDeployComponent{}, nil, nil)
	assert.Equal(t, models.ComponentReconciling, status)
}

func TestKubeDeploymentsWithRestartLabel_IsRestarting(t *testing.T) {
	status := models.ComponentStatusFromDeployment(
		&radixv1.RadixDeployComponent{EnvironmentVariables: map[string]string{operatordefaults.RadixRestartEnvironmentVariable: radixutils.FormatTimestamp(time.Now())}},
		createKubeDeployment(0),
		&radixv1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 2},
			Status:     radixv1.RadixDeployStatus{Reconciled: metav1.NewTime(time.Now().Add(-10 * time.Minute))},
		})

	assert.Equal(t, models.ComponentRestarting, status)
}

func TestKubeDeploymentsWithoutReplicas_IsStopped(t *testing.T) {
	status := models.ComponentStatusFromDeployment(
		&radixv1.RadixDeployComponent{},
		createKubeDeployment(0),
		&radixv1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		})
	assert.Equal(t, models.StoppedComponent, status)
}

func TestKubeDeployment_IsConsistent(t *testing.T) {
	status := models.ComponentStatusFromDeployment(
		&radixv1.RadixDeployComponent{},
		createKubeDeployment(1),
		&radixv1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		})
	assert.Equal(t, models.ConsistentComponent, status)
}

func TestKubeDeployment_IsOutdated(t *testing.T) {
	status := models.ComponentStatusFromDeployment(
		&radixv1.RadixDeployComponent{},
		createKubeDeployment(1),
		&radixv1.RadixDeployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 2},
		})
	assert.Equal(t, models.ComponentOutdated, status)
}

func createKubeDeployment(replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "helloworld",
			Annotations:     map[string]string{kube.RadixDeploymentObservedGeneration: "1"},
			OwnerReferences: []metav1.OwnerReference{{Controller: pointers.Ptr(true)}}},
		Spec: appsv1.DeploymentSpec{Replicas: pointers.Ptr[int32](replicas)},
	}
}
