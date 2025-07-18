package deployment

import (
	"context"
	stderrors "errors"
	"fmt"

	"dario.cat/mergo"
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// JobComponentsBuilder builds RD job components for a given environment
type JobComponentsBuilder interface {
	JobComponents(ctx context.Context) ([]v1.RadixDeployJobComponent, error)
}

type jobComponentsBuilder struct {
	ra                            *v1.RadixApplication
	env                           string
	componentImages               pipeline.DeployComponentImages
	defaultEnvVars                v1.EnvVarsMap
	preservingDeployJobComponents []v1.RadixDeployJobComponent
}

// NewJobComponentsBuilder constructor for JobComponentsBuilder
func NewJobComponentsBuilder(ra *v1.RadixApplication, env string, componentImages pipeline.DeployComponentImages, defaultEnvVars v1.EnvVarsMap, preservingDeployJobComponents []v1.RadixDeployJobComponent) JobComponentsBuilder {
	return &jobComponentsBuilder{
		ra:                            ra,
		env:                           env,
		componentImages:               componentImages,
		defaultEnvVars:                defaultEnvVars,
		preservingDeployJobComponents: preservingDeployJobComponents,
	}
}

func (c *jobComponentsBuilder) JobComponents(ctx context.Context) ([]v1.RadixDeployJobComponent, error) {
	var jobs []v1.RadixDeployJobComponent
	preservingDeployJobComponentMap := slice.Reduce(c.preservingDeployJobComponents, make(map[string]v1.RadixDeployJobComponent), func(acc map[string]v1.RadixDeployJobComponent, jobComponent v1.RadixDeployJobComponent) map[string]v1.RadixDeployJobComponent {
		acc[jobComponent.GetName()] = jobComponent
		return acc
	})

	for _, radixJobComponent := range c.ra.Spec.Jobs {
		if preservingDeployComponent, ok := preservingDeployJobComponentMap[radixJobComponent.GetName()]; ok {
			jobs = append(jobs, preservingDeployComponent)
			continue
		}
		environmentSpecificConfig := c.getEnvironmentConfig(radixJobComponent)
		if !radixJobComponent.GetEnabledForEnvironmentConfig(environmentSpecificConfig) {
			continue
		}
		deployJob, err := c.buildJobComponent(ctx, radixJobComponent, environmentSpecificConfig, c.defaultEnvVars)
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

func (c *jobComponentsBuilder) buildJobComponent(ctx context.Context, radixJobComponent v1.RadixJobComponent, environmentSpecificConfig *v1.RadixJobComponentEnvironmentConfig, defaultEnvVars v1.EnvVarsMap) (*v1.RadixDeployJobComponent, error) {
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
	image, err := getImagePath(componentName, componentImage, radixJobComponent.ImageTagName, environmentSpecificConfig)
	if err != nil {
		errs = append(errs, err)
	}
	volumeMounts, err := getRadixCommonComponentVolumeMounts(&radixJobComponent, environmentSpecificConfig)
	if err != nil {
		errs = append(errs, err)
	}
	failurePolicy, err := getRadixJobComponentFailurePolicy(radixJobComponent, environmentSpecificConfig)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, stderrors.Join(errs...)
	}

	deployJob := v1.RadixDeployJobComponent{
		Name:                 componentName,
		Ports:                radixJobComponent.Ports,
		Secrets:              radixJobComponent.Secrets,
		Monitoring:           getRadixCommonComponentMonitoring(&radixJobComponent, environmentSpecificConfig),
		MonitoringConfig:     radixJobComponent.MonitoringConfig,
		Payload:              radixJobComponent.Payload,
		SchedulerPort:        radixJobComponent.SchedulerPort,
		Image:                image,
		EnvironmentVariables: getRadixCommonComponentEnvVars(&radixJobComponent, environmentSpecificConfig, defaultEnvVars),
		Resources:            getRadixCommonComponentResources(&radixJobComponent, environmentSpecificConfig),
		Node:                 getRadixCommonComponentNode(ctx, &radixJobComponent, environmentSpecificConfig),
		SecretRefs:           getRadixCommonComponentRadixSecretRefs(&radixJobComponent, environmentSpecificConfig),
		BackoffLimit:         getRadixJobComponentBackoffLimit(radixJobComponent, environmentSpecificConfig),
		TimeLimitSeconds:     getRadixJobComponentTimeLimitSeconds(radixJobComponent, environmentSpecificConfig),
		Identity:             identity,
		Notifications:        notifications,
		ReadOnlyFileSystem:   getRadixCommonComponentReadOnlyFileSystem(&radixJobComponent, environmentSpecificConfig),
		VolumeMounts:         volumeMounts,
		Runtime:              componentImage.Runtime,
		Command:              radixJobComponent.GetCommandForEnvironmentConfig(environmentSpecificConfig),
		Args:                 radixJobComponent.GetArgsForEnvironmentConfig(environmentSpecificConfig),
		BatchStatusRules:     getRadixJobComponentBatchStatusRules(&radixJobComponent, environmentSpecificConfig),
		FailurePolicy:        failurePolicy,
	}
	return &deployJob, nil
}

func getRadixJobComponentFailurePolicy(job v1.RadixJobComponent, jobEnvConfig *v1.RadixJobComponentEnvironmentConfig) (*v1.RadixJobComponentFailurePolicy, error) {
	var dst *v1.RadixJobComponentFailurePolicy
	if job.FailurePolicy != nil {
		dst = job.FailurePolicy.DeepCopy()
	}

	if jobEnvConfig != nil && jobEnvConfig.FailurePolicy != nil {
		if dst == nil {
			dst = &v1.RadixJobComponentFailurePolicy{}
		}

		if err := mergo.Merge(dst, jobEnvConfig.FailurePolicy, mergo.WithOverride, mergo.WithOverrideEmptySlice, mergo.WithTransformers(mergoTranformers)); err != nil {
			return nil, fmt.Errorf("failed to merge failurePolicy from environment config: %w", err)
		}
	}

	return dst, nil
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
