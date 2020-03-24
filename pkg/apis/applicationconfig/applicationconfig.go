package applicationconfig

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/utils/branch"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixTypes "github.com/equinor/radix-operator/pkg/client/clientset/versioned/typed/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
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

// IsThereAnythingToDeploy Checks if given branch requires deployment to environments
func (app *ApplicationConfig) IsThereAnythingToDeploy(branch string) (bool, map[string]bool) {
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

	err = app.syncPrivateImageHubSecrets()
	if err != nil {
		log.Errorf("Failed to create private image hub secrets. %v", err)
		return err
	}

	err = app.grantAccessToPrivateImageHubSecret()
	if err != nil {
		log.Warnf("failed to grant access to private image hub secret %v", err)
		return err
	}

	err = app.syncBuildSecrets()
	if err != nil {
		log.Errorf("Failed to create build secrets. %v", err)
		return err
	}

	return nil
}

// createEnvironments Will create environments defined in the radix config
func (app *ApplicationConfig) createEnvironments() error {
	targetEnvs := getTargetEnvironmentsAsMap("", app.config)

	for env := range targetEnvs {

		trueVar := true
		envConfig := &v1.RadixEnvironment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "radix.equinor.com/v1",
				Kind:       "RadixEnvironment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", app.config.Name, env),
				Labels: map[string]string{
					kube.RadixAppLabel: app.config.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					metav1.OwnerReference{
						APIVersion: "radix.equinor.com/v1",
						Kind:       "RadixRegistration",
						Name:       app.registration.Name,
						UID:        app.registration.UID,
						Controller: &trueVar,
					},
				},
			},
			Spec: v1.RadixEnvironmentSpec{
				AppName: app.config.Name,
				EnvName: env,
			},
			Status: v1.RadixEnvironmentStatus{
				Reconciled: metav1.Time{},
			},
		}

		app.applyEnvironment(envConfig)
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
		if value == false {
			falseCount++
		}
	}
	if falseCount == len(targetEnvs) {
		return true
	}

	return false
}

// applyEnvironment creates an environment or applies changes if it exists
func (app *ApplicationConfig) applyEnvironment(newRe *v1.RadixEnvironment) error {
	logger := log.WithFields(log.Fields{"environment": newRe.ObjectMeta.Name})
	logger.Debugf("Apply environment %s", newRe.Name)

	repository := app.radixclient.RadixV1().RadixEnvironments()

	oldRe, err := repository.Get(newRe.Name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// Environment does not exist yet

		newRe, err = repository.Create(newRe)
		if err != nil {
			return fmt.Errorf("Failed to create RadixEnvironment object: %v", err)
		}
		logger.Debugf("Created RadixEnvironment: %s", newRe.Name)

	} else if err != nil {
		return fmt.Errorf("Failed to get RadixEnvironment object: %v", err)

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

	oldReJSON, err := json.Marshal(oldRe)
	if err != nil {
		return fmt.Errorf("Failed to marshal old RadixEnvironment object: %v", err)
	}

	newReJSON, err := json.Marshal(newRe)
	if err != nil {
		return fmt.Errorf("Failed to marshal new RadixEnvironment object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldReJSON, newReJSON, v1.RadixEnvironment{})

	if string(patchBytes) != "{}" {
		patchedLimitRange, err := repository.Patch(newRe.Name, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch RadixEnvironment object: %v", err)
		}
		logger.Debugf("Patched RadixEnvironment: %s", patchedLimitRange.Name)
	} else {
		logger.Debugf("No need to patch RadixEnvironment: %s ", newRe.Name)
	}

	return nil
}
