package restore

import (
	"context"
	"fmt"

	kube "github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/runtime"
)

// RestoreAlertPlugin is a restore item action plugin for Velero
type RestoreAlertPlugin struct {
	Log  logrus.FieldLogger
	Kube *kube.Kube
}

// AppliesTo returns information about which resources this action should be invoked for.
// A RestoreItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.g
func (p *RestoreAlertPlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"radixalerts.radix.equinor.com"},
	}, nil
}

// Execute allows the RestorePlugin to perform arbitrary logic with the item being restored,
// in this case, setting a custom annotation on the item being restored.
func (p *RestoreAlertPlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	var ra radixv1.RadixAlert

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.ItemFromBackup.UnstructuredContent(), &ra); err != nil {
		return nil, fmt.Errorf("unable to convert unstructured item to RadixAlert: %w", err)
	}

	logger := getLoggerForObject(p.Log, &ra)
	logger.Info("Validating restore action")

	radixAppName := ra.Labels[kube.RadixAppLabel]
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
