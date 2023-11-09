package applicationconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	commonErrors "github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
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
	kubeclient     kubernetes.Interface
	radixclient    radixclient.Interface
	kubeutil       *kube.Kube
	registration   *radixv1.RadixRegistration
	config         *radixv1.RadixApplication
	dnsAliasConfig *dnsalias.DNSConfig
}

// NewApplicationConfig Constructor
func NewApplicationConfig(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, registration *radixv1.RadixRegistration, config *radixv1.RadixApplication, dnsAliasConfig *dnsalias.DNSConfig) *ApplicationConfig {
	return &ApplicationConfig{
		kubeclient:     kubeclient,
		radixclient:    radixclient,
		kubeutil:       kubeutil,
		registration:   registration,
		config:         config,
		dnsAliasConfig: dnsAliasConfig,
	}
}

// GetRadixApplicationConfig returns the provided config
func (app *ApplicationConfig) GetRadixApplicationConfig() *radixv1.RadixApplication {
	return app.config
}

// GetRadixRegistration returns the provided radix registration
func (app *ApplicationConfig) GetRadixRegistration() *radixv1.RadixRegistration {
	return app.registration
}

// GetComponent Gets the component for a provided name
func GetComponent(ra *radixv1.RadixApplication, name string) radixv1.RadixCommonComponent {
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

// GetComponentEnvironmentConfig Gets environment config of component. This method is used by radix-api
func GetComponentEnvironmentConfig(ra *radixv1.RadixApplication, envName, componentName string) radixv1.RadixCommonEnvironmentConfig {
	// TODO: Add interface for RA + EnvConfig
	return GetEnvironment(GetComponent(ra, componentName), envName)
}

// GetEnvironment Gets environment config of component
func GetEnvironment(component radixv1.RadixCommonComponent, envName string) radixv1.RadixCommonEnvironmentConfig {
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
func GetConfigBranch(rr *radixv1.RadixRegistration) string {
	return utils.TernaryString(strings.TrimSpace(rr.Spec.ConfigBranch) == "", ConfigBranchFallback, rr.Spec.ConfigBranch)
}

// IsConfigBranch Checks if given branch is where radix config lives
func IsConfigBranch(branch string, rr *radixv1.RadixRegistration) bool {
	return strings.EqualFold(branch, GetConfigBranch(rr))
}

// GetTargetEnvironments Checks if given branch requires deployment to environments
func (app *ApplicationConfig) GetTargetEnvironments(branchToBuild string) []string {
	var targetEnvs []string
	for _, env := range app.config.Spec.Environments {
		if env.Build.From != "" && branch.MatchesPattern(env.Build.From, branchToBuild) {
			targetEnvs = append(targetEnvs, env.Name)
		}
	}
	return targetEnvs
}

// ApplyConfigToApplicationNamespace Will apply the config to app namespace so that the operator can act on it
func (app *ApplicationConfig) ApplyConfigToApplicationNamespace() error {
	appNamespace := utils.GetAppNamespace(app.config.Name)

	existingRA, err := app.radixclient.RadixV1().RadixApplications(appNamespace).Get(context.TODO(), app.config.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("RadixApplication %s doesn't exist in namespace %s, creating now", app.config.Name, appNamespace)
			if err = radixvalidators.CanRadixApplicationBeInserted(app.radixclient, app.config, app.dnsAliasConfig); err != nil {
				return err
			}
			_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Create(context.TODO(), app.config, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create radix application. %v", err)
			}
			log.Infof("RadixApplication %s saved to ns %s", app.config.Name, appNamespace)
			return nil
		}
		return fmt.Errorf("failed to get radix application. %v", err)
	}

	log.Debugf("RadixApplication %s exists in namespace %s", app.config.Name, appNamespace)
	if reflect.DeepEqual(app.config.Spec, existingRA.Spec) {
		log.Infof("No changes to RadixApplication %s in namespace %s", app.config.Name, appNamespace)
		return nil
	}

	if err = radixvalidators.CanRadixApplicationBeInserted(app.radixclient, app.config, app.dnsAliasConfig); err != nil {
		return err
	}

	// Update RA if different
	log.Debugf("RadixApplication %s in namespace %s has changed, updating now", app.config.Name, appNamespace)
	// For an update, ResourceVersion of the new object must be the same with the old object
	app.config.SetResourceVersion(existingRA.GetResourceVersion())
	_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Update(context.TODO(), app.config, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update existing radix application. %v", err)
	}
	log.Infof("RadixApplication %s updated in namespace %s", app.config.Name, appNamespace)
	return nil
}

// OnSync is called when an application config is applied to application namespace
// It compares the actual state with the desired, and attempts to
// converge the two
func (app *ApplicationConfig) OnSync() error {
	if err := app.createEnvironments(); err != nil {
		log.Errorf("Failed to create namespaces for app environments %s. %v", app.config.Name, err)
		return err
	}
	if err := app.syncPrivateImageHubSecrets(); err != nil {
		log.Errorf("Failed to create private image hub secrets. %v", err)
		return err
	}
	if err := utils.GrantAppReaderAccessToSecret(app.kubeutil, app.registration, defaults.PrivateImageHubReaderRoleName, defaults.PrivateImageHubSecretName); err != nil {
		log.Warnf("failed to grant reader access to private image hub secret %v", err)
	}
	if err := utils.GrantAppAdminAccessToSecret(app.kubeutil, app.registration, defaults.PrivateImageHubSecretName, defaults.PrivateImageHubSecretName); err != nil {
		log.Warnf("failed to grant access to private image hub secret %v", err)
		return err
	}
	if err := app.syncBuildSecrets(); err != nil {
		log.Errorf("Failed to create build secrets. %v", err)
		return err
	}
	if err := app.createOrUpdateDNSAliases(); err != nil {
		return fmt.Errorf("failed to process DNS aliases: %w", err)
	}
	return nil
}

func (app *ApplicationConfig) createEnvironments() error {
	var errs []error
	for _, env := range app.config.Spec.Environments {
		err := app.applyEnvironment(utils.NewEnvironmentBuilder().
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
		if err != nil {
			errs = append(errs, err)
		}
	}
	return commonErrors.Concat(errs)
}

// applyEnvironment creates an environment or applies changes if it exists
func (app *ApplicationConfig) applyEnvironment(newRe *radixv1.RadixEnvironment) error {
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

func (app *ApplicationConfig) getPortForDNSAlias(dnsAlias radixv1.DNSAlias) (int32, error) {
	component, componentFound := slice.FindFirst(app.config.Spec.Components, func(c radixv1.RadixComponent) bool {
		return c.Name == dnsAlias.Component
	})
	if !componentFound {
		// TODO test
		return 0, fmt.Errorf("component %s does not exist in the application %s", dnsAlias.Component, app.config.GetName())
	}
	if !component.GetEnabledForEnvironment(dnsAlias.Environment) {
		// TODO test
		return 0, fmt.Errorf("component %s is not enabled for the environment %s in the application %s", dnsAlias.Component, dnsAlias.Environment, app.config.GetName())
	}
	componentPublicPort := getComponentPublicPort(&component)
	if componentPublicPort == nil {
		// TODO test
		return 0, fmt.Errorf("component %s does not have public port in the application %s", dnsAlias.Component, app.config.GetName())
	}
	return componentPublicPort.Port, nil
}

// patchDifference creates a mergepatch, comparing old and new RadixEnvironments and issues the patch to radix
func patchDifference(repository radixTypes.RadixEnvironmentInterface, oldRe *radixv1.RadixEnvironment, newRe *radixv1.RadixEnvironment, logger *log.Entry) error {
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

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldReJSON, radixEnvironmentJSON, radixv1.RadixEnvironment{})
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
