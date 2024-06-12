package deployment

import (
	"fmt"
	"os"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type jobSchedulerComponent struct {
	*radixv1.RadixDeployJobComponent
	radixDeployment *radixv1.RadixDeployment
}

func newJobSchedulerComponent(jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment) radixv1.RadixCommonDeployComponent {
	return &jobSchedulerComponent{
		jobComponent,
		rd,
	}
}

func (js *jobSchedulerComponent) GetImage() string {
	containerRegistry := os.Getenv(defaults.ContainerRegistryEnvironmentVariable)
	radixJobScheduler := os.Getenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
	radixJobSchedulerImageUrl := fmt.Sprintf("%s/%s", containerRegistry, radixJobScheduler)
	return radixJobSchedulerImageUrl
}

func (js *jobSchedulerComponent) GetPorts() []radixv1.ComponentPort {
	if js.RadixDeployJobComponent.SchedulerPort == nil {
		return nil
	}

	return []radixv1.ComponentPort{
		{
			Name: defaults.RadixJobSchedulerPortName,
			Port: *js.RadixDeployJobComponent.SchedulerPort,
		},
	}
}

func (js *jobSchedulerComponent) GetEnvironmentVariables() radixv1.EnvVarsMap {
	envVarsMap := js.EnvironmentVariables.DeepCopy()
	if envVarsMap == nil {
		envVarsMap = radixv1.EnvVarsMap{}
	}
	envVarsMap[defaults.RadixDeploymentEnvironmentVariable] = js.radixDeployment.Name
	envVarsMap[defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable] = os.Getenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable)
	return envVarsMap
}

func (js *jobSchedulerComponent) GetSecrets() []string {
	return nil
}

func (js *jobSchedulerComponent) GetMonitoring() bool {
	return false
}

func (js *jobSchedulerComponent) GetResources() *radixv1.ResourceRequirements {
	return &radixv1.ResourceRequirements{
		Limits: map[string]string{
			"memory": "500M",
		},
		Requests: map[string]string{
			"cpu":    "20m",
			"memory": "500M",
		},
	}
}

func (js *jobSchedulerComponent) GetReadOnlyFileSystem() *bool {
	return pointers.Ptr(true)
}

func (js *jobSchedulerComponent) IsAlwaysPullImageOnDeploy() bool {
	return true
}

func (js *jobSchedulerComponent) GetNode() *radixv1.RadixNode {
	// Job configuration in radixconfig.yaml contains section "node", which supposed to configure scheduled jobs by RadixDeployment
	// "node" section settings should not be applied to the JobScheduler component itself
	return nil
}

func (js *jobSchedulerComponent) GetRuntime() *radixv1.Runtime {
	return &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}
}

func isDeployComponentJobSchedulerDeployment(deployComponent radixv1.RadixCommonDeployComponent) bool {
	_, isJobScheduler := interface{}(deployComponent).(*jobSchedulerComponent)
	return isJobScheduler
}
