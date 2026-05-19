package models

import (
	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	"github.com/equinor/radix-operator/api-server/api/utils/predicate"
	"github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

// BuildDeployment builds a Deployment model.
func BuildDeployment(
	rr *radixv1.RadixRegistration, ra *radixv1.RadixApplication, rd *radixv1.RadixDeployment, deploymentList []appsv1.Deployment,
	podList []corev1.Pod, hpaList []autoscalingv2.HorizontalPodAutoscaler, secretList []corev1.Secret, eventList []corev1.Event,
	rjList []radixv1.RadixJob, certs []cmv1.Certificate, certRequests []cmv1.CertificateRequest, tlsValidator tlsvalidation.Validator,
	scaledObjects []v1alpha1.ScaledObject,
) *deploymentModels.Deployment {
	components := BuildComponents(ra, rd, deploymentList, podList, hpaList, secretList, eventList, certs, certRequests, tlsValidator, scaledObjects)

	// The only error that can be returned from DeploymentBuilder is related to errors from github.com/imdario/mergo
	// This type of error will only happen if incorrect objects (e.g. incompatible structs) are sent as arguments to mergo,
	// and we should consider to panic the error in the code calling merge.
	// It will currently panic the error here.
	radixJob, _ := slice.FindFirst(rjList, func(radixJob radixv1.RadixJob) bool {
		return radixJob.GetName() == rd.GetLabels()[kube.RadixJobNameLabel]
	})
	deployment, err := deploymentModels.NewDeploymentBuilder().
		WithRadixRegistration(rr).
		WithRadixDeployment(rd).
		WithPipelineJob(&radixJob).
		WithGitCommitHash(rd.Annotations[kube.RadixCommitAnnotation]).
		WithGitTags(rd.Annotations[kube.RadixGitTagsAnnotation]).
		WithComponents(components).
		BuildDeployment()
	if err != nil {
		panic(err)
	}

	return deployment

}

func GetActiveDeploymentForAppEnv(appName, envName string, rds []radixv1.RadixDeployment) (radixv1.RadixDeployment, bool) {
	return slice.FindFirst(rds, isActiveDeploymentForAppAndEnv(appName, envName))
}

func isActiveDeploymentForAppAndEnv(appName, envName string) func(rd radixv1.RadixDeployment) bool {
	envNs := operatorUtils.GetEnvironmentNamespace(appName, envName)
	return func(rd radixv1.RadixDeployment) bool {
		return predicate.IsActiveRadixDeployment(rd) && rd.Namespace == envNs
	}
}
