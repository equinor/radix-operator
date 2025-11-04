package deployment

import (
	"os"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type JobSchedulerComponent struct {
	radixJob        *radixv1.RadixDeployJobComponent
	radixDeployment *radixv1.RadixDeployment
}

// NewJobSchedulerComponent Constructor
func NewJobSchedulerComponent(jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment) radixv1.RadixCommonDeployComponent {
	return &JobSchedulerComponent{
		jobComponent,
		rd,
	}
}

func (js *JobSchedulerComponent) GetName() string {
	return js.radixJob.GetName()
}

func (js *JobSchedulerComponent) GetType() radixv1.RadixComponentType {
	return js.radixJob.GetType()
}

func (js *JobSchedulerComponent) GetImage() string {
	return os.Getenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
}

func (js *JobSchedulerComponent) GetPorts() []radixv1.ComponentPort {
	if js.radixJob.SchedulerPort == nil {
		return nil
	}

	return []radixv1.ComponentPort{
		{
			Name: defaults.RadixJobSchedulerPortName,
			Port: *js.radixJob.SchedulerPort,
		},
	}
}

func (js *JobSchedulerComponent) GetEnvironmentVariables() radixv1.EnvVarsMap {
	envVarsMap := js.radixJob.EnvironmentVariables.DeepCopy()
	if envVarsMap == nil {
		envVarsMap = radixv1.EnvVarsMap{}
	}
	envVarsMap[defaults.RadixDeploymentEnvironmentVariable] = js.radixDeployment.Name
	envVarsMap[defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable] = os.Getenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable)
	return envVarsMap
}

func (js *JobSchedulerComponent) SetEnvironmentVariables(envVars radixv1.EnvVarsMap) {
	js.radixJob.SetEnvironmentVariables(envVars)
}

func (js *JobSchedulerComponent) GetSecrets() []string {
	return nil
}

func (js *JobSchedulerComponent) GetSecretRefs() radixv1.RadixSecretRefs {
	return js.radixJob.GetSecretRefs()
}

func (js *JobSchedulerComponent) GetMonitoring() bool {
	return js.radixJob.GetMonitoring()
}

func (js *JobSchedulerComponent) GetMonitoringConfig() radixv1.MonitoringConfig {
	return js.radixJob.GetMonitoringConfig()
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

func (js *JobSchedulerComponent) GetVolumeMounts() []radixv1.RadixVolumeMount {
	return js.radixJob.GetVolumeMounts()
}

func (js *JobSchedulerComponent) IsAlwaysPullImageOnDeploy() bool {
	return true
}

func (js *JobSchedulerComponent) GetReplicas() *int {
	return js.radixJob.GetReplicas()
}

func (js *JobSchedulerComponent) GetReplicasOverride() *int {
	return js.radixJob.GetReplicasOverride()
}

func (js *JobSchedulerComponent) GetHorizontalScaling() *radixv1.RadixHorizontalScaling {
	return js.radixJob.GetHorizontalScaling()
}

func (js *JobSchedulerComponent) GetPublicPort() string {
	return js.radixJob.GetPublicPort()
}

func (js *JobSchedulerComponent) IsPublic() bool {
	return js.radixJob.IsPublic()
}

func (js *JobSchedulerComponent) GetExternalDNS() []radixv1.RadixDeployExternalDNS {
	return js.radixJob.GetExternalDNS()
}

func (js *JobSchedulerComponent) IsDNSAppAlias() bool {
	return js.radixJob.IsDNSAppAlias()
}

func (js *JobSchedulerComponent) GetIngressConfiguration() []string {
	return js.radixJob.GetIngressConfiguration()
}

func (js *JobSchedulerComponent) GetNode() *radixv1.RadixNode {
	// Job configuration in radixconfig.yaml contains section "node", which supposed to configure scheduled jobs by RadixDeployment
	// "node" section settings should not be applied to the JobScheduler component itself
	return nil
}

func (js *JobSchedulerComponent) GetAuthentication() *radixv1.Authentication {
	return js.radixJob.GetAuthentication()
}

func (js *JobSchedulerComponent) GetIdentity() *radixv1.Identity {
	return js.radixJob.GetIdentity()
}

func (js *JobSchedulerComponent) GetReadOnlyFileSystem() *bool {
	return pointers.Ptr(true)
}

func (js *JobSchedulerComponent) GetRuntime() *radixv1.Runtime {
	return &radixv1.Runtime{Architecture: radixv1.RuntimeArchitectureAmd64}
}

func (js *JobSchedulerComponent) GetNetwork() *radixv1.Network {
	return js.radixJob.GetNetwork()
}

func (js *JobSchedulerComponent) GetHealthChecks() *radixv1.RadixHealthChecks {
	return nil
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

func (js *JobSchedulerComponent) GetRunAsUser() *int64 {
	return js.radixJob.GetRunAsUser()
}

// IsDeployComponentJobSchedulerDeployment Checks if deployComponent is a JobScheduler deployment
func IsDeployComponentJobSchedulerDeployment(deployComponent radixv1.RadixCommonDeployComponent) bool {
	_, isJobScheduler := interface{}(deployComponent).(*JobSchedulerComponent)
	return isJobScheduler
}
