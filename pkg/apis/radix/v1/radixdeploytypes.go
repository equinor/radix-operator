package v1

import (
	"reflect"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDeployment describe a deployment
type RadixDeployment struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty"`
	Spec               RadixDeploymentSpec `json:"spec"`
	Status             RadixDeployStatus   `json:"status"`
}

// GetComponentByName returns the component matching the name parameter, or nil if not found
func (rd *RadixDeployment) GetComponentByName(name string) *RadixDeployComponent {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return &component
		}
	}
	return nil
}

// GetJobComponentByName returns the job matching the name parameter, or nil if not found
func (rd *RadixDeployment) GetJobComponentByName(name string) *RadixDeployJobComponent {
	for _, jobComponent := range rd.Spec.Jobs {
		if strings.EqualFold(jobComponent.Name, name) {
			return &jobComponent
		}
	}
	return nil
}

// GetCommonComponentByName returns the component or job matching the name parameter, or nil if not found
func (rd *RadixDeployment) GetCommonComponentByName(name string) RadixCommonDeployComponent {
	if comp := rd.GetComponentByName(name); comp != nil {
		return comp
	}
	return rd.GetJobComponentByName(name)
}

// RadixDeployStatus is the status for a rd
type RadixDeployStatus struct {
	ActiveFrom meta_v1.Time         `json:"activeFrom"`
	ActiveTo   meta_v1.Time         `json:"activeTo"`
	Condition  RadixDeployCondition `json:"condition"`
	Reconciled meta_v1.Time         `json:"reconciled"`
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
	AppName          string                         `json:"appname"`
	Components       []RadixDeployComponent         `json:"components"`
	Jobs             []RadixDeployJobComponent      `json:"jobs"`
	Environment      string                         `json:"environment"`
	ImagePullSecrets []core_v1.LocalObjectReference `json:"imagePullSecrets"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDeploymentList is a list of Radix deployments
type RadixDeploymentList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []RadixDeployment `json:"items"`
}

// RadixDeployExternalDNS is the spec for an external DNS alias
type RadixDeployExternalDNS struct {
	// Fully qualified domain name (FQDN), e.g. myapp.example.com.
	// +kubebuilder:validation:MinLength=4
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:Pattern=`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`
	FQDN string `json:"fqdn"`

	// Enable automatic issuing and renewal of TLS certificate
	// +kubebuilder:default:=false
	// +optional
	UseCertificateAutomation bool `json:"useCertificateAutomation,omitempty"`
}

// RadixDeployComponent defines a single component within a RadixDeployment - maps to single deployment/service/ingress etc
type RadixDeployComponent struct {
	Name             string          `json:"name"`
	Image            string          `json:"image"`
	Ports            []ComponentPort `json:"ports"`
	Replicas         *int            `json:"replicas"`
	ReplicasOverride *int            `json:"replicasOverride"`
	// Deprecated: For backwards compatibility Public is still supported, new code should use PublicPort instead
	Public               bool                     `json:"public"`
	PublicPort           string                   `json:"publicPort,omitempty"`
	EnvironmentVariables EnvVarsMap               `json:"environmentVariables,omitempty"`
	Secrets              []string                 `json:"secrets,omitempty"`
	SecretRefs           RadixSecretRefs          `json:"secretRefs,omitempty"`
	IngressConfiguration []string                 `json:"ingressConfiguration,omitempty"`
	DNSAppAlias          bool                     `json:"dnsAppAlias,omitempty"`
	ExternalDNS          []RadixDeployExternalDNS `json:"externalDNS,omitempty"`
	// Deprecated: For backward compatibility we must still support this field. New code should use ExternalDNS instead.
	DNSExternalAlias        []string                `json:"dnsExternalAlias,omitempty"`
	Monitoring              bool                    `json:"monitoring"`
	MonitoringConfig        MonitoringConfig        `json:"monitoringConfig,omitempty"`
	Resources               ResourceRequirements    `json:"resources,omitempty"`
	HorizontalScaling       *RadixHorizontalScaling `json:"horizontalScaling,omitempty"`
	AlwaysPullImageOnDeploy bool                    `json:"alwaysPullImageOnDeploy"`
	VolumeMounts            []RadixVolumeMount      `json:"volumeMounts,omitempty"`
	Node                    RadixNode               `json:"node,omitempty"`
	Authentication          *Authentication         `json:"authentication,omitempty"`
	Identity                *Identity               `json:"identity,omitempty"`
	ReadOnlyFileSystem      *bool                   `json:"readOnlyFileSystem,omitempty"`
	Runtime                 *Runtime                `json:"runtime,omitempty"`
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
func (deployComponent *RadixDeployComponent) GetReplicasOverride() *int {
	return deployComponent.ReplicasOverride
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

func (deployComponent *RadixDeployComponent) GetExternalDNS() []RadixDeployExternalDNS {
	if len(deployComponent.ExternalDNS) > 0 {
		return deployComponent.ExternalDNS
	}
	return slice.Map(deployComponent.DNSExternalAlias, func(alias string) RadixDeployExternalDNS {
		return RadixDeployExternalDNS{FQDN: alias, UseCertificateAutomation: false}
	})
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

func (deployComponent *RadixDeployComponent) GetReadOnlyFileSystem() *bool {
	return deployComponent.ReadOnlyFileSystem
}

func (deployComponent *RadixDeployComponent) GetRuntime() *Runtime {
	return deployComponent.Runtime
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
func (deployJobComponent *RadixDeployJobComponent) GetReplicasOverride() *int {
	return nil
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

func (deployJobComponent *RadixDeployJobComponent) GetExternalDNS() []RadixDeployExternalDNS {
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

func (deployJobComponent *RadixDeployJobComponent) GetReadOnlyFileSystem() *bool {
	return deployJobComponent.ReadOnlyFileSystem
}

func (deployJobComponent *RadixDeployJobComponent) GetRuntime() *Runtime {
	return deployJobComponent.Runtime
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
	Name                    string                    `json:"name"`
	Environment             string                    `json:"environment"`
	Image                   string                    `json:"image"`
	Ports                   []ComponentPort           `json:"ports"`
	EnvironmentVariables    EnvVarsMap                `json:"environmentVariables,omitempty"`
	Secrets                 []string                  `json:"secrets,omitempty"`
	SecretRefs              RadixSecretRefs           `json:"secretRefs,omitempty"`
	Monitoring              bool                      `json:"monitoring"`
	MonitoringConfig        MonitoringConfig          `json:"monitoringConfig,omitempty"`
	Resources               ResourceRequirements      `json:"resources,omitempty"`
	VolumeMounts            []RadixVolumeMount        `json:"volumeMounts,omitempty"`
	SchedulerPort           *int32                    `json:"schedulerPort,omitempty"`
	Payload                 *RadixJobComponentPayload `json:"payload,omitempty"`
	AlwaysPullImageOnDeploy bool                      `json:"alwaysPullImageOnDeploy"`
	Node                    RadixNode                 `json:"node,omitempty"`
	TimeLimitSeconds        *int64                    `json:"timeLimitSeconds,omitempty"`
	BackoffLimit            *int32                    `json:"backoffLimit,omitempty"`
	Identity                *Identity                 `json:"identity,omitempty"`
	Notifications           *Notifications            `json:"notifications,omitempty"`
	ReadOnlyFileSystem      *bool                     `json:"readOnlyFileSystem,omitempty"`
	Runtime                 *Runtime                  `json:"runtime,omitempty"`
	// BatchStatusRules Rules define how a batch status is set corresponding to batch job statuses
	// +optional
	BatchStatusRules []BatchStatusRule `json:"batchStatusRules,omitempty"`
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
	GetReplicasOverride() *int
	GetHorizontalScaling() *RadixHorizontalScaling
	GetPublicPort() string
	IsPublic() bool
	GetExternalDNS() []RadixDeployExternalDNS
	IsDNSAppAlias() bool
	GetIngressConfiguration() []string
	GetNode() *RadixNode
	GetAuthentication() *Authentication
	SetName(name string)
	SetVolumeMounts(mounts []RadixVolumeMount)
	GetIdentity() *Identity
	GetReadOnlyFileSystem() *bool
	GetRuntime() *Runtime
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
