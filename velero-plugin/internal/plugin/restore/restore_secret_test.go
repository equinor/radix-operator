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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type restoreSecretPluginTest struct {
	suite.Suite
	kubeClient  *fake.Clientset
	radixClient *fakeradix.Clientset
	kubeUtil    *kube.Kube
}

func TestRestoreSecretPlugin(t *testing.T) {
	suite.Run(t, new(restoreSecretPluginTest))
}

func (s *restoreSecretPluginTest) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil, nil)
}

func (s *restoreSecretPluginTest) Test_AppliesTo() {
	expected := velero.ResourceSelector{IncludedResources: []string{"secrets"}}
	plugin := restore.RestoreSecretPlugin{}
	actual, err := plugin.AppliesTo()
	s.NoError(err)
	s.Equal(expected, actual)
}

func (s *restoreSecretPluginTest) Test_Execute_RadixAppLabelNotSet() {
	source := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"label-foo": "label-bar"},
			Annotations: map[string]string{"annotation-foo": "annotation-bar"},
		},
		Data: map[string][]byte{"foo": []byte("bar")},
	}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unstructured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unstructured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreSecretPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.False(output.SkipRestore)
	var outputSource corev1.Secret
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), &outputSource)
	s.Require().NoError(err)
	s.Equal(source, outputSource)
}

func (s *restoreSecretPluginTest) Test_Execute_RadixClientError() {
	getError := errors.New("any error")
	s.radixClient.PrependReactor("get", "radixregistrations", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, getError

	})

	source := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
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

	plugin := restore.RestoreSecretPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	_, err = plugin.Execute(input)
	s.ErrorIs(err, getError)
}

func (s *restoreSecretPluginTest) Test_Execute_RadixRegistrationMissing() {
	source := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
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

	plugin := restore.RestoreSecretPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.True(output.SkipRestore)
}

func (s *restoreSecretPluginTest) Test_Execute_RadixRegistrationExist() {
	const appName = "any-app"
	rr := &radixv1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
		},
	}
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	s.Require().NoError(err)
	source := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
				"label-foo":        "label-bar",
			},
			Annotations: map[string]string{"annotation-foo": "annotation-bar"},
		},
		Data: map[string][]byte{"foo": []byte("bar")},
	}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unstructured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unstructured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreSecretPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.False(output.SkipRestore)
	var outputSource corev1.Secret
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), &outputSource)
	s.Require().NoError(err)
	s.Equal(source, outputSource)
}
