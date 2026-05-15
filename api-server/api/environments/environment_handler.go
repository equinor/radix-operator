package environments

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/api-server/api/deployments"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	"github.com/equinor/radix-operator/api-server/api/events"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	apimodels "github.com/equinor/radix-operator/api-server/api/models"
	"github.com/equinor/radix-operator/api-server/api/pods"
	"github.com/equinor/radix-operator/api-server/api/utils"
	"github.com/equinor/radix-operator/api-server/api/utils/predicate"
	"github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation"
	"github.com/equinor/radix-operator/api-server/models"
	deployUtils "github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// EnvironmentHandlerOptions defines a configuration function
type EnvironmentHandlerOptions func(*EnvironmentHandler)

// WithAccounts configures all EnvironmentHandler fields
func WithAccounts(accounts models.Accounts) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.deployHandler = deployments.Init(accounts)
		eh.eventHandler = events.Init(accounts)
		eh.accounts = accounts
	}
}

// WithEventHandler configures the eventHandler used by EnvironmentHandler
func WithEventHandler(eventHandler events.EventHandler) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.eventHandler = eventHandler
	}
}

// WithTLSValidator configures the tlsValidator used by EnvironmentHandler
func WithTLSValidator(validator tlsvalidation.Validator) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.tlsValidator = validator
	}
}

func WithComponentStatuserFunc(statuser deploymentModels.ComponentStatuserFunc) EnvironmentHandlerOptions {
	return func(eh *EnvironmentHandler) {
		eh.ComponentStatuser = statuser
	}
}

// EnvironmentHandlerFactory defines a factory function for EnvironmentHandler
type EnvironmentHandlerFactory func(accounts models.Accounts) EnvironmentHandler

// NewEnvironmentHandlerFactory creates a new EnvironmentHandlerFactory
func NewEnvironmentHandlerFactory(opts ...EnvironmentHandlerOptions) EnvironmentHandlerFactory {
	return func(accounts models.Accounts) EnvironmentHandler {
		// We must make a new slice and copy values from opts into it.
		// Appending to the original opts will modify its underlying array and cause a memory leak.
		newOpts := make([]EnvironmentHandlerOptions, len(opts), len(opts)+1)
		copy(newOpts, opts)
		newOpts = append(newOpts, WithAccounts(accounts))
		eh := Init(newOpts...)
		return eh
	}
}

// EnvironmentHandler Instance variables
type EnvironmentHandler struct {
	deployHandler     deployments.DeployHandler
	eventHandler      events.EventHandler
	accounts          models.Accounts
	tlsValidator      tlsvalidation.Validator
	ComponentStatuser deploymentModels.ComponentStatuserFunc
}

// Init Constructor.
// Use the WithAccounts configuration function to configure a 'ready to use' EnvironmentHandler.
// EnvironmentHandlerOptions are processed in the sequence they are passed to this function.
func Init(opts ...EnvironmentHandlerOptions) EnvironmentHandler {
	eh := EnvironmentHandler{
		ComponentStatuser: deploymentModels.ComponentStatusFromDeployment,
	}

	for _, opt := range opts {
		opt(&eh)
	}

	return eh
}

