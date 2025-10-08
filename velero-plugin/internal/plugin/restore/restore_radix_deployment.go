package restore

import (
	"context"
	"encoding/json"
	"fmt"

	kube "github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// RestoreRadixDeploymentPlugin is a restore item action plugin for Velero
type RestoreRadixDeploymentPlugin struct {
	Log  logrus.FieldLogger
	Kube *kube.Kube
}

// AppliesTo returns information about which resources this action should be invoked for.
// A RestoreItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.
func (p *RestoreRadixDeploymentPlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"radixdeployments.radix.equinor.com"},
	}, nil
}

// Execute allows the RestorePlugin to perform arbitrary logic with the item being restored,
// in this case, setting a custom annotation on the item being restored.
func (p *RestoreRadixDeploymentPlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	metadata, err := meta.Accessor(input.Item)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{}, err
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	var rd radixv1.RadixDeployment
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.ItemFromBackup.UnstructuredContent(), &rd); err != nil {
		return nil, fmt.Errorf("unable to convert unstructured item to RadixDeployment: %w", err)
	}

	logger := getLoggerForObject(p.Log, &rd)
	logger.Info("Validating restore action")

	restoredStatus, err := json.Marshal(rd.Status)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{}, err
	}

	radixAppName := rd.Labels[kube.RadixAppLabel]
	rrExists, err := radixRegistrationExists(context.TODO(), p.Kube, radixAppName)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{}, err
	}
	if rrExists {
		logger.Infof("Setting annotation %s", kube.RestoredStatusAnnotation)
		annotations[kube.RestoredStatusAnnotation] = string(restoredStatus)
		metadata.SetAnnotations(annotations)
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	logger.Infof("RadixRegistration '%s' does not exist", radixAppName)
	return &velero.RestoreItemActionExecuteOutput{
		SkipRestore: true,
	}, nil
}
