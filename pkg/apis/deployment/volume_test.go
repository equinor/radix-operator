package deployment

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/volumemount"
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
	pvcs, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, pvcs.Items, 1, "the claim of the previous spec must be deleted")
	assert.Equal(t, changedClaims[0], pvcs.Items[0].Name)
	pvs, err := kubeclient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, pvs.Items, 1, "the volume of the previous spec must be deleted")
	assert.Equal(t, pvcs.Items[0].Spec.VolumeName, pvs.Items[0].Name)
}

func Test_BlobFuse2_CurrentMatchingClaimIsReused(t *testing.T) {
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

	// Replace the pair with a matching pair named like a previous operator version did
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
	require.NoError(t, kubeclient.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), hashPvc.Name, metav1.DeleteOptions{}))
	require.NoError(t, kubeclient.CoreV1().PersistentVolumes().Delete(context.Background(), hashPv.Name, metav1.DeleteOptions{}))

	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-2"))
	require.NoError(t, err)
	deployment, err = kubeclient.AppsV1().Deployments(namespace).Get(context.Background(), compName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{legacyPvc.Name}, pvcClaimNames(deployment.Spec.Template.Spec.Volumes))
	pvcs, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, pvcs.Items, 1, "no new claim must be created")
}

func Test_BlobFuse2_JobVolumesAreReconciledForJobAuxDeployment(t *testing.T) {
	appName, envName, jobName := "anyapp", "anyenv", "anyjob"
	namespace := utils.GetEnvironmentNamespace(appName, envName)
	tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, _, certClient := SetupTest(t)
	builderFor := func(rdName, container string) utils.DeploymentBuilder {
		return utils.NewDeploymentBuilder().
			WithDeploymentName(rdName).
			WithRadixApplication(utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))).
			WithAppName(appName).
			WithEnvironment(envName).
			WithJobComponents(utils.NewDeployJobComponentBuilder().WithName(jobName).WithVolumeMounts(blobFuse2TestVolumeMount(container)))
	}
	getAuxClaimName := func() string {
		scheduler, err := kubeclient.AppsV1().Deployments(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Empty(t, scheduler.Spec.Template.Spec.Volumes, "the job scheduler must not have the job volumes")
		assert.Empty(t, scheduler.Spec.Template.Spec.Containers[0].VolumeMounts)
		aux, err := kubeclient.AppsV1().Deployments(namespace).Get(context.Background(), defaults.GetJobAuxKubeDeployName(jobName), metav1.GetOptions{})
		require.NoError(t, err)
		claimNames := pvcClaimNames(aux.Spec.Template.Spec.Volumes)
		require.Len(t, claimNames, 1)
		assert.Equal(t, []corev1.VolumeMount{{Name: aux.Spec.Template.Spec.Volumes[0].Name, MountPath: "/mnt/blob"}}, aux.Spec.Template.Spec.Containers[0].VolumeMounts, "the aux container must mount the job volumes, as a canary for the jobs")
		return claimNames[0]
	}
	listPvcNames := func() []string {
		pvcs, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		var names []string
		for _, pvc := range pvcs.Items {
			names = append(names, pvc.Name)
		}
		return names
	}

	rd1, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-1", "container1"))
	require.NoError(t, err)
	claim1 := getAuxClaimName()
	pvc, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), claim1, metav1.GetOptions{})
	require.NoError(t, err, "the claim used by the aux deployment must exist")
	assert.Equal(t, jobName, pvc.Labels[kube.RadixComponentLabel])

	// The batch syncer ensures volumes for the job in the RadixDeployment, and must get the same claim
	batchVolumes, _, err := volumemount.EnsureComponentVolumes(context.Background(), kubeUtil, appName, rd1, rd1.GetJobComponentByName(jobName))
	require.NoError(t, err)
	assert.Equal(t, []string{claim1}, pvcClaimNames(batchVolumes), "the RadixDeployment and RadixBatch syncers must use the same claim")
	assert.Equal(t, []string{claim1}, listPvcNames())

	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-2", "container2"))
	require.NoError(t, err)
	claim2 := getAuxClaimName()
	assert.NotEqual(t, claim1, claim2, "a changed job spec must use a new claim")
	assert.Equal(t, []string{claim2}, listPvcNames(), "the claim of the previous job spec must be deleted")
}

func Test_BlobFuse2_StorageOfRemovedComponentIsDeleted(t *testing.T) {
	appName, envName := "anyapp", "anyenv"
	namespace := utils.GetEnvironmentNamespace(appName, envName)
	tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, _, certClient := SetupTest(t)
	builderFor := func(rdName string, compNames ...string) utils.DeploymentBuilder {
		var components []utils.DeployComponentBuilder
		for _, compName := range compNames {
			components = append(components, utils.NewDeployComponentBuilder().WithName(compName).WithVolumeMounts(blobFuse2TestVolumeMount("container1")))
		}
		return utils.NewDeploymentBuilder().
			WithDeploymentName(rdName).
			WithRadixApplication(utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))).
			WithAppName(appName).
			WithEnvironment(envName).
			WithComponents(components...)
	}
	listPvcAndPvComponents := func() ([]string, []string) {
		pvcs, err := kubeclient.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		pvs, err := kubeclient.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		var pvcComponents, pvComponents []string
		for _, pvc := range pvcs.Items {
			pvcComponents = append(pvcComponents, pvc.Labels[kube.RadixComponentLabel])
		}
		for _, pv := range pvs.Items {
			pvComponents = append(pvComponents, pv.Labels[kube.RadixComponentLabel])
		}
		return pvcComponents, pvComponents
	}

	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-1", "comp1", "comp2"))
	require.NoError(t, err)
	pvcComponents, pvComponents := listPvcAndPvComponents()
	assert.ElementsMatch(t, []string{"comp1", "comp2"}, pvcComponents)
	assert.ElementsMatch(t, []string{"comp1", "comp2"}, pvComponents)

	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, dynamicClient, certClient, builderFor("rd-2", "comp1"))
	require.NoError(t, err)
	pvcComponents, pvComponents = listPvcAndPvComponents()
	assert.Equal(t, []string{"comp1"}, pvcComponents)
	assert.Equal(t, []string{"comp1"}, pvComponents)
}
