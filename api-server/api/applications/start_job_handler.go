package applications

import (
	"context"
	"regexp"

	jobController "github.com/equinor/radix-operator/api-server/api/jobs"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/middleware/auth"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const radixGitHubWebhookUserNameRegEx = `^system:serviceaccount:radix-github-webhook-[\w]+:radix-github-webhook$`

// HandleStartPipelineJob Handles the creation of a pipeline jobController for an application
func HandleStartPipelineJob(ctx context.Context, radixClient versioned.Interface, appName string, pipeline *pipelineJob.Definition, jobParameters *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	if _, err := radixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{}); err != nil {
		return nil, err
	}

	job, err := buildPipelineJob(ctx, appName, pipeline, jobParameters)
	if err != nil {
		return nil, err
	}
	return createPipelineJob(ctx, radixClient, appName, job)
}

func createPipelineJob(ctx context.Context, radixClient versioned.Interface, appName string, job *v1.RadixJob) (*jobModels.JobSummary, error) {
	log.Ctx(ctx).Info().Msgf("Starting jobController: %s, %s", job.GetName(), jobController.WorkerImage)
	appNamespace := k8sObjectUtils.GetAppNamespace(appName)
	job, err := radixClient.RadixV1().RadixJobs(appNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	log.Ctx(ctx).Info().Msgf("Started jobController: %s, %s", job.GetName(), jobController.WorkerImage)
	return jobModels.GetSummaryFromRadixJob(job), nil
}

func buildPipelineJob(ctx context.Context, appName string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) (*v1.RadixJob, error) {
	jobName, imageTag := jobController.GetUniqueJobName()
	if len(jobSpec.ImageTag) > 0 {
		imageTag = jobSpec.ImageTag
	}

	var buildSpec v1.RadixBuildSpec
	var promoteSpec v1.RadixPromoteSpec
	var deploySpec v1.RadixDeploySpec
	var applyConfigSpec v1.RadixApplyConfigSpec

	triggeredFromWebhook, err := getTriggeredFromWebhook(ctx)
	if err != nil {
		return nil, err
	}

	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		buildSpec = v1.RadixBuildSpec{
			ImageTag:              imageTag,
			Branch:                jobSpec.Branch, //nolint:staticcheck
			ToEnvironment:         jobSpec.ToEnvironment,
			CommitID:              jobSpec.CommitID,
			PushImage:             jobSpec.PushImage,
			OverrideUseBuildCache: jobSpec.OverrideUseBuildCache,
			RefreshBuildCache:     jobSpec.RefreshBuildCache,
			GitRef:                jobSpec.GitRef,
			GitRefType:            v1.GitRefType(jobSpec.GitRefType),
		}
	case v1.Promote:
		promoteSpec = v1.RadixPromoteSpec{
			DeploymentName:  jobSpec.DeploymentName,
			FromEnvironment: jobSpec.FromEnvironment,
			ToEnvironment:   jobSpec.ToEnvironment,
			CommitID:        jobSpec.CommitID,
		}
	case v1.Deploy:
		deploySpec = v1.RadixDeploySpec{
			ToEnvironment:      jobSpec.ToEnvironment,
			ImageTagNames:      jobSpec.ImageTagNames,
			CommitID:           jobSpec.CommitID,
			ComponentsToDeploy: jobSpec.ComponentsToDeploy,
		}
	case v1.ApplyConfig:
		applyConfigSpec = v1.RadixApplyConfigSpec{
			DeployExternalDNS: jobSpec.DeployExternalDNS != nil && *jobSpec.DeployExternalDNS,
		}
	}

	job := v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: jobSpec.Branch, //nolint:staticcheck
			},
		},
		Spec: v1.RadixJobSpec{
			AppName:              appName,
			PipeLineType:         pipeline.Type,
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
