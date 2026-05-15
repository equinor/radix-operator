package buildstatus

import (
	"context"
	"sort"
	"strings"

	build_models "github.com/equinor/radix-operator/api-server/api/buildstatus/models"
	"github.com/equinor/radix-operator/api-server/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildStatusHandler struct {
	accounts      models.Accounts
	pipelineBadge build_models.PipelineBadge
}

func Init(accounts models.Accounts, pipelineBadge build_models.PipelineBadge) BuildStatusHandler {
	return BuildStatusHandler{accounts: accounts, pipelineBadge: pipelineBadge}
}

// GetBuildStatusForApplication Gets a list of build status for environments
func (handler BuildStatusHandler) GetBuildStatusForApplication(ctx context.Context, appName, env, pipeline string) ([]byte, error) {
	var output []byte

	// Get latest RJ
	serviceAccount := handler.accounts.ServiceAccount
	namespace := operatorUtils.GetAppNamespace(appName)

	// Get list of Jobs in the namespace
	radixJobs, err := serviceAccount.RadixClient.RadixV1().RadixJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var buildCondition v1.RadixJobCondition
	if latestPipelineJob := getLatestPipelineJobToEnvironment(radixJobs.Items, env, pipeline); latestPipelineJob != nil {
		buildCondition = latestPipelineJob.Status.Condition
	}

	output, err = handler.pipelineBadge.GetBadge(ctx, buildCondition, v1.RadixPipelineType(pipeline))
	if err != nil {
		return nil, err
	}

	return output, nil
}

func getLatestPipelineJobToEnvironment(jobs []v1.RadixJob, env, pipeline string) *v1.RadixJob {
	// Filter out all BuildDeploy jobs
	allBuildDeployJobs := []v1.RadixJob{}
	for _, job := range jobs {
		if strings.EqualFold(string(job.Spec.PipeLineType), pipeline) {
			allBuildDeployJobs = append(allBuildDeployJobs, job)
		}
	}

	// Sort the slice by created date (In descending order)
	sort.Slice(allBuildDeployJobs[:], func(i, j int) bool {
		return allBuildDeployJobs[j].CreationTimestamp.Before(&allBuildDeployJobs[i].CreationTimestamp)
	})

	// Get status of the last job to requested environment
	for _, buildDeployJob := range allBuildDeployJobs {
		for _, targetEnvironment := range buildDeployJob.Status.TargetEnvs {
			if targetEnvironment == env {
				return &buildDeployJob
			}
		}
	}

	return nil

}
