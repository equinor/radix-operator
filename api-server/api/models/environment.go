package models

import (
	"context"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/equinor/radix-common/utils/slice"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	secretModels "github.com/equinor/radix-operator/api-server/api/secrets/models"
	"github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// BuildEnvironment builds and Environment model.
func BuildEnvironment(ctx context.Context, rr *radixv1.RadixRegistration, ra *radixv1.RadixApplication, re *radixv1.RadixEnvironment, rdList []radixv1.RadixDeployment,
	rjList []radixv1.RadixJob, deploymentList []appsv1.Deployment, podList []corev1.Pod, hpaList []autoscalingv2.HorizontalPodAutoscaler,
	secretList []corev1.Secret, secretProviderClassList []secretsstorev1.SecretProviderClass, eventList []corev1.Event,
	certs []cmv1.Certificate, certRequests []cmv1.CertificateRequest, tlsValidator tlsvalidation.Validator, scaledObjects []v1alpha1.ScaledObject,
) *environmentModels.Environment {
	var buildFromBranch string
	var activeDeployment *deploymentModels.Deployment
	var secrets []secretModels.Secret

	if raEnv := getRadixApplicationEnvironment(ra, re.Spec.EnvName); raEnv != nil {
		buildFromBranch = raEnv.Build.From
	}

	if activeRd, ok := slice.FindFirst(rdList, isActiveDeploymentForAppAndEnv(ra.Name, re.Spec.EnvName)); ok {
		activeDeployment = BuildDeployment(rr, ra, &activeRd, deploymentList, podList, hpaList, secretList, eventList, rjList, certs, certRequests, tlsValidator, scaledObjects)
		secrets = BuildSecrets(ctx, secretList, secretProviderClassList, &activeRd)
		if len(activeDeployment.BuiltFromBranch) > 0 {
			buildFromBranch = activeDeployment.BuiltFromBranch
		}
	}

	return &environmentModels.Environment{
		Name:             re.Spec.EnvName,
		BranchMapping:    buildFromBranch,
		Status:           getEnvironmentConfigurationStatus(re).String(),
		Deployments:      BuildDeploymentSummaryList(rr, rdList, rjList),
		ActiveDeployment: activeDeployment,
		Secrets:          secrets,
	}
}

func getRadixApplicationEnvironment(ra *radixv1.RadixApplication, envName string) *radixv1.Environment {
	if env, ok := slice.FindFirst(ra.Spec.Environments, func(env radixv1.Environment) bool { return env.Name == envName }); ok {
		return &env
	}
	return nil
}
