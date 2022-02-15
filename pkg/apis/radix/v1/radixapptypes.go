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
	SecretRefs              RadixSecretRefs          `json:"secretRefs,omitempty" yaml:"secretRefs,omitempty"`
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
	SecretRefs              RadixSecretRefs         `json:"secretRefs,omitempty" yaml:"secretRefs,omitempty"`
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
	SecretRefs        RadixSecretRefs                      `json:"secretRefs,omitempty" yaml:"secretRefs,omitempty"`
	EnvironmentConfig []RadixJobComponentEnvironmentConfig `json:"environmentConfig,omitempty" yaml:"environmentConfig,omitempty"`
	Variables         EnvVarsMap                           `json:"variables" yaml:"variables"`
	Resources         ResourceRequirements                 `json:"resources,omitempty" yaml:"resources,omitempty"`
	Node              RadixNode                            `json:"node,omitempty" yaml:"node,omitempty"`
	TimeLimitSeconds  *int64                               `json:"timeLimitSeconds,omitempty" yaml:"timeLimitSeconds,omitempty"`
}

// RadixJobComponentEnvironmentConfig defines environment specific settings
// for a single job component within a RadixApplication
type RadixJobComponentEnvironmentConfig struct {
	Environment      string               `json:"environment" yaml:"environment"`
	RunAsNonRoot     bool                 `json:"runAsNonRoot" yaml:"runAsNonRoot"`
	Monitoring       bool                 `json:"monitoring" yaml:"monitoring"`
	Resources        ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	Variables        EnvVarsMap           `json:"variables" yaml:"variables"`
	ImageTagName     string               `json:"imageTagName" yaml:"imageTagName"`
	VolumeMounts     []RadixVolumeMount   `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	Node             RadixNode            `json:"node,omitempty" yaml:"node,omitempty"`
	SecretRefs       RadixSecretRefs      `json:"secretRefs,omitempty" yaml:"secretRefs,omitempty"`
	TimeLimitSeconds *int64               `json:"timeLimitSeconds,omitempty" yaml:"timeLimitSeconds,omitempty"`
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

//IsKnownVolumeMount Gets if volume mount is supported
func IsKnownVolumeMount(volumeMount string) bool {
	return IsKnownBlobFlexVolumeMount(volumeMount) ||
		IsKnownCsiAzureVolumeMount(volumeMount)
}

//IsKnownCsiAzureVolumeMount Supported volume mount type CSI Azure Blob volume
func IsKnownCsiAzureVolumeMount(volumeMount string) bool {
	switch volumeMount {
	case string(MountTypeBlobCsiAzure), string(MountTypeFileCsiAzure):
		return true
	}
	return false
}

//IsKnownBlobFlexVolumeMount Supported volume mount type Azure Blobfuse
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

//RadixSecretRefType Radix secret-ref of type
type RadixSecretRefType string

const (
	//RadixSecretRefTypeAzureKeyVault Radix secret-ref of type Azure Key vault
	RadixSecretRefTypeAzureKeyVault RadixSecretRefType = "az-keyvault"
)

// RadixSecretRefs defines secret vault
type RadixSecretRefs struct {
	// AzureKeyVaults. List of RadixSecretRefs-s, containing Azure Key Vault configurations
	AzureKeyVaults []RadixAzureKeyVault `json:"azureKeyVaults,omitempty" yaml:"azureKeyVaults,omitempty"`
}

// RadixAzureKeyVault defines Azure Key Vault
type RadixAzureKeyVault struct {
	// Name. Name of the Azure Key Vault
	Name string `json:"name" yaml:"name"`
	// Path. Optional. Path within replicas, where secrets are mapped as files. Default: /mnt/azure-key-vault/<key-vault-name>/<component-name>
	Path *string `json:"path,omitempty" yaml:"path,omitempty"`
	// Items. Azure Key Vault items
	Items []RadixAzureKeyVaultItem `json:"items" yaml:"items"`
}

//RadixAzureKeyVaultObjectType Azure Key Vault item type
type RadixAzureKeyVaultObjectType string

const (
	//RadixAzureKeyVaultObjectTypeSecret Azure Key Vault item of type secret
	RadixAzureKeyVaultObjectTypeSecret RadixAzureKeyVaultObjectType = "secret"
	//RadixAzureKeyVaultObjectTypeKey Azure Key Vault item of type key
	RadixAzureKeyVaultObjectTypeKey RadixAzureKeyVaultObjectType = "key"
	//RadixAzureKeyVaultObjectTypeCert Azure Key Vault item of type certificate
	RadixAzureKeyVaultObjectTypeCert RadixAzureKeyVaultObjectType = "cert"
)

//RadixAzureKeyVaultK8sSecretType Azure Key Vault secret item Kubernetes type
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
	Name string `json:"name" yaml:"name"`
	// EnvVar. Name of the environment variable within replicas, containing Azure Key Vault object value
	EnvVar string `json:"envVar" yaml:"envVar"`
	// Type. Optional. Type of the Azure KeyVault object: secret (default), key, cert
	Type *RadixAzureKeyVaultObjectType `json:"type,omitempty" yaml:"type,omitempty"`
	// Alias. Optional.It is not yet fully supported by the Azure CSI Key vault driver. Specify the filename of the object when written to disk. Defaults to objectName if not provided.
	Alias *string `json:"alias,omitempty" yaml:"alias,omitempty"`
	// Version. Optional. object versions, default to the latest, if empty
	Version *string `json:"version,omitempty" yaml:"version,omitempty"`
	// Format. Optional. The format of the Azure Key Vault object, supported types are pem and pfx. objectFormat: pfx is only supported with objectType: secret and PKCS12 or ECC certificates. Default format for certificates is pem.
	Format *string `json:"format,omitempty" yaml:"format,omitempty"`
	// Encoding. Optional. Setting object encoding to base64 and object format to pfx will fetch and write the base64 decoded pfx binary
	Encoding *string `json:"encoding,omitempty" yaml:"encoding,omitempty"`
	// K8SSecretType. Optional. Setting object k8s secret type.
	// Allowed types: opaque (default), tls. It corresponds to "Opaque" and "kubernetes.io/tls" secret types: https://kubernetes.io/docs/concepts/configuration/secret/#secret-types
	K8sSecretType *RadixAzureKeyVaultK8sSecretType `json:"k8sSecretType,omitempty" yaml:"k8sSecretType,omitempty"`
}

//Authentication Radix authentication settings
type Authentication struct {
	//ClientCertificate Authentication client certificate
	ClientCertificate *ClientCertificate `json:"clientCertificate,omitempty" yaml:"clientCertificate,omitempty"`
	OAuth2            *OAuth2            `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`
}

//ClientCertificate Authentication client certificate parameters
type ClientCertificate struct {
	//Verification Client certificate verification type
	Verification *VerificationType `json:"verification,omitempty" yaml:"verification,omitempty"`
	//PassCertificateToUpstream Should a certificate be passed to upstream
	PassCertificateToUpstream *bool `json:"passCertificateToUpstream,omitempty" yaml:"passCertificateToUpstream,omitempty"`
}

// SessionStoreType type of session store
type SessionStoreType string

const (
	// SessionStoreCookie use cookies for session store
	SessionStoreCookie SessionStoreType = "cookie"
	// SessionStoreRedis use redis for session store
	SessionStoreRedis SessionStoreType = "redis"
)

//VerificationType Certificate verification type
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
	ClientID string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	// Scope. Optional. The requested scope by the OAuth code flow
	// Default: openid profile email
	Scope string `json:"scope,omitempty" yaml:"scope,omitempty"`
	// SetXAuthRequestHeaders. Optional. Defines if X-Auth-* headers should added to the request
	// Sets the X-Auth-Request-User, X-Auth-Request-Groups, X-Auth-Request-Email, X-Auth-Request-Preferred-Username and X-Auth-Request-Access-Token
	// from values in the Access Token redeemed by the OAuth Proxy
	// Default: false
	SetXAuthRequestHeaders *bool `json:"setXAuthRequestHeaders,omitempty" yaml:"setXAuthRequestHeaders,omitempty"`
	// SetAuthorizationHeader. Optional. Defines if the IDToken received by the OAuth Proxy should be added to the Authorization header
	// Default: false
	SetAuthorizationHeader *bool `json:"setAuthorizationHeader,omitempty" yaml:"setAuthorizationHeader,omitempty"`
	// ProxyPrefix. Optional. The url root path that OAuth Proxy should be nested under
	// Default: /oauth2
	ProxyPrefix string `json:"proxyPrefix,omitempty" yaml:"proxyPrefix,omitempty"`
	// LoginURL. Optional. Authentication endpoint
	// Must be set if OIDC.SkipDiscovery is true
	LoginURL string `json:"loginUrl,omitempty" yaml:"loginUrl,omitempty"`
	// RedeemURL. Optional. Endpoint to redeem the authorization code received from the OAuth code flow
	// Must be set if OIDC.SkipDiscovery is true
	RedeemURL string `json:"redeemUrl,omitempty" yaml:"redeemUrl,omitempty"`
	// OIDC. Optional. Defines OIDC settings
	OIDC *OAuth2OIDC `json:"oidc,omitempty" yaml:"oidc,omitempty"`
	// Cookie. Optional. Settings for the session cookie
	Cookie *OAuth2Cookie `json:"cookie,omitempty" yaml:"cookie,omitempty"`
	// SessionStoreType. Optional. Specifies where to store the session data
	// Allowed values: cookie, redis
	// Default: cookie
	SessionStoreType SessionStoreType `json:"sessionStoreType,omitempty" yaml:"sessionStoreType,omitempty"`
	// CookieStore. Optional. Settings for cookie that stores session data when SessionStoreType is cookie
	CookieStore *OAuth2CookieStore `json:"cookieStore,omitempty" yaml:"cookieStore,omitempty"`
	// RedisStore. Optional. Settings for Redis store when SessionStoreType is redis
	RedisStore *OAuth2RedisStore `json:"redisStore,omitempty" yaml:"redisStore,omitempty"`
}

// OAuth2Cookie defines properties for the oauth cookie
type OAuth2Cookie struct {
	// Name. Optional. Defines the name of the OAuth session cookie
	// Default: _oauth2_proxy
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Expire. Optional. The expire timeframe for the session cookie
	// Default: 168h0m0s
	Expire string `json:"expire,omitempty" yaml:"expire,omitempty"`
	// Refresh. Optional. The interval between cookie refreshes
	// The value must be a shorter timeframe than Expire
	// Default 60m0s
	Refresh string `json:"refresh,omitempty" yaml:"refresh,omitempty"`
	// SameSite. Optional. The samesite cookie attribute
	// Allowed values: strict, lax, none or empty
	// Default: lax
	SameSite CookieSameSiteType `json:"sameSite,omitempty" yaml:"sameSite,omitempty"`
}

// OAuth2OIDC defines OIDC properties for oauth proxy
type OAuth2OIDC struct {
	// IssuerURL. Optional. The OIDC issuer URL
	// Default: https://login.microsoftonline.com/3aa4a235-b6e2-48d5-9195-7fcf05b459b0/v2.0
	IssuerURL string `json:"issuerUrl,omitempty" yaml:"issuerUrl,omitempty"`
	// JWKSURL. Optional. OIDC JWKS URL for token verification; required if OIDC discovery is disabled
	JWKSURL string `json:"jwksUrl,omitempty" yaml:"jwksUrl,omitempty"`
	// SkipDiscovery. Optional. Defines if OIDC endpoint discovery should be bypassed
	// LoginURL, RedeemURL, JWKSURL must be configured if discovery is disabled
	// Default: false
	SkipDiscovery *bool `json:"skipDiscovery,omitempty" yaml:"skipDiscovery,omitempty"`
	// InsecureSkipVerifyNonce. Optional. Skip verifying the OIDC ID Token's nonce claim
	// Default: false
	InsecureSkipVerifyNonce *bool `json:"insecureSkipVerifyNonce,omitempty" yaml:"insecureSkipVerifyNonce,omitempty"`
}

// OAuth2RedisStore properties for redis session storage
type OAuth2RedisStore struct {
	// ConnectionURL. The URL for the Redis server when SessionStoreType is redis
	ConnectionURL string `json:"connectionUrl,omitempty" yaml:"connectionUrl,omitempty"`
}

// OAuth2CookieStore properties for cookie session storage
type OAuth2CookieStore struct {
	// Minimal. Optional. Strips OAuth tokens from cookies if they are not needed (only when SessionStoreType is cookie)
	// Cookie.Refresh must be 0, and both SetXAuthRequestHeaders and SetAuthorizationHeader must be false if this setting is true
	// Default: false
	Minimal *bool `json:"minimal,omitempty" yaml:"minimal,omitempty"`
}

//RadixCommonComponent defines a common component interface for Radix components
type RadixCommonComponent interface {
	//GetName Gets component name
	GetName() string
	//GetNode Gets component node parameters
	GetNode() *RadixNode
	//GetVariables Gets component environment variables
	GetVariables() EnvVarsMap
	//GetSecrets Gets component secrets
	GetSecrets() []string
	//GetSecretRefs Gets component secret-refs
	GetSecretRefs() RadixSecretRefs
	//GetResources Gets component resources
	GetResources() ResourceRequirements
	//GetEnvironmentConfig Gets component environment configuration
	GetEnvironmentConfig() []RadixCommonEnvironmentConfig
}

func (component *RadixComponent) GetName() string {
	return component.Name
}

func (component *RadixComponent) GetNode() *RadixNode {
	return &component.Node
}

func (component *RadixComponent) GetVariables() EnvVarsMap {
	return component.Variables
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

func (component *RadixComponent) GetEnvironmentConfig() []RadixCommonEnvironmentConfig {
	var environmentConfigs []RadixCommonEnvironmentConfig
	for _, environmentConfig := range component.EnvironmentConfig {
		environmentConfigs = append(environmentConfigs, environmentConfig)
	}
	return environmentConfigs
}

func (component *RadixJobComponent) GetName() string {
	return component.Name
}

func (component *RadixJobComponent) GetNode() *RadixNode {
	return &component.Node
}

func (component *RadixJobComponent) GetSecretRefs() RadixSecretRefs {
	return component.SecretRefs
}

func (component *RadixJobComponent) GetResources() ResourceRequirements {
	return component.Resources
}

func (component *RadixJobComponent) GetSecrets() []string {
	return component.Secrets
}

func (component *RadixJobComponent) GetVariables() EnvVarsMap {
	return component.Variables
}

func (component *RadixJobComponent) GetEnvironmentConfig() []RadixCommonEnvironmentConfig {
	var environmentConfigs []RadixCommonEnvironmentConfig
	for _, environmentConfig := range component.EnvironmentConfig {
		environmentConfigs = append(environmentConfigs, environmentConfig)
	}
	return environmentConfigs
}

func (component *RadixJobComponent) GetVolumeMountsForEnvironment(env string) []RadixVolumeMount {
	for _, envConfig := range component.EnvironmentConfig {
		if strings.EqualFold(env, envConfig.Environment) {
			return envConfig.VolumeMounts
		}
	}
	return nil
}
