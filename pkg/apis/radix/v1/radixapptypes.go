package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DynamicTagNameInEnvironmentConfig Pattern to indicate that the
// image tag should be taken from the environment config
const DynamicTagNameInEnvironmentConfig = "{imageTagName}"

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixApplication describe an application
type RadixApplication struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixApplicationSpec `json:"spec" yaml:"spec"`
}

//RadixApplicationSpec is the spec for an application
type RadixApplicationSpec struct {
	Build            *BuildSpec             `json:"build" yaml:"build"`
	Environments     []Environment          `json:"environments" yaml:"environments"`
	Jobs             []RadixJobComponent    `json:"jobs" yaml:"jobs"`
	Components       []RadixComponent       `json:"components" yaml:"components"`
	DNSAppAlias      AppAlias               `json:"dnsAppAlias" yaml:"dnsAppAlias"`
	DNSExternalAlias []ExternalAlias        `json:"dnsExternalAlias" yaml:"dnsExternalAlias"`
	PrivateImageHubs PrivateImageHubEntries `json:"privateImageHubs" yaml:"privateImageHubs"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixApplicationList is a list of Radix applications
type RadixApplicationList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixApplication `json:"items" yaml:"items"`
}

//SecretsMap is a map of secrets (weird)
type SecretsMap map[string]string

// EnvVarsMap maps environment variable keys to their values
type EnvVarsMap map[string]string

//BuildSpec defines the specification for building the components
type BuildSpec struct {
	Secrets []string `json:"secrets" yaml:"secrets"`
}

//Environment defines a Radix application environment
type Environment struct {
	Name  string   `json:"name" yaml:"name"`
	Build EnvBuild `json:"build,omitempty" yaml:"build,omitempty"`
}

// EnvBuild defines build parameters of a specific environment
type EnvBuild struct {
	From string `json:"from,omitempty" yaml:"from,omitempty"`
}

// AppAlias defines a URL alias for this application. The URL will be of form <app-name>.apps.radix.equinor.com
type AppAlias struct {
	Environment string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Component   string `json:"component,omitempty" yaml:"component,omitempty"`
}

// ExternalAlias defines a URL alias for this application with ability to bring-your-own certificate
type ExternalAlias struct {
	Alias       string `json:"alias,omitempty" yaml:"alias,omitempty"`
	Environment string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Component   string `json:"component,omitempty" yaml:"component,omitempty"`
}

// ComponentPort defines the port number, protocol and port for a service
type ComponentPort struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

// ResourceList Placeholder for resouce specifications in the config
type ResourceList map[string]string

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Limits ResourceList `json:"limits,omitempty" yaml:"limits,omitempty"`
	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if that is explicitly specified,
	// otherwise to an implementation-defined value.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Requests ResourceList `json:"requests,omitempty" yaml:"requests,omitempty"`
}

// RadixComponent defines a single component within a RadixApplication - maps to single deployment/service/ingress etc
type RadixComponent struct {
	Name                    string                   `json:"name" yaml:"name"`
	SourceFolder            string                   `json:"src" yaml:"src"`
	Image                   string                   `json:"image" yaml:"image"`
	DockerfileName          string                   `json:"dockerfileName" yaml:"dockerfileName"`
	Ports                   []ComponentPort          `json:"ports" yaml:"ports"`
	Public                  bool                     `json:"public" yaml:"public"` // Deprecated: For backwards compatibility Public is still supported, new code should use PublicPort instead
	PublicPort              string                   `json:"publicPort,omitempty" yaml:"publicPort,omitempty"`
	Secrets                 []string                 `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	IngressConfiguration    []string                 `json:"ingressConfiguration,omitempty" yaml:"ingressConfiguration,omitempty"`
	EnvironmentConfig       []RadixEnvironmentConfig `json:"environmentConfig,omitempty" yaml:"environmentConfig,omitempty"`
	Variables               EnvVarsMap               `json:"variables" yaml:"variables"`
	Resources               ResourceRequirements     `json:"resources,omitempty" yaml:"resources,omitempty"`
	AlwaysPullImageOnDeploy *bool                    `json:"alwaysPullImageOnDeploy" yaml:"alwaysPullImageOnDeploy"`
	Node                    RadixNode                `json:"node,omitempty" yaml:"node,omitempty"`
	Authentication          *Authentication          `json:"authentication,omitempty" yaml:"authentication,omitempty"`
}

