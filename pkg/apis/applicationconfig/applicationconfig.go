package applicationconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixTypes "github.com/equinor/radix-operator/pkg/client/clientset/versioned/typed/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

// ConfigBranchFallback The branch to use for radix config if ConfigBranch is not configured on the radix registration
const ConfigBranchFallback = "master"

// ApplicationConfig Instance variables
type ApplicationConfig struct {
	kubeclient   kubernetes.Interface
	radixclient  radixclient.Interface
	kubeutil     *kube.Kube
	registration *v1.RadixRegistration
	config       *v1.RadixApplication
}

// NewApplicationConfig Constructor
func NewApplicationConfig(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	registration *v1.RadixRegistration,
	config *v1.RadixApplication) (*ApplicationConfig, error) {
	return &ApplicationConfig{
		kubeclient,
		radixclient,
		kubeutil,
		registration,
		config}, nil
}

// GetRadixApplicationConfig returns the provided config
func (app *ApplicationConfig) GetRadixApplicationConfig() *v1.RadixApplication {
	return app.config
}

// GetRadixRegistration returns the provided radix registration
func (app *ApplicationConfig) GetRadixRegistration() *v1.RadixRegistration {
	return app.registration
}

// GetComponent Gets the component for a provided name
func GetComponent(ra *v1.RadixApplication, name string) v1.RadixCommonComponent {
	for _, component := range ra.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return &component
		}
	}
	for _, jobComponent := range ra.Spec.Jobs {
		if strings.EqualFold(jobComponent.Name, name) {
			return &jobComponent
		}
	}
	return nil
}

// GetComponentEnvironmentConfig Gets environment config of component
func GetComponentEnvironmentConfig(ra *v1.RadixApplication, envName, componentName string) v1.RadixCommonEnvironmentConfig {
	// TODO: Add interface for RA + EnvConfig
	return GetEnvironment(GetComponent(ra, componentName), envName)
}

// GetEnvironment Gets environment config of component
func GetEnvironment(component v1.RadixCommonComponent, envName string) v1.RadixCommonEnvironmentConfig {
	if component == nil {
		return nil
	}

	for _, environment := range component.GetEnvironmentConfig() {
		if strings.EqualFold(environment.GetEnvironment(), envName) {
			return environment
		}
	}
	return nil
}

// GetConfigBranch Returns config branch name from radix registration, or "master" if not set.
func GetConfigBranch(rr *v1.RadixRegistration) string {
	return utils.TernaryString(strings.TrimSpace(rr.Spec.ConfigBranch) == "", ConfigBranchFallback, rr.Spec.ConfigBranch)
}

// IsConfigBranch Checks if given branch is where radix config lives
func IsConfigBranch(branch string, rr *v1.RadixRegistration) bool {
	return strings.EqualFold(branch, GetConfigBranch(rr))
}

// IsThereAnythingToDeploy Checks if given branch requires deployment to environments
func (app *ApplicationConfig) IsThereAnythingToDeploy(branch string) (bool, map[string]bool) {
	return IsThereAnythingToDeployForRadixApplication(branch, app.config)
}

// IsThereAnythingToDeployForRadixApplication Checks if given branch requires deployment to environments
func IsThereAnythingToDeployForRadixApplication(branch string, ra *v1.RadixApplication) (bool, map[string]bool) {
	targetEnvs := getTargetEnvironmentsAsMap(branch, ra)
	if isTargetEnvsEmpty(targetEnvs) {
		return false, targetEnvs
	}
	return true, targetEnvs
}

