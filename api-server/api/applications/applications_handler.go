package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/equinor/radix-operator/api-server/api/environments"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/api-server/api/middleware/auth"
	apimodels "github.com/equinor/radix-operator/api-server/api/models"
	"github.com/equinor/radix-operator/api-server/api/utils/warningcollector"
	"github.com/equinor/radix-operator/api-server/internal/config"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	jobPipeline "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog/log"
	authorizationapi "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/retry"
)

type hasAccessToGetConfigMapFunc func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error)
type CollectContextWarningsFunc func(ctx context.Context) []string
type ApplicationHandlerOption func(ah *ApplicationHandler)

// ApplicationHandler Instance variables
type ApplicationHandler struct {
	environmentHandler              environments.EnvironmentHandler
	accounts                        models.Accounts
	config                          config.Config
	hasAccessToGetConfigMap         hasAccessToGetConfigMapFunc
	getWarningCollectionFromContext CollectContextWarningsFunc
}

// NewApplicationHandler Constructor
func NewApplicationHandler(accounts models.Accounts, config config.Config, hasAccessToGetConfigMap hasAccessToGetConfigMapFunc, options ...ApplicationHandlerOption) ApplicationHandler {
	ah := ApplicationHandler{
		environmentHandler:              environments.Init(environments.WithAccounts(accounts)),
		accounts:                        accounts,
		config:                          config,
		hasAccessToGetConfigMap:         hasAccessToGetConfigMap,
		getWarningCollectionFromContext: warningcollector.GetWarningCollectionFromContext,
	}

	for _, option := range options {
		option(&ah)
	}

	return ah
}

func (ah *ApplicationHandler) getUserAccount() models.Account {
	return ah.accounts.UserAccount
}

func (ah *ApplicationHandler) getServiceAccount() models.Account {
	return ah.accounts.ServiceAccount
}

// GetApplication handler for GetApplication
func (ah *ApplicationHandler) GetApplication(ctx context.Context, appName string) (*applicationModels.Application, error) {
	rr, err := kubequery.GetRadixRegistration(ctx, ah.accounts.UserAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	ra, err := kubequery.GetRadixApplication(ctx, ah.accounts.UserAccount.RadixClient, appName)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	reList, err := kubequery.GetRadixEnvironments(ctx, ah.accounts.ServiceAccount.RadixClient, appName)
	if err != nil {
		return nil, err
	}
	rjList, err := kubequery.GetRadixJobs(ctx, ah.getUserAccount().RadixClient, appName)
	if err != nil {
		return nil, err
	}
	envNames := slice.Map(reList, func(re v1.RadixEnvironment) string { return re.Spec.EnvName })
	rdList, err := kubequery.GetRadixDeploymentsForEnvironments(ctx, ah.accounts.UserAccount.RadixClient, appName, envNames, 10)
	if err != nil {
		return nil, err
	}

	userIsAdmin, err := ah.userIsAppAdmin(ctx, appName)
	if err != nil {
		return nil, err
	}

	dnsAliases := kubequery.GetDNSAliases(ctx, ah.accounts.UserAccount.RadixClient, ra)
	application := apimodels.BuildApplication(rr, ra, reList, rdList, rjList, userIsAdmin, dnsAliases, ah.config.DNSZone)
	return application, nil
}

// RegisterApplication handler for RegisterApplication
func (ah *ApplicationHandler) RegisterApplication(ctx context.Context, applicationRegistrationRequest applicationModels.ApplicationRegistrationRequest) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	var err error

	application := applicationRegistrationRequest.ApplicationRegistration
	creator := auth.GetOriginator(ctx)

	if len(application.SharedSecret) == 0 {
		application.SharedSecret = radixutils.RandString(20)
		log.Ctx(ctx).Debug().Msg("There is no Shared Secret specified for the registering application - a random Shared Secret has been generated")
	}

	radixRegistration, err := applicationModels.NewApplicationRegistrationBuilder().
		WithAppRegistration(application).
		WithAppID(ulid.Make().String()).
		WithCreator(creator).
		BuildRR()
	if err != nil {
		return nil, err
	}

	err = ah.validateUserIsMemberOfAdGroups(ctx, applicationRegistrationRequest.ApplicationRegistration.Name, applicationRegistrationRequest.ApplicationRegistration.AdGroups)
	if err != nil {
		return nil, err
	}

	if !applicationRegistrationRequest.AcknowledgeWarnings {
		warnings, err := ah.ValidateRadixRegistration(ctx, radixRegistration, false)
		if err != nil {
			return nil, err
		}
		if len(warnings) > 0 {
			return &applicationModels.ApplicationRegistrationUpsertResponse{Warnings: warnings}, nil
		}
	}

	radixRegistration, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Create(ctx, radixRegistration, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newApplication := applicationModels.NewApplicationRegistrationBuilder().WithRadixRegistration(radixRegistration).Build()
	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: &newApplication,
	}, nil
}

