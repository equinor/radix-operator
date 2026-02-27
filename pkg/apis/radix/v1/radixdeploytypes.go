package v1

import (
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Active From",type="string",JSONPath=".status.activeFrom"
// +kubebuilder:printcolumn:name="Active To",type="string",JSONPath=".status.activeTo"
// +kubebuilder:printcolumn:name="Condition",type="string",JSONPath=".status.condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Reconciled",type="date",JSONPath=".status.reconciled",priority=1
// +kubebuilder:printcolumn:name="Branch",type="string",JSONPath=".metadata.annotations.radix-branch",priority=1
// +kubebuilder:resource:path=radixdeployments,shortName=rd
// +kubebuilder:subresource:status

// RadixDeployment describe a deployment
type RadixDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              RadixDeploymentSpec `json:"spec"`

	// Status is the observed state of the RadixDeployment
	// +kubebuilder:validation:Optional
	Status RadixDeployStatus `json:"status,omitzero"`
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

type RadixDeploymentReconcileStatus string

const (
	RadixDeploymentReconcileSucceeded RadixDeploymentReconcileStatus = "Succeeded"
	RadixDeploymentReconcileFailed    RadixDeploymentReconcileStatus = "Failed"
)

// RadixDeployStatus represents the current state of a RadixDeployment
type RadixDeployStatus struct {
	ActiveFrom metav1.Time `json:"activeFrom"`

	// +kubebuilder:validation:Optional
	ActiveTo  metav1.Time          `json:"activeTo,omitzero"`
	Condition RadixDeployCondition `json:"condition"`

	// Reconciled is the timestamp of the last successful reconciliation
	// +kubebuilder:validation:Optional
	Reconciled metav1.Time `json:"reconciled,omitzero"`

	// ObservedGeneration is the generation observed by the controller
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReconcileStatus indicates whether the last reconciliation succeeded or failed
	// +kubebuilder:validation:Optional
	ReconcileStatus RadixDeploymentReconcileStatus `json:"reconcileStatus,omitempty"`

	// Message provides additional information about the reconciliation state, typically error details when reconciliation fails
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`
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
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	AppName string `json:"appname"`

	// +optional
	Components []RadixDeployComponent `json:"components,omitempty"`

	// +optional
	Jobs []RadixDeployJobComponent `json:"jobs,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	Environment string `json:"environment"`

	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDeploymentList is a list of Radix deployments
type RadixDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RadixDeployment `json:"items"`
}

// RadixDeployExternalDNS is the spec for an external DNS alias
type RadixDeployExternalDNS struct {
	// Fully qualified domain name (FQDN), e.g. myapp.example.com.
	// +kubebuilder:validation:Required
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
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +listType=map
	// +listMapKey=name
	// +optional
	Ports []ComponentPort `json:"ports,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=64
	// +optional
	Replicas *int `json:"replicas,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=64
	// +optional
	ReplicasOverride *int `json:"replicasOverride,omitempty"`

	// Deprecated: For backwards compatibility Public is still supported, new code should use PublicPort instead
	// +optional
	Public bool `json:"public"`

	// +optional
	PublicPort string `json:"publicPort,omitempty"`

	// +optional
	EnvironmentVariables EnvVarsMap `json:"environmentVariables,omitempty"`

	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// +optional
	IngressConfiguration []string `json:"ingressConfiguration,omitempty"`

	// +optional
	DNSAppAlias bool `json:"dnsAppAlias,omitempty"`

	// +optional
	ExternalDNS []RadixDeployExternalDNS `json:"externalDNS,omitempty"`

	// Deprecated: For backward compatibility we must still support this field. New code should use ExternalDNS instead.
	// +optional
	DNSExternalAlias []string `json:"dnsExternalAlias,omitempty"`

	// +optional
	HealthChecks *RadixHealthChecks `json:"healthChecks,omitempty"`

	// +optional
	Monitoring bool `json:"monitoring"`

	// +optional
	MonitoringConfig MonitoringConfig `json:"monitoringConfig,omitempty"`

	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// +optional
	HorizontalScaling *RadixHorizontalScaling `json:"horizontalScaling,omitempty"`

	// +optional
	AlwaysPullImageOnDeploy bool `json:"alwaysPullImageOnDeploy"`

	// +optional
	VolumeMounts []RadixVolumeMount `json:"volumeMounts,omitempty"`

	// Deprecated: use Runtime.NodeType instead.
	// Defines GPU requirements for the component.
	// More info: https://www.radix.equinor.com/radix-config#node
	// +optional
	Node RadixNode `json:"node,omitempty"`

	// +optional
	Authentication *Authentication `json:"authentication,omitempty"`

	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// +optional
	ReadOnlyFileSystem *bool `json:"readOnlyFileSystem,omitempty"`

	// +optional
	Runtime *Runtime `json:"runtime,omitempty"`

	// +optional
	Network *Network `json:"network,omitempty"`

	// GetCommand Entrypoint array. Not executed within a shell.
	// +optional
	Command []string `json:"command,omitempty"`

	// GetArgs Arguments to the entrypoint.
	// +optional
	Args []string `json:"args,omitempty"`
	// User ID to run the container as
	// More info: https://www.radix.equinor.com/radix-config#runasuser
	// +optional
	// +kubebuilder:validation:Minimum=1
	RunAsUser *int64 `json:"runAsUser,omitempty"`
}

func (deployComponent *RadixDeployComponent) GetHealthChecks() *RadixHealthChecks {
	if deployComponent.HealthChecks == nil {
		return nil
	}
	if deployComponent.HealthChecks.ReadinessProbe == nil &&
		deployComponent.HealthChecks.LivenessProbe == nil &&
		deployComponent.HealthChecks.StartupProbe == nil {
		return nil
	}

	return deployComponent.HealthChecks
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

func (deployComponent *RadixDeployComponent) GetPublicPortNumber() (int32, bool) {
	if deployComponent == nil {
		return 0, false
	}

	if len(deployComponent.Ports) == 0 {
		return 0, false
	}

	if deployComponent.PublicPort != "" {
		for _, port := range deployComponent.Ports {
			if port.Name == deployComponent.PublicPort {
				return port.Port, true
			}
		}

		return 0, false
	}

	if deployComponent.Public {
		return deployComponent.Ports[0].Port, true
	}

	return 0, false
}

func (deployComponent *RadixDeployComponent) IsPublic() bool {
	return len(deployComponent.PublicPort) > 0
}

func (deployComponent *RadixDeployComponent) GetExternalDNS() []RadixDeployExternalDNS {
	if len(deployComponent.ExternalDNS) > 0 {
		return deployComponent.ExternalDNS
	}
	return slice.Map(deployComponent.DNSExternalAlias, //nolint:staticcheck
		func(alias string) RadixDeployExternalDNS {
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
	return &deployComponent.Node //nolint:staticcheck
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

func (deployComponent *RadixDeployComponent) GetNetwork() *Network {
	return deployComponent.Network
}

func (deployComponent *RadixDeployComponent) SetEnvironmentVariables(envVars EnvVarsMap) {
	deployComponent.EnvironmentVariables = envVars
}

func (deployComponent *RadixDeployComponent) GetCommand() []string {
	return deployComponent.Command
}

func (deployComponent *RadixDeployComponent) GetArgs() []string {
	return deployComponent.Args
}

func (deployComponent *RadixDeployComponent) GetRunAsUser() *int64 {
	return deployComponent.RunAsUser
}

func (deployComponent *RadixDeployComponent) HasZeroReplicas() bool {
	return deployComponent.GetReplicas() != nil && *deployComponent.GetReplicas() == 0
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

func (deployJobComponent *RadixDeployJobComponent) GetPublicPortNumber() (int32, bool) {
	return 0, false
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
	return &deployJobComponent.Node //nolint:staticcheck
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

func (deployJobComponent *RadixDeployJobComponent) GetNetwork() *Network {
	return nil
}

func (deployJobComponent *RadixDeployJobComponent) SetEnvironmentVariables(envVars EnvVarsMap) {
	deployJobComponent.EnvironmentVariables = envVars
}

func (deployJobComponent *RadixDeployJobComponent) GetCommand() []string {
	return deployJobComponent.Command
}

func (deployJobComponent *RadixDeployJobComponent) GetArgs() []string {
	return deployJobComponent.Args
}

func (deployJobComponent *RadixDeployJobComponent) GetRunAsUser() *int64 {
	return deployJobComponent.RunAsUser
}

func (deployJobComponent *RadixDeployJobComponent) HasZeroReplicas() bool {
	return deployJobComponent.GetReplicas() != nil && *deployJobComponent.GetReplicas() == 0
}

// GetNrOfReplicas gets number of replicas component will run
func (deployComponent *RadixDeployComponent) GetNrOfReplicas() int32 {
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

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +listType=map
	// +listMapKey=name
	// +optional
	Ports []ComponentPort `json:"ports,omitempty"`

	// +optional
	EnvironmentVariables EnvVarsMap `json:"environmentVariables,omitempty"`

	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// +optional
	Monitoring bool `json:"monitoring"`

	// +optional
	MonitoringConfig MonitoringConfig `json:"monitoringConfig,omitempty"`

	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// +optional
	VolumeMounts []RadixVolumeMount `json:"volumeMounts,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	SchedulerPort int32 `json:"schedulerPort"`

	// +optional
	Payload *RadixJobComponentPayload `json:"payload,omitempty"`

	// +optional
	AlwaysPullImageOnDeploy bool `json:"alwaysPullImageOnDeploy"`

	// Deprecated: use Runtime.NodeType instead.
	// +optional
	Node RadixNode `json:"node,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +optional
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// +optional
	Notifications *Notifications `json:"notifications,omitempty"`

	// +optional
	ReadOnlyFileSystem *bool `json:"readOnlyFileSystem,omitempty"`

	// +optional
	Runtime *Runtime `json:"runtime,omitempty"`

	// BatchStatusRules Rules define how a batch status is set corresponding to batch job statuses
	// +optional
	BatchStatusRules []BatchStatusRule `json:"batchStatusRules,omitempty"`

	// FailurePolicy specifies the policy of handling failed job replicas
	// +optional
	FailurePolicy *RadixJobComponentFailurePolicy `json:"failurePolicy,omitempty"`

	// GetCommand Entrypoint array. Not executed within a shell.
	// +optional
	Command []string `json:"command,omitempty"`

	// GetArgs Arguments to the entrypoint.
	// +optional
	Args []string `json:"args,omitempty"`
	// User ID to run the container as
	// More info: https://www.radix.equinor.com/radix-config#runasuser
	// +optional
	// +kubebuilder:validation:Minimum=1
	RunAsUser *int64 `json:"runAsUser,omitempty"`
}

func (r *RadixDeployJobComponent) GetHealthChecks() *RadixHealthChecks {
	return nil
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
	GetPublicPortNumber() (int32, bool)
	IsPublic() bool
	GetExternalDNS() []RadixDeployExternalDNS
	IsDNSAppAlias() bool
	GetIngressConfiguration() []string
	GetNode() *RadixNode
	GetAuthentication() *Authentication
	GetIdentity() *Identity
	GetReadOnlyFileSystem() *bool
	GetRuntime() *Runtime
	GetNetwork() *Network
	GetHealthChecks() *RadixHealthChecks
	// GetCommand Entrypoint array. Not executed within a shell.
	GetCommand() []string
	// GetArgs Arguments to the entrypoint.
	GetArgs() []string
	// HasZeroReplicas returns true if the component has zero replicas
	HasZeroReplicas() bool
	GetRunAsUser() *int64
}

// IsActive The RadixDeployment is active
func (rd *RadixDeployment) IsActive() bool {
	return rd.Status.Condition == DeploymentActive
}
