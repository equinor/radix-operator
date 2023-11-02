package radixvalidators_test

import (
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func Test_valid_rd_returns_true(t *testing.T) {
	validRD := createValidRD()
	err := radixvalidators.CanRadixDeploymentBeInserted(validRD)

	assert.NoError(t, err)
}

type updateRDFunc func(rr *v1.RadixDeployment)

func Test_invalid_rd_returns_false(t *testing.T) {
	var testScenarios = []struct {
		name     string
		updateRD updateRDFunc
	}{
		{"to long app name", func(rd *v1.RadixDeployment) {
			rd.Name = "way.toooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo.long-app-name"
		}},
		{"invalid app name", func(rd *v1.RadixDeployment) {
			rd.Name = "invalid,char.appname"
		}},
		{"empty app name", func(rd *v1.RadixDeployment) {
			rd.Name = ""
		}},
		{"invalid nr replicas", func(rd *v1.RadixDeployment) {
			*rd.Spec.Components[0].Replicas = -1
		}},
		{"invalid nr replicas", func(rd *v1.RadixDeployment) {
			*rd.Spec.Components[0].Replicas = 200
		}},
		{"invalid component name", func(rd *v1.RadixDeployment) {
			rd.Spec.Components[0].Name = "invalid,char.appname"
		}},
		{"invalid env name", func(rd *v1.RadixDeployment) {
			rd.Spec.Environment = "invalid,char.appname"
		}},
		{"invalid hpa config minReplicas and maxReplicas are not set", func(rd *v1.RadixDeployment) {
			rd.Spec.Components[0].HorizontalScaling = &v1.RadixHorizontalScaling{}
		}},
		{"invalid hpa config maxReplicas is not set and minReplicas is set", func(rd *v1.RadixDeployment) {
			minReplica := int32(3)
			rd.Spec.Components[0].HorizontalScaling = &v1.RadixHorizontalScaling{}
			rd.Spec.Components[0].HorizontalScaling.MinReplicas = &minReplica
		}},
		{"invalid job name", func(rd *v1.RadixDeployment) {
			rd.Spec.Jobs[0].Name = "invalid,char.appname"
		}},
		{"job scheduler port is not set", func(rd *v1.RadixDeployment) {
			rd.Spec.Jobs[0].SchedulerPort = nil
		}},
		{"job payload path is not set", func(rd *v1.RadixDeployment) {
			rd.Spec.Jobs[1].Payload.Path = ""
		}},
	}

	for _, testcase := range testScenarios {
		t.Run(testcase.name, func(t *testing.T) {
			validRD := createValidRD()
			testcase.updateRD(validRD)
			err := radixvalidators.CanRadixDeploymentBeInserted(validRD)

			assert.Error(t, err)
		})
	}
}

func Test_invalid_git_tags_returns_error(t *testing.T) {
	validGitTags := "v1.12 v1.13"
	err := radixvalidators.GitTagsContainIllegalChars(validGitTags)
	assert.NoError(t, err)

	invalidGitTags := "v1.12\" v1.13"
	err = radixvalidators.GitTagsContainIllegalChars(invalidGitTags)
	assert.Error(t, err)

	invalidGitTags2 := "v1.12' v1.13"
	err = radixvalidators.GitTagsContainIllegalChars(invalidGitTags2)
	assert.Error(t, err)
}

func createValidRD() *v1.RadixDeployment {
	validRD, _ := utils.GetRadixDeployFromFile("testdata/radixdeploy.yaml")

	return validRD
}

func validRDSetup() (kubernetes.Interface, radixclient.Interface) {
	validRR, _ := utils.GetRadixRegistrationFromFile("testdata/radixregistration.yaml")
	kubeclient := kubefake.NewSimpleClientset()
	client := radixfake.NewSimpleClientset(validRR)

	return kubeclient, client
}
