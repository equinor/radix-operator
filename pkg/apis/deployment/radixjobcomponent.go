package deployment

import (
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
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
	defaultEnvVars  v1.EnvVarsMap
}

// NewJobComponentsBuilder constructor for JobComponentsBuilder
func NewJobComponentsBuilder(ra *v1.RadixApplication, env string, componentImages map[string]pipeline.ComponentImage, defaultEnvVars v1.EnvVarsMap) JobComponentsBuilder {
	return &jobComponentsBuilder{
		ra:              ra,
		env:             env,
		componentImages: componentImages,
		defaultEnvVars:  defaultEnvVars,
	}
}

func (c *jobComponentsBuilder) JobComponents() []v1.RadixDeployJobComponent {
	var jobs []v1.RadixDeployJobComponent

	for _, radixJobComponent := range c.ra.Spec.Jobs {
		environmentSpecificConfig := c.getEnvironmentConfig(radixJobComponent)
		if !radixJobComponent.GetEnabledForEnv(environmentSpecificConfig) {
			continue
		}
		deployJob := c.buildJobComponent(radixJobComponent, environmentSpecificConfig, c.defaultEnvVars)
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

func (c *jobComponentsBuilder) buildJobComponent(radixJobComponent v1.RadixJobComponent, environmentSpecificConfig *v1.RadixJobComponentEnvironmentConfig, defaultEnvVars v1.EnvVarsMap) v1.RadixDeployJobComponent {
	componentName := radixJobComponent.Name
	deployJob := v1.RadixDeployJobComponent{
		Name:             componentName,
		Ports:            radixJobComponent.Ports,
		Secrets:          radixJobComponent.Secrets,
		Monitoring:       false,
		MonitoringConfig: radixJobComponent.MonitoringConfig,
		RunAsNonRoot:     false,
		Payload:          radixJobComponent.Payload,
		SchedulerPort:    radixJobComponent.SchedulerPort,
	}

	if environmentSpecificConfig != nil {
		deployJob.Monitoring = environmentSpecificConfig.Monitoring
		deployJob.VolumeMounts = environmentSpecificConfig.VolumeMounts
		deployJob.RunAsNonRoot = environmentSpecificConfig.RunAsNonRoot
		deployJob.TimeLimitSeconds = environmentSpecificConfig.TimeLimitSeconds
	}

	if deployJob.TimeLimitSeconds == nil {
		if radixJobComponent.TimeLimitSeconds != nil {
			deployJob.TimeLimitSeconds = radixJobComponent.TimeLimitSeconds
		} else {
			deployJob.TimeLimitSeconds = numbers.Int64Ptr(defaults.RadixJobTimeLimitSeconds)
		}
	}

	componentImage := c.componentImages[componentName]
	deployJob.Image = getImagePath(&componentImage, environmentSpecificConfig)
	deployJob.EnvironmentVariables = getRadixCommonComponentEnvVars(&radixJobComponent, environmentSpecificConfig, defaultEnvVars)
	deployJob.Resources = getRadixCommonComponentResources(&radixJobComponent, environmentSpecificConfig)
	deployJob.Node = getRadixCommonComponentNode(&radixJobComponent, environmentSpecificConfig)
	deployJob.SecretRefs = getRadixCommonComponentRadixSecretRefs(&radixJobComponent, environmentSpecificConfig)

	return deployJob
}
