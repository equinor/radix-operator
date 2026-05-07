package models

import (
	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildDeploymentSummaryList builds a list of DeploymentSummary models.
func BuildDeploymentSummaryList(rr *radixv1.RadixRegistration, rdList []radixv1.RadixDeployment, rjList []radixv1.RadixJob) []*deploymentModels.DeploymentSummary {
	return slice.Map(rdList, func(rd radixv1.RadixDeployment) *deploymentModels.DeploymentSummary {
		return BuildDeploymentSummary(&rd, rr, rjList)
	})
}

// BuildDeploymentSummary builds a DeploymentSummary model.
func BuildDeploymentSummary(rd *radixv1.RadixDeployment, rr *radixv1.RadixRegistration, rjList []radixv1.RadixJob) *deploymentModels.DeploymentSummary {
	var rj *radixv1.RadixJob
	if foundRj, ok := slice.FindFirst(rjList, func(rj radixv1.RadixJob) bool { return rd.Labels[kube.RadixJobNameLabel] == rj.Name }); ok {
		rj = &foundRj
	}

	// The only error that can be returned from DeploymentBuilder is related to errors from github.com/imdario/mergo
	// This type of error will only happen if incorrect objects (e.g. incompatible structs) are sent as arguments to mergo,
	// and we should consider to panic the error in the code calling merge.
	// For now we will panic the error here.
	deploymentSummary, err := deploymentModels.
		NewDeploymentBuilder().
		WithRadixDeployment(rd).
		WithPipelineJob(rj).
		WithRadixRegistration(rr).
		BuildDeploymentSummary()
	if err != nil {
		panic(err)
	}
	return deploymentSummary
}
