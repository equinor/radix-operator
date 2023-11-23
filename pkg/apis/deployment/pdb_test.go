package deployment

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestHorizontalScaleChangePDB(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest()
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	componentThreeName := "componentThreeName"

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(pointers.Ptr(4)).
				WithHorizontalScaling(numbers.Int32Ptr(2), 4, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 3000).
				WithPublicPort("http").WithHorizontalScaling(nil, 2, nil, nil)))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
	assert.Equal(t, 3, len(expectedDeployments), "Number of deployments wasn't as expected")
	assert.Equal(t, componentOneName, deployments.Items[0].Name, "app deployment not there")

	// Check PDB is added
	pdbs, _ := client.PolicyV1().PodDisruptionBudgets(utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(pdbs.Items))
	assert.Equal(t, componentOneName, pdbs.Items[0].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
	assert.Equal(t, componentOneName, pdbs.Items[0].ObjectMeta.Labels[kube.RadixComponentLabel])
	assert.Equal(t, int32(1), pdbs.Items[0].Spec.MinAvailable.IntVal)

	// Remove components
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)).
				WithSecrets([]string{"a_secret"})))

	assert.NoError(t, err)
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")
		assert.Equal(t, componentTwoName, deployments.Items[0].Name, "app deployment not there")
	})

	// Check PDB is removed
	pdbs, _ = client.PolicyV1().PodDisruptionBudgets(utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(pdbs.Items))
}

func TestObjectSynced_MultiComponentToOneComponent_HandlesPdbChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest()
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	componentThreeName := "componentThreeName"

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(pointers.Ptr(4)),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 3000).
				WithPublicPort("http")))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
	assert.Equal(t, 3, len(expectedDeployments), "Number of deployments wasn't as expected")
	assert.Equal(t, componentOneName, deployments.Items[0].Name, "app deployment not there")

	// Check PDB is added
	pdbs, _ := client.PolicyV1().PodDisruptionBudgets(utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(pdbs.Items))
	assert.Equal(t, componentOneName, pdbs.Items[0].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
	assert.Equal(t, componentOneName, pdbs.Items[0].ObjectMeta.Labels[kube.RadixComponentLabel])
	assert.Equal(t, int32(1), pdbs.Items[0].Spec.MinAvailable.IntVal)

	// Remove components
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)).
				WithSecrets([]string{"a_secret"})))

	assert.NoError(t, err)
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")
		assert.Equal(t, componentTwoName, deployments.Items[0].Name, "app deployment not there")
	})

	// Check PDB is removed
	pdbs, _ = client.PolicyV1().PodDisruptionBudgets(utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(pdbs.Items))
}

