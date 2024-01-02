package applicationconfig

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GetConfigBranch Returns config branch name from radix registration, or "master" if not set.
func GetConfigBranch(rr *radixv1.RadixRegistration) string {
	return utils.TernaryString(strings.TrimSpace(rr.Spec.ConfigBranch) == "", ConfigBranchFallback, rr.Spec.ConfigBranch)
}

// IsConfigBranch Checks if given branch is where radix config lives
func IsConfigBranch(branch string, rr *radixv1.RadixRegistration) bool {
	return strings.EqualFold(branch, GetConfigBranch(rr))
}

// GetTargetEnvironments Checks if given branch requires deployment to environments
func GetTargetEnvironments(branchToBuild string, ra *radixv1.RadixApplication) []string {
	var targetEnvs []string
	for _, env := range ra.Spec.Environments {
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
	if err := app.syncEnvironments(); err != nil {
		log.Errorf("Failed to create namespaces for app environments %s. %v", app.config.Name, err)
		return err
	}
	if err := app.syncPrivateImageHubSecrets(); err != nil {
		log.Errorf("Failed to create private image hub secrets. %v", err)
		return err
	}

	if err := app.syncBuildSecrets(); err != nil {
		log.Errorf("Failed to create build secrets. %v", err)
		return err
	}
	if err := app.syncDNSAliases(); err != nil {
		return fmt.Errorf("failed to process DNS aliases: %w", err)
	}
	return app.syncSubPipelineServiceAccounts()
}
