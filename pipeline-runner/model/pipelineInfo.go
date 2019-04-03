package model

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

type PipelineInfo struct {
	RadixRegistration  *v1.RadixRegistration
	RadixApplication   *v1.RadixApplication
	TargetEnvironments map[string]bool
	JobName            string
	Branch             string
	CommitID           string
	ImageTag           string
	UseCache           string
}

func (pipelineInfo PipelineInfo) GetAppName() string {
	return pipelineInfo.RadixRegistration.GetName()
}
