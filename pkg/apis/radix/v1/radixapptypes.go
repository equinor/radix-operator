package v1

import (
	"strings"

	commonUtils "github.com/equinor/radix-common/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DynamicTagNameInEnvironmentConfig Pattern to indicate that the
// image tag should be taken from the environment config
const DynamicTagNameInEnvironmentConfig = "{imageTagName}"

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=radixapplications,shortName=ra

// RadixApplication describes an application
type RadixApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Specification for an application.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/
	Spec RadixApplicationSpec `json:"spec"`
}

// RadixApplicationSpec is the specification for an application.
type RadixApplicationSpec struct {
	// Build contains configuration used by pipeline jobs.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#build
	// +optional
	Build *BuildSpec `json:"build,omitempty"`

	// List of environments belonging to the application.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#environments
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	Environments []Environment `json:"environments"`

	// List of job specification for the application.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#jobs
	// +listType=map
	// +listMapKey=name
	// +optional
	Jobs []RadixJobComponent `json:"jobs,omitempty"`

	// List of component specification for the application.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#components
	// +listType=map
	// +listMapKey=name
	// +optional
	Components []RadixComponent `json:"components,omitempty"`

	// Configure a component and environment to be linked to the app alias DNS record.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#dnsappalias
	// +optional
	DNSAppAlias AppAlias `json:"dnsAppAlias,omitempty"`

	// List of external DNS names and which component and environment incoming requests shall be routed to.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#dnsexternalalias
	// +listType=map
	// +listMapKey=alias
	// +optional
	DNSExternalAlias []ExternalAlias `json:"dnsExternalAlias,omitempty"`

	// Defines protected container registries used by components or jobs.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#privateimagehubs
	// +optional
	PrivateImageHubs PrivateImageHubEntries `json:"privateImageHubs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixApplicationList is a collection of RadixApplication.
type RadixApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RadixApplication `json:"items"`
}

// Map of environment variables in the form '<envvarname>: <value>'
type EnvVarsMap map[string]string

// BuildSpec contains configuration used by pipeline jobs.
type BuildSpec struct {
	// Defines a list of secrets that will be passed as ARGs when building Dockerfile.
	// The secrets can also be accessed in sub-pipelines.
	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// Defines variables that will be available in sub-pipelines.
	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// Enables BuildKit when building Dockerfile.
	// More info about BuildKit: https://docs.docker.com/build/buildkit/
	// +optional
	UseBuildKit *bool `json:"useBuildKit,omitempty"`
}

// Environment contains environment specific configuration.
type Environment struct {
	// Name of the environment.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Name string `json:"name"`

	// Build configuration for the environment.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#build-2
	// +optional
	Build EnvBuild `json:"build,omitempty"`

	// Configure egress traffic rules for components and jobs.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#egress
	// +optional
	Egress EgressConfig `json:"egress,omitempty"`
}

// EnvBuild contains configuration used to determine how to build an environment.
type EnvBuild struct {
	// Name of the Github branch to build from
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +optional
	From string `json:"from,omitempty"`

	// Defines variables that will be available in sub-pipelines
	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`
}

// EgressConfig contains egress configuration.
type EgressConfig struct {
	// Allow or deny outgoing traffic to the public IP of the Radix cluster.
	// +optional
	AllowRadix *bool `json:"allowRadix,omitempty"`

	// Defines a list of egress rules.
	// +kubebuilder:validation:MaxItems=1000
	// +optional
	Rules []EgressRule `json:"rules,omitempty"`
}

// +kubebuilder:validation:Pattern=`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\/([0-9]|[1-2][0-9]|3[0-2]))?$`
type EgressDestination string

// EgressRule defines an egress rule.
type EgressRule struct {
	// List of allowed destinations.
	// Each destination must be a valid IPv4 CIDR.
	// +kubebuilder:validation:MinItems=1
	Destinations []EgressDestination `json:"destinations"`

	// List of allowed ports.
	// +kubebuilder:validation:MinItems=1
	Ports []EgressPort `json:"ports"`
}

// EgressPort defines a port in context of EgressRule.
type EgressPort struct {
	// Port number.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Protocol.
	// +kubebuilder:validation:Enum=TCP;UDP
	Protocol string `json:"protocol"`
}

// AppAlias contains information about the component and
// environment to be linked to the app alias DNS record.
type AppAlias struct {
	// Name of the environment for the component.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Environment string `json:"environment,omitempty"`

	// Name of the component that shall receive the incoming requests.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Component string `json:"component,omitempty"`
}

// ExternalAlias defines mapping between an external DNS name and a component and environment.
type ExternalAlias struct {
	// DNS name, e.g. myapp.example.com.
	// +kubebuilder:validation:MinLength=4
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:Pattern=`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`
	Alias string `json:"alias"`

	// Name of the environment for the component.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Environment string `json:"environment"`

	// Name of the component that shall receive the incoming requests.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Component string `json:"component"`
}

// ComponentPort defines a named port.
type ComponentPort struct {
	// Name of the port.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Name string `json:"name"`

	// Port number.
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

// ResourceList defines a resource and a value.
type ResourceList map[string]string

// ResourceRequirements describes the compute resource requirements.
// More info: https://www.radix.equinor.com/references/reference-radix-config/#resources-common
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// +optional
	Limits ResourceList `json:"limits,omitempty"`

	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if
	// that is explicitly specified, otherwise to an implementation-defined value.
	// +optional
	Requests ResourceList `json:"requests,omitempty"`
}

