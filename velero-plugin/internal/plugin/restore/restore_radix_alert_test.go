package restore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/velero-plugin/internal/plugin/restore"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type restoreRadixAlertPluginTest struct {
	suite.Suite
	kubeClient  *fake.Clientset
	radixClient *fakeradix.Clientset
	kubeUtil    *kube.Kube
}

func TestRestoreRadixAlertPlugin(t *testing.T) {
	suite.Run(t, new(restoreRadixAlertPluginTest))
}

func (s *restoreRadixAlertPluginTest) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil, nil)
}

func (s *restoreRadixAlertPluginTest) Test_AppliesTo() {
	expected := velero.ResourceSelector{IncludedResources: []string{"radixalerts.radix.equinor.com"}}
	plugin := restore.RestoreAlertPlugin{}
	actual, err := plugin.AppliesTo()
	s.NoError(err)
	s.Equal(expected, actual)
}

func (s *restoreRadixAlertPluginTest) Test_Execute_RadixClientError() {
	getError := errors.New("any error")
	s.radixClient.PrependReactor("get", "radixregistrations", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, getError

	})

	source := radixv1.RadixAlert{}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unstructured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unstructured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreAlertPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	_, err = plugin.Execute(input)
	s.ErrorIs(err, getError)
}

func (s *restoreRadixAlertPluginTest) Test_Execute_RadixRegistrationMissing() {
	source := radixv1.RadixAlert{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixAppLabel: "any-app",
			},
		},
	}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unstructured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unstructured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreAlertPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.True(output.SkipRestore)
}

func (s *restoreRadixAlertPluginTest) Test_Execute_RadixRegistrationExist() {
	const appName = "any-app"
	rr := &radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{
			Name: appName,
		},
	}
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, v1.CreateOptions{})
	s.Require().NoError(err)
	source := radixv1.RadixAlert{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
				"label-foo":        "label-bar",
			},
			Annotations: map[string]string{"annotation-foo": "annotation-bar"},
		},
		Spec: radixv1.RadixAlertSpec{
			Alerts: []radixv1.Alert{
				{Alert: "foo", Receiver: "bar"},
			},
		},
	}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unstructured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unstructured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreAlertPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.False(output.SkipRestore)
	var outputSource radixv1.RadixAlert
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), &outputSource)
	s.Require().NoError(err)
	s.Equal(source, outputSource)
}
