package restore

import (
	"context"
	"fmt"

	kube "github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RestoreConfigMapPlugin is a restore item action plugin for Velero
type RestoreConfigMapPlugin struct {
	Log  logrus.FieldLogger
	Kube *kube.Kube
}

// AppliesTo returns information about which resources this action should be invoked for.
// A RestoreItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.g
func (p *RestoreConfigMapPlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"configmaps"},
	}, nil
}

// Execute allows the RestorePlugin to perform arbitrary logic with the item being restored,
// in this case, setting a custom annotation on the item being restored.
func (p *RestoreConfigMapPlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	var configMap corev1.ConfigMap
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.ItemFromBackup.UnstructuredContent(), &configMap); err != nil {
		return nil, fmt.Errorf("unable to convert unstructured item to ConfigMap: %w", err)
	}

	logger := getLoggerForObject(p.Log, &configMap)
	logger.Info("Validating restore action")

	radixAppName := configMap.Labels[kube.RadixAppLabel]
	if radixAppName == "" {
		logger.Info("radix-app label not set")
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	rrExists, err := radixRegistrationExists(context.TODO(), p.Kube, radixAppName)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{}, err
	}
	if rrExists {
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	logger.Infof("RadixRegistration '%s' does not exist", radixAppName)
	return &velero.RestoreItemActionExecuteOutput{
		SkipRestore: true,
	}, nil
}
