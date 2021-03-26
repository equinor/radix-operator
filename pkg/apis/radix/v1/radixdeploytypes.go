package v1

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDeployment describe a deployment
type RadixDeployment struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixDeploymentSpec `json:"spec" yaml:"spec"`
	Status             RadixDeployStatus   `json:"status" yaml:"status"`
}

func (rd *RadixDeployment) GetComponentByName(name string) *RadixDeployComponent {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return &component
		}
	}
	return nil
}

func (rd *RadixDeployment) GetJobComponentByName(name string) *RadixDeployJobComponent {
	for _, jobComponent := range rd.Spec.Jobs {
		if strings.EqualFold(jobComponent.Name, name) {
			return &jobComponent
		}
	}
	return nil
}

//RadixDeployStatus is the status for a rd
type RadixDeployStatus struct {
	ActiveFrom meta_v1.Time         `json:"activeFrom" yaml:"activeFrom"`
	ActiveTo   meta_v1.Time         `json:"activeTo" yaml:"activeTo"`
	Condition  RadixDeployCondition `json:"condition" yaml:"condition"`
	Reconciled meta_v1.Time         `json:"reconciled" yaml:"reconciled"`
}

// RadixDeployCondition Holds the condition of a component
type RadixDeployCondition string

// These are valid conditions of a deployment.
const (
	// Active means the radix deployment is active and should be consolidated
	DeploymentActive RadixDeployCondition = "Active"
	// Inactive means radix deployment is inactive and should not be consolidated
	DeploymentInactive RadixDeployCondition = "Inactive"
)

//RadixDeploymentSpec is the spec for a deployment
type RadixDeploymentSpec struct {
	AppName          string                         `json:"appname" yaml:"appname"`
	Components       []RadixDeployComponent         `json:"components"`
	Jobs             []RadixDeployJobComponent      `json:"jobs"`
	Environment      string                         `json:"environment" yaml:"environment"`
	ImagePullSecrets []core_v1.LocalObjectReference `json:"imagePullSecrets" yaml:"imagePullSecrets"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixDeploymentList is a list of Radix deployments
type RadixDeploymentList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixDeployment `json:"items" yaml:"items"`
}

//RadixDeployComponent defines a single component within a RadixDeployment - maps to single deployment/service/ingress etc
type RadixDeployComponent struct {
	Name         string          `json:"name" yaml:"name"`
	RunAsNonRoot bool            `json:"runAsNonRoot" yaml:"runAsNonRoot"`
	Image        string          `json:"image" yaml:"image"`
	Ports        []ComponentPort `json:"ports" yaml:"ports"`
	Replicas     *int            `json:"replicas" yaml:"replicas"`
	// Deprecated: For backwards compatibility Public is still supported, new code should use PublicPort instead
	Public                  bool                    `json:"public" yaml:"public"`
	PublicPort              string                  `json:"publicPort,omitempty" yaml:"publicPort,omitempty"`
	EnvironmentVariables    EnvVarsMap              `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	Secrets                 []string                `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	IngressConfiguration    []string                `json:"ingressConfiguration,omitempty" yaml:"ingressConfiguration,omitempty"`
	DNSAppAlias             bool                    `json:"dnsAppAlias,omitempty" yaml:"dnsAppAlias,omitempty"`
	DNSExternalAlias        []string                `json:"dnsExternalAlias,omitempty" yaml:"dnsExternalAlias,omitempty"`
	Monitoring              bool                    `json:"monitoring" yaml:"monitoring"`
	Resources               ResourceRequirements    `json:"resources,omitempty" yaml:"resources,omitempty"`
	HorizontalScaling       *RadixHorizontalScaling `json:"horizontalScaling,omitempty" yaml:"horizontalScaling,omitempty"`
	AlwaysPullImageOnDeploy bool                    `json:"alwaysPullImageOnDeploy" yaml:"alwaysPullImageOnDeploy"`
	VolumeMounts            []RadixVolumeMount      `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Node                    RadixNode               `json:"node,omitempty" yaml:"node,omitempty"`
}

func (deployComponent *RadixDeployComponent) GetName() string {
	return deployComponent.Name
}

func (deployComponent *RadixDeployComponent) GetType() string {
	return defaults.RadixComponentTypeComponent
}

func (deployComponent *RadixDeployComponent) GetImage() string {
	return deployComponent.Image
}

func (deployComponent *RadixDeployComponent) GetPorts() []ComponentPort {
	return deployComponent.Ports
}

func (deployComponent *RadixDeployComponent) GetEnvironmentVariables() *EnvVarsMap {
	return &deployComponent.EnvironmentVariables
}

func (deployComponent *RadixDeployComponent) GetSecrets() []string {
	return deployComponent.Secrets
}

func (deployComponent *RadixDeployComponent) GetMonitoring() bool {
	return deployComponent.Monitoring
}

func (deployComponent *RadixDeployComponent) GetResources() *ResourceRequirements {
	return &deployComponent.Resources
}

func (deployComponent *RadixDeployComponent) GetVolumeMounts() []RadixVolumeMount {
	return deployComponent.VolumeMounts
}

func (deployComponent *RadixDeployComponent) IsAlwaysPullImageOnDeploy() bool {
	return deployComponent.AlwaysPullImageOnDeploy
}

func (deployComponent *RadixDeployComponent) GetReplicas() *int {
	return deployComponent.Replicas
}

func (deployComponent *RadixDeployComponent) GetHorizontalScaling() *RadixHorizontalScaling {
	return deployComponent.HorizontalScaling
}

func (deployComponent *RadixDeployComponent) GetPublicPort() string {
	return deployComponent.PublicPort
}

func (deployComponent *RadixDeployComponent) IsPublic() bool {
	return len(deployComponent.PublicPort) > 0
}

