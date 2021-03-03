package kube

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	v1Lister "github.com/equinor/radix-operator/pkg/client/listers/radix/v1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appsv1Listers "k8s.io/client-go/listers/apps/v1"
	coreListers "k8s.io/client-go/listers/core/v1"
	networkingListers "k8s.io/client-go/listers/networking/v1beta1"
	rbacListers "k8s.io/client-go/listers/rbac/v1"
	"os"
	"strings"
)

// Radix Annotations
const (
	RadixBranchAnnotation          = "radix-branch"
	RadixComponentImagesAnnotation = "radix-component-images"
	RadixUpdateTimeAnnotation      = "radix-update-time"

	// See https://github.com/equinor/radix-velero-plugin/blob/master/velero-plugins/deployment/restore.go
	RestoredStatusAnnotation = "equinor.com/velero-restored-status"
)

// Radix Labels
const (
	RadixAppLabel                = "radix-app"
	RadixEnvLabel                = "radix-env"
	RadixComponentLabel          = "radix-component"
	RadixJobNameLabel            = "radix-job-name"
	RadixBuildLabel              = "radix-build"
	RadixCommitLabel             = "radix-commit"
	RadixImageTagLabel           = "radix-image-tag"
	RadixJobTypeLabel            = "radix-job-type"
	RadixJobTypeJob              = "job" // Outer job
	RadixJobTypeBuild            = "build"
	RadixJobTypeCloneConfig      = "clone-config"
	RadixJobTypeJobSchedule      = "job-schedule"
	RadixAppAliasLabel           = "radix-app-alias"
	RadixExternalAliasLabel      = "radix-app-external-alias"
	RadixActiveClusterAliasLabel = "radix-app-active-cluster-alias"
	RadixMountTypeLabel          = "mount-type"

	// Only for backward compatibility
	RadixBranchDeprecated = "radix-branch"
)

// Kube  Stuct for accessing lower level kubernetes functions
type Kube struct {
	kubeClient               kubernetes.Interface
	radixclient              radixclient.Interface
	RrLister                 v1Lister.RadixRegistrationLister
	ReLister                 v1Lister.RadixEnvironmentLister
	RdLister                 v1Lister.RadixDeploymentLister
	NamespaceLister          coreListers.NamespaceLister
	ConfigMapLister          coreListers.ConfigMapLister
	SecretLister             coreListers.SecretLister
	DeploymentLister         appsv1Listers.DeploymentLister
	IngressLister            networkingListers.IngressLister
	ServiceLister            coreListers.ServiceLister
	RoleBindingLister        rbacListers.RoleBindingLister
	ClusterRoleBindingLister rbacListers.ClusterRoleBindingLister
	RoleLister               rbacListers.RoleLister
	ClusterRoleLister        rbacListers.ClusterRoleLister
	ServiceAccountLister     coreListers.ServiceAccountLister
	LimitRangeLister         coreListers.LimitRangeLister
}

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixOperatorComponent": "kube-api"})
}

// New Constructor
func New(client kubernetes.Interface, radixClient radixclient.Interface) (*Kube, error) {
	kube := &Kube{
		kubeClient:  client,
		radixclient: radixClient,
	}
	return kube, nil
}