// RadixEnvironmentConfig defines environment specific settings for a single component within a RadixApplication
type RadixEnvironmentConfig struct {
	Environment             string                  `json:"environment" yaml:"environment"`
	RunAsNonRoot            bool                    `json:"runAsNonRoot" yaml:"runAsNonRoot"`
	Replicas                *int                    `json:"replicas" yaml:"replicas"`
	Monitoring              bool                    `json:"monitoring" yaml:"monitoring"`
	Resources               ResourceRequirements    `json:"resources,omitempty" yaml:"resources,omitempty"`
	Variables               EnvVarsMap              `json:"variables" yaml:"variables"`
	HorizontalScaling       *RadixHorizontalScaling `json:"horizontalScaling,omitempty" yaml:"horizontalScaling,omitempty"`
	ImageTagName            string                  `json:"imageTagName" yaml:"imageTagName"`
	AlwaysPullImageOnDeploy *bool                   `json:"alwaysPullImageOnDeploy,omitempty" yaml:"alwaysPullImageOnDeploy,omitempty"`
	VolumeMounts            []RadixVolumeMount      `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Node                    RadixNode               `json:"node,omitempty" yaml:"node,omitempty"`
	Authentication          *Authentication         `json:"authentication,omitempty" yaml:"authentication,omitempty"`
}

// RadixJobComponent defines a single job component within a RadixApplication
// The job component is used by the radix-job-scheduler to create Kubernetes Job objects
type RadixJobComponent struct {
	Name              string                               `json:"name" yaml:"name"`
	SourceFolder      string                               `json:"src" yaml:"src"`
	Image             string                               `json:"image" yaml:"image"`
	DockerfileName    string                               `json:"dockerfileName" yaml:"dockerfileName"`
	SchedulerPort     *int32                               `json:"schedulerPort,omitempty" yaml:"schedulerPort,omitempty"`
	Payload           *RadixJobComponentPayload            `json:"payload,omitempty" yaml:"payload,omitempty"`
	Ports             []ComponentPort                      `json:"ports" yaml:"ports"`
	Secrets           []string                             `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	EnvironmentConfig []RadixJobComponentEnvironmentConfig `json:"environmentConfig,omitempty" yaml:"environmentConfig,omitempty"`
	Variables         EnvVarsMap                           `json:"variables" yaml:"variables"`
	Resources         ResourceRequirements                 `json:"resources,omitempty" yaml:"resources,omitempty"`
	Node              RadixNode                            `json:"node,omitempty" yaml:"node,omitempty"`
}

// RadixJobComponentEnvironmentConfig defines environment specific settings
// for a single job component within a RadixApplication
type RadixJobComponentEnvironmentConfig struct {
	Environment  string               `json:"environment" yaml:"environment"`
	RunAsNonRoot bool                 `json:"runAsNonRoot" yaml:"runAsNonRoot"`
	Monitoring   bool                 `json:"monitoring" yaml:"monitoring"`
	Resources    ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	Variables    EnvVarsMap           `json:"variables" yaml:"variables"`
	ImageTagName string               `json:"imageTagName" yaml:"imageTagName"`
	VolumeMounts []RadixVolumeMount   `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Node         RadixNode            `json:"node,omitempty" yaml:"node,omitempty"`
}

// RadixJobComponentPayload defines the path and where the payload received by radix-job-scheduler
// will be mounted to the job container
type RadixJobComponentPayload struct {
	Path string `json:"path" yaml:"path"`
}

// RadixHorizontalScaling defines configuration for horizontal pod autoscaler. It is kept as close as the HorizontalPodAutoscalerSpec
// If set, this will override replicas config
type RadixHorizontalScaling struct {
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty" yaml:"minReplicas,omitempty"`
	MaxReplicas int32  `json:"maxReplicas" yaml:"maxReplicas"`
}

// PrivateImageHubEntries - key = imagehubserver
type PrivateImageHubEntries map[string]*RadixPrivateImageHubCredential

// RadixPrivateImageHubCredential defines a private image hub available during deployment time
type RadixPrivateImageHubCredential struct {
	Username string `json:"username" yaml:"username"`
	Email    string `json:"email" yaml:"email"`
}

// RadixVolumeMount defines volume to be mounted to the container
type RadixVolumeMount struct {
	Type      MountType `json:"type" yaml:"type"`
	Name      string    `json:"name" yaml:"name"`
	Container string    `json:"container" yaml:"container"`
	Path      string    `json:"path" yaml:"path"`
}

// MountType Holds types of mount
type MountType string

// These are valid types of mount
const (
	// MountTypeBlob Use of azure/blobfuse flexvolume
	MountTypeBlob MountType = "blob"
)

// RadixNode defines node attributes, where container should be scheduled
type RadixNode struct {
	// Gpu Optional. Holds lists of node GPU types, with dashed types to exclude
	Gpu string `json:"gpu" yaml:"gpu"`
	// GpuCount Optional. Holds minimum count of GPU on node
	GpuCount string `json:"gpuCount" yaml:"gpuCount"`
}

type Authentication struct {
	ClientCertificate *ClientCertificate `json:"clientCertificate,omitempty" yaml:"clientCertificate,omitempty"`
}

type ClientCertificate struct {
	Verification              *VerificationType `json:"verification,omitempty" yaml:"verification,omitempty"`
	PassCertificateToUpstream *bool             `json:"passCertificateToUpstream,omitempty" yaml:"passCertificateToUpstream,omitempty"`
}

type VerificationType string

const (
	VerificationTypeOff          VerificationType = "off"
	VerificationTypeOn           VerificationType = "on"
	VerificationTypeOptional     VerificationType = "optional"
	VerificationTypeOptionalNoCa VerificationType = "optional_no_ca"
)

//RadixCommonComponent defines a common component interface for Radix components
type RadixCommonComponent interface {
	GetName() string
	GetNode() *RadixNode
}

func (component *RadixComponent) GetName() string {
	return component.Name
}

func (component *RadixComponent) GetNode() *RadixNode {
	return &component.Node
}

func (component *RadixJobComponent) GetName() string {
	return component.Name
}

func (component *RadixJobComponent) GetNode() *RadixNode {
	return &component.Node
}