// GetEnvironmentSummary handles api calls and returns a slice of EnvironmentSummary data for each environment
func (eh EnvironmentHandler) GetEnvironmentSummary(ctx context.Context, appName string) ([]*environmentModels.EnvironmentSummary, error) {
	rr, err := kubequery.GetRadixRegistration(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	ra, err := kubequery.GetRadixApplication(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		// This is no error, as the application may only have been just registered
		if errors.IsNotFound(err) {
			return []*environmentModels.EnvironmentSummary{}, nil
		}
		return nil, err
	}
	reList, err := kubequery.GetRadixEnvironments(ctx, eh.accounts.ServiceAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	rjList, err := kubequery.GetRadixJobs(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	envNames := slice.Map(reList, func(re radixv1.RadixEnvironment) string { return re.Spec.EnvName })
	rdList, err := kubequery.GetRadixDeploymentsForEnvironments(ctx, eh.accounts.UserAccount.RadixClient, appName, envNames, 10)
	if err != nil {
		return nil, err
	}

	environments := apimodels.BuildEnvironmentSummaryList(rr, ra, reList, rdList, rjList)
	return environments, nil
}

// GetEnvironment Handler for GetEnvironment
func (eh EnvironmentHandler) GetEnvironment(ctx context.Context, appName, envName string) (*environmentModels.Environment, error) {
	rr, err := kubequery.GetRadixRegistration(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}

	ra, err := kubequery.GetRadixApplication(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	re, err := kubequery.GetRadixEnvironment(ctx, eh.accounts.ServiceAccount.RadixClient, appName, envName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, environmentModels.NonExistingEnvironment(err, appName, envName)
		}
		return nil, err
	}
	rdList, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	rjList, err := kubequery.GetRadixJobs(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	deploymentList, err := kubequery.GetDeploymentsForEnvironment(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	componentPodList, err := kubequery.GetPodsForEnvironmentComponents(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	hpaList, err := kubequery.GetHorizontalPodAutoscalersForEnvironment(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	scaledObjects, err := kubequery.GetScaledObjectsForEnvironment(ctx, eh.accounts.UserAccount.KedaClient, appName, envName)
	if err != nil {
		return nil, err
	}
	noJobPayloadReq, err := labels.NewRequirement(kube.RadixSecretTypeLabel, selection.NotEquals, []string{string(kube.RadixSecretJobPayload)})
	if err != nil {
		return nil, err
	}
	secretList, err := kubequery.GetSecretsForEnvironment(ctx, eh.accounts.ServiceAccount.Client, appName, envName, *noJobPayloadReq)
	if err != nil {
		return nil, err
	}
	secretProviderClassList, err := kubequery.GetSecretProviderClassesForEnvironment(ctx, eh.accounts.ServiceAccount.SecretProviderClient, appName, envName)
	if err != nil {
		return nil, err
	}
	eventList, err := kubequery.GetEventsForEnvironment(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	certs, err := kubequery.GetCertificatesForEnvironment(ctx, eh.accounts.ServiceAccount.CertManagerClient, appName, envName)
	if err != nil {
		return nil, err
	}
	certRequests, err := kubequery.GetCertificateRequestsForEnvironment(ctx, eh.accounts.ServiceAccount.CertManagerClient, appName, envName)
	if err != nil {
		return nil, err
	}

	env := apimodels.BuildEnvironment(ctx, rr, ra, re, rdList, rjList, deploymentList, componentPodList, hpaList, secretList, secretProviderClassList, eventList, certs, certRequests, eh.tlsValidator, scaledObjects)
	return env, nil
}

// CreateEnvironment Handler for CreateEnvironment. Creates an environment if it does not exist
func (eh EnvironmentHandler) CreateEnvironment(ctx context.Context, appName, envName string) (*radixv1.RadixEnvironment, error) {
	// ensure application exists
	rr, err := eh.accounts.UserAccount.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// idempotent creation of RadixEnvironment
	re, err := eh.accounts.ServiceAccount.RadixClient.RadixV1().RadixEnvironments().Create(ctx, k8sObjectUtils.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(appName).
		WithEnvironmentName(envName).
		WithRegistrationOwner(rr).
		BuildRE(),
		metav1.CreateOptions{})
	// if an error is anything other than already-exist, return it
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	return re, nil
}

// DeleteEnvironment Handler for DeleteEnvironment. Deletes an environment if it is considered orphaned
func (eh EnvironmentHandler) DeleteEnvironment(ctx context.Context, appName, envName string) error {
	re, err := kubequery.GetRadixEnvironment(ctx, eh.accounts.ServiceAccount.RadixClient, appName, envName)
	if err != nil {
		return err
	}

	if !re.Status.Orphaned && re.Status.OrphanedTimestamp == nil {
		// Must be removed from radix config first
		return environmentModels.CannotDeleteNonOrphanedEnvironment(appName, envName)
	}

	// idempotent removal of RadixEnvironment
	err = eh.accounts.ServiceAccount.RadixClient.RadixV1().RadixEnvironments().Delete(ctx, re.Name, metav1.DeleteOptions{})
	// if an error is anything other than not-found, return it
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

// getNotOrphanedEnvNames returns a slice of non-unique-names of not-orphaned environments
func (eh EnvironmentHandler) getNotOrphanedEnvNames(ctx context.Context, appName string) ([]string, error) {
	reList, err := kubequery.GetRadixEnvironments(ctx, eh.accounts.ServiceAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	return slice.Map(
		slice.FindAll(reList, predicate.IsNotOrphanEnvironment),
		func(re radixv1.RadixEnvironment) string { return re.Spec.EnvName },
	), nil
}

// GetLogs handler for GetLogs
func (eh EnvironmentHandler) GetLogs(ctx context.Context, appName, envName, podName string, sinceTime *time.Time, logLines *int64, previousLog bool, follow bool) (io.ReadCloser, error) {
	podHandler := pods.Init(eh.accounts.UserAccount.Client)
	return podHandler.HandleGetEnvironmentPodLog(ctx, appName, envName, podName, "", sinceTime, logLines, previousLog, follow)
}

// GetScheduledJobLogs handler for GetScheduledJobLogs
func (eh EnvironmentHandler) GetScheduledJobLogs(ctx context.Context, appName, envName, scheduledJobName, replicaName string, sinceTime *time.Time, logLines *int64, follow bool) (io.ReadCloser, error) {
	handler := pods.Init(eh.accounts.UserAccount.Client)
	return handler.HandleGetEnvironmentScheduledJobLog(ctx, appName, envName, scheduledJobName, replicaName, "", sinceTime, logLines, follow)
}

// GetAuxiliaryResourcePodLog handler for GetAuxiliaryResourcePodLog
func (eh EnvironmentHandler) GetAuxiliaryResourcePodLog(ctx context.Context, appName, envName, componentName, auxType, podName string, sinceTime *time.Time, logLines *int64, follow bool) (io.ReadCloser, error) {
	podHandler := pods.Init(eh.accounts.UserAccount.Client)
	return podHandler.HandleGetEnvironmentAuxiliaryResourcePodLog(ctx, appName, envName, componentName, auxType, podName, sinceTime, logLines, follow)
}

// StopEnvironment Stops all components in the environment
func (eh EnvironmentHandler) StopEnvironment(ctx context.Context, appName, envName string) error {
	radixDeployment, err := kubequery.GetLatestRadixDeployment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return err
	}
	if radixDeployment == nil {
		return http.ValidationError(radixv1.KindRadixDeployment, "no radix deployments found")
	}

	log.Ctx(ctx).Info().Msgf("Stopping components in environment %s, %s", envName, appName)
	for _, deployComponent := range radixDeployment.Spec.Components {
		err := eh.StopComponent(ctx, appName, envName, deployComponent.GetName(), true)
		if err != nil {
			return err
		}
	}
	return nil
}

// ResetManuallyStoppedComponentsInEnvironment Starts all components in the environment
func (eh EnvironmentHandler) ResetManuallyStoppedComponentsInEnvironment(ctx context.Context, appName, envName string) error {
	radixDeployment, err := kubequery.GetLatestRadixDeployment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return err
	}
	if radixDeployment == nil {
		return http.ValidationError(radixv1.KindRadixDeployment, "no radix deployments found")
	}

	log.Ctx(ctx).Info().Msgf("Starting components in environment %s, %s", envName, appName)
	for _, deployComponent := range radixDeployment.Spec.Components {
		if override := deployComponent.GetReplicasOverride(); override != nil && *override == 0 {
			if err := eh.ResetScaledComponent(ctx, appName, envName, deployComponent.GetName(), true); err != nil {
				return err
			}
		}
	}
	return nil
}

// RestartEnvironment Restarts all components in the environment
func (eh EnvironmentHandler) RestartEnvironment(ctx context.Context, appName, envName string) error {
	radixDeployment, err := kubequery.GetLatestRadixDeployment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return err
	}
	if radixDeployment == nil {
		return http.ValidationError(radixv1.KindRadixDeployment, "no radix deployments found")
	}

	log.Ctx(ctx).Info().Msgf("Restarting components in environment %s, %s", envName, appName)
	for _, deployComponent := range radixDeployment.Spec.Components {
		err := eh.RestartComponent(ctx, appName, envName, deployComponent.GetName(), true)
		if err != nil {
			return err
		}
	}
	return nil
}

// StopApplication Stops all components in all environments of the application
func (eh EnvironmentHandler) StopApplication(ctx context.Context, appName string) error {
	environmentNames, err := eh.getNotOrphanedEnvNames(ctx, appName)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Stopping components in the application %s", appName)
	for _, environmentName := range environmentNames {
		err := eh.StopEnvironment(ctx, appName, environmentName)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartApplication Starts all components in all environments of the application
func (eh EnvironmentHandler) StartApplication(ctx context.Context, appName string) error {
	environmentNames, err := eh.getNotOrphanedEnvNames(ctx, appName)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Starting components in the application %s", appName)
	for _, environmentName := range environmentNames {
		err := eh.ResetManuallyStoppedComponentsInEnvironment(ctx, appName, environmentName)
		if err != nil {
			return err
		}
	}
	return nil
}

// RestartApplication Restarts all components in all environments of the application
func (eh EnvironmentHandler) RestartApplication(ctx context.Context, appName string) error {
	environmentNames, err := eh.getNotOrphanedEnvNames(ctx, appName)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Restarting components in the application %s", appName)
	for _, environmentName := range environmentNames {
		err := eh.RestartEnvironment(ctx, appName, environmentName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (eh EnvironmentHandler) getRadixCommonComponentUpdater(ctx context.Context, appName, envName, componentName string) (radixDeployCommonComponentUpdater, error) {
	rd, err := kubequery.GetLatestRadixDeployment(ctx, eh.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	if rd == nil {
		return nil, http.ValidationError(radixv1.KindRadixDeployment, "no radix deployments found")
	}
	baseUpdater := &baseComponentUpdater{
		appName:         appName,
		envName:         envName,
		componentName:   componentName,
		radixDeployment: rd,
	}
	var updater radixDeployCommonComponentUpdater
	var componentToPatch radixv1.RadixCommonDeployComponent
	componentIndex, componentToPatch := deployUtils.GetDeploymentComponent(rd, componentName)
	if !radixutils.IsNil(componentToPatch) {
		updater = &radixDeployComponentUpdater{base: baseUpdater}
	} else {
		componentIndex, componentToPatch = deployUtils.GetDeploymentJobComponent(rd, componentName)
		if radixutils.IsNil(componentToPatch) {
			return nil, environmentModels.NonExistingComponent(appName, componentName)
		}
		updater = &radixDeployJobComponentUpdater{base: baseUpdater}
	}

	hpas, err := kubequery.GetHorizontalPodAutoscalersForEnvironment(ctx, eh.accounts.UserAccount.Client, appName, envName)
	if err != nil {
		return nil, err
	}
	scalers, err := kubequery.GetScaledObjectsForEnvironment(ctx, eh.accounts.UserAccount.KedaClient, appName, envName)
	if err != nil {
		return nil, err
	}

	baseUpdater.componentIndex = componentIndex
	baseUpdater.componentToPatch = componentToPatch

	ra, err := kubequery.GetRadixApplication(ctx, eh.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	baseUpdater.environmentConfig = utils.GetComponentEnvironmentConfig(ra, envName, componentName)
	baseUpdater.componentState, err = eh.getComponentStateFromSpec(ctx, rd, componentToPatch, hpas, scalers)
	if err != nil {
		return nil, err
	}
	return updater, nil
}

func (eh EnvironmentHandler) commit(ctx context.Context, updater radixDeployCommonComponentUpdater, commitFunc func(updater radixDeployCommonComponentUpdater) error) error {
	rd := updater.getRadixDeployment()
	oldJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}

	if err := commitFunc(updater); err != nil {
		return err
	}
	newJSON, err := json.Marshal(rd)
	if err != nil {
		return err
	}
	err = eh.patch(ctx, rd.GetNamespace(), rd.GetName(), oldJSON, newJSON)
	if err != nil {
		return err
	}
	return nil
}