// ChangeRegistrationDetails handler for ChangeRegistrationDetails
func (ah *ApplicationHandler) ChangeRegistrationDetails(ctx context.Context, appName string, applicationRegistrationRequest applicationModels.ApplicationRegistrationRequest) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {
	application := applicationRegistrationRequest.ApplicationRegistration
	if appName != application.Name {
		return nil, radixhttp.ValidationError("Radix Registration", fmt.Sprintf("App name %s does not correspond with application name %s", appName, application.Name))
	}

	currentRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	radixRegistration, err := applicationModels.NewApplicationRegistrationBuilder().WithAppRegistration(application).BuildRR()
	if err != nil {
		return nil, err
	}

	updatedRegistration := currentRegistration.DeepCopy()

	// Only these fields can change over time
	updatedRegistration.Spec.CloneURL = radixRegistration.Spec.CloneURL
	updatedRegistration.Spec.SharedSecret = radixRegistration.Spec.SharedSecret
	updatedRegistration.Spec.AdGroups = radixRegistration.Spec.AdGroups
	updatedRegistration.Spec.ReaderAdGroups = radixRegistration.Spec.ReaderAdGroups
	updatedRegistration.Spec.Owner = radixRegistration.Spec.Owner
	updatedRegistration.Spec.ConfigurationItem = radixRegistration.Spec.ConfigurationItem
	updatedRegistration.Spec.ConfigBranch = radixRegistration.Spec.ConfigBranch
	updatedRegistration.Spec.RadixConfigFullName = radixRegistration.Spec.RadixConfigFullName

	warnings, err := ah.ValidateRadixRegistration(ctx, radixRegistration, true)
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 && !applicationRegistrationRequest.AcknowledgeWarnings {
		return &applicationModels.ApplicationRegistrationUpsertResponse{Warnings: warnings}, nil
	}

	updatedRegistration, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(ctx, updatedRegistration, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	updatedApplication := applicationModels.NewApplicationRegistrationBuilder().WithRadixRegistration(updatedRegistration).Build()
	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: &updatedApplication,
	}, nil
}

