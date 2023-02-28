package deployment

import (
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// JobComponentsBuilder builds RD job components for a given environment
type JobComponentsBuilder interface {
	JobComponents() ([]v1.RadixDeployJobComponent, error)
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

func (c *jobComponentsBuilder) JobComponents() ([]v1.RadixDeployJobComponent, error) {
	var jobs []v1.RadixDeployJobComponent

	for _, radixJobComponent := range c.ra.Spec.Jobs {
		environmentSpecificConfig := c.getEnvironmentConfig(radixJobComponent)
		if !radixJobComponent.GetEnabledForEnv(environmentSpecificConfig) {
			continue
		}
		deployJob, err := c.buildJobComponent(radixJobComponent, environmentSpecificConfig, c.defaultEnvVars)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, deployJob)
	}

	return jobs, nil
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

func (c *jobComponentsBuilder) buildJobComponent(radixJobComponent v1.RadixJobComponent, environmentSpecificConfig *v1.RadixJobComponentEnvironmentConfig, defaultEnvVars v1.EnvVarsMap) (v1.RadixDeployJobComponent, error) {
	componentName := radixJobComponent.Name
	deployJob := v1.RadixDeployJobComponent{
		Name:             componentName,
		Ports:            radixJobComponent.Ports,
		Secrets:          radixJobComponent.Secrets,
		Monitoring:       false,
		MonitoringConfig: radixJobComponent.MonitoringConfig,
		Payload:          radixJobComponent.Payload,
		SchedulerPort:    radixJobComponent.SchedulerPort,
		Notifications:    radixJobComponent.Notifications,
	}

	if environmentSpecificConfig != nil {
		deployJob.Monitoring = environmentSpecificConfig.Monitoring
		deployJob.VolumeMounts = environmentSpecificConfig.VolumeMounts
		deployJob.TimeLimitSeconds = environmentSpecificConfig.TimeLimitSeconds
		if environmentSpecificConfig.Notifications != nil {
			deployJob.Notifications = environmentSpecificConfig.Notifications
		}
	}

	if deployJob.TimeLimitSeconds == nil {
		if radixJobComponent.TimeLimitSeconds != nil {
			deployJob.TimeLimitSeconds = radixJobComponent.TimeLimitSeconds
		} else {
			deployJob.TimeLimitSeconds = numbers.Int64Ptr(defaults.RadixJobTimeLimitSeconds)
		}
	}

	identity, err := getRadixCommonComponentIdentity(&radixJobComponent, environmentSpecificConfig)
	if err != nil {
		return v1.RadixDeployJobComponent{}, err
	}

	componentImage := c.componentImages[componentName]
	deployJob.Image = getImagePath(&componentImage, environmentSpecificConfig)
	deployJob.EnvironmentVariables = getRadixCommonComponentEnvVars(&radixJobComponent, environmentSpecificConfig, defaultEnvVars)
	deployJob.Resources = getRadixCommonComponentResources(&radixJobComponent, environmentSpecificConfig)
	deployJob.Node = getRadixCommonComponentNode(&radixJobComponent, environmentSpecificConfig)
	deployJob.SecretRefs = getRadixCommonComponentRadixSecretRefs(&radixJobComponent, environmentSpecificConfig)
	deployJob.Identity = identity

	return deployJob, nil
}
