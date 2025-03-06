package labels

import (
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/kube"
)

const (
	AzureWorkloadIdentityUse = "azure.workload.identity/use"
)

// GetLabelsForEnvironment Get Pipeline object labels for a target build environment
func GetLabelsForEnvironment(ctx model.Context, targetEnv string) map[string]string {
	appName := ctx.GetPipelineInfo().GetAppName()
	imageTag := ctx.GetPipelineInfo().GetRadixImageTag()
	return map[string]string{
		kube.RadixAppLabel:      appName,
		kube.RadixEnvLabel:      targetEnv,
		kube.RadixJobNameLabel:  ctx.GetPipelineInfo().GetRadixPipelineJobName(),
		kube.RadixImageTagLabel: imageTag,
	}
}
