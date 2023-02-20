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

	// Specification for an application
	Spec RadixApplicationSpec `json:"spec"`
}

// RadixApplicationSpec is the spec for an application.
// More info: https://www.radix.equinor.com/references/reference-radix-config/
type RadixApplicationSpec struct {
	// Defines build configuration
	// +optional
	Build *BuildSpec `json:"build,omitempty"`

	// Defines the environments for the application.
	// +listType:=map
	// +listMapKey:=name
	// +kubebuilder:validation:MinItems:=1
	Environments []Environment `json:"environments"`

	// Defines the jobs for the application.
	// +listType:=map
	// +listMapKey:=name
	// +optional
	Jobs []RadixJobComponent `json:"jobs,omitempty"`

	// Defines the components for the application.
	// +listType:=map
	// +listMapKey:=name
	// +kubebuilder:validation:MinItems:=1
	Components []RadixComponent `json:"components"`

	// +optional
	DNSAppAlias AppAlias `json:"dnsAppAlias,omitempty"`

	// Defines mapping between external DNS aliases and components.
	// +listType:=map
	// +listMapKey:=alias
	// +optional
	DNSExternalAlias []ExternalAlias `json:"dnsExternalAlias,omitempty"`

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

// BuildSpec contains configuration used when building the application
type BuildSpec struct {
	// Defines a list of secrets to be used in Dockerfile or sub-pipelines
	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// Defines variables that will be available in sub-pipelines
	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// Enables BuildKit when for docker build.
	// More info about BuildKit: https://docs.docker.com/build/buildkit/
	// +optional
	UseBuildKit *bool `json:"useBuildKit,omitempty"`
}

// Defines an application environment
type Environment struct {
	// Name of the environment
	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Pattern:=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Name string `json:"name"`

	// +optional
	Build EnvBuild `json:"build,omitempty"`

	// Defines egress configuration for the environment
	// More info: https://www.radix.equinor.com/references/reference-radix-config/#egress
	// +optional
	Egress EgressConfig `json:"egress,omitempty"`
}

// Defines build parameters
type EnvBuild struct {
	// Name of the Github branch to build from
	// +kubebuilder:validation:MaxLength:=255
	// +optional
	From string `json:"from,omitempty"`

	// Defines variables that will be available in sub-pipelines
	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`
}

// Defines an egress configuration
type EgressConfig struct {
	// Allow or deny outgoing traffic to the public IP for the Radix cluster.
	// +optional
	AllowRadix *bool `json:"allowRadix,omitempty"`

	// Defines a list of egress rules
	// +kubebuilder:validation:MaxItems:=1000
	// +optional
	Rules []EgressRule `json:"rules,omitempty"`
}

// EgressRule defines an egress rule in network policy
type EgressRule struct {
	// List of allowed destinations
	// Each destination must be a valid CIDR
	// +kubebuilder:validation:MinItems:=1
	Destinations []string `json:"destinations"`

	// +kubebuilder:validation:MinItems:=1
	Ports []EgressPort `json:"ports"`
}

// EgressPort defines a port in context of EgressRule
type EgressPort struct {
	// +kubebuilder:validation:Minimum:=1024
	// +kubebuilder:validation:Maximum:=65535
	Port int32 `json:"port"`

	// +kubebuilder:validation:Enum=TCP;UDP
	Protocol string `json:"protocol"`
}

// Defines a URL alias for this application.
// The URL will be of form <app-name>.app.radix.equinor.com
type AppAlias struct {
	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Pattern:=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Environment string `json:"environment"`

	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Pattern:=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Component string `json:"component"`
}

// ExternalAlias defines a URL alias for this application with ability to bring-your-own certificate
type ExternalAlias struct {
	// +kubebuilder:validation:MaxLength:=255
	Alias string `json:"alias"`

	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Pattern:=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Environment string `json:"environment"`

	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Pattern:=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Component string `json:"component"`
}

// ComponentPort defines the port number, protocol and port for a service
type ComponentPort struct {
	// +kubebuilder:validation:MaxLength:=15
	Name string `json:"name"`

	// +kubebuilder:validation:Minimum:=1024
	// +kubebuilder:validation:Maximum:=65535
	Port int32 `json:"port"`
}

// ResourceList Placeholder for resouce specifications in the config
type ResourceList map[string]string

// ResourceRequirements describes the compute resource requirements.
// More info:
//   - https://www.radix.equinor.com/references/reference-radix-config/#resources-common
//   - https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// +optional
	Limits ResourceList `json:"limits,omitempty"`
	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if that is explicitly specified,
	// otherwise to an implementation-defined value.
	// +optional
	Requests ResourceList `json:"requests,omitempty"`
}

// RadixComponent defines a single component within a RadixApplication - maps to single deployment/service/ingress etc
type RadixComponent struct {
	// +kubebuilder:validation:Pattern:=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Name string `json:"name"`

	// +optional
	SourceFolder string `json:"src,omitempty"`

	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	DockerfileName string `json:"dockerfileName,omitempty"`

	// +kubebuilder:validation:MinItems:=1
	// +listType:=map
	// +listMapKey:=name
	Ports []ComponentPort `json:"ports"`

	// +optional
	MonitoringConfig MonitoringConfig `json:"monitoringConfig,omitempty"`

	// +optional
	Public bool `json:"public,omitempty"` // Deprecated: For backwards compatibility Public is still supported, new code should use PublicPort instead

	// +kubebuilder:validation:MaxLength:=15
	// +kubebuilder:validation:Pattern:=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	// +optional
	PublicPort string `json:"publicPort,omitempty"`

	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// +optional
	IngressConfiguration []string `json:"ingressConfiguration,omitempty"`

	// +optional
	EnvironmentConfig []RadixEnvironmentConfig `json:"environmentConfig,omitempty"`

	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// +optional
	AlwaysPullImageOnDeploy *bool `json:"alwaysPullImageOnDeploy,omitempty"`

	// +optional
	Node RadixNode `json:"node,omitempty"`

	// +optional
	Authentication *Authentication `json:"authentication,omitempty"`

	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// RadixEnvironmentConfig defines environment specific settings for a single component within a RadixApplication
type RadixEnvironmentConfig struct {
	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Pattern:=^(([a-z0-9][-a-z0-9.]*)?[a-z0-9])?$
	Environment string `json:"environment"`

	// +kubebuilder:validation:Minimum:=0
	// +optional
	Replicas *int `json:"replicas,omitempty"`

	// +optional
	Monitoring bool `json:"monitoring"`

	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// +optional
	HorizontalScaling *RadixHorizontalScaling `json:"horizontalScaling,omitempty"`

	// +optional
	ImageTagName string `json:"imageTagName,omitempty"`

	// +optional
	AlwaysPullImageOnDeploy *bool `json:"alwaysPullImageOnDeploy,omitempty"`

	// +optional
	VolumeMounts []RadixVolumeMount `json:"volumeMounts,omitempty"`

	// +optional
	Node RadixNode `json:"node,omitempty"`

	// +optional
	Authentication *Authentication `json:"authentication,omitempty"`

	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// RadixJobComponent defines a single job component within a RadixApplication
// The job component is used by the radix-job-scheduler-server to create Kubernetes Job objects
type RadixJobComponent struct {
	Name string `json:"name"`

	// +optional
	SourceFolder string `json:"src,omitempty"`

	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	DockerfileName string `json:"dockerfileName,omitempty"`

	// +optional
	SchedulerPort *int32 `json:"schedulerPort,omitempty"`

	// +optional
	Payload *RadixJobComponentPayload `json:"payload,omitempty"`

	// +optional
	Ports []ComponentPort `json:"ports,omitempty"`

	// +optional
	MonitoringConfig MonitoringConfig `json:"monitoringConfig,omitempty"`

	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// +optional
	EnvironmentConfig []RadixJobComponentEnvironmentConfig `json:"environmentConfig,omitempty"`

	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// +optional
	Node RadixNode `json:"node,omitempty"`

	// +optional
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// RadixJobComponentEnvironmentConfig defines environment specific settings
// for a single job component within a RadixApplication
type RadixJobComponentEnvironmentConfig struct {
	Environment string `json:"environment"`

	// +optional
	Monitoring bool `json:"monitoring,omitempty"`

	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// +optional
	Variables EnvVarsMap `json:"variables,omitempty"`

	// +optional
	ImageTagName string `json:"imageTagName,omitempty"`

	// +optional
	VolumeMounts []RadixVolumeMount `json:"volumeMounts,omitempty"`

	// +optional
	Node RadixNode `json:"node,omitempty"`

	// +optional
	SecretRefs RadixSecretRefs `json:"secretRefs,omitempty"`

	// +optional
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// +optional
	Identity *Identity `json:"identity,omitempty"`

	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// RadixJobComponentPayload defines the path and where the payload received by radix-job-scheduler-server
// will be mounted to the job container
type RadixJobComponentPayload struct {
	Path string `json:"path"`
}

// RadixHorizontalScaling defines configuration for horizontal pod autoscaler. It is kept as close as the HorizontalPodAutoscalerSpec
// If set, this will override replicas config
type RadixHorizontalScaling struct {
	// +kubebuilder:validation:Minimum:=0
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// +kubebuilder:validation:Minimum:=1
	MaxReplicas int32 `json:"maxReplicas"`
}

// Defines authentication information for private image registries.
type PrivateImageHubEntries map[string]*RadixPrivateImageHubCredential

// Defines the user name and email to use when pulling images
// from a private image repository.
type RadixPrivateImageHubCredential struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

// RadixVolumeMount defines volume to be mounted to the container
type RadixVolumeMount struct {
	// +kubebuilder:validation:Enum=blob;azure-blob;azure-file
	Type MountType `json:"type"`

	Name string `json:"name"`

	// +optional
	Container string `json:"container,omitempty"` //Outdated. Use Storage instead

	Storage string `json:"storage"` //Container name, file Share name, etc.

	Path string `json:"path"` //Path within the pod (replica), where the volume mount has been mounted to

	// +optional
	GID string `json:"gid,omitempty"` //Optional. Volume mount owner GroupID. Used when drivers do not honor fsGroup securityContext setting. https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/driver-parameters.md

	// +optional
	UID string `json:"uid,omitempty"` //Optional. Volume mount owner UserID. Used instead of GID.

	// +optional
	SkuName string `json:"skuName,omitempty"` //Available values: Standard_LRS (default), Premium_LRS, Standard_GRS, Standard_RAGRS. https://docs.microsoft.com/en-us/rest/api/storagerp/srp_sku_types

	// +optional
	RequestsStorage string `json:"requestsStorage,omitempty"` //Requests resource storage size. Default "1Mi". https://kubernetes.io/docs/tasks/configure-pod-container/configure-persistent-volume-storage/#create-a-persistentvolumeclaim

	// +kubebuilder:validation:Enum=ReadOnlyMany;ReadWriteOnce;ReadWriteMany
	// +optional
	AccessMode string `json:"accessMode,omitempty"` //Available values: ReadOnlyMany (default) - read-only by many nodes, ReadWriteOnce - read-write by a single node, ReadWriteMany - read-write by many nodes. https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes

	// +kubebuilder:validation:Enum=Immediate;WaitForFirstConsumer
	// +optional
	BindingMode string `json:"bindingMode,omitempty"` //Volume binding mode. Available values: Immediate (default), WaitForFirstConsumer. https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode
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

// GetStorageClassProvisionerByVolumeMountType convert volume mount type to Storage Class provisioner
func GetStorageClassProvisionerByVolumeMountType(volumeMountType MountType) (string, bool) {
	switch volumeMountType {
	case MountTypeBlobCsiAzure:
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
	case string(MountTypeBlobCsiAzure), string(MountTypeFileCsiAzure):
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
	// Gpu Optional. Holds lists of node GPU types, with dashed types to exclude
	Gpu string `json:"gpu"`

	// GpuCount Optional. Holds minimum count of GPU on node
	// +optional
	GpuCount string `json:"gpuCount,omitempty"`
}

// MonitoringConfig Monitoring configuration
type MonitoringConfig struct {
	// Port name
	PortName string `json:"portName"`

	// HTTP path to scrape for metrics.
	// +optional
	Path string `json:"path,omitempty"`
}

// RadixSecretRefType Radix secret-ref of type
type RadixSecretRefType string

const (
	//RadixSecretRefTypeAzureKeyVault Radix secret-ref of type Azure Key vault
	RadixSecretRefTypeAzureKeyVault RadixSecretRefType = "az-keyvault"
)

// RadixSecretRefs defines secret vault
type RadixSecretRefs struct {
	// AzureKeyVaults. List of RadixSecretRefs-s, containing Azure Key Vault configurations
	// +optional
	AzureKeyVaults []RadixAzureKeyVault `json:"azureKeyVaults"`
}

// RadixAzureKeyVault defines Azure Key Vault
type RadixAzureKeyVault struct {
	// Name. Name of the Azure Key Vault
	Name string `json:"name"`

	// Path. Optional. Path within replicas, where secrets are mapped as files. Default: /mnt/azure-key-vault/<key-vault-name>/<component-name>
	// +optional
	Path *string `json:"path,omitempty"`

	// Items. Azure Key Vault items
	// +kubebuilder:validation:MinItems:=1
	Items []RadixAzureKeyVaultItem `json:"items"`
}

// RadixAzureKeyVaultObjectType Azure Key Vault item type
type RadixAzureKeyVaultObjectType string

const (
	//RadixAzureKeyVaultObjectTypeSecret Azure Key Vault item of type secret
	RadixAzureKeyVaultObjectTypeSecret RadixAzureKeyVaultObjectType = "secret"
	//RadixAzureKeyVaultObjectTypeKey Azure Key Vault item of type key
	RadixAzureKeyVaultObjectTypeKey RadixAzureKeyVaultObjectType = "key"
	//RadixAzureKeyVaultObjectTypeCert Azure Key Vault item of type certificate
	RadixAzureKeyVaultObjectTypeCert RadixAzureKeyVaultObjectType = "cert"
)

// RadixAzureKeyVaultK8sSecretType Azure Key Vault secret item Kubernetes type
type RadixAzureKeyVaultK8sSecretType string

const (
	//RadixAzureKeyVaultK8sSecretTypeOpaque Azure Key Vault secret item Kubernetes type Opaque
	RadixAzureKeyVaultK8sSecretTypeOpaque RadixAzureKeyVaultK8sSecretType = "opaque"
	//RadixAzureKeyVaultK8sSecretTypeTls Azure Key Vault secret item Kubernetes type kubernetes.io/tls
	RadixAzureKeyVaultK8sSecretTypeTls RadixAzureKeyVaultK8sSecretType = "tls"
)

// RadixAzureKeyVaultItem defines Azure Key Vault setting: secrets, keys, certificates
type RadixAzureKeyVaultItem struct {
	// Name. Name of the Azure Key Vault object
	Name string `json:"name"`

	// EnvVar. Name of the environment variable within replicas, containing Azure Key Vault object value
	// +optional
	EnvVar string `json:"envVar,omitempty"`

	// Type. Optional. Type of the Azure KeyVault object: secret (default), key, cert
	// +kubebuilder:validation:Enum=secret;key;cert
	// +optional
	Type *RadixAzureKeyVaultObjectType `json:"type,omitempty"`

	// Alias. Optional.It is not yet fully supported by the Azure CSI Key vault driver. Specify the filename of the object when written to disk. Defaults to objectName if not provided.
	// +optional
	Alias *string `json:"alias,omitempty"`

	// Version. Optional. object versions, default to the latest, if empty
	// +optional
	Version *string `json:"version,omitempty"`

	// Format. Optional. The format of the Azure Key Vault object, supported types are pem and pfx. objectFormat: pfx is only supported with objectType: secret and PKCS12 or ECC certificates. Default format for certificates is pem.
	// +optional
	Format *string `json:"format,omitempty"`

	// Encoding. Optional. Setting object encoding to base64 and object format to pfx will fetch and write the base64 decoded pfx binary
	// +optional
	Encoding *string `json:"encoding,omitempty"`

	// K8SSecretType. Optional. Setting object k8s secret type.
	// Allowed types: opaque (default), tls. It corresponds to "Opaque" and "kubernetes.io/tls" secret types: https://kubernetes.io/docs/concepts/configuration/secret/#secret-types
	// +kubebuilder:validation:Enum=opaque;tls
	// +optional
	K8sSecretType *RadixAzureKeyVaultK8sSecretType `json:"k8sSecretType,omitempty"`
}

// Authentication Radix authentication settings
type Authentication struct {
	//ClientCertificate Authentication client certificate
	// +optional
	ClientCertificate *ClientCertificate `json:"clientCertificate,omitempty"`

	// +optional
	OAuth2 *OAuth2 `json:"oauth2,omitempty"`
}

// ClientCertificate Authentication client certificate parameters
type ClientCertificate struct {
	//Verification Client certificate verification type
	// +kubebuilder:validation:Enum=on;off;optional;optional_no_ca
	// +optional
	Verification *VerificationType `json:"verification,omitempty"`

	//PassCertificateToUpstream Should a certificate be passed to upstream
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
	//VerificationTypeOff Certificate verification is off
	VerificationTypeOff VerificationType = "off"
	//VerificationTypeOn Certificate verification is on
	VerificationTypeOn VerificationType = "on"
	//VerificationTypeOptional Certificate verification is optional
	VerificationTypeOptional VerificationType = "optional"
	//VerificationTypeOptionalNoCa Certificate verification is optional no certificate authority
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
	// ClientID. The OAuth2 client ID
	ClientID string `json:"clientId"`

	// Scope. Optional. The requested scope by the OAuth code flow
	// Default: openid profile email
	// +optional
	Scope string `json:"scope,omitempty"`

	// SetXAuthRequestHeaders. Optional. Defines if X-Auth-* headers should added to the request
	// Sets the X-Auth-Request-User, X-Auth-Request-Groups, X-Auth-Request-Email, X-Auth-Request-Preferred-Username and X-Auth-Request-Access-Token
	// from values in the Access Token redeemed by the OAuth Proxy
	// Default: false
	// +optional
	SetXAuthRequestHeaders *bool `json:"setXAuthRequestHeaders,omitempty"`

	// SetAuthorizationHeader. Optional. Defines if the IDToken received by the OAuth Proxy should be added to the Authorization header
	// Default: false
	// +optional
	SetAuthorizationHeader *bool `json:"setAuthorizationHeader,omitempty"`

	// ProxyPrefix. Optional. The url root path that OAuth Proxy should be nested under
	// Default: /oauth2
	// +optional
	ProxyPrefix string `json:"proxyPrefix,omitempty"`

	// LoginURL. Optional. Authentication endpoint
	// Must be set if OIDC.SkipDiscovery is true
	// +optional
	LoginURL string `json:"loginUrl,omitempty"`

	// RedeemURL. Optional. Endpoint to redeem the authorization code received from the OAuth code flow
	// Must be set if OIDC.SkipDiscovery is true
	// +optional
	RedeemURL string `json:"redeemUrl,omitempty"`

	// OIDC. Optional. Defines OIDC settings
	// +optional
	OIDC *OAuth2OIDC `json:"oidc,omitempty"`

	// Cookie. Optional. Settings for the session cookie
	// +optional
	Cookie *OAuth2Cookie `json:"cookie,omitempty"`

	// SessionStoreType. Optional. Specifies where to store the session data
	// Allowed values: cookie, redis
	// Default: cookie
	// +kubebuilder:validation:Enum=cookie;redis
	// +optional
	SessionStoreType SessionStoreType `json:"sessionStoreType,omitempty"`

	// CookieStore. Optional. Settings for cookie that stores session data when SessionStoreType is cookie
	// +optional
	CookieStore *OAuth2CookieStore `json:"cookieStore,omitempty"`

	// RedisStore. Optional. Settings for Redis store when SessionStoreType is redis
	// +optional
	RedisStore *OAuth2RedisStore `json:"redisStore,omitempty"`
}

// OAuth2Cookie defines properties for the oauth cookie
type OAuth2Cookie struct {
	// Name. Optional. Defines the name of the OAuth session cookie
	// Default: _oauth2_proxy
	// +optional
	Name string `json:"name,omitempty"`

	// Expire. Optional. The expire timeframe for the session cookie
	// Default: 168h0m0s
	// +optional
	Expire string `json:"expire,omitempty"`

	// Refresh. Optional. The interval between cookie refreshes
	// The value must be a shorter timeframe than Expire
	// Default 60m0s
	// +optional
	Refresh string `json:"refresh,omitempty"`

	// SameSite. Optional. The samesite cookie attribute
	// Allowed values: strict, lax, none or empty
	// Default: lax
	// +kubebuilder:validation:Enum=strict;lax;none;""
	// +optional
	SameSite CookieSameSiteType `json:"sameSite,omitempty"`
}

// OAuth2OIDC defines OIDC properties for oauth proxy
type OAuth2OIDC struct {
	// IssuerURL. Optional. The OIDC issuer URL
	// Default: https://login.microsoftonline.com/3aa4a235-b6e2-48d5-9195-7fcf05b459b0/v2.0
	// +optional
	IssuerURL string `json:"issuerUrl,omitempty"`

	// JWKSURL. Optional. OIDC JWKS URL for token verification; required if OIDC discovery is disabled
	// +optional
	JWKSURL string `json:"jwksUrl,omitempty"`

	// SkipDiscovery. Optional. Defines if OIDC endpoint discovery should be bypassed
	// LoginURL, RedeemURL, JWKSURL must be configured if discovery is disabled
	// Default: false
	// +optional
	SkipDiscovery *bool `json:"skipDiscovery,omitempty"`

	// InsecureSkipVerifyNonce. Optional. Skip verifying the OIDC ID Token's nonce claim
	// Default: false
	// +optional
	InsecureSkipVerifyNonce *bool `json:"insecureSkipVerifyNonce,omitempty"`
}

// OAuth2RedisStore properties for redis session storage
type OAuth2RedisStore struct {
	// ConnectionURL. The URL for the Redis server when SessionStoreType is redis
	ConnectionURL string `json:"connectionUrl"`
}

// OAuth2CookieStore properties for cookie session storage
type OAuth2CookieStore struct {
	// Minimal. Optional. Strips OAuth tokens from cookies if they are not needed (only when SessionStoreType is cookie)
	// Cookie.Refresh must be 0, and both SetXAuthRequestHeaders and SetAuthorizationHeader must be false if this setting is true
	// Default: false
	// +optional
	Minimal *bool `json:"minimal,omitempty"`
}

// Identity configuration for federation with external identity providers
type Identity struct {
	// Azure identity configuration
	// +optional
	Azure *AzureIdentity `json:"azure,omitempty"`
}

// AzureIdentity properties for Azure AD Workload Identity
type AzureIdentity struct {
	// ClientId is the client ID for a user defined managed identity
	// or application ID for an application registration
	ClientId string `json:"clientId"`
}

// RadixCommonComponent defines a common component interface for Radix components
type RadixCommonComponent interface {
	//GetName Gets component name
	GetName() string
	//GetSourceFolder Gets component source folder
	GetSourceFolder() string
	//GetImage Gets component image
	GetImage() string
	//GetNode Gets component node parameters
	GetNode() *RadixNode
	//GetVariables Gets component environment variables
	GetVariables() EnvVarsMap
	//GetPorts Gets component ports
	GetPorts() []ComponentPort
	//GetMonitoringConfig Gets component monitoring configuration
	GetMonitoringConfig() MonitoringConfig
	//GetSecrets Gets component secrets
	GetSecrets() []string
	//GetSecretRefs Gets component secret-refs
	GetSecretRefs() RadixSecretRefs
	//GetResources Gets component resources
	GetResources() ResourceRequirements
	//GetIdentity Get component identity
	GetIdentity() *Identity
	//GetEnvironmentConfig Gets component environment configuration
	GetEnvironmentConfig() []RadixCommonEnvironmentConfig
	//GetEnvironmentConfigsMap Get component environment configuration as map by names
	GetEnvironmentConfigsMap() map[string]RadixCommonEnvironmentConfig
	//getEnabled Gets the component status if it is enabled in the application
	getEnabled() bool
	//GetEnabledForEnv Gets the component status if it is enabled in the application for an environment
	GetEnabledForEnv(RadixCommonEnvironmentConfig) bool
	//GetEnvironmentConfigByName  Gets component environment configuration by its name
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
