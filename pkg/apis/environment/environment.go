package environment

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Environment is the aggregate-root for manipulating RadixEnvironments
type Environment struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	config      *v1.RadixEnvironment
	regConfig   *v1.RadixRegistration
}

// NewEnvironment is the constructor for Environment
func NewEnvironment(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	config *v1.RadixEnvironment,
	regConfig *v1.RadixRegistration) (Environment, error) {

	return Environment{
		kubeclient,
		radixclient,
		kubeutil,
		config,
		regConfig}, nil
}

// OnSync is called by the handler when changes are applied and must be
// reconceliated with current state.
func (env *Environment) OnSync() error {

	// create a globally unique namespace name
	namespaceName := utils.GetEnvironmentNamespace(env.config.Spec.AppName, env.config.Spec.EnvName)

	err := env.ApplyNamespace(namespaceName)
	if err != nil {
		return fmt.Errorf("Failed to apply namespace %s: %v", namespaceName, err)
	}

	err = env.ApplyAdGroupRoleBinding(namespaceName)
	if err != nil {
		return fmt.Errorf("Failed to apply RBAC on namespace %s: %v", namespaceName, err)
	}

	err = env.ApplyLimitRange(namespaceName)
	if err != nil {
		return fmt.Errorf("Failed to apply limit range on namespace %s: %v", namespaceName, err)
	}

	return nil
}

// ApplyNamespace sets up namespace metadata and applies configuration to kubernetes
func (env *Environment) ApplyNamespace(name string) error {

	// get key to use for namespace annotation to pick up private image hubs
	imagehubKey := fmt.Sprintf("%s-sync", defaults.PrivateImageHubSecretName)
	labels := map[string]string{
		"sync":                         "cluster-wildcard-tls-cert",
		"cluster-wildcard-sync":        "cluster-wildcard-tls-cert",
		"app-wildcard-sync":            "app-wildcard-tls-cert",
		"active-cluster-wildcard-sync": "active-cluster-wildcard-tls-cert",
		imagehubKey:                    env.config.Spec.AppName,
		kube.RadixAppLabel:             env.config.Spec.AppName,
		kube.RadixEnvLabel:             env.config.Spec.EnvName,
	}

	// get owner reference from RadixRegistration
	trueVar := true
	ownerRef := []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixEnvironment",
			Name:       env.regConfig.Name,
			UID:        env.regConfig.UID,
			Controller: &trueVar,
		},
	}

	return env.kubeutil.ApplyNamespace(name, labels, ownerRef)
}

// ApplyAdGroupRoleBinding grants access to environment namespace
func (env *Environment) ApplyAdGroupRoleBinding(namespace string) error {

	adGroups, err := application.GetAdGroups(env.regConfig)
	if err != nil {
		return err
	}

	roleBinding := kube.GetRolebindingToClusterRole(env.config.Spec.AppName, defaults.AppAdminEnvironmentRoleName, adGroups)

	return env.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}

const limitRangeName = "mem-cpu-limit-range-env"

// ApplyLimitRange sets resource usage limits to provided namespace
func (env *Environment) ApplyLimitRange(namespace string) error {

	defaultCPULimit := defaults.GetDefaultCPULimit()
	defaultMemoryLimit := defaults.GetDefaultMemoryLimit()
	defaultCPURequest := defaults.GetDefaultCPURequest()
	defaultMemoryRequest := defaults.GetDefaultMemoryRequest()

	// if not all limits are defined, then don't put any limits on namespace
	if defaultCPULimit == nil ||
		defaultMemoryLimit == nil ||
		defaultCPURequest == nil ||
		defaultMemoryRequest == nil {
		log.Warningf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := env.kubeutil.BuildLimitRange(namespace,
		limitRangeName, env.config.Spec.AppName,
		*defaultCPULimit,
		*defaultMemoryLimit,
		*defaultCPURequest,
		*defaultMemoryRequest)

	return env.kubeutil.ApplyLimitRange(namespace, limitRange)
}
