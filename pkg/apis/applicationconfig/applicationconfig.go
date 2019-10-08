package applicationconfig

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/utils/branch"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MagicBranch The branch that radix config lives on
const MagicBranch = "master"

// ApplicationConfig Instance variables
type ApplicationConfig struct {
	kubeclient   kubernetes.Interface
	radixclient  radixclient.Interface
	kubeutil     *kube.Kube
	registration *v1.RadixRegistration
	config       *radixv1.RadixApplication
}

// NewApplicationConfig Constructor
func NewApplicationConfig(kubeclient kubernetes.Interface, radixclient radixclient.Interface, registration *v1.RadixRegistration, config *radixv1.RadixApplication) (*ApplicationConfig, error) {
	kubeutil, err := kube.New(kubeclient)
	if err != nil {
		log.Errorf("Failed initializing ApplicationConfig")
		return nil, err
	}

	return &ApplicationConfig{
		kubeclient,
		radixclient,
		kubeutil,
		registration,
		config}, nil
}

// GetComponent Gets the component for a provided name
func GetComponent(ra *v1.RadixApplication, name string) *v1.RadixComponent {
	for _, component := range ra.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return &component
		}
	}

	return nil
}

// GetComponentEnvironmentConfig Gets environment config of component
func GetComponentEnvironmentConfig(ra *v1.RadixApplication, envName, componentName string) *v1.RadixEnvironmentConfig {
	return GetEnvironment(GetComponent(ra, componentName), envName)
}

// GetEnvironment Gets environment config of component
func GetEnvironment(component *v1.RadixComponent, envName string) *v1.RadixEnvironmentConfig {
	if component == nil {
		return nil
	}

	for _, environment := range component.EnvironmentConfig {
		if strings.EqualFold(environment.Environment, envName) {
			return &environment
		}
	}

	return nil
}

// IsMagicBranch Checks if given branch is were radix config lives
func IsMagicBranch(branch string) bool {
	return strings.EqualFold(branch, MagicBranch)
}

// IsBranchMappedToEnvironment Checks if given branch has a mapping
func (app *ApplicationConfig) IsBranchMappedToEnvironment(branch string) (bool, map[string]bool) {
	targetEnvs := getTargetEnvironmentsAsMap(branch, app.config)
	if isTargetEnvsEmpty(targetEnvs) {
		return false, targetEnvs
	}

	return true, targetEnvs
}

// ApplyConfigToApplicationNamespace Will apply the config to app namespace so that the operator can act on it
func (app *ApplicationConfig) ApplyConfigToApplicationNamespace() error {
	appNamespace := utils.GetAppNamespace(app.config.Name)

	existingRA, err := app.radixclient.RadixV1().RadixApplications(appNamespace).Get(app.config.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("RadixApplication %s doesn't exist in namespace %s, creating now", app.config.Name, appNamespace)
			_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Create(app.config)
			if err != nil {
				return fmt.Errorf("failed to create radix application. %v", err)
			}
			log.Infof("RadixApplication %s saved to ns %s", app.config.Name, appNamespace)
			return nil
		}
		return fmt.Errorf("failed to get radix application. %v", err)
	}

	// Update RA if different
	log.Debugf("RadixApplication %s exists in namespace %s", app.config.Name, appNamespace)
	if !reflect.DeepEqual(app.config.Spec, existingRA.Spec) {
		log.Debugf("RadixApplication %s in namespace %s has changed, updating now", app.config.Name, appNamespace)
		// For an update, ResourceVersion of the new object must be the same with the old object
		app.config.SetResourceVersion(existingRA.GetResourceVersion())
		_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Update(app.config)
		if err != nil {
			return fmt.Errorf("failed to update existing radix application. %v", err)
		}
		log.Infof("RadixApplication %s updated in namespace %s", app.config.Name, appNamespace)
	} else {
		log.Infof("No changes to RadixApplication %s in namespace %s", app.config.Name, appNamespace)
	}

	return nil
}

// OnSync is called when an application config is applied to application namespace
// It compares the actual state with the desired, and attempts to
// converge the two
func (app *ApplicationConfig) OnSync() error {
	err := app.createEnvironments()
	if err != nil {
		log.Errorf("Failed to create namespaces for app environments %s. %v", app.config.Name, err)
		return err
	}

	return nil
}

// CreateEnvironments Will create environments defined in the radix config
func (app *ApplicationConfig) createEnvironments() error {
	targetEnvs := getTargetEnvironmentsAsMap("", app.config)

	for env := range targetEnvs {
		namespaceName := utils.GetEnvironmentNamespace(app.config.Name, env)

		annotations, err := application.GetNamespaceAnnotationsOfRegistration(app.registration)
		if err != nil {
			return err
		}

		ownerRef := application.GetOwnerReferenceOfRegistration(app.registration)
		labels := map[string]string{
			"sync":                         "cluster-wildcard-tls-cert",
			"cluster-wildcard-sync":        "cluster-wildcard-tls-cert",
			"app-wildcard-sync":            "app-wildcard-tls-cert",
			"active-cluster-wildcard-sync": "active-cluster-wildcard-tls-cert",
			kube.RadixAppLabel:             app.config.Name,
			kube.RadixEnvLabel:             env,
		}

		err = app.kubeutil.ApplyNamespace(namespaceName, annotations, labels, ownerRef)
		if err != nil {
			return err
		}

		err = app.grantAppAdminAccessToNs(namespaceName)
		if err != nil {
			return fmt.Errorf("Failed to apply RBAC on namespace %s: %v", namespaceName, err)
		}

		err = app.createLimitRangeOnEnvironmentNamespace(namespaceName)
		if err != nil {
			return fmt.Errorf("Failed to apply limit range on namespace %s: %v", namespaceName, err)
		}
	}

	return nil
}

func getTargetEnvironmentsAsMap(branchToBuild string, radixApplication *radixv1.RadixApplication) map[string]bool {
	targetEnvs := make(map[string]bool)
	for _, env := range radixApplication.Spec.Environments {
		if env.Build.From != "" && branch.MatchesPattern(env.Build.From, branchToBuild) {
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
