package internal

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
)

// ComponentAttributesProvider handles component attributes configuration
type ComponentAttributesProvider struct {
	PipelineInfo *model.PipelineInfo
}

// IsEnabledForEnvironment checks if component attributes mutations are enabled for the specified environment
func (p *ComponentAttributesProvider) IsEnabledForEnvironment(envName string, _ radixv1.RadixApplication, _ radixv1.RadixDeployment) bool {
	return p.PipelineInfo.PipelineArguments.ToEnvironment == envName
}

// Mutate mutates component attributes
func (p *ComponentAttributesProvider) Mutate(source, target *radixv1.RadixDeployment) error {
	target.Name = source.GetName()
	target.ObjectMeta.Labels[kube.RadixJobNameLabel] = source.GetLabels()[kube.RadixJobNameLabel]

	if err := internal.MergeRadixDeploymentWithRadixApplicationAttributes(p.PipelineInfo.RadixApplication, target, target, p.PipelineInfo.PipelineArguments.ToEnvironment, p.PipelineInfo.DeployEnvironmentComponentImages[p.PipelineInfo.PipelineArguments.ToEnvironment]); err != nil {
		return err
	}
	return radixvalidators.CanRadixDeploymentBeInserted(target)
}
