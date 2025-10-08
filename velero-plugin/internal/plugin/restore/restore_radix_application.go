package restore

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/runtime"
)

// RestoreRadixApplicationPlugin is a restore item action plugin for Velero
type RestoreRadixApplicationPlugin struct {
	Log  logrus.FieldLogger
	Kube *kube.Kube
}

// AppliesTo returns information about which resources this action should be invoked for.
// A RestoreItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.
func (p *RestoreRadixApplicationPlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"radixapplications.radix.equinor.com"},
	}, nil
}

// Execute allows the RestorePlugin to perform arbitrary logic with the item being restored,
// in this case, setting a custom annotation on the item being restored.
func (p *RestoreRadixApplicationPlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	var ra radixv1.RadixApplication
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.ItemFromBackup.UnstructuredContent(), &ra); err != nil {
		return nil, fmt.Errorf("unable to convert unstructured item to RadixApplication: %w", err)
	}

	logger := getLoggerForObject(p.Log, &ra)
	logger.Info("Validating restore action")

	rrExists, err := radixRegistrationExists(context.TODO(), p.Kube, ra.Name)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{}, err
	}
	if rrExists {
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	logger.Infof("RadixRegistration '%s' does not exist", ra.Name)
	return &velero.RestoreItemActionExecuteOutput{
		SkipRestore: true,
	}, nil
}
