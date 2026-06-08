package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-common/utils/slice"
	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	jobmodels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"regexp"

	jobController "github.com/equinor/radix-operator/api-server/api/jobs"

	"github.com/equinor/radix-operator/api-server/api/middleware/auth"

	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
)

type PipelineService struct {
	RadixClient radixclient.Interface
}

// TriggerPipelinePromote Triggers promote pipeline for an application
func (svc *PipelineService) TriggerPipelinePromote(ctx context.Context, appName string, pipelineParameters applicationModels.PipelineParametersPromote) (*jobmodels.JobSummary, error) {

	deploymentName := pipelineParameters.DeploymentName
	fromEnvironment := pipelineParameters.FromEnvironment
	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(deploymentName) == "" || strings.TrimSpace(fromEnvironment) == "" || strings.TrimSpace(toEnvironment) == "" {
		return nil, radixhttp.ValidationError("Radix Application Pipeline", "Deployment name, from environment and to environment are required for \"promote\" pipeline")
	}

	log.Ctx(ctx).Info().Msgf("Creating promote pipeline jobController for %s using deployment %s from environment %s into environment %s", appName, deploymentName, fromEnvironment, toEnvironment)

	radixDeployment, err := svc.getRadixDeploymentForPromotePipeline(ctx, appName, fromEnvironment, deploymentName)
	if err != nil {
		return nil, err
	}
	pipelineParameters.DeploymentName = radixDeployment.GetName()

	jobParameters := pipelineParameters.MapPipelineParametersPromoteToJobParameter()
	jobParameters.CommitID = radixDeployment.GetLabels()[kube.RadixCommitLabel]
	jobSummary, err := svc.startPipelineJob(ctx, appName, radixv1.Promote, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

func (svc *PipelineService) getRadixDeploymentForPromotePipeline(ctx context.Context, appName string, envName, deploymentName string) (*radixv1.RadixDeployment, error) {
	radixDeployment, err := kubequery.GetRadixDeploymentByName(ctx, svc.RadixClient, appName, envName, deploymentName)
	if err == nil {
		return radixDeployment, nil
	}
	if !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get deployment %s for the app %s, environment %s: %v", deploymentName, appName, envName, err)
	}
	envRadixDeployments, err := kubequery.GetRadixDeploymentsForEnvironment(ctx, svc.RadixClient, appName, envName)
	if err != nil {
		return nil, err
	}
	radixDeployments := slice.FindAll(envRadixDeployments, func(rd radixv1.RadixDeployment) bool { return strings.HasSuffix(rd.Name, deploymentName) })
	if len(radixDeployments) != 1 {
		return nil, errors.New("invalid or not existing deployment name")
	}
	return &radixDeployments[0], nil
}

// TriggerPipelineDeploy Triggers deploy pipeline for an application
func (svc *PipelineService) TriggerPipelineDeploy(ctx context.Context, appName string, pipelineParameters applicationModels.PipelineParametersDeploy) (*jobmodels.JobSummary, error) {

	toEnvironment := pipelineParameters.ToEnvironment

	if strings.TrimSpace(toEnvironment) == "" {
		return nil, radixhttp.ValidationError("Radix Application Pipeline", "To environment is required for \"deploy\" pipeline")
	}

	log.Ctx(ctx).Info().Msgf("Creating deploy pipeline jobController for %s into environment %s", appName, toEnvironment)

	jobParameters := pipelineParameters.MapPipelineParametersDeployToJobParameter()

	jobSummary, err := svc.startPipelineJob(ctx, appName, radixv1.Deploy, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

// TriggerPipelineApplyConfig Triggers apply config pipeline for an application
func (svc *PipelineService) TriggerPipelineApplyConfig(ctx context.Context, appName string, pipelineParameters applicationModels.PipelineParametersApplyConfig) (*jobmodels.JobSummary, error) {
	log.Ctx(ctx).Info().Msgf("Creating apply config pipeline jobController for %s", appName)

	jobParameters := pipelineParameters.MapPipelineParametersApplyConfigToJobParameter()

	jobSummary, err := svc.startPipelineJob(ctx, appName, radixv1.ApplyConfig, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

// TriggerPipelineBuild Triggers build pipeline for an application
func (svc *PipelineService) TriggerPipelineBuild(ctx context.Context, appName string, pipelineParameters applicationModels.PipelineParametersBuild) (*jobModels.JobSummary, error) {
	return svc.triggerPipelineBuildOrBuildDeploy(ctx, appName, radixv1.Build, pipelineParameters)
}

// TriggerPipelineBuildDeploy Triggers build-deploy pipeline for an application
func (svc *PipelineService) TriggerPipelineBuildDeploy(ctx context.Context, appName string, pipelineParameters applicationModels.PipelineParametersBuild) (*jobModels.JobSummary, error) {
	return svc.triggerPipelineBuildOrBuildDeploy(ctx, appName, radixv1.BuildDeploy, pipelineParameters)
}

func (svc *PipelineService) triggerPipelineBuildOrBuildDeploy(ctx context.Context, appName string, pipeline radixv1.RadixPipelineType, pipelineParameters applicationModels.PipelineParametersBuild) (*jobmodels.JobSummary, error) {
	jobParameters := pipelineParameters.MapPipelineParametersBuildToJobParameter()
	envName := pipelineParameters.ToEnvironment
	commitID := pipelineParameters.CommitID

	if strings.TrimSpace(appName) == "" || strings.TrimSpace(jobParameters.GitRef) == "" {
		return nil, applicationModels.AppNameAndBranchAreRequiredForStartingPipeline()
	}

	log.Ctx(ctx).Info().Msgf("Creating build pipeline jobController for %s on %s %s for commit %s", appName, jobParameters.GitRefType, jobParameters.GitRef, commitID)
	radixRegistration, err := svc.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Check if branch is mapped
	if !applicationconfig.IsConfigBranch(jobParameters.GitRef, radixRegistration) {
		ra, err := svc.RadixClient.RadixV1().RadixApplications(operatorUtils.GetAppNamespace(appName)).Get(ctx, appName, metav1.GetOptions{})
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

	log.Ctx(ctx).Info().Str("envName", envName).Msgf("Creating build pipeline job for %s on %s %s for commit %s", appName, jobParameters.GitRefType, jobParameters.GitRef, commitID)

	jobSummary, err := svc.startPipelineJob(ctx, appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}

	return jobSummary, nil
}

const radixGitHubWebhookUserNameRegEx = `^system:serviceaccount:radix-github-webhook-[\w]+:radix-github-webhook$`

// startPipelineJob Handles the creation of a pipeline jobController for an application
func (svc *PipelineService) startPipelineJob(ctx context.Context, appName string, pipeline radixv1.RadixPipelineType, jobParameters *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	if _, err := svc.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{}); err != nil {
		return nil, err
	}

	job, err := buildPipelineJob(ctx, appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}
	return svc.createPipelineJob(ctx, appName, job)
}

func (svc *PipelineService) createPipelineJob(ctx context.Context, appName string, job *radixv1.RadixJob) (*jobModels.JobSummary, error) {
	log.Ctx(ctx).Info().Msgf("Starting jobController: %s", job.GetName())
	appNamespace := k8sObjectUtils.GetAppNamespace(appName)
	job, err := svc.RadixClient.RadixV1().RadixJobs(appNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	log.Ctx(ctx).Info().Msgf("Started jobController: %s", job.GetName())
	return jobModels.GetSummaryFromRadixJob(job), nil
}

func buildPipelineJob(ctx context.Context, appName string, pipeline radixv1.RadixPipelineType, jobSpec *jobModels.JobParameters) (*radixv1.RadixJob, error) {
	jobName, imageTag := jobController.GetUniqueJobName()
	if len(jobSpec.ImageTag) > 0 {
		imageTag = jobSpec.ImageTag
	}

	var buildSpec radixv1.RadixBuildSpec
	var promoteSpec radixv1.RadixPromoteSpec
	var deploySpec radixv1.RadixDeploySpec
	var applyConfigSpec radixv1.RadixApplyConfigSpec

	triggeredFromWebhook, err := getTriggeredFromWebhook(ctx)
	if err != nil {
		return nil, err
	}

	switch pipeline {
	case radixv1.BuildDeploy, radixv1.Build:
		buildSpec = radixv1.RadixBuildSpec{
			ImageTag:              imageTag,
			Branch:                jobSpec.Branch, //nolint:staticcheck
			ToEnvironment:         jobSpec.ToEnvironment,
			CommitID:              jobSpec.CommitID,
			PushImage:             jobSpec.PushImage,
			OverrideUseBuildCache: jobSpec.OverrideUseBuildCache,
			RefreshBuildCache:     jobSpec.RefreshBuildCache,
			GitRef:                jobSpec.GitRef,
			GitRefType:            radixv1.GitRefType(jobSpec.GitRefType),
		}
	case radixv1.Promote:
		promoteSpec = radixv1.RadixPromoteSpec{
			DeploymentName:  jobSpec.DeploymentName,
			FromEnvironment: jobSpec.FromEnvironment,
			ToEnvironment:   jobSpec.ToEnvironment,
			CommitID:        jobSpec.CommitID,
		}
	case radixv1.Deploy:
		deploySpec = radixv1.RadixDeploySpec{
			ToEnvironment:      jobSpec.ToEnvironment,
			ImageTagNames:      jobSpec.ImageTagNames,
			CommitID:           jobSpec.CommitID,
			ComponentsToDeploy: jobSpec.ComponentsToDeploy,
		}
	case radixv1.ApplyConfig:
		applyConfigSpec = radixv1.RadixApplyConfigSpec{
			DeployExternalDNS: jobSpec.DeployExternalDNS != nil && *jobSpec.DeployExternalDNS,
		}
	}

	job := radixv1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: jobSpec.Branch, //nolint:staticcheck
			},
		},
		Spec: radixv1.RadixJobSpec{
			AppName:              appName,
			PipeLineType:         pipeline,
			Build:                buildSpec,
			Promote:              promoteSpec,
			Deploy:               deploySpec,
			ApplyConfig:          applyConfigSpec,
			TriggeredFromWebhook: triggeredFromWebhook,
			TriggeredBy:          getTriggeredBy(ctx, jobSpec.TriggeredBy),
		},
	}

	return &job, nil
}

func getTriggeredFromWebhook(ctx context.Context) (bool, error) {
	re, err := regexp.Compile(radixGitHubWebhookUserNameRegEx)
	if err != nil {
		return false, err
	}
	userIdGithubWebhookSa := re.Match([]byte(auth.GetOriginator(ctx)))
	return userIdGithubWebhookSa, nil
}

func getTriggeredBy(ctx context.Context, triggeredBy string) string {
	if triggeredBy != "" && triggeredBy != "<nil>" {
		return triggeredBy
	}
	return auth.GetOriginator(ctx)
}