func TestObjectSynced_ScalingReplicas_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest()
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)

	// Define one component with >1 replicas and one component with <2 replicas
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(4)),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)),
		))

	assert.NoError(t, err)

	// Check PDB is added
	pdbs, _ := client.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(pdbs.Items))
	assert.Equal(t, componentOneName, pdbs.Items[0].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
	assert.Equal(t, componentOneName, pdbs.Items[0].ObjectMeta.Labels[kube.RadixComponentLabel])
	assert.Equal(t, int32(1), pdbs.Items[0].Spec.MinAvailable.IntVal)

	// Define two components with <2 replicas
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(pointers.Ptr(1)),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)),
		))

	assert.NoError(t, err)

	// Check PDB is removed
	pdbs, _ = client.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(pdbs.Items))

	// Define one component with >1 replicas and one component with <2 replicas
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(pointers.Ptr(10)),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)),
		))

	assert.NoError(t, err)

	// Check PDB is added
	pdbs, _ = client.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(pdbs.Items))
	assert.Equal(t, componentOneName, pdbs.Items[0].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
	assert.Equal(t, componentOneName, pdbs.Items[0].ObjectMeta.Labels[kube.RadixComponentLabel])
	assert.Equal(t, int32(1), pdbs.Items[0].Spec.MinAvailable.IntVal)

	// Delete component with >1 replicas. Expect PDBs to be removed
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)),
		))

	assert.NoError(t, err)

	// Check PDB is removed
	pdbs, _ = client.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(pdbs.Items))

	componentThreeName := "componentThreeName"

	// Set 3 components with >1 replicas. Expect 3 PDBs
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(pointers.Ptr(4)),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(3)),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 3000).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(2))))

	assert.NoError(t, err)

	// Check PDBs are added
	pdbs, _ = client.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.TODO(), metav1.ListOptions{})

	assert.Equal(t, 3, len(pdbs.Items))
	assert.Equal(t, componentOneName, pdbs.Items[0].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
	assert.Equal(t, componentThreeName, pdbs.Items[1].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
	assert.Equal(t, componentTwoName, pdbs.Items[2].Spec.Selector.MatchLabels[kube.RadixComponentLabel])

}

func TestObjectSynced_HorizontalScalingReplicas_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest()
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)

	// Define one component with >1 replicas and one component with <2 replicas
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(pointers.Ptr(4)).
				WithHorizontalScaling(numbers.Int32Ptr(4), 6, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)),
		))

	assert.NoError(t, err)

	// Check PDB is added
	pdbs, _ := client.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(pdbs.Items))
	assert.Equal(t, componentOneName, pdbs.Items[0].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
	assert.Equal(t, componentOneName, pdbs.Items[0].ObjectMeta.Labels[kube.RadixComponentLabel])
	assert.Equal(t, int32(1), pdbs.Items[0].Spec.MinAvailable.IntVal)

	// Define two components with <2 replicas
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(pointers.Ptr(1)).
				WithHorizontalScaling(numbers.Int32Ptr(1), 6, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(pointers.Ptr(0)),
		))

	assert.NoError(t, err)

	// Check PDB is removed
	pdbs, _ = client.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(pdbs.Items))
}

func TestObjectSynced_UpdatePdb_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest()
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)

	// Define a component with >1 replicas
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(pointers.Ptr(10)),
		))

	assert.NoError(t, err)

	existingPdb, _ := client.PolicyV1().PodDisruptionBudgets(envNamespace).Get(context.TODO(), utils.GetPDBName(componentOneName), metav1.GetOptions{})
	generatedPdb := utils.GetPDBConfig(componentOneName, envNamespace)
	generatedPdb.ObjectMeta.Labels[kube.RadixComponentLabel] = "wrong"
	generatedPdb.Spec.Selector.MatchLabels[kube.RadixComponentLabel] = "wrong"

	patchBytes, err := kube.MergePodDisruptionBudgets(existingPdb, generatedPdb)
	assert.NoError(t, err)

	_, err = client.PolicyV1().PodDisruptionBudgets(envNamespace).Patch(context.TODO(), utils.GetPDBName(componentOneName), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	assert.NoError(t, err)

	pdbWithWrongLabels, _ := client.PolicyV1().PodDisruptionBudgets(envNamespace).Get(context.TODO(), utils.GetPDBName(componentOneName), metav1.GetOptions{})
	assert.Equal(t, "wrong", pdbWithWrongLabels.ObjectMeta.Labels[kube.RadixComponentLabel])
	assert.Equal(t, "wrong", pdbWithWrongLabels.Spec.Selector.MatchLabels[kube.RadixComponentLabel])

	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(pointers.Ptr(9)),
		))

	assert.NoError(t, err)

	pdbWithCorrectLabels, _ := client.PolicyV1().PodDisruptionBudgets(envNamespace).Get(context.TODO(), utils.GetPDBName(componentOneName), metav1.GetOptions{})
	assert.Equal(t, componentOneName, pdbWithCorrectLabels.ObjectMeta.Labels[kube.RadixComponentLabel])
	assert.Equal(t, componentOneName, pdbWithCorrectLabels.Spec.Selector.MatchLabels[kube.RadixComponentLabel])
}
