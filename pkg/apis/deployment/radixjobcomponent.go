package deployment

import (
	stderrors "errors"

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
	componentImages pipeline.DeployComponentImages
	defaultEnvVars  v1.EnvVarsMap
}

// NewJobComponentsBuilder constructor for JobComponentsBuilder
func NewJobComponentsBuilder(ra *v1.RadixApplication, env string, componentImages pipeline.DeployComponentImages, defaultEnvVars v1.EnvVarsMap) JobComponentsBuilder {
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
		jobs = append(jobs, *deployJob)
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

func (c *jobComponentsBuilder) buildJobComponent(radixJobComponent v1.RadixJobComponent, environmentSpecificConfig *v1.RadixJobComponentEnvironmentConfig, defaultEnvVars v1.EnvVarsMap) (*v1.RadixDeployJobComponent, error) {
	componentName := radixJobComponent.Name
	componentImage := c.componentImages[componentName]

	var errs []error
	identity, err := getRadixCommonComponentIdentity(&radixJobComponent, environmentSpecificConfig)
	if err != nil {
		errs = append(errs, err)
	}
	notifications, err := getRadixJobComponentNotification(&radixJobComponent, environmentSpecificConfig)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, stderrors.Join(errs...)
	}

	image, err := getImagePath(componentName, componentImage, environmentSpecificConfig)
	if err != nil {
		return nil, err
	}
	deployJob := v1.RadixDeployJobComponent{
		Name:                 componentName,
		Ports:                radixJobComponent.Ports,
		Secrets:              radixJobComponent.Secrets,
		Monitoring:           false,
		MonitoringConfig:     radixJobComponent.MonitoringConfig,
		Payload:              radixJobComponent.Payload,
		SchedulerPort:        radixJobComponent.SchedulerPort,
		Image:                image,
		EnvironmentVariables: getRadixCommonComponentEnvVars(&radixJobComponent, environmentSpecificConfig, defaultEnvVars),
		Resources:            getRadixCommonComponentResources(&radixJobComponent, environmentSpecificConfig),
		Node:                 getRadixCommonComponentNode(&radixJobComponent, environmentSpecificConfig),
		SecretRefs:           getRadixCommonComponentRadixSecretRefs(&radixJobComponent, environmentSpecificConfig),
		BackoffLimit:         getRadixJobComponentBackoffLimit(radixJobComponent, environmentSpecificConfig),
		TimeLimitSeconds:     getRadixJobComponentTimeLimitSeconds(radixJobComponent, environmentSpecificConfig),
		Identity:             identity,
		Notifications:        notifications,
	}

	if environmentSpecificConfig != nil {
		deployJob.Monitoring = environmentSpecificConfig.Monitoring
		deployJob.VolumeMounts = environmentSpecificConfig.VolumeMounts
	}

	return &deployJob, nil
}

func getRadixJobComponentTimeLimitSeconds(radixJobComponent v1.RadixJobComponent, environmentSpecificConfig *v1.RadixJobComponentEnvironmentConfig) *int64 {
	if environmentSpecificConfig != nil && environmentSpecificConfig.TimeLimitSeconds != nil {
		return environmentSpecificConfig.TimeLimitSeconds
	}

	if radixJobComponent.TimeLimitSeconds != nil {
		return radixJobComponent.TimeLimitSeconds
	}

	return numbers.Int64Ptr(defaults.RadixJobTimeLimitSeconds)
}

func getRadixJobComponentBackoffLimit(radixJobComponent v1.RadixJobComponent, environmentSpecificConfig *v1.RadixJobComponentEnvironmentConfig) *int32 {
	if environmentSpecificConfig != nil && environmentSpecificConfig.BackoffLimit != nil {
		return environmentSpecificConfig.BackoffLimit
	}
	return radixJobComponent.BackoffLimit
}
