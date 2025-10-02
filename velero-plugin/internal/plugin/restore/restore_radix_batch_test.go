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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type restoreRadixBatchPluginTest struct {
	suite.Suite
	kubeClient  *fake.Clientset
	radixClient *fakeradix.Clientset
	kubeUtil    *kube.Kube
}

func TestRestoreRadixBatchPlugin(t *testing.T) {
	suite.Run(t, new(restoreRadixBatchPluginTest))
}

func (s *restoreRadixBatchPluginTest) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, nil, nil)
}

func (s *restoreRadixBatchPluginTest) Test_AppliesTo() {
	expected := velero.ResourceSelector{IncludedResources: []string{"radixbatches.radix.equinor.com"}}
	plugin := restore.RestoreRadixBatchPlugin{}
	actual, err := plugin.AppliesTo()
	s.NoError(err)
	s.Equal(expected, actual)
}

func (s *restoreRadixBatchPluginTest) Test_Execute_RadixClientError() {
	getError := errors.New("any error")
	s.radixClient.PrependReactor("get", "radixregistrations", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, getError

	})

	source := radixv1.RadixBatch{}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unstructured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unstructured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreRadixBatchPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	_, err = plugin.Execute(input)
	s.ErrorIs(err, getError)
}

func (s *restoreRadixBatchPluginTest) Test_Execute_RadixRegistrationMissing() {
	source := radixv1.RadixBatch{
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

	plugin := restore.RestoreRadixBatchPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.True(output.SkipRestore)
}

func (s *restoreRadixBatchPluginTest) Test_Execute_RadixRegistrationExist() {
	const appName = "any-app"
	rr := &radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{
			Name: appName,
		},
	}
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, v1.CreateOptions{})
	s.Require().NoError(err)
	source := radixv1.RadixBatch{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
				"label-foo":        "label-bar",
			},
			Annotations: map[string]string{"annotation-foo": "annotation-bar"},
		},
		Spec: radixv1.RadixBatchSpec{
			BatchId: "any-id",
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				Job: "any-job",
			},
			Jobs: []radixv1.RadixBatchJob{
				{
					Name:  "any-name",
					JobId: "any-job-id",
				},
			},
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{
				Type:       radixv1.BatchConditionTypeActive,
				Reason:     "any-reason",
				Message:    "any-message",
				ActiveTime: &v1.Time{Time: time.Now()},
			},
			JobStatuses: []radixv1.RadixBatchJobStatus{
				{
					Name:  "any-name",
					Phase: radixv1.BatchJobPhaseActive,
					RadixBatchJobPodStatuses: []radixv1.RadixBatchJobPodStatus{
						{
							Name:  "pod-name",
							Phase: radixv1.PodPending,
						},
					},
				},
			},
		},
	}
	sourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&source)
	s.Require().NoError(err)
	input := &velero.RestoreItemActionExecuteInput{
		Item:           &unstructured.Unstructured{Object: sourceUnstructured},
		ItemFromBackup: &unstructured.Unstructured{Object: sourceUnstructured},
	}

	plugin := restore.RestoreRadixBatchPlugin{
		Log:  logrus.New(),
		Kube: s.kubeUtil,
	}

	output, err := plugin.Execute(input)
	s.Require().NoError(err)
	s.False(output.SkipRestore)
	var outputSource radixv1.RadixBatch
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(output.UpdatedItem.UnstructuredContent(), &outputSource)
	s.Require().NoError(err)
	expectedObjectMeta := source.ObjectMeta.DeepCopy()
	expectedRestoreAnnotationValue, err := json.Marshal(source.Status)
	s.Require().NoError(err)
	expectedObjectMeta.Annotations[kube.RestoredStatusAnnotation] = string(expectedRestoreAnnotationValue)
	s.Equal(source.Spec, outputSource.Spec)
	s.Equal(*expectedObjectMeta, outputSource.ObjectMeta)
}
