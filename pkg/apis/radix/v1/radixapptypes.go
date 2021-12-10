package v1

import (
	"strings"

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
// The job component is used by the radix-job-scheduler-server to create Kubernetes Job objects
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

// RadixJobComponentPayload defines the path and where the payload received by radix-job-scheduler-server
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
	Type            MountType `json:"type" yaml:"type"`
	Name            string    `json:"name" yaml:"name"`
	Container       string    `json:"container" yaml:"container"`             //Outdated. Use Storage instead
	Storage         string    `json:"storage" yaml:"storage"`                 //Container name, file Share name, etc.
	Path            string    `json:"path" yaml:"path"`                       //Path within the pod (replica), where the volume mount has been mounted to
	GID             string    `json:"gid" yaml:"gid"`                         //Optional. Volume mount owner GroupID. Used when drivers do not honor fsGroup securityContext setting. https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/driver-parameters.md
	UID             string    `json:"uid" yaml:"uid"`                         //Optional. Volume mount owner UserID. Used instead of GID.
	SkuName         string    `json:"skuName" yaml:"skuName"`                 //Available values: Standard_LRS (default), Premium_LRS, Standard_GRS, Standard_RAGRS. https://docs.microsoft.com/en-us/rest/api/storagerp/srp_sku_types
	RequestsStorage string    `json:"requestsStorage" yaml:"requestsStorage"` //Requests resource storage size. Default "1Mi". https://kubernetes.io/docs/tasks/configure-pod-container/configure-persistent-volume-storage/#create-a-persistentvolumeclaim
	AccessMode      string    `json:"accessMode" yaml:"accessMode"`           //Available values: ReadOnlyMany (default) - read-only by many nodes, ReadWriteOnce - read-write by a single node, ReadWriteMany - read-write by many nodes. https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes
	BindingMode     string    `json:"bindingMode" yaml:"bindingMode"`         //Volume binding mode. Available values: Immediate (default), WaitForFirstConsumer. https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode
}

// MountType Holds types of mount
type MountType string