func (deployComponent *RadixDeployComponent) GetDNSExternalAlias() []string {
	return deployComponent.DNSExternalAlias
}

func (deployComponent *RadixDeployComponent) IsDNSAppAlias() bool {
	return deployComponent.DNSAppAlias
}

func (deployComponent *RadixDeployComponent) GetIngressConfiguration() []string {
	return deployComponent.IngressConfiguration
}

func (deployComponent *RadixDeployComponent) GetRunAsNonRoot() bool {
	return deployComponent.RunAsNonRoot
}

func (deployComponent *RadixDeployComponent) GetNode() *RadixNode {
	return &deployComponent.Node
}

func (deployJobComponent *RadixDeployJobComponent) GetName() string {
	return deployJobComponent.Name
}

func (deployJobComponent *RadixDeployJobComponent) GetType() string {
	return defaults.RadixComponentTypeJobScheduler
}

func (deployJobComponent *RadixDeployJobComponent) GetImage() string {
	return deployJobComponent.Image
}

func (deployJobComponent *RadixDeployJobComponent) GetPorts() []ComponentPort {
	return deployJobComponent.Ports
}

func (deployJobComponent *RadixDeployJobComponent) GetEnvironmentVariables() *EnvVarsMap {
	return &deployJobComponent.EnvironmentVariables
}

func (deployJobComponent *RadixDeployJobComponent) GetSecrets() []string {
	return deployJobComponent.Secrets
}

func (deployJobComponent *RadixDeployJobComponent) GetMonitoring() bool {
	return deployJobComponent.Monitoring
}

func (deployJobComponent *RadixDeployJobComponent) GetResources() *ResourceRequirements {
	return &deployJobComponent.Resources
}

func (deployJobComponent *RadixDeployJobComponent) GetVolumeMounts() []RadixVolumeMount {
	return deployJobComponent.VolumeMounts
}

func (deployJobComponent *RadixDeployJobComponent) IsAlwaysPullImageOnDeploy() bool {
	return deployJobComponent.AlwaysPullImageOnDeploy
}

func (deployJobComponent *RadixDeployJobComponent) GetReplicas() *int {
	return numbers.IntPtr(1)
}

func (deployJobComponent *RadixDeployJobComponent) GetHorizontalScaling() *RadixHorizontalScaling {
	return nil
}

func (deployJobComponent *RadixDeployJobComponent) GetPublicPort() string {
	return ""
}

func (deployJobComponent *RadixDeployJobComponent) IsPublic() bool {
	return false
}

func (deployJobComponent *RadixDeployJobComponent) GetDNSExternalAlias() []string {
	return nil
}

func (deployJobComponent *RadixDeployJobComponent) IsDNSAppAlias() bool {
	return false
}

func (deployJobComponent *RadixDeployJobComponent) GetIngressConfiguration() []string {
	return nil
}

func (deployJobComponent *RadixDeployJobComponent) GetRunAsNonRoot() bool {
	return deployJobComponent.RunAsNonRoot
}

func (deployJobComponent *RadixDeployJobComponent) GetNode() *RadixNode {
	return &deployJobComponent.Node
}

// GetNrOfReplicas gets number of replicas component will run
func (deployComponent RadixDeployComponent) GetNrOfReplicas() int32 {
	replicas := int32(1)
	if deployComponent.HorizontalScaling != nil && deployComponent.HorizontalScaling.MinReplicas != nil {
		replicas = *deployComponent.HorizontalScaling.MinReplicas
	} else if deployComponent.Replicas != nil {
		replicas = int32(*deployComponent.Replicas)
	}
	return replicas
}

//RadixDeployJobComponent defines a single job component within a RadixDeployment
// The job component is used by the radix-job-scheduler to create Kubernetes Job objects
type RadixDeployJobComponent struct {
	Name                    string                    `json:"name" yaml:"name"`
	Image                   string                    `json:"image" yaml:"image"`
	Ports                   []ComponentPort           `json:"ports" yaml:"ports"`
	EnvironmentVariables    EnvVarsMap                `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	Secrets                 []string                  `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Monitoring              bool                      `json:"monitoring" yaml:"monitoring"`
	Resources               ResourceRequirements      `json:"resources,omitempty" yaml:"resources,omitempty"`
	VolumeMounts            []RadixVolumeMount        `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	SchedulerPort           *int32                    `json:"schedulerPort,omitempty" yaml:"schedulerPort,omitempty"`
	Payload                 *RadixJobComponentPayload `json:"payload,omitempty" yaml:"payload,omitempty"`
	RunAsNonRoot            bool                      `json:"runAsNonRoot" yaml:"runAsNonRoot"`
	AlwaysPullImageOnDeploy bool                      `json:"alwaysPullImageOnDeploy" yaml:"alwaysPullImageOnDeploy"`
	Node                    RadixNode                 `json:"node,omitempty" yaml:"node,omitempty"`
}

//RadixCommonDeployComponent defines a common component interface a RadixDeployment
type RadixCommonDeployComponent interface {
	GetName() string
	GetType() string
	GetImage() string
	GetPorts() []ComponentPort
	GetEnvironmentVariables() *EnvVarsMap
	GetSecrets() []string
	GetMonitoring() bool
	GetResources() *ResourceRequirements
	GetVolumeMounts() []RadixVolumeMount
	IsAlwaysPullImageOnDeploy() bool
	GetReplicas() *int
	GetHorizontalScaling() *RadixHorizontalScaling
	GetPublicPort() string
	IsPublic() bool
	GetDNSExternalAlias() []string
	IsDNSAppAlias() bool
	GetIngressConfiguration() []string
	GetRunAsNonRoot() bool
	GetNode() *RadixNode
}
