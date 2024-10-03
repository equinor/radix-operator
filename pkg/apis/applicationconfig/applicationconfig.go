package applicationconfig

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	logger         zerolog.Logger
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
		logger:         log.Logger.With().Str("resource_kind", radixv1.KindRadixApplication).Str("resource_name", cache.MetaObjectToName(&config.ObjectMeta).String()).Logger(),
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
func (app *ApplicationConfig) ApplyConfigToApplicationNamespace(ctx context.Context) error {
	appNamespace := utils.GetAppNamespace(app.config.Name)

	existingRA, err := app.radixclient.RadixV1().RadixApplications(appNamespace).Get(ctx, app.config.Name, metav1.GetOptions{})
	if err != nil {
		if k8errors.IsNotFound(err) {
			app.logger.Debug().Msgf("RadixApplication %s doesn't exist in namespace %s, creating now", app.config.Name, appNamespace)
			if err = radixvalidators.CanRadixApplicationBeInserted(ctx, app.radixclient, app.config, app.dnsAliasConfig); err != nil {
				return err
			}
			_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Create(ctx, app.config, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create radix application. %v", err)
			}
			app.logger.Info().Msgf("RadixApplication %s saved to ns %s", app.config.Name, appNamespace)
			return nil
		}
		return fmt.Errorf("failed to get radix application. %v", err)
	}

	app.logger.Debug().Msgf("RadixApplication %s exists in namespace %s", app.config.Name, appNamespace)
	if reflect.DeepEqual(app.config.Spec, existingRA.Spec) {
		app.logger.Info().Msgf("No changes to RadixApplication %s in namespace %s", app.config.Name, appNamespace)
		return nil
	}

	if err = radixvalidators.CanRadixApplicationBeInserted(ctx, app.radixclient, app.config, app.dnsAliasConfig); err != nil {
		return err
	}

	// Update RA if different
	app.logger.Debug().Msgf("RadixApplication %s in namespace %s has changed, updating now", app.config.Name, appNamespace)
	// For an update, ResourceVersion of the new object must be the same with the old object
	app.config.SetResourceVersion(existingRA.GetResourceVersion())
	_, err = app.radixclient.RadixV1().RadixApplications(appNamespace).Update(ctx, app.config, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update existing radix application: %w", err)
	}
	app.logger.Info().Msgf("RadixApplication %s updated in namespace %s", app.config.Name, appNamespace)
	return nil
}

// OnSync is called when an application config is applied to application namespace
// It compares the actual state with the desired, and attempts to
// converge the two
func (app *ApplicationConfig) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().Str("resource_kind", radixv1.KindRadixApplication).Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")

	if err := app.syncEnvironments(ctx); err != nil {
		return fmt.Errorf("failed to create namespaces for app environments %s: %w", app.config.Name, err)
	}
	if err := app.syncPrivateImageHubSecrets(ctx); err != nil {
		return fmt.Errorf("failed to create private image hub secrets: %w", err)
	}

	if err := app.syncBuildSecrets(ctx); err != nil {
		return fmt.Errorf("failed to create build secrets: %w", err)
	}

	if err := app.syncDNSAliases(ctx); err != nil {
		return fmt.Errorf("failed to process DNS aliases: %w", err)
	}

	if err := app.syncSubPipelineServiceAccounts(ctx); err != nil {
		return fmt.Errorf("failed to sync pipeline service accounts: %w", err)
	}

	return nil
}

func (app *ApplicationConfig) annotateOrphanedEnvironments(ctx context.Context, appEnvironments []radixv1.Environment) error {
	environments, err := app.kubeutil.ListEnvironmentsWithSelector(ctx, labels.ForApplicationName(app.config.Name).String())
	if err != nil {
		return err
	}
	appEnvNames := slice.Reduce(appEnvironments, make(map[string]struct{}), func(acc map[string]struct{}, env radixv1.Environment) map[string]struct{} {
		acc[env.Name] = struct{}{}
		return acc
	})
	orphanedEnvironments := slice.FindAll(environments, func(radixEnvironment *radixv1.RadixEnvironment) bool {
		_, envExistsInApp := appEnvNames[radixEnvironment.Spec.EnvName]
		return !envExistsInApp
	})
	var errs []error
	for _, radixEnvironment := range orphanedEnvironments {
		annotations := radixEnvironment.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		if _, ok := annotations[kube.RadixEnvironmentIsOrphanedAnnotation]; !ok {
			annotations[kube.RadixEnvironmentIsOrphanedAnnotation] = radixutils.FormatTimestamp(time.Now())
			radixEnvironment.SetAnnotations(annotations)
			if err = app.updateRadixEnvironment(ctx, radixEnvironment); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
