package applicationconfig

import (
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MagicBranch The branch that radix config lives on
const MagicBranch = "master"

// Application Instance variables
type Application struct {
	kubeclient   kubernetes.Interface
	radixclient  radixclient.Interface
	kubeutil     *kube.Kube
	registration *v1.RadixRegistration
	config       *radixv1.RadixApplication
}

// NewApplication Constructor
func NewApplication(kubeclient kubernetes.Interface, radixclient radixclient.Interface, registration *v1.RadixRegistration, config *radixv1.RadixApplication) (Application, error) {
	kubeutil, err := kube.New(kubeclient)
	if err != nil {
		return Application{}, err
	}

	return Application{
		kubeclient,
		radixclient,
		kubeutil,
		registration,
		config}, nil
}

// IsMagicBranch Checks if given branch is were radix config lives
func IsMagicBranch(branch string) bool {
	return strings.EqualFold(branch, MagicBranch)
}

// IsBranchMappedToEnvironment Checks if given branch has a mapping
func (app Application) IsBranchMappedToEnvironment(branch string) (bool, map[string]bool) {
	targetEnvs := getTargetEnvironmentsAsMap(branch, app.config)
	if isTargetEnvsEmpty(targetEnvs) {
		return false, targetEnvs
	}

	return true, targetEnvs
}

// ApplyConfigToApplicationNamespace Will apply the config to app namespace so that the operator can act on it
func (app Application) ApplyConfigToApplicationNamespace() error {
	appNamespace := utils.GetAppNamespace(app.registration.Name)
	_, err := app.radixclient.RadixV1().RadixApplications(appNamespace).Create(app.config)
	if errors.IsAlreadyExists(err) {
		err = app.radixclient.RadixV1().RadixApplications(appNamespace).Delete(app.config.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete radix application. %v", err)
		}

		_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Create(app.config)
	}
	if err != nil {
		return fmt.Errorf("failed to apply radix application. %v", err)
	}
	log.Infof("RadixApplication %s saved to ns %s", app.config.Name, appNamespace)
	return nil
}

// CreateEnvironments Will create environments defined in the radix config
func (app Application) CreateEnvironments() error {
	targetEnvs := getTargetEnvironmentsAsMap("", app.config)

	for env := range targetEnvs {
		namespaceName := utils.GetEnvironmentNamespace(app.registration.Name, env)
		ownerRef := application.GetOwnerReferenceOfRegistration(app.registration)
		labels := map[string]string{
			"sync":                  "cluster-wildcard-tls-cert",
			"cluster-wildcard-sync": "cluster-wildcard-tls-cert",
			"app-wildcard-sync":     "app-wildcard-tls-cert",
			kube.RadixAppLabel:      app.registration.Name,
			kube.RadixEnvLabel:      env,
		}

		err := app.kubeutil.ApplyNamespace(namespaceName, labels, ownerRef)
		if err != nil {
			return err
		}

		err = app.GrantAppAdminAccessToNs(namespaceName)
		if err != nil {
			return fmt.Errorf("Failed to apply RBAC on namespace %s: %v", namespaceName, err)
		}
	}

	return nil
}

// GrantAppAdminAccessToNs Grant access to environment namespace
func (app Application) GrantAppAdminAccessToNs(namespace string) error {
	registration := app.registration
	subjects := kube.GetRoleBindingGroups(registration.Spec.AdGroups)
	clusterRoleName := "radix-app-admin-envs"

	roleBinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"radixApp":         registration.Name, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel: registration.Name,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: subjects,
	}

	return app.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}

func getTargetEnvironmentsAsMap(branch string, radixApplication *radixv1.RadixApplication) map[string]bool {
	targetEnvs := make(map[string]bool)
	for _, env := range radixApplication.Spec.Environments {
		if branch == env.Build.From {
			// Deploy environment
			targetEnvs[env.Name] = true
		} else {
			// Only create namespace for environment
			targetEnvs[env.Name] = false
		}
	}
	return targetEnvs
}

func isTargetEnvsEmpty(targetEnvs map[string]bool) bool {
	if len(targetEnvs) == 0 {
		return true
	}

	// Check if all values are false
	falseCount := 0
	for _, value := range targetEnvs {
		if value == false {
			falseCount++
		}
	}
	if falseCount == len(targetEnvs) {
		return true
	}

	return false
}
