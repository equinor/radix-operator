package models

import (
	"errors"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/api-server/api/secrets/suffix"
	"github.com/equinor/radix-operator/api-server/api/utils/secret"
	volumemountUtils "github.com/equinor/radix-operator/api-server/api/utils/volumemount"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

// ComponentBuilder Builds DTOs
type ComponentBuilder interface {
	WithStatus(ComponentStatus) ComponentBuilder
	WithPodNames([]string) ComponentBuilder
	WithReplicaSummaryList([]ReplicaSummary) ComponentBuilder
	WithSchedulerPort(schedulerPort *int32) ComponentBuilder
	WithScheduledJobPayloadPath(scheduledJobPayloadPath string) ComponentBuilder
	WithRadixEnvironmentVariables(map[string]string) ComponentBuilder
	WithComponent(radixv1.RadixCommonDeployComponent) ComponentBuilder
	WithAuxiliaryResource(AuxiliaryResource) ComponentBuilder
	WithNotifications(*radixv1.Notifications) ComponentBuilder
	WithHorizontalScalingSummary(*HorizontalScalingSummary) ComponentBuilder
	WithExternalDNS(externalDNS []ExternalDNS) ComponentBuilder
	WithRuntime(*radixv1.Runtime) ComponentBuilder
	BuildComponentSummary() (*ComponentSummary, error)
	BuildComponent() (*Component, error)
}

type componentBuilder struct {
	componentName             string
	componentType             string
	status                    ComponentStatus
	componentImage            string
	podNames                  []string
	replicaSummaryList        []ReplicaSummary
	environmentVariables      map[string]string
	radixEnvironmentVariables map[string]string
	secrets                   []string
	ports                     []Port
	schedulerPort             *int32
	scheduledJobPayloadPath   string
	auxResource               AuxiliaryResource
	identity                  *Identity
	notifications             *Notifications
	hpa                       *HorizontalScalingSummary
	externalDNS               []ExternalDNS
	errors                    []error
	commitID                  string
	gitTags                   string
	resources                 *radixv1.ResourceRequirements
	runtime                   *radixv1.Runtime
	replicasOverride          *int
	network                   *Network
}

func (b *componentBuilder) WithStatus(status ComponentStatus) ComponentBuilder {
	b.status = status
	return b
}

func (b *componentBuilder) WithPodNames(podNames []string) ComponentBuilder {
	b.podNames = podNames
	return b
}

func (b *componentBuilder) WithReplicaSummaryList(replicaSummaryList []ReplicaSummary) ComponentBuilder {
	b.replicaSummaryList = replicaSummaryList
	return b
}

func (b *componentBuilder) WithRadixEnvironmentVariables(radixEnvironmentVariables map[string]string) ComponentBuilder {
	b.radixEnvironmentVariables = radixEnvironmentVariables
	return b
}

func (b *componentBuilder) WithSchedulerPort(schedulerPort *int32) ComponentBuilder {
	b.schedulerPort = schedulerPort
	return b
}

func (b *componentBuilder) WithScheduledJobPayloadPath(scheduledJobPayloadPath string) ComponentBuilder {
	b.scheduledJobPayloadPath = scheduledJobPayloadPath
	return b
}

func (b *componentBuilder) WithAuxiliaryResource(auxResource AuxiliaryResource) ComponentBuilder {
	b.auxResource = auxResource
	return b
}

func (b *componentBuilder) WithComponent(component radixv1.RadixCommonDeployComponent) ComponentBuilder {
	b.componentName = component.GetName()
	b.componentType = string(component.GetType())
	b.componentImage = component.GetImage()
	b.resources = component.GetResources()
	b.commitID = component.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable]
	b.gitTags = component.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable]
	b.runtime = component.GetRuntime()
	b.replicasOverride = component.GetReplicasOverride()

	ports := []Port{}
	if component.GetPorts() != nil {
		for _, port := range component.GetPorts() {
			ports = append(ports, Port{
				Name:     port.Name,
				Port:     port.Port,
				IsPublic: port.Name == component.GetPublicPort(),
			})
		}
	}

	b.ports = ports
	b.secrets = component.GetSecrets()

	for _, volumeMount := range component.GetVolumeMounts() {
		if volumeMount.HasEmptyDir() || volumeMount.UseAzureIdentity() {
			continue
		}
		volumeMountType := volumeMount.GetVolumeMountType()
		switch volumeMountType {
		case radixv1.MountTypeBlobFuse2FuseCsiAzure, radixv1.MountTypeBlobFuse2Fuse2CsiAzure:
			secretName := defaults.GetCsiAzureVolumeMountCredsSecretName(component.GetName(), volumeMount.Name)
			b.secrets = append(b.secrets, secretName+defaults.CsiAzureCredsAccountKeyPartSuffix)
			if len(volumemountUtils.GetBlobFuse2VolumeMountStorageAccount(volumeMount)) == 0 {
				b.secrets = append(b.secrets, secretName+defaults.CsiAzureCredsAccountNamePartSuffix)
			}
		}
	}

	secretRef := component.GetSecretRefs()
	if secretRef.AzureKeyVaults != nil {
		for _, azureKeyVault := range secretRef.AzureKeyVaults {
			if azureKeyVault.UseAzureIdentity == nil || !*azureKeyVault.UseAzureIdentity {
				secretName := defaults.GetCsiAzureKeyVaultCredsSecretName(component.GetName(), azureKeyVault.Name)
				b.secrets = append(b.secrets, secretName+defaults.CsiAzureKeyVaultCredsClientIdSuffix)
				b.secrets = append(b.secrets, secretName+defaults.CsiAzureKeyVaultCredsClientSecretSuffix)
			}
			for _, item := range azureKeyVault.Items {
				b.secrets = append(b.secrets, secret.GetSecretNameForAzureKeyVaultItem(component.GetName(), azureKeyVault.Name, &item))
			}
		}
	}

	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() {
		if ingress.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
			b.secrets = append(b.secrets, utils.GetComponentClientCertificateSecretName(component.GetName()))
		}
		if auth.OAuth2 != nil {
			oauth2, err := defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()).MergeWith(auth.OAuth2)
			if err != nil {
				b.errors = append(b.errors, err)
			}
			if !auth.OAuth2.GetUseAzureIdentity() {
				b.secrets = append(b.secrets, component.GetName()+suffix.OAuth2ClientSecret)
			}
			b.secrets = append(b.secrets, component.GetName()+suffix.OAuth2CookieSecret)

			if oauth2.SessionStoreType == radixv1.SessionStoreRedis {
				b.secrets = append(b.secrets, component.GetName()+suffix.OAuth2RedisPassword)
			}
		}
	}

	if identity := component.GetIdentity(); identity != nil {
		b.identity = &Identity{}
		if azure := identity.Azure; azure != nil {
			b.identity.Azure = &AzureIdentity{}
			b.identity.Azure.ClientId = azure.ClientId
			b.identity.Azure.ServiceAccountName = utils.GetComponentServiceAccountName(component.GetName())
			for _, azureKeyVault := range component.GetSecretRefs().AzureKeyVaults {
				if azureKeyVault.UseAzureIdentity != nil && *azureKeyVault.UseAzureIdentity {
					b.identity.Azure.AzureKeyVaults = append(b.identity.Azure.AzureKeyVaults, azureKeyVault.Name)
				}
			}
		}
	}

	b.environmentVariables = component.GetEnvironmentVariables()

	if network := component.GetNetwork(); network != nil {
		b.network = &Network{}

		if ing := network.Ingress; ing != nil {
			b.network.Ingress = &Ingress{}

			if publicIngress := ing.Public; publicIngress != nil {
				b.network.Ingress.Public = &IngressPublic{}

				if allow := publicIngress.Allow; allow != nil {
					b.network.Ingress.Public.Allow = slice.Map(*allow, func(v radixv1.IPOrCIDR) string { return string(v) })
				}
			}
		}
	}

	return b
}

