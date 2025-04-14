package labels

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/kube"
)

const (
	AzureWorkloadIdentityUse = "azure.workload.identity/use"
)

// GetSubPipelineLabelsForEnvironment Get Pipeline object labels for a target build environment
func GetSubPipelineLabelsForEnvironment(pipelineInfo *model.PipelineInfo, env string) map[string]string {
	appName := pipelineInfo.GetAppName()
	imageTag := pipelineInfo.GetRadixImageTag()
	return map[string]string{
		kube.RadixAppLabel:      appName,
		kube.RadixEnvLabel:      env,
		kube.RadixJobNameLabel:  pipelineInfo.GetRadixPipelineJobName(),
		kube.RadixImageTagLabel: imageTag,
	}
}