// NewWithListers Constructor
func NewWithListers(client kubernetes.Interface,
	radixclient radixclient.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	radixInformerFactory informers.SharedInformerFactory) (*Kube, error) {
	kube := &Kube{
		kubeClient:               client,
		radixclient:              radixclient,
		RrLister:                 radixInformerFactory.Radix().V1().RadixRegistrations().Lister(),
		ReLister:                 radixInformerFactory.Radix().V1().RadixEnvironments().Lister(),
		RdLister:                 radixInformerFactory.Radix().V1().RadixDeployments().Lister(),
		NamespaceLister:          kubeInformerFactory.Core().V1().Namespaces().Lister(),
		ConfigMapLister:          kubeInformerFactory.Core().V1().ConfigMaps().Lister(),
		SecretLister:             kubeInformerFactory.Core().V1().Secrets().Lister(),
		DeploymentLister:         kubeInformerFactory.Apps().V1().Deployments().Lister(),
		ServiceLister:            kubeInformerFactory.Core().V1().Services().Lister(),
		IngressLister:            kubeInformerFactory.Networking().V1beta1().Ingresses().Lister(),
		RoleBindingLister:        kubeInformerFactory.Rbac().V1().RoleBindings().Lister(),
		ClusterRoleBindingLister: kubeInformerFactory.Rbac().V1().ClusterRoleBindings().Lister(),
		RoleLister:               kubeInformerFactory.Rbac().V1().Roles().Lister(),
		ClusterRoleLister:        kubeInformerFactory.Rbac().V1().ClusterRoles().Lister(),
		LimitRangeLister:         kubeInformerFactory.Core().V1().LimitRanges().Lister(),
	}

	return kube, nil
}

func isEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}

func (kube *Kube) AppendDefaultVariables(currentEnvironment string, environmentVariables []corev1.EnvVar, isPublic bool, namespace, appName, componentName string, ports []v1.ComponentPort, radixDeploymentLabels map[string]string) []corev1.EnvVar {
	clusterName, err := kube.GetClusterName()
	if err != nil {
		return environmentVariables
	}

	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil
	}

	clusterType := os.Getenv(defaults.OperatorClusterTypeEnvironmentVariable)
	if clusterType != "" {
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixClusterTypeEnvironmentVariable,
			Value: clusterType,
		})
	}

	containerRegistry, err := kube.GetContainerRegistry()
	if err != nil {
		return environmentVariables
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.ContainerRegistryEnvironmentVariable,
		Value: containerRegistry,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixDNSZoneEnvironmentVariable,
		Value: dnsZone,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.ClusternameEnvironmentVariable,
		Value: clusterName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.EnvironmentnameEnvironmentVariable,
		Value: currentEnvironment,
	})

	if isPublic {
		canonicalHostName := getHostName(componentName, namespace, clusterName, dnsZone)
		publicHostName := ""

		if isActiveCluster(clusterName) {
			publicHostName = getActiveClusterHostName(componentName, namespace)
		} else {
			publicHostName = canonicalHostName
		}

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.PublicEndpointEnvironmentVariable,
			Value: publicHostName,
		})
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.CanonicalEndpointEnvironmentVariable,
			Value: canonicalHostName,
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixAppEnvironmentVariable,
		Value: appName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixComponentEnvironmentVariable,
		Value: componentName,
	})

	if len(ports) > 0 {
		portNumbers, portNames := getPortNumbersAndNamesString(ports)
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixPortsEnvironmentVariable,
			Value: portNumbers,
		})

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixPortNamesEnvironmentVariable,
			Value: portNames,
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixCommitHashEnvironmentVariable,
		Value: radixDeploymentLabels[RadixCommitLabel],
	})

	return environmentVariables
}

func getHostName(componentName, namespace, clustername, dnsZone string) string {
	hostnameTemplate := "%s-%s.%s.%s"
	return fmt.Sprintf(hostnameTemplate, componentName, namespace, clustername, dnsZone)
}

func isActiveCluster(clustername string) bool {
	activeClustername := os.Getenv(defaults.ActiveClusternameEnvironmentVariable)
	return strings.EqualFold(clustername, activeClustername)
}

func getActiveClusterHostName(componentName, namespace string) string {
	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s.%s", componentName, namespace, dnsZone)
}

func getPortNumbersAndNamesString(ports []v1.ComponentPort) (string, string) {
	portNumbers := "("
	portNames := "("
	portsSize := len(ports)
	for i, portObj := range ports {
		if i < portsSize-1 {
			portNumbers += fmt.Sprint(portObj.Port) + " "
			portNames += fmt.Sprint(portObj.Name) + " "
		} else {
			portNumbers += fmt.Sprint(portObj.Port) + ")"
			portNames += fmt.Sprint(portObj.Name) + ")"
		}
	}
	return portNumbers, portNames
}