// These are valid types of mount
const (
	// MountTypeBlob Use of azure/blobfuse flexvolume
	MountTypeBlob MountType = "blob"
	// MountTypeBlobCsiAzure Use of azure/csi driver for blob in Azure storage account
	MountTypeBlobCsiAzure MountType = "azure-blob"
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

//GetStorageClassProvisionerByVolumeMountType convert volume mount type to Storage Class provisioner
func GetStorageClassProvisionerByVolumeMountType(volumeMountType MountType) (string, bool) {
	switch volumeMountType {
	case MountTypeBlobCsiAzure:
		return ProvisionerBlobCsiAzure, true
	case MountTypeFileCsiAzure:
		return ProvisionerFileCsiAzure, true
	}
	return "", false
}

//GetCsiAzureStorageClassProvisioners CSI Azure provisioners
func GetCsiAzureStorageClassProvisioners() []string {
	return []string{ProvisionerBlobCsiAzure, ProvisionerFileCsiAzure}
}

func IsKnownVolumeMount(volumeMount string) bool {
	return IsKnownBlobFlexVolumeMount(volumeMount) ||
		IsKnownCsiAzureVolumeMount(volumeMount)
}

func IsKnownCsiAzureVolumeMount(volumeMount string) bool {
	switch volumeMount {
	case string(MountTypeBlobCsiAzure), string(MountTypeFileCsiAzure):
		return true
	}
	return false
}

func IsKnownBlobFlexVolumeMount(volumeMount string) bool {
	return volumeMount == string(MountTypeBlob)
}

// RadixNode defines node attributes, where container should be scheduled
type RadixNode struct {
	// Gpu Optional. Holds lists of node GPU types, with dashed types to exclude
	Gpu string `json:"gpu" yaml:"gpu"`
	// GpuCount Optional. Holds minimum count of GPU on node
	GpuCount string `json:"gpuCount" yaml:"gpuCount"`
}

type VerificationType string

const (
	VerificationTypeOff          VerificationType = "off"
	VerificationTypeOn           VerificationType = "on"
	VerificationTypeOptional     VerificationType = "optional"
	VerificationTypeOptionalNoCa VerificationType = "optional_no_ca"
)

type Authentication struct {
	ClientCertificate *ClientCertificate `json:"clientCertificate,omitempty" yaml:"clientCertificate,omitempty"`
	OAuth2            *OAuth2            `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`
}

type ClientCertificate struct {
	Verification              *VerificationType `json:"verification,omitempty" yaml:"verification,omitempty"`
	PassCertificateToUpstream *bool             `json:"passCertificateToUpstream,omitempty" yaml:"passCertificateToUpstream,omitempty"`
}

// SessionStoreType defines type of session storage
type SessionStoreType string

const (
	// Cookie session storage
	SessionStoreCookie SessionStoreType = "cookie"
	// Redis session storage
	SessionStoreRedis SessionStoreType = "redis"
)

// OAuth2 defines oauth proxy settings for the component
type OAuth2 struct {
	// The OAUTH client ID
	ClientID string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	// The OAUTH scope specification
	Scope string `json:"scope,omitempty" yaml:"scope,omitempty"`
	// SetXAuthRequestHeaders adds X-Auth-Request-User, X-Auth-Request-Groups, X-Auth-Request-Email,
	// X-Auth-Request-Preferred-Username and X-Auth-Request-Access-Token to the response headers.
	// Values in the headers are extracted from the OAUTH access token
	SetXAuthRequestHeaders *bool `json:"setXAuthRequestHeaders,omitempty" yaml:"setXAuthRequestHeaders,omitempty"`
	// SetAuthorizationHeader sets the OAUTH IDToken in the Authorization Bearer headers
	SetAuthorizationHeader *bool `json:"setAuthorizationHeader,omitempty" yaml:"setAuthorizationHeader,omitempty"`
	// Authenticate emails with the specified domain. A comma separated list of domain may be specified
	EmailDomain string `json:"emailDomain,omitempty" yaml:"emailDomain,omitempty"`
	// The URL root path that the oauth proxy should be nested under
	ProxyPrefix string `json:"proxyPrefix,omitempty" yaml:"proxyPrefix,omitempty"`
	// Authentication endpoint
	LoginURL string `json:"loginUrl,omitempty" yaml:"loginUrl,omitempty"`
	// Endpoint to redeem the authorization code
	RedeemURL string `json:"redeemURL,omitempty" yaml:"redeemURL,omitempty"`
	// OIDC settings
	OIDC *OAuth2OIDC `json:"oidc,omitempty" yaml:"oidc,omitempty"`
	// Cookie settings
	Cookie *OAuth2Cookie `json:"cookie,omitempty" yaml:"cookie,omitempty"`
	// Session store type for backend
	SessionStoreType SessionStoreType `json:"sessionStoreType,omitempty" yaml:"sessionStoreType,omitempty"`
	// Cookie session storage settings
	CookieStore *OAuth2CookieStore `json:"cookieStore,omitempty" yaml:"cookieStore,omitempty"`
	// Redis session storage settings
	RedisStore *OAuth2RedisStore `json:"redisStore,omitempty" yaml:"redisStore,omitempty"`
}

// OAuth2Cookie defines properties for the oauth cookie
type OAuth2Cookie struct {
	// Name of the cookie used to store information about the authenticated session
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	Domain   string `json:"domain,omitempty" yaml:"domain,omitempty"`
	Expire   string `json:"expire,omitempty" yaml:"expire,omitempty"`
	Refresh  string `json:"refresh,omitempty" yaml:"refresh,omitempty"`
	SameSite string `json:"sameSite,omitempty" yaml:"sameSite,omitempty"`
}

// OAuth2OIDC defines OIDC properties for oauth proxy
type OAuth2OIDC struct {
	IssuerURL               string `json:"issuerUrl,omitempty" yaml:"issuerUrl,omitempty"`
	JWKSURL                 string `json:"jwksUrl,omitempty" yaml:"jwksUrl,omitempty"`
	EmailClaim              string `json:"emailClaim,omitempty" yaml:"emailClaim,omitempty"`
	GroupsClaim             string `json:"groupsClaim,omitempty" yaml:"groupsClaim,omitempty"`
	SkipDiscovery           *bool  `json:"skipDiscovery,omitempty" yaml:"skipDiscovery,omitempty"`
	InsecureSkipVerifyNonce *bool  `json:"insecureSkipVerifyNonce,omitempty" yaml:"insecureSkipVerifyNonce,omitempty"`
}

// OAuth2RedisStore properties for redis session storage
type OAuth2RedisStore struct {
	ConnectionURL string `json:"connectionUrl,omitempty" yaml:"connectionUrl,omitempty"`
}

// OAuth2CookieStore properties for cookie session storage
type OAuth2CookieStore struct {
	Minimal *bool `json:"minimal,omitempty" yaml:"minimal,omitempty"`
}

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

func (component *RadixJobComponent) GetVolumeMountsForEnvironment(env string) []RadixVolumeMount {
	for _, envConfig := range component.EnvironmentConfig {
		if strings.EqualFold(env, envConfig.Environment) {
			return envConfig.VolumeMounts
		}
	}
	return nil
}