func (b *componentBuilder) WithNotifications(notifications *radixv1.Notifications) ComponentBuilder {
	if notifications == nil {
		b.notifications = nil
		return b
	}
	b.notifications = &Notifications{
		Webhook: notifications.Webhook,
	}
	return b
}

func (b *componentBuilder) WithHorizontalScalingSummary(hpa *HorizontalScalingSummary) ComponentBuilder {
	b.hpa = hpa
	return b
}

func (b *componentBuilder) WithExternalDNS(externalDNS []ExternalDNS) ComponentBuilder {
	b.externalDNS = externalDNS
	return b
}

func (b *componentBuilder) WithRuntime(runtime *radixv1.Runtime) ComponentBuilder {
	b.runtime = runtime
	return b
}

func (b *componentBuilder) buildError() error {
	if len(b.errors) == 0 {
		return nil
	}

	return errors.Join(b.errors...)
}

func (b *componentBuilder) buildRuntimeModel() *Runtime {
	return NewRuntime(b.runtime)
}

func (b *componentBuilder) BuildComponentSummary() (*ComponentSummary, error) {
	summary := ComponentSummary{
		Name:     b.componentName,
		Type:     b.componentType,
		Image:    b.componentImage,
		CommitID: b.commitID,
		GitTags:  b.gitTags,
		Runtime:  b.buildRuntimeModel(),
	}
	if b.resources != nil && (len(b.resources.Limits) > 0 || len(b.resources.Requests) > 0) {
		summary.Resources = pointers.Ptr(ConvertRadixResourceRequirements(*b.resources))
	}
	return &summary, b.buildError()
}

func (b *componentBuilder) BuildComponent() (*Component, error) {
	variables := radixv1.EnvVarsMap{}
	for name, value := range b.environmentVariables {
		variables[name] = value
	}

	for name, value := range b.radixEnvironmentVariables {
		variables[name] = value
	}

	component := Component{
		Name:                     b.componentName,
		Type:                     b.componentType,
		Status:                   b.status.String(),
		Image:                    b.componentImage,
		Ports:                    b.ports,
		Secrets:                  b.secrets,
		Variables:                variables,
		Replicas:                 b.podNames,
		ReplicaList:              b.replicaSummaryList,
		ReplicasOverride:         b.replicasOverride,
		SchedulerPort:            b.schedulerPort,
		ScheduledJobPayloadPath:  b.scheduledJobPayloadPath,
		AuxiliaryResource:        b.auxResource,
		Identity:                 b.identity,
		Notifications:            b.notifications,
		ExternalDNS:              b.externalDNS,
		HorizontalScalingSummary: b.hpa,
		CommitID:                 variables[defaults.RadixCommitHashEnvironmentVariable],
		GitTags:                  variables[defaults.RadixGitTagsEnvironmentVariable],
		Runtime:                  b.buildRuntimeModel(),
		Network:                  b.network,
	}
	if b.resources != nil && (len(b.resources.Limits) > 0 || len(b.resources.Requests) > 0) {
		component.Resources = pointers.Ptr(ConvertRadixResourceRequirements(*b.resources))
	}
	return &component, b.buildError()
}

// NewComponentBuilder Constructor for application component
func NewComponentBuilder() ComponentBuilder {
	return &componentBuilder{}
}
