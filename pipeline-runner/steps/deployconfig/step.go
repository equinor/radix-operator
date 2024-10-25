package deployconfig

import (
	"context"
	"fmt"
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DeployConfigStepImplementation Step to deploy RD into environment
type DeployConfigStepImplementation struct {
	stepType               pipeline.StepType
	radixDeploymentWatcher watcher.RadixDeploymentWatcher
	model.DefaultStepImplementation
}

// NewDeployConfigStep Constructor
func NewDeployConfigStep(radixDeploymentWatcher watcher.RadixDeploymentWatcher) model.Step {
	return &DeployConfigStepImplementation{
		stepType:               pipeline.DeployConfigStep,
		radixDeploymentWatcher: radixDeploymentWatcher,
	}
}

// ImplementationForType Override of default step method
func (cli *DeployConfigStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *DeployConfigStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: deploy application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *DeployConfigStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to deploy application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *DeployConfigStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	var updaters []DeploymentUpdater
	if pipelineInfo.PipelineArguments.DeployConfigStep.DeployExternalDNS {
		updaters = append(updaters, &externalDNSDeployer{})
	}
	// if pipelineInfo.PipelineArguments.DeployConfigStep.DeployAppAlias {
	// 	updaters = append(updaters, nil)
	// }

	handler := deployHandler{updaters: updaters, pipelineInfo: *pipelineInfo, kubeutil: cli.GetKubeutil(), rdWatcher: cli.radixDeploymentWatcher}
	if err := handler.deploy(ctx); err != nil {
		return fmt.Errorf("failed to deploy config: %w", err)
	}

	return nil
}

type DeploymentUpdater interface {
	MustDeployEnvironment(envName string, ra *radixv1.RadixApplication, activeRd *radixv1.RadixDeployment) bool
	UpdateDeployment(target, source *radixv1.RadixDeployment) error
}

type externalDNSDeployer struct{}

func (d *externalDNSDeployer) MustDeployEnvironment(envName string, ra *radixv1.RadixApplication, activeRd *radixv1.RadixDeployment) bool {
	if slices.ContainsFunc(ra.Spec.DNSExternalAlias, func(alias radixv1.ExternalAlias) bool { return alias.Environment == envName }) {
		return true
	}

	if activeRd != nil && slices.ContainsFunc(activeRd.Spec.Components, func(comp radixv1.RadixDeployComponent) bool { return len(comp.GetExternalDNS()) > 0 }) {
		return true
	}

	return false
}

func (d *externalDNSDeployer) UpdateDeployment(target, source *radixv1.RadixDeployment) error {
	for i, targetComp := range target.Spec.Components {
		sourceComp, found := slice.FindFirst(source.Spec.Components, func(c radixv1.RadixDeployComponent) bool { return c.Name == targetComp.Name })
		if !found {
			return fmt.Errorf("component %s not found in active deployment", targetComp.Name)
		}
		target.Spec.Components[i].ExternalDNS = sourceComp.GetExternalDNS()
	}

	return nil
}
