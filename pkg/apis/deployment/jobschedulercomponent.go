package deployment

import (
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type jobSchedulerComponent struct {
	*v1.RadixDeployJobComponent
	radixDeployment *v1.RadixDeployment
}

func NewJobSchedulerComponent(jobComponent *v1.RadixDeployJobComponent, rd *v1.RadixDeployment) v1.RadixCommonDeployComponent {
	return &jobSchedulerComponent{
		jobComponent,
		rd,
	}
}

func (js *jobSchedulerComponent) GetType() string {
	return defaults.RadixComponentTypeJobScheduler
}

func (js *jobSchedulerComponent) GetImage() string {
	containerRegistry := os.Getenv(defaults.ContainerRegistryEnvironmentVariable)
	radixJobScheduler := os.Getenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
	radixJobSchedulerImageUrl := fmt.Sprintf("%s/%s", containerRegistry, radixJobScheduler)
	return radixJobSchedulerImageUrl
}

func (js *jobSchedulerComponent) GetPorts() []v1.ComponentPort {
	if js.RadixDeployJobComponent.SchedulerPort == nil {
		return nil
	}

	return []v1.ComponentPort{
		{
			Name: defaults.RadixJobSchedulerPortName,
			Port: *js.RadixDeployJobComponent.SchedulerPort,
		},
	}
}

func (js *jobSchedulerComponent) GetEnvironmentVariables() *v1.EnvVarsMap {
	return &v1.EnvVarsMap{
		defaults.RadixDeploymentEnvironmentVariable:               js.radixDeployment.Name,
		defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable:    os.Getenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable),
		defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable: os.Getenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable),
	}
}

func (js *jobSchedulerComponent) GetSecrets() []string {
	return nil
}

func (js *jobSchedulerComponent) GetMonitoring() bool {
	return false
}

func (js *jobSchedulerComponent) GetResources() *v1.ResourceRequirements {
	return &v1.ResourceRequirements{}
}

func (js *jobSchedulerComponent) GetVolumeMounts() []v1.RadixVolumeMount {
	return nil
}

func (js *jobSchedulerComponent) IsAlwaysPullImageOnDeploy() bool {
	return true
}

func (js *jobSchedulerComponent) GetRunAsNonRoot() bool {
	return true
}