// ApplyConfigToApplicationNamespace Will apply the config to app namespace so that the operator can act on it
func (app *ApplicationConfig) ApplyConfigToApplicationNamespace() error {
	appNamespace := utils.GetAppNamespace(app.config.Name)

	existingRA, err := app.radixclient.RadixV1().RadixApplications(appNamespace).Get(context.TODO(), app.config.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("RadixApplication %s doesn't exist in namespace %s, creating now", app.config.Name, appNamespace)
			_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Create(context.TODO(), app.config, metav1.CreateOptions{})
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
		_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Update(context.TODO(), app.config, metav1.UpdateOptions{})
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

	err = app.syncPrivateImageHubSecrets()
	if err != nil {
		log.Errorf("Failed to create private image hub secrets. %v", err)
		return err
	}

	err = utils.GrantAppReaderAccessToSecret(app.kubeutil, app.registration, defaults.PrivateImageHubReaderRoleName, defaults.PrivateImageHubSecretName)
	if err != nil {
		log.Warnf("failed to grant reader access to private image hub secret %v", err)
	}

	err = utils.GrantAppAdminAccessToSecret(app.kubeutil, app.registration, defaults.PrivateImageHubSecretName, defaults.PrivateImageHubSecretName)
	if err != nil {
		log.Warnf("failed to grant access to private image hub secret %v", err)
		return err
	}

	err = app.syncBuildSecrets()
	if err != nil {
		log.Errorf("Failed to create build secrets. %v", err)
		return err
	}

	err = app.createOrUpdateDNSAliases()
	if err != nil {
		return fmt.Errorf("failed to process DNS aliases: %w", err)
	}
	return nil
}

// createEnvironments Will create environments defined in the radix config
func (app *ApplicationConfig) createEnvironments() error {

	for _, env := range app.config.Spec.Environments {
		app.applyEnvironment(utils.NewEnvironmentBuilder().
			WithAppName(app.config.Name).
			WithAppLabel().
			WithEnvironmentName(env.Name).
			WithRegistrationOwner(app.registration).
			WithEgressConfig(env.Egress).
			// Orphaned flag will be set by the environment handler but until
			// reconciliation we must ensure it is false
			// Update: It seems Update method does not update status object when using real k8s client, but the fake client does.
			// Only an explicit call to UpdateStatus can update status object, and this is only done by the RadixEnvironment controller.
			WithOrphaned(false).
			BuildRE())
	}

	return nil
}

func getTargetEnvironmentsAsMap(branchToBuild string, radixApplication *v1.RadixApplication) map[string]bool {
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
		if !value {
			falseCount++
		}
	}
	return falseCount == len(targetEnvs)
}

// applyEnvironment creates an environment or applies changes if it exists
func (app *ApplicationConfig) applyEnvironment(newRe *v1.RadixEnvironment) error {
	logger := log.WithFields(log.Fields{"environment": newRe.ObjectMeta.Name})
	logger.Debugf("Apply environment %s", newRe.Name)

	repository := app.radixclient.RadixV1().RadixEnvironments()

	// Get environment from cache, instead than for cluster
	oldRe, err := app.kubeutil.GetEnvironment(newRe.Name)
	if err != nil && errors.IsNotFound(err) {
		// Environment does not exist yet

		newRe, err = repository.Create(context.TODO(), newRe, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create RadixEnvironment object: %v", err)
		}
		logger.Debugf("Created RadixEnvironment: %s", newRe.Name)

	} else if err != nil {
		return fmt.Errorf("failed to get RadixEnvironment object: %v", err)

	} else {
		// Environment already exists

		logger.Debugf("RadixEnvironment object %s already exists, updating the object now", oldRe.Name)
		err = patchDifference(repository, oldRe, newRe, logger)
		if err != nil {
			return err
		}
	}
	return nil
}

// patchDifference creates a mergepatch, comparing old and new RadixEnvironments and issues the patch to radix
func patchDifference(repository radixTypes.RadixEnvironmentInterface, oldRe *v1.RadixEnvironment, newRe *v1.RadixEnvironment, logger *log.Entry) error {
	radixEnvironment := oldRe.DeepCopy()
	radixEnvironment.ObjectMeta.Labels = newRe.ObjectMeta.Labels
	radixEnvironment.ObjectMeta.OwnerReferences = newRe.ObjectMeta.OwnerReferences
	radixEnvironment.ObjectMeta.Annotations = newRe.ObjectMeta.Annotations
	radixEnvironment.Spec = newRe.Spec
	radixEnvironment.Status = newRe.Status

	oldReJSON, err := json.Marshal(oldRe)
	if err != nil {
		return fmt.Errorf("failed to marshal old RadixEnvironment object: %v", err)
	}

	radixEnvironmentJSON, err := json.Marshal(radixEnvironment)
	if err != nil {
		return fmt.Errorf("failed to marshal new RadixEnvironment object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldReJSON, radixEnvironmentJSON, v1.RadixEnvironment{})
	if err != nil {
		return fmt.Errorf("failed to create patch document for RadixEnvironment object: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		// Will perform update as patching does not seem to work for this custom resource
		patchedEnvironment, err := repository.Update(context.TODO(), radixEnvironment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch RadixEnvironment object: %v", err)
		}
		logger.Debugf("Patched RadixEnvironment: %s", patchedEnvironment.Name)
	} else {
		logger.Debugf("No need to patch RadixEnvironment: %s ", newRe.Name)
	}

	return nil
}

func isEmptyPatch(patchBytes []byte) bool {
	return string(patchBytes) == "{}"
}
