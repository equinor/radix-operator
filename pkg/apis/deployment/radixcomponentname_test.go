package deployment

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ComponentNameExistsInRD(t *testing.T) {
	rd := utils.ARadixDeployment().
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("component")).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("job")).
		BuildRD()

	componentName := RadixComponentName("component")
	assert.True(t, componentName.ExistInDeploymentSpec(rd))
	assert.True(t, componentName.ExistInDeploymentSpecComponentList(rd))
	assert.False(t, componentName.ExistInDeploymentSpecJobList(rd))

	jobName := RadixComponentName("job")
	assert.True(t, jobName.ExistInDeploymentSpec(rd))
	assert.False(t, jobName.ExistInDeploymentSpecComponentList(rd))
	assert.True(t, jobName.ExistInDeploymentSpecJobList(rd))

	nonExistingName := RadixComponentName("nonexisting")
	assert.False(t, nonExistingName.ExistInDeploymentSpec(rd))
	assert.False(t, nonExistingName.ExistInDeploymentSpecComponentList(rd))
	assert.False(t, nonExistingName.ExistInDeploymentSpecJobList(rd))
}

func Test_FindCommonComponentInRD(t *testing.T) {
	rd := utils.ARadixDeployment().
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("component")).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("job")).
		BuildRD()

	componentName := RadixComponentName("component")
	comp := componentName.GetCommonDeployComponent(rd)
	assert.NotNil(t, comp)
	assert.Equal(t, "component", comp.GetName())

	jobName := RadixComponentName("job")
	job := jobName.GetCommonDeployComponent(rd)
	assert.NotNil(t, job)
	assert.Equal(t, "job", job.GetName())

	nonExistingName := RadixComponentName("nonexisting")
	nonExisting := nonExistingName.GetCommonDeployComponent(rd)
	assert.Nil(t, nonExisting)

}

func Test_CommonDeployComponentHasPorts(t *testing.T) {
	rd := utils.ARadixDeployment().
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("component1").
				WithPorts([]v1.ComponentPort{{Name: "http", Port: 8080}}),
			utils.NewDeployComponentBuilder().WithName("component2")).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("job1").
				WithPorts([]v1.ComponentPort{{Name: "http", Port: 8080}}).
				WithSchedulerPort(pointers.Ptr[int32](8081)),
			utils.NewDeployJobComponentBuilder().WithName("job2").
				WithPorts([]v1.ComponentPort{{Name: "http", Port: 8080}}),
			utils.NewDeployJobComponentBuilder().WithName("job3").
				WithSchedulerPort(pointers.Ptr[int32](8081)),
			utils.NewDeployJobComponentBuilder().WithName("job4"),
		).
		BuildRD()

	componentName := RadixComponentName("component1")
	hasPorts := componentName.CommonDeployComponentHasPorts(rd)
	assert.True(t, hasPorts)

	componentName = RadixComponentName("component2")
	hasPorts = componentName.CommonDeployComponentHasPorts(rd)
	assert.False(t, hasPorts)

	jobName := RadixComponentName("job1")
	hasPorts = jobName.CommonDeployComponentHasPorts(rd)
	assert.True(t, hasPorts)

	jobName = RadixComponentName("job2")
	hasPorts = jobName.CommonDeployComponentHasPorts(rd)
	assert.True(t, hasPorts)

	jobName = RadixComponentName("job3")
	hasPorts = jobName.CommonDeployComponentHasPorts(rd)
	assert.True(t, hasPorts)

	jobName = RadixComponentName("job4")
	hasPorts = jobName.CommonDeployComponentHasPorts(rd)
	assert.False(t, hasPorts)

	nonExistingName := RadixComponentName("nonexisting")
	hasPorts = nonExistingName.CommonDeployComponentHasPorts(rd)
	assert.False(t, hasPorts)

}

func Test_NewRadixComponentNameFromLabels(t *testing.T) {
	nonRadix := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"anylabel": "component",
			},
		},
	}

	radixLabelled := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixComponentLabel: "component",
			},
		},
	}

	radixAuxLabelled := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixAuxiliaryComponentLabel: "component",
			},
		},
	}

	name, ok := RadixComponentNameFromComponentLabel(radixLabelled)
	assert.True(t, ok)
	assert.Equal(t, RadixComponentName("component"), name)

	name, ok = RadixComponentNameFromAuxComponentLabel(radixAuxLabelled)
	assert.True(t, ok)
	assert.Equal(t, RadixComponentName("component"), name)

	name, ok = RadixComponentNameFromComponentLabel(nonRadix)
	assert.False(t, ok)
	assert.Equal(t, RadixComponentName(""), name)
}
