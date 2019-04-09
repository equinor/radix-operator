package model

import (
	"fmt"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type PipelineType string

const (
	Build          PipelineType = "build"
	BuildAndDeploy PipelineType = "build-deploy"
)

type PipelineInfo struct {
	Type               PipelineType
	RadixRegistration  *v1.RadixRegistration
	RadixApplication   *v1.RadixApplication
	TargetEnvironments map[string]bool
	JobName            string
	Branch             string
	CommitID           string
	ImageTag           string
	UseCache           string
	PushImage          bool
}

func Init(pipelineType string, rr *v1.RadixRegistration, ra *v1.RadixApplication, targetEnv map[string]bool, jobName, branch, commitID, imageTag, useCache, pushImage string) (PipelineInfo, error) {
	pipeType := BuildAndDeploy
	if pipelineType == string(Build) {
		pipeType = Build
	} else if pipelineType == "" || pipelineType == string(BuildAndDeploy) {
		pipeType = BuildAndDeploy // default if empty
	} else {
		return PipelineInfo{}, fmt.Errorf("Does not support pipeline type: %s", pipelineType)
	}

	pushImagebool := pipeType == BuildAndDeploy || !(pushImage == "false" || pushImage == "0") // build and deploy require push

	return PipelineInfo{
		Type:               pipeType,
		RadixRegistration:  rr,
		RadixApplication:   ra,
		TargetEnvironments: targetEnv,
		JobName:            jobName,
		Branch:             branch,
		CommitID:           commitID,
		ImageTag:           imageTag,
		UseCache:           useCache,
		PushImage:          pushImagebool,
	}, nil
}

func (pipelineInfo PipelineInfo) GetAppName() string {
	return pipelineInfo.RadixRegistration.GetName()
}