// ModifyRegistrationDetails handler for ModifyRegistrationDetails
func (ah *ApplicationHandler) ModifyRegistrationDetails(ctx context.Context, appName string, applicationRegistrationPatchRequest applicationModels.ApplicationRegistrationPatchRequest) (*applicationModels.ApplicationRegistrationUpsertResponse, error) {

	currentRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	runUpdate := false
	updatedRegistration := currentRegistration.DeepCopy()

	// Only these fields can change over time
	patchRequest := applicationRegistrationPatchRequest.ApplicationRegistrationPatch
	if patchRequest.AdGroups != nil && !radixutils.ArrayEqualElements(currentRegistration.Spec.AdGroups, *patchRequest.AdGroups) {
		err := ah.validateUserIsMemberOfAdGroups(ctx, appName, *patchRequest.AdGroups)
		if err != nil {
			return nil, err
		}
		updatedRegistration.Spec.AdGroups = *patchRequest.AdGroups
		runUpdate = true
	}
	if patchRequest.AdUsers != nil && !radixutils.ArrayEqualElements(currentRegistration.Spec.AdUsers, *patchRequest.AdUsers) {
		updatedRegistration.Spec.AdUsers = *patchRequest.AdUsers
		runUpdate = true
	}
	if patchRequest.ReaderAdGroups != nil && !radixutils.ArrayEqualElements(currentRegistration.Spec.ReaderAdGroups, *patchRequest.ReaderAdGroups) {
		updatedRegistration.Spec.ReaderAdGroups = *patchRequest.ReaderAdGroups
		runUpdate = true
	}
	if patchRequest.ReaderAdUsers != nil && !radixutils.ArrayEqualElements(currentRegistration.Spec.ReaderAdUsers, *patchRequest.ReaderAdUsers) {
		updatedRegistration.Spec.ReaderAdUsers = *patchRequest.ReaderAdUsers
		runUpdate = true
	}

	if patchRequest.Owner != nil && *patchRequest.Owner != "" {
		updatedRegistration.Spec.Owner = *patchRequest.Owner
		runUpdate = true
	}

	if patchRequest.Repository != nil && *patchRequest.Repository != "" {
		cloneURL := operatorUtils.GetGithubCloneURLFromRepo(*patchRequest.Repository)
		updatedRegistration.Spec.CloneURL = cloneURL
		runUpdate = true
	}

	if patchRequest.ConfigBranch != nil {
		if trimmedBranch := strings.TrimSpace(*patchRequest.ConfigBranch); trimmedBranch != "" {
			updatedRegistration.Spec.ConfigBranch = trimmedBranch
			runUpdate = true
		}
	}

	if patchRequest.ConfigBranch != nil {
		if trimmedBranch := strings.TrimSpace(*patchRequest.ConfigBranch); trimmedBranch != "" {
			updatedRegistration.Spec.ConfigBranch = trimmedBranch
			runUpdate = true
		}
	}

	if trimmedConfigFulleName := strings.TrimSpace(patchRequest.RadixConfigFullName); trimmedConfigFulleName != "" {
		updatedRegistration.Spec.RadixConfigFullName = trimmedConfigFulleName
		runUpdate = true
	}

	if patchRequest.ConfigurationItem != nil {
		if trimmedConfigurationItem := strings.TrimSpace(*patchRequest.ConfigurationItem); trimmedConfigurationItem != "" {
			updatedRegistration.Spec.ConfigurationItem = trimmedConfigurationItem
			runUpdate = true
		}
	}

	if runUpdate {
		warnings, err := ah.ValidateRadixRegistration(ctx, updatedRegistration, true)
		if err != nil {
			return nil, err
		}
		if len(warnings) > 0 && !applicationRegistrationPatchRequest.AcknowledgeWarnings {
			return &applicationModels.ApplicationRegistrationUpsertResponse{Warnings: warnings}, nil
		}

		updatedRegistration, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(ctx, updatedRegistration, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}

	updatedApplication := applicationModels.NewApplicationRegistrationBuilder().WithRadixRegistration(updatedRegistration).Build()
	return &applicationModels.ApplicationRegistrationUpsertResponse{
		ApplicationRegistration: &updatedApplication,
	}, nil
}

func (ah *ApplicationHandler) ValidateRadixRegistration(ctx context.Context, radixRegistration *v1.RadixRegistration, shouldUpdateExisting bool) ([]string, error) {
	var err error

	if shouldUpdateExisting {
		// Make check that this is an existing application
		_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(ctx, radixRegistration, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}})
	} else {
		// Make check that this is a new application
		_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Create(ctx, radixRegistration, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
	}

	warnings := ah.getWarningCollectionFromContext(ctx)
	return warnings, err
}

