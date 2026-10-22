package deployment

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_EmptyDir(t *testing.T) {
	appName, envName, compName := "anyapp", "anyenv", "anycomp"

	tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, _, certClient := SetupTest(t)
	builder := utils.NewDeploymentBuilder().
		WithRadixApplication(utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))).
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(
			utils.NewDeployComponentBuilder().WithName(compName).WithVolumeMounts(
				v1.RadixVolumeMount{Name: "cache", Path: "/cache", EmptyDir: &v1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("50M")}},
				v1.RadixVolumeMount{Name: "log", Path: "/log", EmptyDir: &v1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("100M")}},
			),
		)

	rd, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builder)
	require.NoError(t, err)
	assert.NotNil(t, rd)

	deployment, err := kubeclient.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).Get(context.Background(), compName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Len(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, 2)
	assert.Len(t, deployment.Spec.Template.Spec.Volumes, 2)
}

func blobFuse2TestVolumeMount(container string) v1.RadixVolumeMount {
	return v1.RadixVolumeMount{Name: "blob", Path: "/mnt/blob", BlobFuse2: &v1.RadixBlobFuse2VolumeMount{Container: container}}
}

func pvcClaimNames(volumes []corev1.Volume) []string {
	var names []string
	for _, v := range volumes {
		if v.PersistentVolumeClaim != nil {
			names = append(names, v.PersistentVolumeClaim.ClaimName)
		}
	}
	return names
}

func Test_BlobFuse2_ClaimIsKeptAcrossSyncsAndChangedOnSpecChange(t *testing.T) {
	appName, envName, compName := "anyapp", "anyenv", "anycomp"
	namespace := utils.GetEnvironmentNamespace(appName, envName)
	tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, _, certClient := SetupTest(t)
	builderFor := func(rdName, container string) utils.DeploymentBuilder {
		return utils.NewDeploymentBuilder().
			WithDeploymentName(rdName).
			WithRadixApplication(utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))).
			WithAppName(appName).
			WithEnvironment(envName).
			WithComponents(utils.NewDeployComponentBuilder().WithName(compName).WithVolumeMounts(blobFuse2TestVolumeMount(container)))
	}
	getClaimNames := func() []string {
		deployment, err := kubeclient.AppsV1().Deployments(namespace).Get(context.Background(), compName, metav1.GetOptions{})
		require.NoError(t, err)
		return pvcClaimNames(deployment.Spec.Template.Spec.Volumes)
	}

	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-1", "container1"))
	require.NoError(t, err)
	initialClaims := getClaimNames()
	require.Len(t, initialClaims, 1)
	_, err = kubeclient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), initialClaims[0], metav1.GetOptions{})
	require.NoError(t, err, "the claim used by the deployment must exist")

	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-2", "container1"))
	require.NoError(t, err)
	assert.Equal(t, initialClaims, getClaimNames(), "an unchanged spec must keep the claim")

	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-3", "container2"))
	require.NoError(t, err)
	changedClaims := getClaimNames()
	require.Len(t, changedClaims, 1)
	assert.NotEqual(t, initialClaims, changedClaims, "a changed spec must use a new claim")
	_, err = kubeclient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), changedClaims[0], metav1.GetOptions{})
	require.NoError(t, err)
}

func Test_BlobFuse2_ExistingMatchingClaimUsedByDeploymentIsKept(t *testing.T) {
	appName, envName, compName := "anyapp", "anyenv", "anycomp"
	namespace := utils.GetEnvironmentNamespace(appName, envName)
	tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, _, certClient := SetupTest(t)
	builderFor := func(rdName string) utils.DeploymentBuilder {
		return utils.NewDeploymentBuilder().
			WithDeploymentName(rdName).
			WithRadixApplication(utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))).
			WithAppName(appName).
			WithEnvironment(envName).
			WithComponents(utils.NewDeployComponentBuilder().WithName(compName).WithVolumeMounts(blobFuse2TestVolumeMount("container1")))
	}
	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-1"))
	require.NoError(t, err)

	// Simulate a matching pair with a name from a previous operator version, used by the running deployment
	deployment, err := kubeclient.AppsV1().Deployments(namespace).Get(context.Background(), compName, metav1.GetOptions{})
	require.NoError(t, err)
	hashPvc, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcClaimNames(deployment.Spec.Template.Spec.Volumes)[0], metav1.GetOptions{})
	require.NoError(t, err)
	hashPv, err := kubeclient.CoreV1().PersistentVolumes().Get(context.Background(), hashPvc.Spec.VolumeName, metav1.GetOptions{})
	require.NoError(t, err)
	legacyPv := hashPv.DeepCopy()
	legacyPv.ObjectMeta = metav1.ObjectMeta{Name: "pv-radixvolumemount-legacy", Labels: hashPv.Labels}
	legacyPv.Spec.ClaimRef.Name = "pvc-legacy"
	legacyPv.Spec.CSI.VolumeHandle = "radixvolumemount#legacy"
	_, err = kubeclient.CoreV1().PersistentVolumes().Create(context.Background(), legacyPv, metav1.CreateOptions{})
	require.NoError(t, err)
	legacyPvc := hashPvc.DeepCopy()
	legacyPvc.ObjectMeta = metav1.ObjectMeta{Name: "pvc-legacy", Namespace: namespace, Labels: hashPvc.Labels}
	legacyPvc.Spec.VolumeName = legacyPv.Name
	_, err = kubeclient.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), legacyPvc, metav1.CreateOptions{})
	require.NoError(t, err)
	for i, v := range deployment.Spec.Template.Spec.Volumes {
		if v.PersistentVolumeClaim != nil {
			deployment.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = legacyPvc.Name
		}
	}
	_, err = kubeclient.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	require.NoError(t, err)

	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-2"))
	require.NoError(t, err)
	deployment, err = kubeclient.AppsV1().Deployments(namespace).Get(context.Background(), compName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{legacyPvc.Name}, pvcClaimNames(deployment.Spec.Template.Spec.Volumes))
}

func Test_BlobFuse2_JobSchedulerAuxDeploymentGetsJobVolumes(t *testing.T) {
	appName, envName, jobName := "anyapp", "anyenv", "anyjob"
	namespace := utils.GetEnvironmentNamespace(appName, envName)
	tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, _, certClient := SetupTest(t)
	builder := utils.NewDeploymentBuilder().
		WithRadixApplication(utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))).
		WithAppName(appName).
		WithEnvironment(envName).
		WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(jobName).WithVolumeMounts(blobFuse2TestVolumeMount("container1")))

	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builder)
	require.NoError(t, err)

	scheduler, err := kubeclient.AppsV1().Deployments(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, scheduler.Spec.Template.Spec.Volumes, "the job scheduler must not have the job volumes")
	assert.Empty(t, scheduler.Spec.Template.Spec.Containers[0].VolumeMounts)

	aux, err := kubeclient.AppsV1().Deployments(namespace).Get(context.Background(), defaults.GetJobAuxKubeDeployName(jobName), metav1.GetOptions{})
	require.NoError(t, err)
	claimNames := pvcClaimNames(aux.Spec.Template.Spec.Volumes)
	require.Len(t, claimNames, 1)
	pvc, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), claimNames[0], metav1.GetOptions{})
	require.NoError(t, err, "the claim used by the aux deployment must exist")
	assert.Equal(t, jobName, pvc.Labels[kube.RadixComponentLabel])
}
