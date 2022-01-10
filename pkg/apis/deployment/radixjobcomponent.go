package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// JobComponentsBuilder builds RD job components for a given environment
type JobComponentsBuilder interface {
	JobComponents() []v1.RadixDeployJobComponent
}

type jobComponentsBuilder struct {
	ra              *v1.RadixApplication
	env             string
	componentImages map[string]pipeline.ComponentImage
}

// NewJobComponentsBuilder constructor for JobComponentsBuilder
func NewJobComponentsBuilder(ra *v1.RadixApplication, env string, componentImages map[string]pipeline.ComponentImage) JobComponentsBuilder {
	return &jobComponentsBuilder{
		ra:              ra,
		env:             env,
		componentImages: componentImages,
	}
}

func (c *jobComponentsBuilder) JobComponents() []v1.RadixDeployJobComponent {
	jobs := []v1.RadixDeployJobComponent{}

	for _, appJob := range c.ra.Spec.Jobs {
		deployJob := c.buildJobComponent(appJob)
		jobs = append(jobs, deployJob)
	}

	return jobs
}

func (c *jobComponentsBuilder) getEnvironmentConfig(appJob v1.RadixJobComponent) *v1.RadixJobComponentEnvironmentConfig {
	if appJob.EnvironmentConfig == nil {
		return nil
	}

	for _, environment := range appJob.EnvironmentConfig {
		if environment.Environment == c.env {
			return &environment
		}
	}
	return nil
}

func (c *jobComponentsBuilder) buildJobComponent(radixJobComponent v1.RadixJobComponent) v1.RadixDeployJobComponent {
	componentName := radixJobComponent.Name
	deployJob := v1.RadixDeployJobComponent{
		Name:          componentName,
		Ports:         radixJobComponent.Ports,
		Secrets:       radixJobComponent.Secrets,
		Monitoring:    false,
		RunAsNonRoot:  false,
		Payload:       radixJobComponent.Payload,
		SchedulerPort: radixJobComponent.SchedulerPort,
	}

	environmentSpecificConfig := c.getEnvironmentConfig(radixJobComponent)
	if environmentSpecificConfig != nil {
		deployJob.Environment = environmentSpecificConfig.Environment
		deployJob.Monitoring = environmentSpecificConfig.Monitoring
		deployJob.VolumeMounts = environmentSpecificConfig.VolumeMounts
		deployJob.RunAsNonRoot = environmentSpecificConfig.RunAsNonRoot
	}

	componentImage := c.componentImages[componentName]
	deployJob.Image = getImagePath(&componentImage, environmentSpecificConfig)
	deployJob.EnvironmentVariables = getRadixCommonComponentEnvVars(&radixJobComponent, environmentSpecificConfig)
	deployJob.Resources = getRadixCommonComponentResources(&radixJobComponent, environmentSpecificConfig)
	deployJob.Node = getRadixCommonComponentNode(&radixJobComponent, environmentSpecificConfig)
	deployJob.SecretRefs = getRadixCommonComponentRadixSecretRefs(&radixJobComponent, environmentSpecificConfig)
	return deployJob
}
