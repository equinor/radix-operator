package v1

import (
	"reflect"
	"strings"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// RadixDeployStatus is the status for a rd
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

// RadixDeploymentSpec is the spec for a deployment
type RadixDeploymentSpec struct {
	AppName          string                         `json:"appname" yaml:"appname"`
	Components       []RadixDeployComponent         `json:"components"`
	Jobs             []RadixDeployJobComponent      `json:"jobs"`
	Environment      string                         `json:"environment" yaml:"environment"`
	ImagePullSecrets []core_v1.LocalObjectReference `json:"imagePullSecrets" yaml:"imagePullSecrets"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDeploymentList is a list of Radix deployments
type RadixDeploymentList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixDeployment `json:"items" yaml:"items"`
}

// RadixDeployComponent defines a single component within a RadixDeployment - maps to single deployment/service/ingress etc
type RadixDeployComponent struct {
	Name                    string                  `json:"name" yaml:"name"`
	Image                   string                  `json:"image" yaml:"image"`
	Ports                   []ComponentPort         `json:"ports" yaml:"ports"`
	Replicas                *int                    `json:"replicas" yaml:"replicas"`
	Public                  bool                    `json:"public" yaml:"public"` // Deprecated: For backwards compatibility Public is still supported, new code should use PublicPort instead
	PublicPort              string                  `json:"publicPort,omitempty" yaml:"publicPort,omitempty"`
	EnvironmentVariables    EnvVarsMap              `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	Secrets                 []string                `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	SecretRefs              RadixSecretRefs         `json:"secretRefs,omitempty" yaml:"secretRefs,omitempty"`
	IngressConfiguration    []string                `json:"ingressConfiguration,omitempty" yaml:"ingressConfiguration,omitempty"`
	DNSAppAlias             bool                    `json:"dnsAppAlias,omitempty" yaml:"dnsAppAlias,omitempty"`
	DNSExternalAlias        []string                `json:"dnsExternalAlias,omitempty" yaml:"dnsExternalAlias,omitempty"`
	Monitoring              bool                    `json:"monitoring" yaml:"monitoring"`
	MonitoringConfig        MonitoringConfig        `json:"monitoringConfig,omitempty" yaml:"monitoringConfig,omitempty"`
	Resources               ResourceRequirements    `json:"resources,omitempty" yaml:"resources,omitempty"`
	HorizontalScaling       *RadixHorizontalScaling `json:"horizontalScaling,omitempty" yaml:"horizontalScaling,omitempty"`
	AlwaysPullImageOnDeploy bool                    `json:"alwaysPullImageOnDeploy" yaml:"alwaysPullImageOnDeploy"`
	VolumeMounts            []RadixVolumeMount      `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Node                    RadixNode               `json:"node,omitempty" yaml:"node,omitempty"`
	Authentication          *Authentication         `json:"authentication,omitempty" yaml:"authentication,omitempty"`
	Identity                *Identity               `json:"identity,omitempty" yaml:"identity,omitempty"`
}

func (deployComponent *RadixDeployComponent) GetName() string {
	return deployComponent.Name
}

func (deployComponent *RadixDeployComponent) GetType() RadixComponentType {
	return RadixComponentTypeComponent
}

func (deployComponent *RadixDeployComponent) GetImage() string {
	return deployComponent.Image
}

func (deployComponent *RadixDeployComponent) GetPorts() []ComponentPort {
	return deployComponent.Ports
}

func (deployComponent *RadixDeployComponent) GetEnvironmentVariables() EnvVarsMap {
	return deployComponent.EnvironmentVariables
}

func (deployComponent *RadixDeployComponent) GetSecrets() []string {
	return deployComponent.Secrets
}

func (deployComponent *RadixDeployComponent) GetSecretRefs() RadixSecretRefs {
	return deployComponent.SecretRefs
}

func (deployComponent *RadixDeployComponent) GetMonitoring() bool {
	return deployComponent.Monitoring
}

func (deployComponent *RadixDeployComponent) GetMonitoringConfig() MonitoringConfig {
	return deployComponent.MonitoringConfig
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

func (deployComponent *RadixDeployComponent) GetNode() *RadixNode {
	return &deployComponent.Node
}

func (deployComponent *RadixDeployComponent) GetAuthentication() *Authentication {
	return deployComponent.Authentication
}

func (deployComponent *RadixDeployComponent) GetIdentity() *Identity {
	return deployComponent.Identity
}

func (deployComponent *RadixDeployComponent) SetName(name string) {
	deployComponent.Name = name
}

func (deployComponent *RadixDeployComponent) SetVolumeMounts(mounts []RadixVolumeMount) {
	deployComponent.VolumeMounts = mounts
}

func (deployComponent *RadixDeployComponent) SetEnvironmentVariables(envVars EnvVarsMap) {
	deployComponent.EnvironmentVariables = envVars
}

func (deployJobComponent *RadixDeployJobComponent) GetName() string {
	return deployJobComponent.Name
}

func (deployJobComponent *RadixDeployJobComponent) GetType() RadixComponentType {
	return RadixComponentTypeJob
}

func (deployJobComponent *RadixDeployJobComponent) GetImage() string {
	return deployJobComponent.Image
}

func (deployJobComponent *RadixDeployJobComponent) GetPorts() []ComponentPort {
	return deployJobComponent.Ports
}

func (deployJobComponent *RadixDeployJobComponent) GetEnvironmentVariables() EnvVarsMap {
	return deployJobComponent.EnvironmentVariables
}

func (deployJobComponent *RadixDeployJobComponent) GetSecrets() []string {
	return deployJobComponent.Secrets
}

func (deployComponent *RadixDeployJobComponent) GetSecretRefs() RadixSecretRefs {
	return deployComponent.SecretRefs
}

func (deployJobComponent *RadixDeployJobComponent) GetMonitoring() bool {
	return deployJobComponent.Monitoring
}

func (deployJobComponent *RadixDeployJobComponent) GetMonitoringConfig() MonitoringConfig {
	return deployJobComponent.MonitoringConfig
}

func (deployJobComponent *RadixDeployJobComponent) GetResources() *ResourceRequirements {
	return &deployJobComponent.Resources
}

func (deployJobComponent *RadixDeployJobComponent) GetVolumeMounts() []RadixVolumeMount {
	return deployJobComponent.VolumeMounts
}

func (deployJobComponent *RadixDeployJobComponent) GetEnvironment() string {
	return deployJobComponent.Environment
}

func (deployJobComponent *RadixDeployJobComponent) IsAlwaysPullImageOnDeploy() bool {
	return deployJobComponent.AlwaysPullImageOnDeploy
}

func (deployJobComponent *RadixDeployJobComponent) GetReplicas() *int {
	replicas := 1
	return &replicas
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

func (deployJobComponent *RadixDeployJobComponent) GetNode() *RadixNode {
	return &deployJobComponent.Node
}

func (deployJobComponent *RadixDeployJobComponent) GetAuthentication() *Authentication {
	return nil
}

func (deployJobComponent *RadixDeployJobComponent) GetIdentity() *Identity {
	return deployJobComponent.Identity
}

func (deployJobComponent *RadixDeployJobComponent) GetNotifications() *Notifications {
	return deployJobComponent.Notifications
}

func (deployJobComponent *RadixDeployJobComponent) SetName(name string) {
	deployJobComponent.Name = name
}

func (deployJobComponent *RadixDeployJobComponent) SetVolumeMounts(mounts []RadixVolumeMount) {
	deployJobComponent.VolumeMounts = mounts
}

func (deployJobComponent *RadixDeployJobComponent) SetEnvironmentVariables(envVars EnvVarsMap) {
	deployJobComponent.EnvironmentVariables = envVars
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

// RadixDeployJobComponent defines a single job component within a RadixDeployment
// The job component is used by the radix-job-scheduler to create Kubernetes Job objects
type RadixDeployJobComponent struct {
	Name                    string                    `json:"name" yaml:"name"`
	Environment             string                    `json:"environment" yaml:"environment"`
	Image                   string                    `json:"image" yaml:"image"`
	Ports                   []ComponentPort           `json:"ports" yaml:"ports"`
	EnvironmentVariables    EnvVarsMap                `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	Secrets                 []string                  `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	SecretRefs              RadixSecretRefs           `json:"secretRefs,omitempty" yaml:"secretRefs,omitempty"`
	Monitoring              bool                      `json:"monitoring" yaml:"monitoring"`
	MonitoringConfig        MonitoringConfig          `json:"monitoringConfig,omitempty" yaml:"monitoringConfig,omitempty"`
	Resources               ResourceRequirements      `json:"resources,omitempty" yaml:"resources,omitempty"`
	VolumeMounts            []RadixVolumeMount        `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	SchedulerPort           *int32                    `json:"schedulerPort,omitempty" yaml:"schedulerPort,omitempty"`
	Payload                 *RadixJobComponentPayload `json:"payload,omitempty" yaml:"payload,omitempty"`
	AlwaysPullImageOnDeploy bool                      `json:"alwaysPullImageOnDeploy" yaml:"alwaysPullImageOnDeploy"`
	Node                    RadixNode                 `json:"node,omitempty" yaml:"node,omitempty"`
	TimeLimitSeconds        *int64                    `json:"timeLimitSeconds,omitempty" yaml:"timeLimitSeconds,omitempty"`
	BackoffLimit            *int32                    `json:"backoffLimit,omitempty" yaml:"backoffLimit,omitempty"`
	Identity                *Identity                 `json:"identity,omitempty" yaml:"identity,omitempty"`
	Notifications           *Notifications            `json:"notifications,omitempty"`
}

type RadixComponentType string

const (
	RadixComponentTypeComponent RadixComponentType = "component"
	RadixComponentTypeJob       RadixComponentType = "job"
)

// RadixCommonDeployComponent defines a common component interface a RadixDeployment
type RadixCommonDeployComponent interface {
	GetName() string
	GetType() RadixComponentType
	GetImage() string
	GetPorts() []ComponentPort
	GetEnvironmentVariables() EnvVarsMap
	SetEnvironmentVariables(envVars EnvVarsMap)
	GetSecrets() []string
	GetSecretRefs() RadixSecretRefs
	GetMonitoring() bool
	GetMonitoringConfig() MonitoringConfig
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
	GetNode() *RadixNode
	GetAuthentication() *Authentication
	SetName(name string)
	SetVolumeMounts(mounts []RadixVolumeMount)
	GetIdentity() *Identity
}

// RadixCommonDeployComponentFactory defines a common component factory
type RadixCommonDeployComponentFactory interface {
	Create() RadixCommonDeployComponent
	GetTargetType() reflect.Type
}

type RadixDeployComponentFactory struct{}
type RadixDeployJobComponentFactory struct{}

func (factory RadixDeployComponentFactory) Create() RadixCommonDeployComponent {
	return &RadixDeployComponent{}
}

func (factory RadixDeployComponentFactory) GetTargetType() reflect.Type {
	return reflect.TypeOf(&RadixDeployComponent{}).Elem()
}

func (factory RadixDeployJobComponentFactory) Create() RadixCommonDeployComponent {
	return &RadixDeployJobComponent{}
}

func (factory RadixDeployJobComponentFactory) GetTargetType() reflect.Type {
	return reflect.TypeOf(&RadixDeployJobComponentFactory{}).Elem()
}