// DeleteApplication handler for DeleteApplication
func (ah *ApplicationHandler) DeleteApplication(ctx context.Context, appName string) error {
	// Make check that this is an existing application
	_, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Delete(ctx, appName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

// GetSupportedPipelines handler for GetSupportedPipelines
func (ah *ApplicationHandler) GetSupportedPipelines() []string {
	supportedPipelines := make([]string, 0)
	pipelines := jobPipeline.GetSupportedPipelines()
	for _, pipeline := range pipelines {
		supportedPipelines = append(supportedPipelines, string(pipeline.Type))
	}

	return supportedPipelines
}

// TriggerPipelineBuild Triggers build pipeline for an application
func (ah *ApplicationHandler) TriggerPipelineBuild(ctx context.Context, appName string, r *http.Request) (*jobModels.JobSummary, error) {
	pipelineName := "build"
	jobSummary, err := ah.triggerPipelineBuildOrBuildDeploy(ctx, appName, pipelineName, r)
	if err != nil {
		return nil, err
	}
	return jobSummary, nil
}

// TriggerPipelineBuildDeploy Triggers build-deploy pipeline for an application
func (ah *ApplicationHandler) TriggerPipelineBuildDeploy(ctx context.Context, appName string, r *http.Request) (*jobModels.JobSummary, error) {
	pipelineName := "build-deploy"
	jobSummary, err := ah.triggerPipelineBuildOrBuildDeploy(ctx, appName, pipelineName, r)
	if err != nil {
		return nil, err
	}
	return jobSummary, nil
}

// TriggerPipelinePromote Triggers promote pipeline for an application
func (ah *ApplicationHandler) TriggerPipelinePromote(ctx context.Context, appName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersPromote
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}

	deploymentName := pipelineParameters.DeploymentName
	fromEnvironment := pipelineParameters.FromEnvironment
	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(deploymentName) == "" || strings.TrimSpace(fromEnvironment) == "" || strings.TrimSpace(toEnvironment) == "" {
		return nil, radixhttp.ValidationError("Radix Application Pipeline", "Deployment name, from environment and to environment are required for \"promote\" pipeline")
	}

	log.Ctx(ctx).Info().Msgf("Creating promote pipeline jobController for %s using deployment %s from environment %s into environment %s", appName, deploymentName, fromEnvironment, toEnvironment)

	pipeline, err := jobPipeline.GetPipelineFromName("promote")
	if err != nil {
		return nil, err
	}

	radixDeployment, err := ah.getRadixDeploymentForPromotePipeline(ctx, appName, fromEnvironment, deploymentName)
	if err != nil {
		return nil, err
	}
	pipelineParameters.DeploymentName = radixDeployment.GetName()

	jobParameters := pipelineParameters.MapPipelineParametersPromoteToJobParameter()
	jobParameters.CommitID = radixDeployment.GetLabels()[kube.RadixCommitLabel]
	jobSummary, err := HandleStartPipelineJob(ctx, ah.accounts.UserAccount.RadixClient, appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

func (ah *ApplicationHandler) getRadixDeploymentForPromotePipeline(ctx context.Context, appName string, envName, deploymentName string) (*v1.RadixDeployment, error) {
	radixDeployment, err := kubequery.GetRadixDeploymentByName(ctx, ah.accounts.UserAccount.RadixClient, appName, envName, deploymentName)
	if err == nil {
		return radixDeployment, nil
	}
	if !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get deployment %s for the app %s, environment %s: %v", deploymentName, appName, envName, err)
	}
	envRadixDeployments, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, ah.accounts.UserAccount.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	radixDeployments := slice.FindAll(envRadixDeployments, func(rd v1.RadixDeployment) bool { return strings.HasSuffix(rd.Name, deploymentName) })
	if len(radixDeployments) != 1 {
		return nil, errors.New("invalid or not existing deployment name")
	}
	return &radixDeployments[0], nil
}

// TriggerPipelineDeploy Triggers deploy pipeline for an application
func (ah *ApplicationHandler) TriggerPipelineDeploy(ctx context.Context, appName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersDeploy
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}

	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(toEnvironment) == "" {
		return nil, radixhttp.ValidationError("Radix Application Pipeline", "To environment is required for \"deploy\" pipeline")
	}

	log.Ctx(ctx).Info().Msgf("Creating deploy pipeline jobController for %s into environment %s", appName, toEnvironment)

	pipeline, err := jobPipeline.GetPipelineFromName("deploy")
	if err != nil {
		return nil, err
	}

	jobParameters := pipelineParameters.MapPipelineParametersDeployToJobParameter()

	jobSummary, err := HandleStartPipelineJob(ctx, ah.accounts.UserAccount.RadixClient, appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

// TriggerPipelineApplyConfig Triggers apply config pipeline for an application
func (ah *ApplicationHandler) TriggerPipelineApplyConfig(ctx context.Context, appName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersApplyConfig
	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}

	log.Ctx(ctx).Info().Msgf("Creating apply config pipeline jobController for %s", appName)

	pipeline, err := jobPipeline.GetPipelineFromName("apply-config")
	if err != nil {
		return nil, err
	}

	jobParameters := pipelineParameters.MapPipelineParametersApplyConfigToJobParameter()

	jobSummary, err := HandleStartPipelineJob(ctx, ah.accounts.UserAccount.RadixClient, appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

func (ah *ApplicationHandler) triggerPipelineBuildOrBuildDeploy(ctx context.Context, appName, pipelineName string, r *http.Request) (*jobModels.JobSummary, error) {
	var pipelineParameters applicationModels.PipelineParametersBuild
	userAccount := ah.getUserAccount()

	if err := json.NewDecoder(r.Body).Decode(&pipelineParameters); err != nil {
		return nil, err
	}
	jobParameters := pipelineParameters.MapPipelineParametersBuildToJobParameter()
	envName := pipelineParameters.ToEnvironment
	commitID := pipelineParameters.CommitID

	if strings.TrimSpace(appName) == "" || strings.TrimSpace(jobParameters.GitRef) == "" {
		return nil, applicationModels.AppNameAndBranchAreRequiredForStartingPipeline()
	}

	log.Ctx(ctx).Info().Msgf("Creating build pipeline jobController for %s on %s %s for commit %s", appName, jobParameters.GitRefType, jobParameters.GitRef, commitID)
	radixRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Check if branch is mapped
	if !applicationconfig.IsConfigBranch(jobParameters.GitRef, radixRegistration) {
		ra, err := userAccount.RadixClient.RadixV1().RadixApplications(operatorUtils.GetAppNamespace(appName)).Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		targetEnvironments := applicationconfig.GetAllTargetEnvironments(jobParameters.GitRef, jobParameters.GitRefType, ra)
		if len(targetEnvironments) == 0 {
			return nil, applicationModels.UnmatchedBranchToEnvironment(jobParameters.GitRef)
		}

		if len(envName) > 0 && !slice.Any(targetEnvironments, func(targetEnvName string) bool { return targetEnvName == envName }) {
			return nil, applicationModels.EnvironmentNotMappedToBranch(envName, jobParameters.GitRef)
		}
	}

	pipeline, err := jobPipeline.GetPipelineFromName(pipelineName)
	if err != nil {
		return nil, err
	}

	log.Ctx(ctx).Info().Msgf("Creating build pipeline job for %s on %s %s for commit %s%s", appName, jobParameters.GitRefType, jobParameters.GitRef, commitID,
		radixutils.TernaryString(len(envName) > 0, fmt.Sprintf(", for environment %s", envName), ""))

	jobSummary, err := HandleStartPipelineJob(ctx, ah.accounts.UserAccount.RadixClient, appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

// RegenerateDeployKey Regenerates deploy key and secret and returns the new key
func (ah *ApplicationHandler) RegenerateDeployKey(ctx context.Context, appName string, regenerateDeployKeyAndSecretData applicationModels.RegenerateDeployKeyData) error {
	if regenerateDeployKeyAndSecretData.PrivateKey == "" {
		// Deleting the secret with the private key. This triggers the RR to be reconciled and the new key to be generated
		err := ah.getUserAccount().Client.CoreV1().Secrets(operatorUtils.GetAppNamespace(appName)).Delete(ctx, defaults.GitPrivateKeySecretName, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
		// Wait for new secret to be created
		err = wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Second, true, func(ctx context.Context) (done bool, err error) {
			_, err = ah.accounts.UserAccount.Client.CoreV1().Secrets(operatorUtils.GetAppNamespace(appName)).Get(ctx, defaults.GitPrivateKeySecretName, metav1.GetOptions{})
			if err == nil {
				return true, nil
			}
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		})
		if errors.Is(err, context.DeadlineExceeded) {
			log.Ctx(ctx).Warn().Msgf("context deadline exceeded while waiting for new deploy key secret to be created for application %s", appName)
			return nil
		}
		return err
	}
	// Deriving the public key from the private key in order to test it for validity
	if _, err := operatorUtils.DeriveDeployKeyFromPrivateKey(regenerateDeployKeyAndSecretData.PrivateKey); err != nil {
		return fmt.Errorf("failed to derive public key from private key: %v", err)
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existingSecret, err := ah.getUserAccount().Client.CoreV1().Secrets(operatorUtils.GetAppNamespace(appName)).Get(ctx, defaults.GitPrivateKeySecretName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newSecret := existingSecret.DeepCopy()
		newSecret.Data[defaults.GitPrivateKeySecretKey] = []byte(regenerateDeployKeyAndSecretData.PrivateKey)
		if err := kubequery.PatchSecretMetadata(newSecret, defaults.GitPrivateKeySecretKey, time.Now()); err != nil {
			return err
		}
		_, err = ah.getUserAccount().Client.CoreV1().Secrets(operatorUtils.GetAppNamespace(appName)).Update(ctx, newSecret, metav1.UpdateOptions{})
		return err
	})
}

// RegenerateSharedSecret Regenerates the GitHub webhook secret for an application.
func (ah *ApplicationHandler) RegenerateSharedSecret(ctx context.Context, appName string, regenerateWebhookSecretData applicationModels.RegenerateSharedSecretData) error {
	sharedKey := strings.TrimSpace(regenerateWebhookSecretData.SharedSecret)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Make check that this is an existing application and that the user has access to it
		currentRegistration, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		updatedRegistration := currentRegistration.DeepCopy()
		if len(sharedKey) != 0 {
			updatedRegistration.Spec.SharedSecret = sharedKey
		} else {
			newShareKey, err := uuid.NewUUID()
			if err != nil {
				return fmt.Errorf("failed to generate new shared secret: %v", err)
			}
			updatedRegistration.Spec.SharedSecret = newShareKey.String()
		}

		if reflect.DeepEqual(updatedRegistration, currentRegistration) {
			return nil
		}
		if _, err := ah.ValidateRadixRegistration(ctx, updatedRegistration, true); err != nil {
			return err
		}
		_, err = ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Update(ctx, updatedRegistration, metav1.UpdateOptions{})
		return err
	})
}

func (ah *ApplicationHandler) GetDeployKeyAndSecret(ctx context.Context, appName string) (*applicationModels.DeployKeyAndSecret, error) {
	cm, err := ah.getUserAccount().Client.CoreV1().ConfigMaps(operatorUtils.GetAppNamespace(appName)).Get(ctx, defaults.GitPublicKeyConfigMapName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	publicKey := ""
	if cm != nil {
		publicKey = cm.Data[defaults.GitPublicKeyConfigMapKey]
	}
	rr, err := ah.getUserAccount().RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	sharedSecret := rr.Spec.SharedSecret
	return &applicationModels.DeployKeyAndSecret{
		PublicDeployKey: publicKey,
		SharedSecret:    sharedSecret,
	}, nil
}

func (ah *ApplicationHandler) userIsAppAdmin(ctx context.Context, appName string) (bool, error) {
	switch ah.accounts.UserAccount.Client.(type) {
	case *fake.Clientset:
		return true, nil
	default:
		review, err := ah.accounts.UserAccount.Client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &authorizationapi.SelfSubjectAccessReview{
			Spec: authorizationapi.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationapi.ResourceAttributes{
					Verb:     "patch",
					Group:    "radix.equinor.com",
					Resource: "radixregistrations",
					Name:     appName,
				},
			},
		}, metav1.CreateOptions{})
		return review.Status.Allowed, err
	}
}

func (ah *ApplicationHandler) validateUserIsMemberOfAdGroups(ctx context.Context, appName string, adGroups []string) error {
	if len(adGroups) == 0 {
		return nil
	}
	radixApiAppNamespace := operatorUtils.GetEnvironmentNamespace(ah.config.AppName, ah.config.EnvironmentName)
	name := fmt.Sprintf("access-validation-%s", appName)
	labels := map[string]string{"radix-access-validation": "true"}
	configMapName := fmt.Sprintf("%s-%s", name, strings.ToLower(operatorUtils.RandString(6)))
	role, err := createRoleToGetConfigMap(ctx, ah.accounts.ServiceAccount.Client, radixApiAppNamespace, name, labels, configMapName)
	if err != nil {
		return err
	}
	defer func() {
		err = deleteRole(context.Background(), ah.accounts.ServiceAccount.Client, radixApiAppNamespace, role.GetName())
		if err != nil {
			log.Ctx(ctx).Warn().Msgf("Failed to delete role %s: %v", role.GetName(), err)
		}
	}()
	roleBinding, err := createRoleBindingForRole(ctx, ah.accounts.ServiceAccount.Client, radixApiAppNamespace, role, name, adGroups, labels)
	if err != nil {
		return err
	}
	defer func() {
		err = deleteRoleBinding(context.Background(), ah.accounts.ServiceAccount.Client, radixApiAppNamespace, roleBinding.GetName())
		if err != nil {
			log.Ctx(ctx).Warn().Msgf("Failed to delete role binding %s: %v", roleBinding.GetName(), err)
		}
	}()

	valid, err := ah.hasAccessToGetConfigMap(ctx, ah.accounts.UserAccount.Client, radixApiAppNamespace, configMapName)
	if err != nil {
		return err
	}
	if !valid {
		return userShouldBeMemberOfAdminAdGroupError()
	}
	return nil
}

func createRoleToGetConfigMap(ctx context.Context, kubeClient kubernetes.Interface, namespace, roleName string, labels map[string]string, configMapName string) (*rbacv1.Role, error) {
	return kubeClient.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{GenerateName: roleName, Labels: labels},
		Rules: []rbacv1.PolicyRule{{
			Verbs:         []string{"get"},
			APIGroups:     []string{""},
			Resources:     []string{"configmaps"},
			ResourceNames: []string{configMapName},
		}},
	}, metav1.CreateOptions{})
}

func deleteRole(ctx context.Context, kubeClient kubernetes.Interface, namespace, roleName string) error {
	deletionPropagation := metav1.DeletePropagationBackground
	return kubeClient.RbacV1().Roles(namespace).Delete(ctx, roleName, metav1.DeleteOptions{
		PropagationPolicy: &deletionPropagation,
	})
}

func createRoleBindingForRole(ctx context.Context, kubeClient kubernetes.Interface, namespace string, role *rbacv1.Role, roleBindingName string, adGroups []string, labels map[string]string) (*rbacv1.RoleBinding, error) {
	var subjects []rbacv1.Subject
	for _, adGroup := range adGroups {
		subjects = append(subjects, rbacv1.Subject{
			Kind:     rbacv1.GroupKind,
			Name:     adGroup,
			APIGroup: rbacv1.GroupName,
		})
	}
	newRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: roleBindingName,
			Labels:       labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
					Kind:       k8s.KindRole,
					Name:       role.GetName(),
					UID:        role.GetUID(),
				},
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindRole,
			Name:     role.GetName(),
		}, Subjects: subjects,
	}
	return kubeClient.RbacV1().RoleBindings(namespace).Create(ctx, newRoleBinding, metav1.CreateOptions{})
}

func deleteRoleBinding(ctx context.Context, kubeClient kubernetes.Interface, namespace, roleBindingName string) error {
	return kubeClient.RbacV1().RoleBindings(namespace).Delete(ctx, roleBindingName, metav1.DeleteOptions{})
}