// RadixComponent defines a component.
type RadixComponent struct {
	// Name of the component.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Name string `json:"name"`

	// Path to the Dockerfile that builds the component.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#src
	// +optional
	SourceFolder string `json:"src,omitempty"`

	// Name of the Dockerfile that builds the component.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#dockerfilename
	// +optional
	DockerfileName string `json:"dockerfileName,omitempty"`

	// Name of an existing container image to use when running the component.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#image
	// +optional
	Image string `json:"image,omitempty"`

	// List of ports that the component bind to.
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Ports []ComponentPort `json:"ports"`

	// Configures the monitoring endpoint exposed by the component.
	// This endpoint is used by Prometheus to collect custom metrics.
	// environmentConfig.monitoring must be set to true to enable collection of metrics for an environment.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#monitoringconfig
	// +optional
	MonitoringConfig MonitoringConfig `json:"monitoringConfig,omitempty"`

	// Deprecated, use publicPort instead.
	// +optional
	Public bool `json:"public,omitempty"` // Deprecated: For backwards compatibility Public is still supported, new code should use PublicPort instead

	// Defines which port (name) from the ports list that shall be accessible from the internet.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#publicport
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	// +optional
	PublicPort string `json:"publicPort,omitempty"`

	// List of secret environment variable names.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#secrets
	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// Configuration for external secret stores, like Azure KeyVault.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#secretrefs
	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// Additional configuration settings for ingress traffic.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#ingressconfiguration
	// +optional
	IngressConfiguration []string `json:"ingressConfiguration,omitempty"`

	// Configure environment specific settings for the component.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#environmentconfig
	// +listType=map
	// +listMapKey=environment
	// +optional
	EnvironmentConfig []RadixEnvironmentConfig `json:"environmentConfig,omitempty"`

	// List of environment variables and values.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#variables-common
	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// Configures CPU and memory resources for the component.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#resources-common
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Forces check/pull of images using static tags, e.g. myimage:latest, when deploying using deploy-only.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#alwayspullimageondeploy
	// +optional
	AlwaysPullImageOnDeploy *bool `json:"alwaysPullImageOnDeploy,omitempty"`

	// Defines GPU requirements for the component.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#node
	// +optional
	Node RadixNode `json:"node,omitempty"`

	// Configuration for TLS client certificate or OAuth2 authentication.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#authentication
	// +optional
	Authentication *Authentication `json:"authentication,omitempty"`

	// Configuration for workload identity (federated credentials).
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#identity
	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// Controls if the component shall be deployed.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#enabled
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// RadixEnvironmentConfig defines environment specific settings for component.
type RadixEnvironmentConfig struct {
	// Name of the environment which the settings applies to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Environment string `json:"environment"`

	// Number of desired replicas.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#replicas
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int `json:"replicas,omitempty"`

	// Enabled or disables collection of custom Prometheus metrics.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#monitoring
	// +optional
	Monitoring bool `json:"monitoring"`

	// Environment specific configuration for CPU and memory resources.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#resources
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Environment specific environment variables.
	// Variable names defined here have precedence over variables defined on component level.
	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// Configuration for automatic horizontal scaling of replicas.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#horizontalscaling
	// +optional
	HorizontalScaling *RadixHorizontalScaling `json:"horizontalScaling,omitempty"`

	// The imageTagName allows for flexible configuration of fixed images,
	// built outside of Radix, to be configured with separate tag for each environment.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#imagetagname
	// +optional
	ImageTagName string `json:"imageTagName,omitempty"`

	// Forces check/pull of images using static tags, e.g. myimage:latest, when deploying using deploy-only.
	// +optional
	AlwaysPullImageOnDeploy *bool `json:"alwaysPullImageOnDeploy,omitempty"`

	// Configuration for mounting cloud storage into the component.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#volumemounts
	// +optional
	VolumeMounts []RadixVolumeMount `json:"volumeMounts,omitempty"`

	// Environment specific GPU requirements for the component.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#node
	// +optional
	Node RadixNode `json:"node,omitempty"`

	// Environment specific configuration for TLS client certificate or OAuth2 authentication.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#authentication
	// +optional
	Authentication *Authentication `json:"authentication,omitempty"`

	// Environment specific configuration for external secret stores, like Azure KeyVault.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#secretrefs
	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// Environment specific configuration for workload identity (federated credentials).
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#identity
	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// Controls if the component shall be deployed to this environment.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#enabled
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// RadixJobComponent defines a single job component within a RadixApplication
// The job component is used by the radix-job-scheduler to create Kubernetes Job objects
type RadixJobComponent struct {
	// Name of the environment which the settings applies to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Name string `json:"name"`

	// Path to the Dockerfile that builds the job.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#src-2
	// +optional
	SourceFolder string `json:"src,omitempty"`

	// Name of the Dockerfile that builds the job.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#dockerfilename-2
	// +optional
	DockerfileName string `json:"dockerfileName,omitempty"`

	// Name of an existing container image to use when running the job.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#image-2
	// +optional
	Image string `json:"image,omitempty"`

	// Defines the port number that the job-scheduler API server will listen to.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#schedulerport
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +optional
	SchedulerPort *int32 `json:"schedulerPort,omitempty"`

	// Defines the path where the job payload is mounted.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#payload
	// +optional
	Payload *RadixJobComponentPayload `json:"payload,omitempty"`

	// List of ports that the job binds to.
	// +listType=map
	// +listMapKey=name
	// +optional
	Ports []ComponentPort `json:"ports,omitempty"`

	// Configures the monitoring endpoint exposed by the job.
	// This endpoint is used by Prometheus to collect custom metrics.
	// environmentConfig.monitoring must be set to true to enable collection of metrics for an environment.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#monitoringconfig-2
	// +optional
	MonitoringConfig MonitoringConfig `json:"monitoringConfig,omitempty"`

	// List of secret environment variable names.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#secrets-2
	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// Configuration for external secret stores, like Azure KeyVault.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#secretrefs
	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// Configure environment specific settings for the job.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#environmentconfig-2
	// +listType=map
	// +listMapKey=environment
	// +optional
	EnvironmentConfig []RadixJobComponentEnvironmentConfig `json:"environmentConfig,omitempty"`

	// List of environment variables and values.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#variables-common-2
	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// Configures CPU and memory resources for the job.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#resources-common-2
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Defines GPU requirements for the job.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#node
	// +optional
	Node RadixNode `json:"node,omitempty"`

	// The maximum number of seconds the job can run.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#timelimitseconds
	// +optional
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// Specifies the number of retries before marking this job failed.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#backofflimit
	// +optional
	// +kubebuilder:validation:Minimum:=0
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// Configuration for workload identity (federated credentials).
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#identity-2
	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// Controls if the job shall be deployed.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Notifications about batch or job status changes
	// +optional
	Notifications *Notifications `json:"notifications,omitempty"`
}

// RadixJobComponentEnvironmentConfig defines environment specific settings
// for a single job component within a RadixApplication
type RadixJobComponentEnvironmentConfig struct {
	// Name of the environment which the settings applies to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Environment string `json:"environment"`

	// Enabled or disables collection of custom Prometheus metrics.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#monitoring-2
	// +optional
	Monitoring bool `json:"monitoring,omitempty"`

	// Environment specific configuration for CPU and memory resources.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#resources-3
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Environment specific environment variables.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#variables-2
	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// The imageTagName allows for flexible configuration of fixed images,
	// built outside of Radix, to be configured with separate tag for each environment.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#imagetagname-2
	// +optional
	ImageTagName string `json:"imageTagName,omitempty"`

	// Configuration for mounting cloud storage into the job.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#volumemounts-2
	// +optional
	VolumeMounts []RadixVolumeMount `json:"volumeMounts,omitempty"`

	// Environment specific GPU requirements for the job.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#node
	// +optional
	Node RadixNode `json:"node,omitempty"`

	// Environment specific configuration for external secret stores, like Azure KeyVault.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#secretrefs
	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// Environment specific value for the maximum number of seconds the job can run.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#timelimitseconds-2
	// +optional
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// Environment specific value for the number of retries before marking this job failed.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#backofflimit-2
	// +optional
	// +kubebuilder:validation:Minimum:=0
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// Environment specific configuration for workload identity (federated credentials).
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#identity-2
	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// Controls if the job shall be deployed to this environment.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Notifications about batch or job status changes
	// +optional
	Notifications *Notifications `json:"notifications,omitempty"`
}

// RadixJobComponentPayload defines the path and where the payload received
// by radix-job-scheduler will be mounted to the job container
type RadixJobComponentPayload struct {
	// Path to the folder where payload is mounted
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`
}

// RadixHorizontalScaling defines configuration for horizontal pod autoscaler.
// If set, this will override replicas config
type RadixHorizontalScaling struct {
	// Defines the minimum number of replicas.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// Defines the maximum number of replicas.
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// Defines the resource usage parameters for the horizontal pod autoscaler.
	// +optional
	RadixHorizontalScalingResources *RadixHorizontalScalingResources `json:"resources,omitempty"`
}

type RadixHorizontalScalingResource struct {
	// Defines the resource usage which triggers scaling for the horizontal pod autoscaler.
	// +kubebuilder:validation:Minimum=1
	AverageUtilization *int32 `json:"averageUtilization"`
}

type RadixHorizontalScalingResources struct {
	// Defines the CPU usage parameters for the horizontal pod autoscaler.
	// +optional
	Cpu *RadixHorizontalScalingResource `json:"cpu,omitempty"`

	// Defines the memory usage parameters for the horizontal pod autoscaler.
	// +optional
	Memory *RadixHorizontalScalingResource `json:"memory,omitempty"`
}

// PrivateImageHubEntries defines authentication information for private image registries.
type PrivateImageHubEntries map[string]*RadixPrivateImageHubCredential

// RadixPrivateImageHubCredential contains credentials to use when pulling images
// from a protected container registry.
type RadixPrivateImageHubCredential struct {
	// Username with permission to pull images.
	// The password is set in Radix Web Console.
	// +kubebuilder:validation:MinLength=1
	Username string `json:"username"`

	// The email address linked to the username.
	// +kubebuilder:validation:MinLength=3
	Email string `json:"email"`
}

// RadixVolumeMount defines an external storage resource.
type RadixVolumeMount struct {
	// Type defines the storage type.
	// blob is deprecated, use azure-blob instead.
	// +kubebuilder:validation:Enum=blob;azure-blob;azure-blob2;azure-nfs;azure-file
	Type MountType `json:"type"`

	// User-defined name of the volume mount.
	// Must be unique for the component.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Deprecated. Only required by the deprecated type: blob.
	// +optional
	Container string `json:"container,omitempty"` // Outdated. Use Storage instead

	// Storage defines the name of the container in the external storage resource.
	// +optional
	Storage string `json:"storage"` // Container name, file Share name, etc.

	// Path defines in which directory the external storage is mounted.
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"` // Path within the pod (replica), where the volume mount has been mounted to

	// GID defines the group ID (number) which will be set as owner of the mounted volume.
	// +optional
	GID string `json:"gid,omitempty"` // Optional. Volume mount owner GroupID. Used when drivers do not honor fsGroup securityContext setting. https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/driver-parameters.md

	// UID defines the user ID (number) which will be set as owner of the mounted volume.
	// +optional
	UID string `json:"uid,omitempty"` // Optional. Volume mount owner UserID. Used instead of GID.

	// TODO: describe
	// More info: https://www.radix.equinor.com/guides/volume-mounts/optional-settings/
	// +optional
	SkuName string `json:"skuName,omitempty"` // Available values: Standard_LRS (default), Premium_LRS, Standard_GRS, Standard_RAGRS. https://docs.microsoft.com/en-us/rest/api/storagerp/srp_sku_types

	// TODO: describe
	// More info: https://www.radix.equinor.com/guides/volume-mounts/optional-settings/
	// +optional
	RequestsStorage string `json:"requestsStorage,omitempty"` // Requests resource storage size. Default "1Mi". https://kubernetes.io/docs/tasks/configure-pod-container/configure-persistent-volume-storage/#create-a-persistentvolumeclaim

	// Access mode from a container to an external storage. ReadOnlyMany (default), ReadWriteOnce, ReadWriteMany.
	// More info: https://www.radix.equinor.com/guides/volume-mounts/optional-settings/
	// +kubebuilder:validation:Enum=ReadOnlyMany;ReadWriteOnce;ReadWriteMany;""
	// +optional
	AccessMode string `json:"accessMode,omitempty"` // Available values: ReadOnlyMany (default) - read-only by many nodes, ReadWriteOnce - read-write by a single node, ReadWriteMany - read-write by many nodes. https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes

	// Binding mode from a container to an external storage. Immediate (default), WaitForFirstConsumer.
	// More info: https://www.radix.equinor.com/guides/volume-mounts/optional-settings/
	// +kubebuilder:validation:Enum=Immediate;WaitForFirstConsumer;""
	// +optional
	BindingMode string `json:"bindingMode,omitempty"` // Volume binding mode. Available values: Immediate (default), WaitForFirstConsumer. https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode

	// Configure Streaming mode. Used for blobfuse2.
	// More info: https://github.com/Azure/azure-storage-fuse/blob/main/STREAMING.md
	// +optional
	Streaming *RadixVolumeMountStreaming `json:"streaming,omitempty"` // Optional. Streaming configuration. Used for blobfuse2.
}

// RadixVolumeMountStreaming configure streaming to read and write large files that will not fit in the file cache on the local disk. Used for blobfuse2.
// More info: https://github.com/Azure/azure-storage-fuse/blob/main/STREAMING.md
type RadixVolumeMountStreaming struct {
	// Enable streaming mode. Default true.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Optional. The size of each block to be cached in memory (in MB).
	// +kubebuilder:validation:Minimum=1
	// +optional
	BlockSize *uint64 `json:"blockSize,omitempty"`
	// Optional. The total number of buffers to be cached in memory (in MB).
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxBuffers *uint64 `json:"maxBuffers,omitempty"`
	// Optional. The size of each buffer to be cached in memory (in MB).
	// +kubebuilder:validation:Minimum=1
	// +optional
	BufferSize *uint64 `json:"bufferSize,omitempty"`
	// Optional. Limit total amount of data being cached in memory to conserve memory footprint of blobfuse.
	// +kubebuilder:validation:Minimum=1
	// +optional
	StreamCache *uint64 `json:"streamCache,omitempty"`
	// Optional. The maximum number of blocks to be cached in memory.
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxBlocksPerFile *uint64 `json:"maxBlocksPerFile,omitempty"`
}

// MountType Holds types of mount
type MountType string

// These are valid types of mount
const (
	// MountTypeBlob Use of azure/blobfuse flexvolume
	MountTypeBlob MountType = "blob"
	// MountTypeBlobCsiAzure Use of azure/csi driver for blobfuse in Azure storage account
	MountTypeBlobCsiAzure MountType = "azure-blob"
	// MountTypeBlob2CsiAzure Use of azure/csi driver for blobfuse2 in Azure storage account
	MountTypeBlob2CsiAzure MountType = "azure-blob2"
	// MountTypeNfsCsiAzure Use of azure/csi driver for NFS in Azure storage account
	MountTypeNfsCsiAzure MountType = "azure-nfs"
	// MountTypeFileCsiAzure Use of azure/csi driver for files in Azure storage account
	MountTypeFileCsiAzure MountType = "azure-file"
)

// These are valid storage class provisioners
const (
	// ProvisionerBlobCsiAzure Use of azure/csi driver for blob in Azure storage account
	ProvisionerBlobCsiAzure string = "blob.csi.azure.com"
	// ProvisionerFileCsiAzure Use of azure/csi driver for files in Azure storage account
	ProvisionerFileCsiAzure string = "file.csi.azure.com"
)

// GetStorageClassProvisionerByVolumeMountType convert volume mount type to Storage Class provisioner
func GetStorageClassProvisionerByVolumeMountType(volumeMountType MountType) (string, bool) {
	switch volumeMountType {
	case MountTypeBlobCsiAzure, MountTypeBlob2CsiAzure, MountTypeNfsCsiAzure:
		return ProvisionerBlobCsiAzure, true
	case MountTypeFileCsiAzure:
		return ProvisionerFileCsiAzure, true
	}
	return "", false
}

// GetCsiAzureStorageClassProvisioners CSI Azure provisioners
func GetCsiAzureStorageClassProvisioners() []string {
	return []string{ProvisionerBlobCsiAzure, ProvisionerFileCsiAzure}
}

// IsKnownVolumeMount Gets if volume mount is supported
func IsKnownVolumeMount(volumeMount string) bool {
	return IsKnownBlobFlexVolumeMount(volumeMount) ||
		IsKnownCsiAzureVolumeMount(volumeMount)
}

// IsKnownCsiAzureVolumeMount Supported volume mount type CSI Azure Blob volume
func IsKnownCsiAzureVolumeMount(volumeMount string) bool {
	switch volumeMount {
	case string(MountTypeBlobCsiAzure), string(MountTypeBlob2CsiAzure), string(MountTypeFileCsiAzure):
		return true
	}
	return false
}

// IsKnownBlobFlexVolumeMount Supported volume mount type Azure Blobfuse
func IsKnownBlobFlexVolumeMount(volumeMount string) bool {
	return volumeMount == string(MountTypeBlob)
}

// RadixNode defines node attributes, where container should be scheduled
type RadixNode struct {
	// Defines rules for allowed GPU types.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#gpu
	// +optional
	Gpu string `json:"gpu,omitempty"`

	// Defines minimum number of required GPUs.
	// +optional
	GpuCount string `json:"gpuCount,omitempty"`
}

// MonitoringConfig Monitoring configuration
type MonitoringConfig struct {
	// Defines which port in the ports list where metrics is served.
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	PortName string `json:"portName,omitempty"`

	// Defines the path where metrics is served.
	// +optional
	Path string `json:"path,omitempty"`
}

// RadixSecretRefType Radix secret-ref of type
type RadixSecretRefType string

const (
	// RadixSecretRefTypeAzureKeyVault Radix secret-ref of type Azure Key vault
	RadixSecretRefTypeAzureKeyVault RadixSecretRefType = "az-keyvault"
)

// RadixSecretRefs defines secret vault
type RadixSecretRefs struct {
	// List of Azure Key Vaults to get secrets from.
	// +optional
	AzureKeyVaults []RadixAzureKeyVault `json:"azureKeyVaults,omitempty"`
}

// RadixAzureKeyVault defines an Azure keyvault.
type RadixAzureKeyVault struct {
	// Name of the Azure keyvault.
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=24
	Name string `json:"name"`

	// Path where secrets from the keyvault is mounted.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Path *string `json:"path,omitempty"`

	// List of keyvault items (secrets, keys and certificates).
	// +kubebuilder:validation:MinItems=1
	Items []RadixAzureKeyVaultItem `json:"items"`

	// UseAzureIdentity defines that credentials for accessing Azure Key Vault will be acquired using Azure Workload Identity instead of using a ClientID and Secret.
	// +optional
	UseAzureIdentity *bool `json:"useAzureIdentity,omitempty"`
}

// RadixAzureKeyVaultObjectType Azure Key Vault item type
type RadixAzureKeyVaultObjectType string

const (
	// RadixAzureKeyVaultObjectTypeSecret Azure Key Vault item of type secret
	RadixAzureKeyVaultObjectTypeSecret RadixAzureKeyVaultObjectType = "secret"
	// RadixAzureKeyVaultObjectTypeKey Azure Key Vault item of type key
	RadixAzureKeyVaultObjectTypeKey RadixAzureKeyVaultObjectType = "key"
	// RadixAzureKeyVaultObjectTypeCert Azure Key Vault item of type certificate
	RadixAzureKeyVaultObjectTypeCert RadixAzureKeyVaultObjectType = "cert"
)

// RadixAzureKeyVaultK8sSecretType Azure Key Vault secret item Kubernetes type
type RadixAzureKeyVaultK8sSecretType string

const (
	// RadixAzureKeyVaultK8sSecretTypeOpaque Azure Key Vault secret item Kubernetes type Opaque
	RadixAzureKeyVaultK8sSecretTypeOpaque RadixAzureKeyVaultK8sSecretType = "opaque"
	// RadixAzureKeyVaultK8sSecretTypeTls Azure Key Vault secret item Kubernetes type kubernetes.io/tls
	RadixAzureKeyVaultK8sSecretTypeTls RadixAzureKeyVaultK8sSecretType = "tls"
)

// RadixAzureKeyVaultItem defines Azure Key Vault setting: secrets, keys, certificates
type RadixAzureKeyVaultItem struct {
	// Name of a secret, key or certificate in the keyvault.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=127
	Name string `json:"name"`

	// Defines the name of the environment variable that will contain the value of the secret, key or certificate.
	// +optional
	EnvVar string `json:"envVar,omitempty"`

	// Type of item in the keyvault referenced by the name.
	// +kubebuilder:validation:Enum=secret;key;cert
	// +optional
	Type *RadixAzureKeyVaultObjectType `json:"type,omitempty"`

	// Alias overrides the default file name used when mounting the secret, key or certificate.
	// +kubebuilder:validation:MinLength=1
	// +optional
	Alias *string `json:"alias,omitempty"`

	// Defines that a specific version of a keyvault item should be loaded.
	// The latest version is loaded when this field is not set.
	// +optional
	Version *string `json:"version,omitempty"`

	// Defines the format of the keyvault item.
	// pfx is only supported with type secret and PKCS12 or ECC certificate.
	// Default format for certificates is pem.
	// +kubebuilder:validation:Enum=pem;pfx
	// +optional
	Format *string `json:"format,omitempty"`

	// Encoding defines the encoding of a keyvault item when stored in the container.
	// Setting encoding to base64 and format to pfx will fetch and write the base64 decoded pfx binary.
	// +kubebuilder:validation:Enum=base64
	// +optional
	Encoding *string `json:"encoding,omitempty"`

	// K8sSecretType defines the type of Kubernetes secret the keyvault item will be stored in.
	// opaque corresponds to "Opaque" and "kubernetes.io/tls" secret types: https://kubernetes.io/docs/concepts/configuration/secret/#secret-types
	// +kubebuilder:validation:Enum=opaque;tls
	// +optional
	K8sSecretType *RadixAzureKeyVaultK8sSecretType `json:"k8sSecretType,omitempty"`
}

// Authentication describes authentication options.
type Authentication struct {
	// Configuration for TLS client certificate authentication.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#clientcertificate
	// +optional
	ClientCertificate *ClientCertificate `json:"clientCertificate,omitempty"`

	// Configuration for OAuth2 authentication.
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#oauth2
	// +optional
	OAuth2 *OAuth2 `json:"oauth2,omitempty"`
}

// ClientCertificate Authentication client certificate parameters
type ClientCertificate struct {
	// Defines how the client certificate shall be verified.
	// +kubebuilder:validation:Enum=on;off;optional;optional_no_ca
	// +optional
	Verification *VerificationType `json:"verification,omitempty"`

	// Pass client certificate to backend in header ssl-client-cert.
	// This setting has no effect if verification is set to off.
	// +optional
	PassCertificateToUpstream *bool `json:"passCertificateToUpstream,omitempty"`
}

// SessionStoreType type of session store
type SessionStoreType string

const (
	// SessionStoreCookie use cookies for session store
	SessionStoreCookie SessionStoreType = "cookie"
	// SessionStoreRedis use redis for session store
	SessionStoreRedis SessionStoreType = "redis"
)

// VerificationType Certificate verification type
type VerificationType string

const (
	// VerificationTypeOff Certificate verification is off
	VerificationTypeOff VerificationType = "off"
	// VerificationTypeOn Certificate verification is on
	VerificationTypeOn VerificationType = "on"
	// VerificationTypeOptional Certificate verification is optional
	VerificationTypeOptional VerificationType = "optional"
	// VerificationTypeOptionalNoCa Certificate verification is optional no certificate authority
	VerificationTypeOptionalNoCa VerificationType = "optional_no_ca"
)

// CookieSameSiteType Cookie SameSite value
type CookieSameSiteType string

const (
	// SameSiteStrict Use strict as samesite for cookie
	SameSiteStrict CookieSameSiteType = "strict"
	// SameSiteLax Use lax as samesite for cookie
	SameSiteLax CookieSameSiteType = "lax"
	// SameSiteNone Use none as samesite for cookie. Not supported by IE. See compativility matrix https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie/SameSite#browser_compatibility
	SameSiteNone CookieSameSiteType = "none"
	// SameSiteEmpty Use empty string as samesite for cookie. Modern browsers defaults to lax when SameSite is not set. See compatibility matrix https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie/SameSite#browser_compatibility
	SameSiteEmpty CookieSameSiteType = ""
)

// OAuth2 defines oauth proxy settings for a component
type OAuth2 struct {
	// Client ID of the application.
	// +optional
	ClientID string `json:"clientId"`

	// Requested scopes.
	// +optional
	Scope string `json:"scope,omitempty"`

	// Defines if claims from the access token is added to the X-Auth-Request-User, X-Auth-Request-Groups,
	// X-Auth-Request-Email and X-Auth-Request-Preferred-Username request headers.
	// The access token is passed in the X-Auth-Request-Access-Token header.
	// +optional
	SetXAuthRequestHeaders *bool `json:"setXAuthRequestHeaders,omitempty"`

	// Defines if the IDToken received by the OAuth Proxy should be added to the Authorization header.
	// +optional
	SetAuthorizationHeader *bool `json:"setAuthorizationHeader,omitempty"`

	// Defines the url root path that OAuth Proxy should be nested under.
	// +optional
	ProxyPrefix string `json:"proxyPrefix,omitempty"`

	// Defines the authentication endpoint of the identity provider.
	// Must be set if OIDC.SkipDiscovery is true
	// +optional
	LoginURL string `json:"loginUrl,omitempty"`

	// Defines the endpoint to redeem the authorization code received from the OAuth code flow.
	// Must be set if OIDC.SkipDiscovery is true
	// +optional
	RedeemURL string `json:"redeemUrl,omitempty"`

	// OIDC settings.
	// +optional
	OIDC *OAuth2OIDC `json:"oidc,omitempty"`

	// Session cookie settings.
	// +optional
	Cookie *OAuth2Cookie `json:"cookie,omitempty"`

	// Defines where to store session data.
	// +kubebuilder:validation:Enum=cookie;redis;""
	// +optional
	SessionStoreType SessionStoreType `json:"sessionStoreType,omitempty"`

	// Settings for the cookie that stores session data when SessionStoreType is cookie.
	// +optional
	CookieStore *OAuth2CookieStore `json:"cookieStore,omitempty"`

	// Settings for Redis store when SessionStoreType is redis.
	// +optional
	RedisStore *OAuth2RedisStore `json:"redisStore,omitempty"`
}

// OAuth2Cookie defines properties for the oauth cookie.
type OAuth2Cookie struct {
	// Defines the name of the OAuth session cookie.
	// +optional
	Name string `json:"name,omitempty"`

	// Defines the expire timeframe for the session cookie.
	// +optional
	Expire string `json:"expire,omitempty"`

	// The interval between cookie refreshes.
	// The value must be a shorter timeframe than values set in Expire.
	// +optional
	Refresh string `json:"refresh,omitempty"`

	// Defines the samesite cookie attribute
	// +kubebuilder:validation:Enum=strict;lax;none;""
	// +optional
	SameSite CookieSameSiteType `json:"sameSite,omitempty"`
}

// OAuth2OIDC defines OIDC settings for oauth proxy.
type OAuth2OIDC struct {
	// Defines the OIDC issuer URL.
	// +optional
	IssuerURL string `json:"issuerUrl,omitempty"`

	// Defines the OIDC JWKS URL for token verification.
	// Required if OIDC discovery is disabled.
	// +optional
	JWKSURL string `json:"jwksUrl,omitempty"`

	// Defines if OIDC endpoint discovery should be bypassed.
	// LoginURL, RedeemURL, JWKSURL must be configured if discovery is disabled.
	// +optional
	SkipDiscovery *bool `json:"skipDiscovery,omitempty"`

	// Skip verifying the OIDC ID Token's nonce claim
	// +optional
	InsecureSkipVerifyNonce *bool `json:"insecureSkipVerifyNonce,omitempty"`
}

// OAuth2RedisStore properties for redis session storage.
type OAuth2RedisStore struct {
	// Defines the URL for the Redis server.
	ConnectionURL string `json:"connectionUrl"`
}

// OAuth2CookieStore properties for cookie session storage.
type OAuth2CookieStore struct {
	// Strips OAuth tokens from cookies if they are not needed.
	// Cookie.Refresh must be 0, and both SetXAuthRequestHeaders and SetAuthorizationHeader must be false if this setting is true.
	// +optional
	Minimal *bool `json:"minimal,omitempty"`
}

// Identity configuration for federation with external identity providers.
type Identity struct {
	// Azure identity configuration
	// +optional
	Azure *AzureIdentity `json:"azure,omitempty"`
}

// AzureIdentity properties for Azure AD Workload Identity
type AzureIdentity struct {
	// Defines the Client ID for a user defined managed identity or application ID for an application registration.
	ClientId string `json:"clientId"`
}

// Notifications is the spec for notification about internal events or changes
type Notifications struct {
	// Webhook is a URL for notification about internal events or changes. The URL should be of a Radix component or job-component, with not public port.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Webhook *string `json:"webhook,omitempty"`
}

// RadixCommonComponent defines a common component interface for Radix components
type RadixCommonComponent interface {
	// GetName Gets component name
	GetName() string
	// GetSourceFolder Gets component source folder
	GetSourceFolder() string
	// GetImage Gets component image
	GetImage() string
	// GetNode Gets component node parameters
	GetNode() *RadixNode
	// GetVariables Gets component environment variables
	GetVariables() EnvVarsMap
	// GetPorts Gets component ports
	GetPorts() []ComponentPort
	// GetMonitoringConfig Gets component monitoring configuration
	GetMonitoringConfig() MonitoringConfig
	// GetSecrets Gets component secrets
	GetSecrets() []string
	// GetSecretRefs Gets component secret-refs
	GetSecretRefs() RadixSecretRefs
	// GetResources Gets component resources
	GetResources() ResourceRequirements
	// GetIdentity Get component identity
	GetIdentity() *Identity
	// GetEnvironmentConfig Gets component environment configuration
	GetEnvironmentConfig() []RadixCommonEnvironmentConfig
	// GetEnvironmentConfigsMap Get component environment configuration as map by names
	GetEnvironmentConfigsMap() map[string]RadixCommonEnvironmentConfig
	// getEnabled Gets the component status if it is enabled in the application
	getEnabled() bool
	// GetEnabledForEnv Gets the component status if it is enabled in the application for an environment
	GetEnabledForEnv(RadixCommonEnvironmentConfig) bool
	// GetEnvironmentConfigByName  Gets component environment configuration by its name
	GetEnvironmentConfigByName(environment string) RadixCommonEnvironmentConfig
	GetEnabledForAnyEnvironment(environments []string) bool
}

func (component *RadixComponent) GetName() string {
	return component.Name
}

func (component *RadixComponent) GetSourceFolder() string {
	return component.SourceFolder
}

func (component *RadixComponent) GetImage() string {
	return component.Image
}

func (component *RadixComponent) GetNode() *RadixNode {
	return &component.Node
}

func (component *RadixComponent) GetVariables() EnvVarsMap {
	return component.Variables
}

func (component *RadixComponent) GetPorts() []ComponentPort {
	return component.Ports
}

func (component *RadixComponent) GetMonitoringConfig() MonitoringConfig {
	return component.MonitoringConfig
}

func (component *RadixComponent) GetSecrets() []string {
	return component.Secrets
}

func (component *RadixComponent) GetSecretRefs() RadixSecretRefs {
	return component.SecretRefs
}

func (component *RadixComponent) GetResources() ResourceRequirements {
	return component.Resources
}

func (component *RadixComponent) GetIdentity() *Identity {
	return component.Identity
}

func (component *RadixComponent) getEnabled() bool {
	return component.Enabled == nil || *component.Enabled
}

func (component *RadixComponent) GetEnvironmentConfig() []RadixCommonEnvironmentConfig {
	var environmentConfigs []RadixCommonEnvironmentConfig
	for _, environmentConfig := range component.EnvironmentConfig {
		environmentConfigs = append(environmentConfigs, environmentConfig)
	}
	return environmentConfigs
}

func (component *RadixComponent) GetEnvironmentConfigsMap() map[string]RadixCommonEnvironmentConfig {
	return getEnvironmentConfigMap(component)
}

func getEnvironmentConfigMap(component RadixCommonComponent) map[string]RadixCommonEnvironmentConfig {
	environmentConfigsMap := make(map[string]RadixCommonEnvironmentConfig)
	for _, environmentConfig := range component.GetEnvironmentConfig() {
		config := environmentConfig
		environmentConfigsMap[environmentConfig.GetEnvironment()] = config
	}
	return environmentConfigsMap
}

func (component *RadixComponent) GetEnvironmentConfigByName(environment string) RadixCommonEnvironmentConfig {
	return getEnvironmentConfigByName(environment, component.GetEnvironmentConfig())
}

func (component *RadixComponent) GetEnabledForEnv(envConfig RadixCommonEnvironmentConfig) bool {
	return getEnabled(component, envConfig)
}

func (component *RadixComponent) GetEnabledForAnyEnvironment(environments []string) bool {
	return getEnabledForAnyEnvironment(component, environments)
}

func (component *RadixJobComponent) GetEnabledForEnv(envConfig RadixCommonEnvironmentConfig) bool {
	return getEnabled(component, envConfig)
}

func getEnabled(component RadixCommonComponent, envConfig RadixCommonEnvironmentConfig) bool {
	if commonUtils.IsNil(envConfig) || envConfig.getEnabled() == nil {
		return component.getEnabled()
	}
	return *envConfig.getEnabled()
}

func (component *RadixJobComponent) GetName() string {
	return component.Name
}

func (component *RadixJobComponent) GetSourceFolder() string {
	return component.SourceFolder
}

func (component *RadixJobComponent) GetImage() string {
	return component.Image
}

func (component *RadixJobComponent) GetNode() *RadixNode {
	return &component.Node
}

func (component *RadixJobComponent) GetVariables() EnvVarsMap {
	return component.Variables
}

func (component *RadixJobComponent) GetPorts() []ComponentPort {
	return component.Ports
}

func (component *RadixJobComponent) GetMonitoringConfig() MonitoringConfig {
	return component.MonitoringConfig
}

func (component *RadixJobComponent) GetSecrets() []string {
	return component.Secrets
}

func (component *RadixJobComponent) GetSecretRefs() RadixSecretRefs {
	return component.SecretRefs
}

func (component *RadixJobComponent) GetResources() ResourceRequirements {
	return component.Resources
}

func (component *RadixJobComponent) GetIdentity() *Identity {
	return component.Identity
}

// GetNotifications Get job component notifications
func (component *RadixJobComponent) GetNotifications() *Notifications {
	return component.Notifications
}

func (component *RadixJobComponent) getEnabled() bool {
	return component.Enabled == nil || *component.Enabled
}

func (component *RadixJobComponent) GetEnvironmentConfig() []RadixCommonEnvironmentConfig {
	var environmentConfigs []RadixCommonEnvironmentConfig
	for _, environmentConfig := range component.EnvironmentConfig {
		environmentConfigs = append(environmentConfigs, environmentConfig)
	}
	return environmentConfigs
}

func (component *RadixJobComponent) GetEnvironmentConfigsMap() map[string]RadixCommonEnvironmentConfig {
	return getEnvironmentConfigMap(component)
}

func (component *RadixJobComponent) GetVolumeMountsForEnvironment(env string) []RadixVolumeMount {
	for _, envConfig := range component.EnvironmentConfig {
		if strings.EqualFold(env, envConfig.Environment) {
			return envConfig.VolumeMounts
		}
	}
	return nil
}

func (component *RadixJobComponent) GetEnvironmentConfigByName(environment string) RadixCommonEnvironmentConfig {
	return getEnvironmentConfigByName(environment, component.GetEnvironmentConfig())
}

func (component *RadixJobComponent) GetEnabledForAnyEnvironment(environments []string) bool {
	return getEnabledForAnyEnvironment(component, environments)
}

func getEnvironmentConfigByName(environment string, environmentConfigs []RadixCommonEnvironmentConfig) RadixCommonEnvironmentConfig {
	for _, environmentConfig := range environmentConfigs {
		if strings.EqualFold(environment, environmentConfig.GetEnvironment()) {
			return environmentConfig
		}
	}
	return nil
}

func getEnabledForAnyEnvironment(component RadixCommonComponent, environments []string) bool {
	environmentConfigsMap := component.GetEnvironmentConfigsMap()
	if len(environmentConfigsMap) == 0 {
		return component.getEnabled()
	}
	for _, envName := range environments {
		if component.GetEnabledForEnv(environmentConfigsMap[envName]) {
			return true
		}
	}
	return false
}
