package labels

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/oklog/ulid/v2"
)

const (
	AzureWorkloadIdentityUse = "azure.workload.identity/use"
)

// GetSubPipelineLabelsForEnvironment Get Pipeline object labels for a target build environment
func GetSubPipelineLabelsForEnvironment(pipelineInfo *model.PipelineInfo, env string, appID ulid.ULID) map[string]string {
	return labels.Merge(
		labels.ForApplicationName(pipelineInfo.GetAppName()),
		labels.ForApplicationID(appID),
		labels.ForEnvironmentName(env),
		labels.ForPipelineJobName(pipelineInfo.GetRadixPipelineJobName()),
		labels.ForRadixImageTag(pipelineInfo.GetRadixImageTag()),
	)
}
