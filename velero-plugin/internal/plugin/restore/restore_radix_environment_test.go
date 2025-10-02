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
	unsturctured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type restoreRadixEnvironmentPluginTest struct {
	suite.Suite
	kubeClient  *fake.Clientset
	radixClient *fakeradix.Clientset
	kubeUtil    *kube.Kube
}

func TestRestoreRadixEnvironmentPlugin(t *testing.T) {
	suite.Run(t, new(restoreRadixEnvironmentPluginTest))
}

func (s *restoreRadixEnvironmentPluginTest) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil, nil)
}

func (s *restoreRadixEnvironmentPluginTest) Test_AppliesTo() {
	expected := velero.ResourceSelector{IncludedResources: []string{"radixenvironments.radix.equinor.com"}}
	plugin := restore.RestoreRadixEnvironmentPlugin{}
	actual, err := plugin.AppliesTo()
	s.NoError(err)
	s.Equal(expected, actual)
}

func (s *restoreRadixEnvironmentPluginTest) Test_Execute_RadixClientError() {
	getError := errors.New("any error")
	s.radixClient.PrependReactor("get", "radixregistrations", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, getError

	})

	source := radixv1.RadixEnvironment{}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unsturctured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unsturctured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreRadixEnvironmentPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	_, err = plugin.Execute(input)
	s.ErrorIs(err, getError)
}

func (s *restoreRadixEnvironmentPluginTest) Test_Execute_RadixRegistrationMissing() {
	source := radixv1.RadixEnvironment{
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

	plugin := restore.RestoreRadixEnvironmentPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.True(output.SkipRestore)
}

func (s *restoreRadixEnvironmentPluginTest) Test_Execute_RadixRegistrationExist() {
	const appName = "any-app"
	rr := &radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{
			Name: appName,
		},
	}
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, v1.CreateOptions{})
	s.Require().NoError(err)
	source := radixv1.RadixEnvironment{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
				"label-foo":        "label-bar",
			},
			Annotations: map[string]string{"annotation-foo": "annotation-bar"},
		},
		Spec: radixv1.RadixEnvironmentSpec{
			AppName: "foo",
			EnvName: "bar",
			Egress: radixv1.EgressConfig{
				Rules: []radixv1.EgressRule{
					{Ports: []radixv1.EgressPort{{Port: 8080}}, Destinations: []radixv1.EgressDestination{"0.0.0.0"}},
				},
			},
		},
	}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unsturctured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unsturctured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreRadixEnvironmentPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.False(output.SkipRestore)
	var outputSource radixv1.RadixEnvironment
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), &outputSource)
	s.Require().NoError(err)
	s.Equal(source, outputSource)
}
