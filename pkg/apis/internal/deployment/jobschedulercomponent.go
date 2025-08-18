package deployment

import (
	"fmt"
	"os"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type JobSchedulerComponent struct {
	*radixv1.RadixDeployJobComponent
	RadixDeployment *radixv1.RadixDeployment
}

// NewJobSchedulerComponent Constructor
func NewJobSchedulerComponent(jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment) radixv1.RadixCommonDeployComponent {
	return &JobSchedulerComponent{
		jobComponent,
		rd,
	}
}

func (js *JobSchedulerComponent) GetHealthChecks() *radixv1.RadixHealthChecks {
	return nil
}

func (js *JobSchedulerComponent) GetImage() string {
	containerRegistry := os.Getenv(defaults.ContainerRegistryEnvironmentVariable)
	radixJobScheduler := os.Getenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
	radixJobSchedulerImageUrl := fmt.Sprintf("%s/%s", containerRegistry, radixJobScheduler)
	return radixJobSchedulerImageUrl
}

func (js *JobSchedulerComponent) GetCommand() []string {
	return nil
}

func (js *JobSchedulerComponent) GetArgs() []string {
	return nil
}
func (js *JobSchedulerComponent) HasZeroReplicas() bool {
	return js.GetReplicas() != nil && *js.GetReplicas() == 0
}

func (js *JobSchedulerComponent) GetPorts() []radixv1.ComponentPort {
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

func (js *JobSchedulerComponent) GetEnvironmentVariables() radixv1.EnvVarsMap {
	envVarsMap := js.EnvironmentVariables.DeepCopy()
	if envVarsMap == nil {
		envVarsMap = radixv1.EnvVarsMap{}
	}
	envVarsMap[defaults.RadixDeploymentEnvironmentVariable] = js.RadixDeployment.Name
	envVarsMap[defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable] = os.Getenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable)
	return envVarsMap
}

func (js *JobSchedulerComponent) GetSecrets() []string {
	return nil
}

func (js *JobSchedulerComponent) GetResources() *radixv1.ResourceRequirements {
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

func (js *JobSchedulerComponent) GetReadOnlyFileSystem() *bool {
	return pointers.Ptr(true)
}

func (js *JobSchedulerComponent) IsAlwaysPullImageOnDeploy() bool {
	return true
}

func (js *JobSchedulerComponent) GetNode() *radixv1.RadixNode {
	// Job configuration in radixconfig.yaml contains section "node", which supposed to configure scheduled jobs by RadixDeployment
	// "node" section settings should not be applied to the JobScheduler component itself
	return nil
}

func (js *JobSchedulerComponent) GetRuntime() *radixv1.Runtime {
	return &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}
}

// IsDeployComponentJobSchedulerDeployment Checks if deployComponent is a JobScheduler deployment
func IsDeployComponentJobSchedulerDeployment(deployComponent radixv1.RadixCommonDeployComponent) bool {
	_, isJobScheduler := interface{}(deployComponent).(*JobSchedulerComponent)
	return isJobScheduler
}
