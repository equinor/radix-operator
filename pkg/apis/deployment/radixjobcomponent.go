package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"reflect"
	"strings"

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

func (c *jobComponentsBuilder) buildJobComponent(appJobComponent v1.RadixJobComponent) v1.RadixDeployJobComponent {
	componentName := appJobComponent.Name
	componentImage := c.componentImages[componentName]
	var variables v1.EnvVarsMap
	monitoring := false
	var resources v1.ResourceRequirements
	var node v1.RadixNode
	var volumeMounts []v1.RadixVolumeMount
	var imageTagName string
	image := componentImage.ImagePath
	schedulerPort := appJobComponent.SchedulerPort
	payload := appJobComponent.Payload
	// Runs as root by default unless overridden
	runAsNonRoot := false
	var timeLimitSeconds *int64
	if appJobComponent.TimeLimitSeconds == nil {
		timeLimitSeconds = numbers.Int64Ptr(defaults.RadixJobTimeLimitSeconds)
	} else {
		timeLimitSeconds = appJobComponent.TimeLimitSeconds
	}

	environmentSpecificConfig := c.getEnvironmentConfig(appJobComponent)
	if environmentSpecificConfig != nil {
		variables = environmentSpecificConfig.Variables
		monitoring = environmentSpecificConfig.Monitoring
		resources = environmentSpecificConfig.Resources
		volumeMounts = environmentSpecificConfig.VolumeMounts
		imageTagName = environmentSpecificConfig.ImageTagName
		runAsNonRoot = environmentSpecificConfig.RunAsNonRoot
		node = environmentSpecificConfig.Node
		timeLimitSeconds = environmentSpecificConfig.TimeLimitSeconds
	}

	if variables == nil {
		variables = make(v1.EnvVarsMap)
	}

	updateComponentNode(&appJobComponent, &node)

	// Append common environment variables from appComponent.Variables to variables if not available yet
	for variableKey, variableValue := range appJobComponent.Variables {
		if _, found := variables[variableKey]; !found {
			variables[variableKey] = variableValue
		}
	}

	// Append common resources settings if currently empty
	if reflect.DeepEqual(resources, v1.ResourceRequirements{}) {
		resources = appJobComponent.Resources
	}

	// For deploy-only images, we will replace the dynamic tag with the tag from the environment
	// config
	if !componentImage.Build && strings.HasSuffix(image, v1.DynamicTagNameInEnvironmentConfig) {
		image = strings.ReplaceAll(image, v1.DynamicTagNameInEnvironmentConfig, imageTagName)
	}

	deployJob := v1.RadixDeployJobComponent{
		Name:                 componentName,
		Image:                image,
		Ports:                appJobComponent.Ports,
		Secrets:              appJobComponent.Secrets,
		EnvironmentVariables: variables, // todo: use single EnvVars instead
		Monitoring:           monitoring,
		Resources:            resources,
		VolumeMounts:         volumeMounts,
		SchedulerPort:        schedulerPort,
		Payload:              payload,
		RunAsNonRoot:         runAsNonRoot,
		Node:                 node,
		TimeLimitSeconds:     timeLimitSeconds,
	}

	return deployJob
}
