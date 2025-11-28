package applicationconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/branch"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ConfigBranchFallback The branch to use for radix config if ConfigBranch is not configured on the radix registration
const ConfigBranchFallback = "master"

// ApplicationConfig Instance variables
type ApplicationConfig struct {
	kubeclient   kubernetes.Interface
	radixclient  radixclient.Interface
	kubeutil     *kube.Kube
	registration *radixv1.RadixRegistration
	config       *radixv1.RadixApplication
	logger       zerolog.Logger
}

// NewApplicationConfig Constructor
func NewApplicationConfig(kubeclient kubernetes.Interface, kubeutil *kube.Kube, radixclient radixclient.Interface, registration *radixv1.RadixRegistration, config *radixv1.RadixApplication) *ApplicationConfig {
	return &ApplicationConfig{
		kubeclient:   kubeclient,
		radixclient:  radixclient,
		kubeutil:     kubeutil,
		registration: registration,
		config:       config,
		logger:       log.Logger.With().Str("resource_kind", radixv1.KindRadixApplication).Str("resource_name", cache.MetaObjectToName(&config.ObjectMeta).String()).Logger(),
	}
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

// GetTargetEnvironments Gets applicable target environments to be built for a given branch
func GetTargetEnvironments(gitRef, gitRefType string, ra *radixv1.RadixApplication, triggeredFromWebhook bool) ([]string, []string, []string) {
	var targetEnvs []string
	var ignoredForWebhookEnvs, ignoredForGitRefType []string
	for _, env := range ra.Spec.Environments {
		if env.Build.From == "" || !branch.MatchesPattern(env.Build.From, gitRef) {
			continue
		}
		if triggeredFromWebhook && env.Build.WebhookEnabled != nil && !*env.Build.WebhookEnabled {
			ignoredForWebhookEnvs = append(ignoredForWebhookEnvs, env.Name)
			continue
		}
		if len(env.Build.FromType) > 0 && len(gitRefType) > 0 && env.Build.FromType != gitRefType {
			ignoredForGitRefType = append(ignoredForGitRefType, env.Name)
			continue
		}
		targetEnvs = append(targetEnvs, env.Name)
	}
	return targetEnvs, ignoredForWebhookEnvs, ignoredForGitRefType
}

// GetAllTargetEnvironments Gets all target environments for a given branch
func GetAllTargetEnvironments(gitRef, gitRefType string, ra *radixv1.RadixApplication) []string {
	environments, _, _ := GetTargetEnvironments(gitRef, gitRefType, ra, false)
	return environments
}

// OnSync is called when an application config is applied to application namespace
// It compares the actual state with the desired, and attempts to
// converge the two
func (app *ApplicationConfig) OnSync(ctx context.Context) error {
	ctx = log.Ctx(ctx).With().Str("resource_kind", radixv1.KindRadixApplication).Logger().WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Syncing")
	return app.syncStatus(ctx, app.reconcile(ctx))
}

func (app *ApplicationConfig) reconcile(ctx context.Context) error {
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
		return fmt.Errorf("failed to process dns aliases: %w", err)
	}

	if err := app.syncSubPipelineServiceAccounts(ctx); err != nil {
		return fmt.Errorf("failed to sync pipeline service accounts: %w", err)
	}

	return nil
}
