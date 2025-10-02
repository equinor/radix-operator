package restore_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/velero-plugin/internal/plugin/restore"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unsturctured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type restoreRadixDeploymentPluginTest struct {
	suite.Suite
	kubeClient  *fake.Clientset
	radixClient *fakeradix.Clientset
	kubeUtil    *kube.Kube
}

func TestRestoreRadixDeploymentPlugin(t *testing.T) {
	suite.Run(t, new(restoreRadixDeploymentPluginTest))
}

func (s *restoreRadixDeploymentPluginTest) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil, nil)
}

func (s *restoreRadixDeploymentPluginTest) Test_AppliesTo() {
	expected := velero.ResourceSelector{IncludedResources: []string{"radixdeployments.radix.equinor.com"}}
	plugin := restore.RestoreRadixDeploymentPlugin{}
	actual, err := plugin.AppliesTo()
	s.NoError(err)
	s.Equal(expected, actual)
}

func (s *restoreRadixDeploymentPluginTest) Test_Execute_RadixClientError() {
	getError := errors.New("any error")
	s.radixClient.PrependReactor("get", "radixregistrations", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, getError

	})

	source := radixv1.RadixDeployment{}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unsturctured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unsturctured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreRadixDeploymentPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	_, err = plugin.Execute(input)
	s.ErrorIs(err, getError)
}

func (s *restoreRadixDeploymentPluginTest) Test_Execute_RadixRegistrationMissing() {
	source := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixAppLabel: "any-app",
			},
		},
	}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unsturctured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unsturctured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreRadixDeploymentPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.True(output.SkipRestore)
}

func (s *restoreRadixDeploymentPluginTest) Test_Execute_RadixRegistrationExist() {
	const appName = "any-app"
	rr := &radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{
			Name: appName,
		},
	}
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, v1.CreateOptions{})
	s.Require().NoError(err)
	source := radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
				"label-foo":        "label-bar",
			},
			Annotations: map[string]string{"annotation-foo": "annotation-bar"},
		},
		Spec: radixv1.RadixDeploymentSpec{
			Components: []radixv1.RadixDeployComponent{
				{Name: "any-comp"},
			},
			Jobs: []radixv1.RadixDeployJobComponent{
				{Name: "any-job"},
			},
		},
		Status: radixv1.RadixDeployStatus{
			ActiveFrom: v1.Date(2000, time.January, 1, 1, 1, 1, 1, time.Local),
			ActiveTo:   v1.Date(2001, time.January, 1, 1, 1, 1, 1, time.Local),
			Reconciled: v1.Date(2002, time.January, 1, 1, 1, 1, 1, time.Local),
			Condition:  radixv1.DeploymentActive,
		},
	}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unsturctured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unsturctured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreRadixDeploymentPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.False(output.SkipRestore)
	var outputSource radixv1.RadixDeployment
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), &outputSource)
	s.Require().NoError(err)
	expectedObjectMeta := source.ObjectMeta.DeepCopy()
	expectedRestoreAnnotationValue, err := json.Marshal(source.Status)
	s.Require().NoError(err)
	expectedObjectMeta.Annotations[kube.RestoredStatusAnnotation] = string(expectedRestoreAnnotationValue)
	s.Equal(source.Spec, outputSource.Spec)
	s.Equal(*expectedObjectMeta, outputSource.ObjectMeta)
}
