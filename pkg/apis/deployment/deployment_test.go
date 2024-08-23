// file deepcode ignore HardcodedPassword/test: unit tests
package deployment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	radixutils "github.com/equinor/radix-common/utils"
	radixmaps "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config"
	certificateconfig "github.com/equinor/radix-operator/pkg/apis/config/certificate"
	"github.com/equinor/radix-operator/pkg/apis/config/deployment"
	"github.com/equinor/radix-operator/pkg/apis/config/registry"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	testClusterName = "AnyClusterName"
	testEgressIps   = "0.0.0.0"
)

var testConfig = config.Config{
	DeploymentSyncer: deployment.SyncerConfig{
		TenantID:               "123456789",
		KubernetesAPIPort:      543,
		DeploymentHistoryLimit: 10,
	},
	CertificateAutomation: certificateconfig.AutomationConfig{
		ClusterIssuer: "test-cert-issuer",
		Duration:      10000 * time.Hour,
		RenewBefore:   5000 * time.Hour,
	},
}

func SetupTest(t *testing.T) (*test.Utils, *kubefake.Clientset, *kube.Kube, *radixfake.Clientset, *kedafake.Clientset, *prometheusfake.Clientset, *secretproviderfake.Clientset, *certfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	prometheusClient := prometheusfake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	certClient := certfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(
		kubeclient,
		radixClient,
		kedaClient,
		secretProviderClient,
	)
	handlerTestUtils := test.NewTestUtils(kubeclient, radixClient, kedaClient, secretProviderClient)
	err := handlerTestUtils.CreateClusterPrerequisites(testClusterName, testEgressIps, "anysubid")
	require.NoError(t, err)
	return &handlerTestUtils, kubeclient, kubeUtil, radixClient, kedaClient, prometheusClient, secretProviderClient, certClient
}

func TeardownTest() {
	// Cleanup setup
	_ = os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	_ = os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	_ = os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	_ = os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	_ = os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	_ = os.Unsetenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable)
	_ = os.Unsetenv(defaults.OperatorClusterTypeEnvironmentVariable)
	_ = os.Unsetenv(defaults.OperatorTenantIdEnvironmentVariable)
}

func TestObjectSynced_MultiComponent_ContainsAllElements(t *testing.T) {
	defer TeardownTest()
	commitId := string(uuid.NewUUID())
	const componentNameApp = "app"
	adminGroups, readerGroups := []string{"adm1", "adm2"}, []string{"rdr1", "rdr2"}

	for _, componentsExist := range []bool{true, false} {
		testScenario := utils.TernaryString(componentsExist, "Updating deployment", "Creating deployment")

		tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		defer TeardownTest()
		_ = os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")

		t.Run("Test Suite", func(t *testing.T) {
			aRadixRegistrationBuilder := utils.ARadixRegistration().WithAdGroups(adminGroups).WithReaderAdGroups(readerGroups)
			aRadixApplicationBuilder := utils.ARadixApplication().
				WithRadixRegistration(aRadixRegistrationBuilder)
			environment := "test"
			appName := "edcradix"
			componentNameRedis := "redis"
			componentNameRadixQuote := "radixquote"
			outdatedSecret := "outdatedSecret"
			remainingSecret := "remainingSecret"
			addingSecret := "addingSecret"
			blobVolumeName := "blob_volume_1"
			blobCsiAzureVolumeName := "blobCsiAzure_volume_1"

			if componentsExist {
				// Update component
				existingRadixDeploymentBuilder := utils.ARadixDeployment().
					WithRadixApplication(aRadixApplicationBuilder).
					WithAppName(appName).
					WithImageTag("old_axmz8").
					WithEnvironment(environment).
					WithJobComponents().
					WithLabel(kube.RadixCommitLabel, commitId).
					WithComponents(
						utils.NewDeployComponentBuilder().
							WithImage("old_radixdev.azurecr.io/radix-loadbalancer-html-app:1igdh").
							WithName(componentNameApp).
							WithPort("http", 8081).
							WithPublicPort("http").
							WithDNSAppAlias(true).
							WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commitId).
							WithResource(map[string]string{
								"memory": "65Mi",
								"cpu":    "251m",
							}, map[string]string{
								"memory": "129Mi",
								"cpu":    "501m",
							}).
							WithReplicas(test.IntPtr(2)),
						utils.NewDeployComponentBuilder().
							WithImage("old_radixdev.azurecr.io/radix-loadbalancer-html-redis:1igdh").
							WithName(componentNameRedis).
							WithEnvironmentVariable("a_variable", "3002").
							WithPort("http", 6378).
							WithPublicPort("").
							WithReplicas(test.IntPtr(1)),
						utils.NewDeployComponentBuilder().
							WithImage("old_radixdev.azurecr.io/edcradix-radixquote:axmz8").
							WithName(componentNameRadixQuote).
							WithPort("http", 3001).
							WithPublicPort("http").
							WithSecrets([]string{remainingSecret, addingSecret}))
				_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, existingRadixDeploymentBuilder)
				assert.NoError(t, err)

			} else {
				aRadixDeploymentBuilder := utils.ARadixDeployment().
					WithRadixApplication(aRadixApplicationBuilder).
					WithAppName(appName).
					WithImageTag("axmz8").
					WithEnvironment(environment).
					WithJobComponents().
					WithLabel(kube.RadixCommitLabel, commitId).
					WithComponents(
						utils.NewDeployComponentBuilder().
							WithImage("radixdev.azurecr.io/radix-loadbalancer-html-app:1igdh").
							WithName(componentNameApp).
							WithPort("http", 8080).
							WithPublicPort("http").
							WithDNSAppAlias(true).
							WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commitId).
							WithResource(map[string]string{
								"memory": "64Mi",
								"cpu":    "250m",
							}, map[string]string{
								"memory": "128Mi",
								"cpu":    "500m",
							}).
							WithReplicas(test.IntPtr(4)),
						utils.NewDeployComponentBuilder().
							WithImage("radixdev.azurecr.io/radix-loadbalancer-html-redis:1igdh").
							WithName(componentNameRedis).
							WithEnvironmentVariable("a_variable", "3001").
							WithPort("http", 6379).
							WithPublicPort("").
							WithReplicas(test.IntPtr(0)),
						utils.NewDeployComponentBuilder().
							WithImage("radixdev.azurecr.io/edcradix-radixquote:axmz8").
							WithName(componentNameRadixQuote).
							WithPort("http", 3000).
							WithPublicPort("http").
							WithVolumeMounts(
								radixv1.RadixVolumeMount{
									Type:      radixv1.MountTypeBlob,
									Name:      blobVolumeName,
									Container: "some-container",
									Path:      "some-path",
								},
								radixv1.RadixVolumeMount{
									Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
									Name:    blobCsiAzureVolumeName,
									Storage: "some-storage",
									Path:    "some-path2",
									GID:     "1000",
								},
							).
							WithSecrets([]string{outdatedSecret, remainingSecret}))

				// Test
				_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, aRadixDeploymentBuilder)
				assert.NoError(t, err)
			}

			envNamespace := utils.GetEnvironmentNamespace(appName, environment)

			t.Run(fmt.Sprintf("%s: validate deploy", testScenario), func(t *testing.T) {
				t.Parallel()
				allDeployments, _ := kubeclient.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
				deployments := getDeploymentsForRadixComponents(allDeployments.Items)
				assert.Equal(t, 3, len(deployments), "Number of deployments wasn't as expected")
				assert.Equal(t, componentNameApp, getDeploymentByName(componentNameApp, deployments).Name, "app deployment not there")

				if !componentsExist {
					assert.Equal(t, int32(4), *getDeploymentByName(componentNameApp, deployments).Spec.Replicas, "number of replicas was unexpected")

				} else {
					assert.Equal(t, int32(2), *getDeploymentByName(componentNameApp, deployments).Spec.Replicas, "number of replicas was unexpected")

				}

				pdbs, _ := kubeclient.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Equal(t, 1, len(pdbs.Items))
				assert.Equal(t, "app", pdbs.Items[0].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
				assert.Equal(t, int32(1), pdbs.Items[0].Spec.MinAvailable.IntVal)

				assert.Equal(t, 13, len(getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Env), "number of environment variables was unexpected for component. It should contain default and custom")
				assert.Equal(t, os.Getenv(defaults.ContainerRegistryEnvironmentVariable), getEnvVariableByNameOnDeployment(kubeclient, defaults.ContainerRegistryEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable), getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixDNSZoneEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, "AnyClusterName", getEnvVariableByNameOnDeployment(kubeclient, defaults.ClusternameEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, environment, getEnvVariableByNameOnDeployment(kubeclient, defaults.EnvironmentnameEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, "app-edcradix-test.AnyClusterName.dev.radix.equinor.com", getEnvVariableByNameOnDeployment(kubeclient, defaults.PublicEndpointEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, "app-edcradix-test.AnyClusterName.dev.radix.equinor.com", getEnvVariableByNameOnDeployment(kubeclient, defaults.CanonicalEndpointEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, appName, getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixAppEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, componentNameApp, getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixComponentEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, testEgressIps, getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixActiveClusterEgressIpsEnvironmentVariable, componentNameApp, deployments))

				if !componentsExist {
					assert.Equal(t, "(8080)", getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixPortsEnvironmentVariable, componentNameApp, deployments))
				} else {
					assert.Equal(t, "(8081)", getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixPortsEnvironmentVariable, componentNameApp, deployments))
				}

				assert.Equal(t, "(http)", getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixPortNamesEnvironmentVariable, componentNameApp, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixCommitHashEnvironmentVariable, componentNameApp, deployments))

				if !componentsExist {
					assert.Equal(t, parseQuantity("128Mi"), getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Resources.Limits["memory"])
					assert.Equal(t, parseQuantity("500m"), getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Resources.Limits["cpu"])
					assert.Equal(t, parseQuantity("64Mi"), getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Resources.Requests["memory"])
					assert.Equal(t, parseQuantity("250m"), getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Resources.Requests["cpu"])
				} else {
					assert.Equal(t, parseQuantity("129Mi"), getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Resources.Limits["memory"])
					assert.Equal(t, parseQuantity("501m"), getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Resources.Limits["cpu"])
					assert.Equal(t, parseQuantity("65Mi"), getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Resources.Requests["memory"])
					assert.Equal(t, parseQuantity("251m"), getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Resources.Requests["cpu"])
				}

				assert.Equal(t, componentNameRedis, getDeploymentByName(componentNameRedis, deployments).Name, "redis deployment not there")

				if !componentsExist {
					assert.Equal(t, int32(0), *getDeploymentByName(componentNameRedis, deployments).Spec.Replicas, "number of replicas was unexpected")
				} else {
					assert.Equal(t, int32(1), *getDeploymentByName(componentNameRedis, deployments).Spec.Replicas, "number of replicas was unexpected")
				}

				assert.Equal(t, 11, len(getContainerByName(componentNameRedis, getDeploymentByName(componentNameRedis, deployments).Spec.Template.Spec.Containers).Env), "number of environment variables was unexpected for component. It should contain default and custom")
				assert.True(t, envVariableByNameExistOnDeployment("a_variable", componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.ContainerRegistryEnvironmentVariable, componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixDNSZoneEnvironmentVariable, componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.ClusternameEnvironmentVariable, componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.EnvironmentnameEnvironmentVariable, componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixClusterTypeEnvironmentVariable, componentNameRedis, deployments))

				if !componentsExist {
					assert.Equal(t, "3001", getEnvVariableByNameOnDeployment(kubeclient, "a_variable", componentNameRedis, deployments))
				} else {
					assert.Equal(t, "3002", getEnvVariableByNameOnDeployment(kubeclient, "a_variable", componentNameRedis, deployments))
				}

				assert.True(t, deploymentByNameExists(componentNameRadixQuote, deployments), "radixquote deployment not there")
				spec := getDeploymentByName(componentNameRadixQuote, deployments).Spec
				assert.Equal(t, int32(DefaultReplicas), *spec.Replicas, "number of replicas was unexpected")
				assert.True(t, envVariableByNameExistOnDeployment(defaults.ContainerRegistryEnvironmentVariable, componentNameRadixQuote, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixDNSZoneEnvironmentVariable, componentNameRadixQuote, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.ClusternameEnvironmentVariable, componentNameRadixQuote, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.EnvironmentnameEnvironmentVariable, componentNameRadixQuote, deployments))

				if !componentsExist {
					assert.True(t, envVariableByNameExistOnDeployment(outdatedSecret, componentNameRadixQuote, deployments))
				} else {
					assert.False(t, envVariableByNameExistOnDeployment(outdatedSecret, componentNameRadixQuote, deployments))
				}

				assert.True(t, envVariableByNameExistOnDeployment(remainingSecret, componentNameRadixQuote, deployments))

				if !componentsExist {
					assert.False(t, envVariableByNameExistOnDeployment(addingSecret, componentNameRadixQuote, deployments))
				} else {
					assert.True(t, envVariableByNameExistOnDeployment(addingSecret, componentNameRadixQuote, deployments))
				}

				volumesExist := len(spec.Template.Spec.Volumes) > 1
				volumeMountsExist := len(spec.Template.Spec.Containers[0].VolumeMounts) > 1
				if !componentsExist {
					assert.True(t, volumesExist, "expected existing volumes")
					assert.True(t, volumeMountsExist, "expected existing volume mounts")
				} else {
					assert.False(t, volumesExist, "unexpected existing volumes")
					assert.False(t, volumeMountsExist, "unexpected existing volume mounts")
				}

				for _, componentName := range []string{componentNameApp, componentNameRedis, componentNameRadixQuote} {
					deploy := getDeploymentByName(componentName, deployments)
					assert.Equal(t, componentName, deploy.Spec.Template.Labels[kube.RadixComponentLabel], "invalid/missing value for label component-name")
				}

				expectedStrategy := appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
						MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
					},
				}

				for _, deployment := range deployments {
					assert.Equal(t, expectedStrategy, deployment.Spec.Strategy)

					expectedAffinity := &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorArchitecture}},
						}}}}},
						PodAntiAffinity: &corev1.PodAntiAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 1, PodAffinityTerm: corev1.PodAffinityTerm{TopologyKey: corev1.LabelHostname, LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: kube.RadixAppLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{appName}},
							{Key: kube.RadixComponentLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{deployment.Name}},
						}}}}}},
					}
					assert.Equal(t, expectedAffinity, deployment.Spec.Template.Spec.Affinity)
				}
			})

			t.Run(fmt.Sprintf("%s: validate hpa", testScenario), func(t *testing.T) {
				t.Parallel()
				hpas, _ := kubeclient.AutoscalingV2().HorizontalPodAutoscalers(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Equal(t, 0, len(hpas.Items), "Number of horizontal pod autoscaler wasn't as expected")
			})

			t.Run(fmt.Sprintf("%s: validate service", testScenario), func(t *testing.T) {
				t.Parallel()
				services, _ := kubeclient.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
				expectedServices := getServicesForRadixComponents(&services.Items)
				assert.Equal(t, 3, len(expectedServices), "Number of services wasn't as expected")

				for _, component := range []string{componentNameApp, componentNameRedis, componentNameRadixQuote} {
					svc := getServiceByName(component, services)
					assert.NotNil(t, svc, "component service does not exist")
					assert.Equal(t, map[string]string{kube.RadixComponentLabel: component}, svc.Spec.Selector)
				}
			})

			t.Run(fmt.Sprintf("%s: validate secrets", testScenario), func(t *testing.T) {
				t.Parallel()
				secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})

				if !componentsExist {
					assert.Equal(t, 3, len(secrets.Items), "Number of secrets was not according to spec")
				} else {
					assert.Equal(t, 1, len(secrets.Items), "Number of secrets was not according to spec")
				}

				componentSecretName := utils.GetComponentSecretName(componentNameRadixQuote)
				assert.True(t, secretByNameExists(componentSecretName, secrets), "Component secret is not as expected")
				blobFuseSecretExists := secretByNameExists(defaults.GetBlobFuseCredsSecretName(componentNameRadixQuote, blobVolumeName), secrets)
				blobCsiAzureFuseSecretExists := secretByNameExists(defaults.GetCsiAzureVolumeMountCredsSecretName(componentNameRadixQuote, blobCsiAzureVolumeName), secrets)
				if !componentsExist {
					assert.True(t, blobFuseSecretExists, "expected Blobfuse volume mount secret")
					assert.True(t, blobCsiAzureFuseSecretExists, "expected blob CSI Azure volume mount secret")
				} else {
					assert.False(t, blobFuseSecretExists, "unexpected volume mount secrets")
				}
			})

			t.Run(fmt.Sprintf("%s: validate service accounts", testScenario), func(t *testing.T) {
				t.Parallel()
				serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
			})

			t.Run(fmt.Sprintf("%s: validate roles", testScenario), func(t *testing.T) {
				t.Parallel()
				roles, _ := kubeclient.RbacV1().Roles(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Subset(t, getRoleNames(roles), []string{"radix-app-adm-radixquote", "radix-app-reader-radixquote"})
			})

			t.Run(fmt.Sprintf("%s validate rolebindings", testScenario), func(t *testing.T) {
				t.Parallel()
				rolebindings, _ := kubeclient.RbacV1().RoleBindings(envNamespace).List(context.Background(), metav1.ListOptions{})

				require.Subset(t, getRoleBindingNames(rolebindings), []string{"radix-app-adm-radixquote", "radix-app-reader-radixquote"})
				assert.ElementsMatch(t, adminGroups, slice.Map(getRoleBindingByName("radix-app-adm-radixquote", rolebindings).Subjects, func(s rbacv1.Subject) string { return s.Name }))
				assert.ElementsMatch(t, readerGroups, slice.Map(getRoleBindingByName("radix-app-reader-radixquote", rolebindings).Subjects, func(s rbacv1.Subject) string { return s.Name }))
			})

			t.Run(fmt.Sprintf("%s: validate networkpolicy", testScenario), func(t *testing.T) {
				t.Parallel()
				np, _ := kubeclient.NetworkingV1().NetworkPolicies(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Equal(t, 4, len(np.Items), "Number of networkpolicy was not expected")
			})
		})
	}
}

func TestObjectSynced_Components_AffinityAccordingToSpec(t *testing.T) {
	var (
		appName             = "anyapp"
		environment         = "anyenv"
		comp1, comp2, comp3 = "comp1", "comp2", "comp3"
	)
	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()

	rrBuilder := utils.ARadixRegistration()
	raBuilder := utils.ARadixApplication().WithRadixRegistration(rrBuilder)
	rdBuilder := utils.ARadixDeployment().
		WithRadixApplication(raBuilder).
		WithAppName(appName).
		WithEnvironment(environment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(comp1).
				WithRuntime(nil),
			utils.NewDeployComponentBuilder().
				WithName(comp2).
				WithRuntime(&radixv1.Runtime{Architecture: ""}),
			utils.NewDeployComponentBuilder().
				WithName(comp3).
				WithRuntime(&radixv1.Runtime{Architecture: "customarch"}))

	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
	require.NoError(t, err)

	deployments, err := kubeclient.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, environment)).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	// Check affinity for comp1
	deployment := getDeploymentByName(comp1, deployments.Items)
	require.NotNil(t, deployment)
	expectedAffinity := &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorArchitecture}},
	}}}}}
	assert.Equal(t, expectedAffinity, deployment.Spec.Template.Spec.Affinity.NodeAffinity)

	// Check affinity for comp2
	deployment = getDeploymentByName(comp2, deployments.Items)
	require.NotNil(t, deployment)
	expectedAffinity = &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorArchitecture}},
	}}}}}
	assert.Equal(t, expectedAffinity, deployment.Spec.Template.Spec.Affinity.NodeAffinity)

	// Check affinity for comp3
	deployment = getDeploymentByName(comp3, deployments.Items)
	require.NotNil(t, deployment)
	expectedAffinity = &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"customarch"}},
	}}}}}
	assert.Equal(t, expectedAffinity, deployment.Spec.Template.Spec.Affinity.NodeAffinity)
}

func TestObjectSynced_MultiJob_ContainsAllElements(t *testing.T) {
	const jobSchedulerImage = "radix-job-scheduler:latest"
	defer TeardownTest()
	commitId := string(uuid.NewUUID())
	adminGroups, readerGroups := []string{"adm1", "adm2"}, []string{"rdr1", "rdr2"}

	for _, jobsExist := range []bool{false, true} {
		testScenario := utils.TernaryString(jobsExist, "Updating deployment", "Creating deployment")

		tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")
		os.Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, jobSchedulerImage)

		t.Run("Test Suite", func(t *testing.T) {
			aRadixRegistrationBuilder := utils.ARadixRegistration().WithAdGroups(adminGroups).WithReaderAdGroups(readerGroups)
			aRadixApplicationBuilder := utils.ARadixApplication().
				WithRadixRegistration(aRadixRegistrationBuilder)
			environment := "test"
			appName := "edcradix"
			jobName := "job"
			jobName2 := "job2"
			schedulerPortCreate := int32(8000)
			schedulerPortUpdate := int32(9000)
			outdatedSecret := "outdatedSecret"
			remainingSecret := "remainingSecret"
			addingSecret := "addingSecret"
			blobVolumeName := "blob_volume_1"
			blobCsiAzureVolumeName := "blobCsiAzure_volume_1"
			payloadPath := "payloadpath"
			if jobsExist {
				// Update component
				existingRadixDeploymentBuilder := utils.ARadixDeployment().
					WithDeploymentName("deploy-update").
					WithRadixApplication(aRadixApplicationBuilder).
					WithAppName(appName).
					WithImageTag("old_axmz8").
					WithEnvironment(environment).
					WithJobComponents(
						utils.NewDeployJobComponentBuilder().
							WithName(jobName).
							WithImage("job:latest").
							WithPorts([]radixv1.ComponentPort{{Name: "http", Port: 3002}}).
							WithEnvironmentVariable("a_variable", "a_value").
							WithMonitoring(true).
							WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commitId).
							WithResource(map[string]string{
								"memory": "65Mi",
								"cpu":    "251m",
							}, map[string]string{
								"memory": "129Mi",
								"cpu":    "501m",
							}).
							WithSchedulerPort(&schedulerPortUpdate).
							WithPayloadPath(&payloadPath).
							WithSecrets([]string{remainingSecret, addingSecret}).
							WithAlwaysPullImageOnDeploy(false).
							WithRuntime(&radixv1.Runtime{Architecture: "customarch"}),
					).
					WithComponents()
				_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, existingRadixDeploymentBuilder)
				assert.NoError(t, err)

			} else {
				aRadixDeploymentBuilder := utils.ARadixDeployment().
					WithDeploymentName("deploy-create").
					WithRadixApplication(aRadixApplicationBuilder).
					WithAppName(appName).
					WithImageTag("axmz8").
					WithEnvironment(environment).
					WithJobComponents(
						utils.NewDeployJobComponentBuilder().
							WithName(jobName).
							WithImage("job:latest").
							WithPorts([]radixv1.ComponentPort{{Name: "http", Port: 3002}}).
							WithEnvironmentVariable("a_variable", "a_value").
							WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commitId).
							WithMonitoring(true).
							WithResource(map[string]string{
								"memory": "65Mi",
								"cpu":    "251m",
							}, map[string]string{
								"memory": "129Mi",
								"cpu":    "501m",
							}).
							WithVolumeMounts(
								radixv1.RadixVolumeMount{
									Type:      radixv1.MountTypeBlob,
									Name:      blobVolumeName,
									Container: "some-container",
									Path:      "some-path",
								},
								radixv1.RadixVolumeMount{
									Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
									Name:    blobCsiAzureVolumeName,
									Storage: "some-storage",
									Path:    "some-path",
								}).
							WithSchedulerPort(&schedulerPortCreate).
							WithPayloadPath(&payloadPath).
							WithSecrets([]string{outdatedSecret, remainingSecret}).
							WithAlwaysPullImageOnDeploy(false).
							WithRuntime(&radixv1.Runtime{Architecture: "customarch"}),
						utils.NewDeployJobComponentBuilder().
							WithName(jobName2).WithSchedulerPort(&schedulerPortCreate).WithRuntime(&radixv1.Runtime{Architecture: "customarch"}),
					).
					WithComponents()

				// Test
				_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, aRadixDeploymentBuilder)
				assert.NoError(t, err)
			}

			envNamespace := utils.GetEnvironmentNamespace(appName, environment)

			t.Run(fmt.Sprintf("%s: validate deploy", testScenario), func(t *testing.T) {
				allDeployments, _ := kubeclient.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
				deployments := getDeploymentsForRadixComponents(allDeployments.Items)
				jobAuxDeployments := getDeploymentsForRadixJobAux(allDeployments.Items)
				if jobsExist {
					assert.Equal(t, 1, len(deployments), "Number of deployments wasn't as expected")
					assert.Equal(t, 1, len(jobAuxDeployments), "Unexpected job aux deployments component amount")

				} else {
					assert.Equal(t, 2, len(deployments), "Number of deployments wasn't as expected")
					assert.Equal(t, 2, len(jobAuxDeployments), "Unexpected job aux deployments component amount")
				}

				expectedAuxAffinity := &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
						{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(radixv1.RuntimeArchitectureAmd64)}},
					}}}}},
				}
				for _, deployment := range jobAuxDeployments {
					assert.Equal(t, expectedAuxAffinity, deployment.Spec.Template.Spec.Affinity, "job aux component must not use job's runtime config")
				}

				assert.Equal(t, jobName, getDeploymentByName(jobName, deployments).Name, "app deployment not there")
				assert.Equal(t, int32(1), *getDeploymentByName(jobName, deployments).Spec.Replicas, "number of replicas was unexpected")

				envVars := getContainerByName(jobName, getDeploymentByName(jobName, deployments).Spec.Template.Spec.Containers).Env
				assert.Equal(t, 14, len(envVars), "number of environment variables was unexpected for component. It should contain default and custom")
				assert.Equal(t, "a_value", getEnvVariableByNameOnDeployment(kubeclient, "a_variable", jobName, deployments))
				assert.Equal(t, os.Getenv(defaults.ContainerRegistryEnvironmentVariable), getEnvVariableByNameOnDeployment(kubeclient, defaults.ContainerRegistryEnvironmentVariable, jobName, deployments))
				assert.Equal(t, os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable), getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixDNSZoneEnvironmentVariable, jobName, deployments))
				assert.Equal(t, "AnyClusterName", getEnvVariableByNameOnDeployment(kubeclient, defaults.ClusternameEnvironmentVariable, jobName, deployments))
				assert.Equal(t, environment, getEnvVariableByNameOnDeployment(kubeclient, defaults.EnvironmentnameEnvironmentVariable, jobName, deployments))
				assert.Equal(t, appName, getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixAppEnvironmentVariable, jobName, deployments))
				assert.Equal(t, jobName, getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixComponentEnvironmentVariable, jobName, deployments))
				assert.Equal(t, "300M", getEnvVariableByNameOnDeployment(kubeclient, defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, jobName, deployments))
				assert.Equal(t, "("+defaults.RadixJobSchedulerPortName+")", getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixPortNamesEnvironmentVariable, jobName, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixCommitHashEnvironmentVariable, jobName, deployments))
				assert.Equal(t, testEgressIps, getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixActiveClusterEgressIpsEnvironmentVariable, jobName, deployments))

				if jobsExist {
					assert.Equal(t, "("+fmt.Sprint(schedulerPortUpdate)+")", getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixPortsEnvironmentVariable, jobName, deployments))
				} else {
					assert.Equal(t, "("+fmt.Sprint(schedulerPortCreate)+")", getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixPortsEnvironmentVariable, jobName, deployments))
				}

				if jobsExist {
					assert.Equal(t, "deploy-update", getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixDeploymentEnvironmentVariable, jobName, deployments))
				} else {
					assert.Equal(t, "deploy-create", getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixDeploymentEnvironmentVariable, jobName, deployments))
				}

				var jobNames []string
				if jobsExist {
					jobNames = []string{jobName}
				} else {
					jobNames = []string{jobName, jobName2}
				}

				for _, job := range jobNames {
					deploy := getDeploymentByName(job, deployments)
					assert.Equal(t, job, deploy.Spec.Template.Labels[kube.RadixComponentLabel], "invalid/missing value for label component-name")
					assert.Equal(t, "true", deploy.Spec.Template.Labels[kube.RadixPodIsJobSchedulerLabel], "invalid/missing value for label is-job-scheduler-pod")

					expectedAffinity := &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(radixv1.RuntimeArchitectureAmd64)}},
						}}}}},
						PodAntiAffinity: &corev1.PodAntiAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 1, PodAffinityTerm: corev1.PodAffinityTerm{TopologyKey: corev1.LabelHostname, LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: kube.RadixAppLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{appName}},
							{Key: kube.RadixComponentLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{job}},
						}}}}}},
					}
					assert.Equal(t, expectedAffinity, deploy.Spec.Template.Spec.Affinity, "job api server must not use job's runtime config")
				}

			})

			t.Run(fmt.Sprintf("%s: validate hpa", testScenario), func(t *testing.T) {
				hpas, _ := kubeclient.AutoscalingV2().HorizontalPodAutoscalers(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Equal(t, 0, len(hpas.Items), "Number of horizontal pod autoscaler wasn't as expected")
			})

			t.Run(fmt.Sprintf("%s: validate service", testScenario), func(t *testing.T) {
				services, _ := kubeclient.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
				expectedServices := getServicesForRadixComponents(&services.Items)
				var jobNames []string
				if jobsExist {
					jobNames = []string{jobName}
					assert.Equal(t, 1, len(expectedServices), "Number of services wasn't as expected")
				} else {
					jobNames = []string{jobName, jobName2}
					assert.Equal(t, 2, len(expectedServices), "Number of services wasn't as expected")
				}

				for _, job := range jobNames {
					svc := getServiceByName(job, services)
					assert.NotNil(t, svc, "job service does not exist")
					assert.Equal(t, map[string]string{kube.RadixComponentLabel: job, kube.RadixPodIsJobSchedulerLabel: "true"}, svc.Spec.Selector)
				}
			})

			t.Run(fmt.Sprintf("%s: validate secrets", testScenario), func(t *testing.T) {
				secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})

				if !jobsExist {
					assert.Equal(t, 3, len(secrets.Items), "Number of secrets was not according to spec")
				} else {
					assert.Equal(t, 1, len(secrets.Items), "Number of secrets was not according to spec")
				}

				jobSecretName := utils.GetComponentSecretName(jobName)
				assert.True(t, secretByNameExists(jobSecretName, secrets), "Job secret is not as expected")

				blobFuseSecretExists := secretByNameExists(defaults.GetBlobFuseCredsSecretName(jobName, blobVolumeName), secrets)
				blobCsiAzureFuseSecretExists := secretByNameExists(defaults.GetCsiAzureVolumeMountCredsSecretName(jobName, blobCsiAzureVolumeName), secrets)
				if !jobsExist {
					assert.True(t, blobFuseSecretExists, "expected Blobfuse volume mount secret")
					assert.True(t, blobCsiAzureFuseSecretExists, "expected blob CSI Azure volume mount secret")
				} else {
					assert.False(t, blobFuseSecretExists, "unexpected volume mount secrets")
				}
			})

			t.Run(fmt.Sprintf("%s: validate service accounts", testScenario), func(t *testing.T) {
				serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
			})

			t.Run(fmt.Sprintf("%s: validate roles", testScenario), func(t *testing.T) {
				t.Parallel()
				roles, _ := kubeclient.RbacV1().Roles(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Subset(t, getRoleNames(roles), []string{"radix-app-adm-job", "radix-app-reader-job"})
			})

			t.Run(fmt.Sprintf("%s validate rolebindings", testScenario), func(t *testing.T) {
				rolebindings, _ := kubeclient.RbacV1().RoleBindings(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Subset(t, getRoleBindingNames(rolebindings), []string{"radix-app-adm-job", "radix-app-reader-job", defaults.RadixJobSchedulerRoleName})
				assert.ElementsMatch(t, adminGroups, slice.Map(getRoleBindingByName("radix-app-adm-job", rolebindings).Subjects, func(s rbacv1.Subject) string { return s.Name }))
				assert.ElementsMatch(t, readerGroups, slice.Map(getRoleBindingByName("radix-app-reader-job", rolebindings).Subjects, func(s rbacv1.Subject) string { return s.Name }))
			})

			t.Run(fmt.Sprintf("%s: validate networkpolicy", testScenario), func(t *testing.T) {
				np, _ := kubeclient.NetworkingV1().NetworkPolicies(envNamespace).List(context.Background(), metav1.ListOptions{})
				assert.Equal(t, 4, len(np.Items), "Number of networkpolicy was not expected")
			})
		})
	}
}

func getServicesForRadixComponents(services *[]corev1.Service) []corev1.Service {
	var result []corev1.Service
	for _, svc := range *services {
		if _, ok := svc.Labels[kube.RadixComponentLabel]; ok {
			result = append(result, svc)
		}
	}
	return result
}

func getDeploymentsForRadixComponents(deployments []appsv1.Deployment) []appsv1.Deployment {
	var result []appsv1.Deployment
	for _, depl := range deployments {
		if _, ok := depl.Labels[kube.RadixComponentLabel]; ok {
			if _, ok := depl.Labels[kube.RadixPodIsJobAuxObjectLabel]; ok {
				continue
			}
			result = append(result, depl)
		}
	}
	return result
}

func getDeploymentsForRadixJobAux(deployments []appsv1.Deployment) []appsv1.Deployment {
	var result []appsv1.Deployment
	for _, depl := range deployments {
		if _, ok := depl.Labels[kube.RadixComponentLabel]; ok {
			if _, ok := depl.Labels[kube.RadixPodIsJobAuxObjectLabel]; ok {
				result = append(result, depl)
			}
		}
	}
	return result
}

func TestObjectSynced_MultiComponent_NonActiveCluster_ContainsOnlyClusterSpecificIngresses(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("edcradix").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "some.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "another.alias.com"}),
			utils.NewDeployComponentBuilder().
				WithName("redis").
				WithPort("http", 6379).
				WithPublicPort(""),
			utils.NewDeployComponentBuilder().
				WithName("radixquote").
				WithPort("http", 3000).
				WithPublicPort("http")))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("edcradix", "test")

	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Only cluster specific ingresses for the two public components should appear")
	assert.Truef(t, ingressByNameExists("app", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("radixquote", ingresses), "Cluster specific ingress for public component should exist")

	appIngress := getIngressByName("app", ingresses)
	assert.Equal(t, int32(8080), appIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, appIngress.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Empty(t, appIngress.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Empty(t, appIngress.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "true", appIngress.Labels[kube.RadixDefaultAliasLabel], "Ingress should be default")
	assert.Equal(t, "app", appIngress.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")

	quoteIngress := getIngressByName("radixquote", ingresses)
	assert.Equal(t, int32(3000), quoteIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, quoteIngress.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Empty(t, quoteIngress.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Empty(t, quoteIngress.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "true", quoteIngress.Labels[kube.RadixDefaultAliasLabel], "Ingress should be default")
	assert.Equal(t, "radixquote", quoteIngress.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")
}

func TestObjectSynced_ReadOnlyFileSystem(t *testing.T) {
	type scenarioSpec struct {
		readOnlyFileSystem         *bool
		expectedReadOnlyFileSystem *bool
	}

	tests := map[string]scenarioSpec{
		"notSet": {readOnlyFileSystem: nil, expectedReadOnlyFileSystem: nil},
		"false":  {readOnlyFileSystem: pointers.Ptr(false), expectedReadOnlyFileSystem: pointers.Ptr(false)},
		"true":   {readOnlyFileSystem: pointers.Ptr(true), expectedReadOnlyFileSystem: pointers.Ptr(true)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
			defer TeardownTest()
			envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")
			_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
				WithAppName("any-app").
				WithEnvironment("any-env").
				WithComponents(
					utils.NewDeployComponentBuilder().
						WithName("app").
						WithReadOnlyFileSystem(test.readOnlyFileSystem)))

			assert.NoError(t, err)
			deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
			for _, deployment := range deployments.Items {
				assert.Equal(t, test.expectedReadOnlyFileSystem, deployment.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
			}
		})
	}

}

func TestObjectSynced_MultiComponent_ActiveCluster_ContainsAllIngresses(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("edcradix").
		WithEnvironment("test").
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "external1.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "external2.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "external3.alias.com", UseCertificateAutomation: true}),
			utils.NewDeployComponentBuilder().
				WithName("redis").
				WithPort("http", 6379).
				WithPublicPort(""),
			utils.NewDeployComponentBuilder().
				WithName("radixquote").
				WithPort("http", 3000).
				WithPublicPort("http")))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("edcradix", "test")

	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 8, len(ingresses.Items), "Number of ingresses was not according to public components, app alias and number of external aliases")
	assert.Truef(t, ingressByNameExists("app", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("radixquote", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("edcradix-url-alias", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("external1.alias.com", ingresses), "App should have external1 external alias")
	assert.Truef(t, ingressByNameExists("external2.alias.com", ingresses), "App should have external2 external alias")
	assert.Truef(t, ingressByNameExists("external3.alias.com", ingresses), "App should have external3 external alias")
	assert.Truef(t, ingressByNameExists("app-active-cluster-url-alias", ingresses), "App should have another external alias")
	assert.Truef(t, ingressByNameExists("radixquote-active-cluster-url-alias", ingresses), "Radixquote should have had an ingress")

	appAlias := getIngressByName("edcradix-url-alias", ingresses)
	assert.Equal(t, int32(8080), appAlias.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Equal(t, "true", appAlias.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Empty(t, appAlias.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Empty(t, appAlias.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", appAlias.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Len(t, appAlias.Annotations, 0)
	assert.Equal(t, "edcradix.app.dev.radix.equinor.com", appAlias.Spec.Rules[0].Host, "App should have an external alias")

	externalDNS1 := getIngressByName("external1.alias.com", ingresses)
	assert.Equal(t, int32(8080), externalDNS1.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, externalDNS1.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", externalDNS1.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Empty(t, externalDNS1.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", externalDNS1.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "external1.alias.com", externalDNS1.Spec.Rules[0].Host, "App should have an external alias")

	externalDNS2 := getIngressByName("external2.alias.com", ingresses)
	assert.Equal(t, int32(8080), externalDNS2.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, externalDNS2.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", externalDNS2.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Empty(t, externalDNS2.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", externalDNS2.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "external2.alias.com", externalDNS2.Spec.Rules[0].Host, "App should have an external alias")

	externalDNS3 := getIngressByName("external3.alias.com", ingresses)
	assert.Equal(t, int32(8080), externalDNS3.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, externalDNS3.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", externalDNS3.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Empty(t, externalDNS3.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", externalDNS3.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "external3.alias.com", externalDNS3.Spec.Rules[0].Host, "App should have an external alias")

	appActiveClusterIngress := getIngressByName("app-active-cluster-url-alias", ingresses)
	assert.Equal(t, int32(8080), appActiveClusterIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, appActiveClusterIngress.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Empty(t, appActiveClusterIngress.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "true", appActiveClusterIngress.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", appActiveClusterIngress.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")

	quoteActiveClusterIngress := getIngressByName("radixquote-active-cluster-url-alias", ingresses)
	assert.Equal(t, int32(3000), quoteActiveClusterIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, quoteActiveClusterIngress.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Empty(t, quoteActiveClusterIngress.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "true", quoteActiveClusterIngress.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "radixquote", quoteActiveClusterIngress.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")

}

func TestObjectSynced_ServiceAccountSettingsAndRbac(t *testing.T) {
	defer TeardownTest()
	// Test
	t.Run("app with component use default SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, pointers.Ptr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})

	t.Run("app with component using identity use custom SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		appName, envName, componentName, clientId, newClientId := "any-app", "any-env", "any-component", "any-client-id", "new-client-id"

		// Deploy component with Azure identity must create custom SA
		_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		sa := serviceAccounts.Items[0]
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), sa.Name)
		assert.Equal(t, map[string]string{"azure.workload.identity/client-id": clientId}, sa.Annotations)
		assert.Equal(t, map[string]string{kube.RadixComponentLabel: componentName, kube.IsServiceAccountForComponent: "true", "azure.workload.identity/use": "true"}, sa.Labels)

		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, pointers.Ptr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
		assert.Equal(t, "true", expectedDeployments[0].Spec.Template.Labels["azure.workload.identity/use"])

		// Deploy component with new Azure identity must update SA
		_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: newClientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		sa = serviceAccounts.Items[0]
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), sa.Name)
		assert.Equal(t, map[string]string{"azure.workload.identity/client-id": newClientId}, sa.Annotations)
		assert.Equal(t, map[string]string{kube.RadixComponentLabel: componentName, kube.IsServiceAccountForComponent: "true", "azure.workload.identity/use": "true"}, sa.Labels)

		deployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		expectedDeployments = getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, pointers.Ptr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
		assert.Equal(t, "true", expectedDeployments[0].Spec.Template.Labels["azure.workload.identity/use"])

		// Redploy component without Azure identity should delete custom SA
		_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(utils.NewDeployComponentBuilder().WithName(componentName)).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")

		deployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		expectedDeployments = getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, pointers.Ptr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
		_, hasLabel := expectedDeployments[0].Spec.Template.Labels["azure.workload.identity/use"]
		assert.False(t, hasLabel)
	})

	t.Run("app with component using identity fails if SA exist with missing is-service-account-for-component label", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		appName, envName, componentName, clientId := "any-app", "any-env", "any-component", "any-client-id"
		_, err := client.CoreV1().ServiceAccounts("any-app-any-env").Create(
			context.Background(),
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: utils.GetComponentServiceAccountName(componentName), Labels: map[string]string{kube.RadixComponentLabel: componentName}}},
			metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		assert.Error(t, err)
	})

	t.Run("app with component using identity success if SA exist with correct labels", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		appName, envName, componentName, clientId := "any-app", "any-env", "any-component", "any-client-id"
		_, err := client.CoreV1().ServiceAccounts("any-app-any-env").Create(
			context.Background(),
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: utils.GetComponentServiceAccountName(componentName), Labels: map[string]string{kube.RadixComponentLabel: componentName, kube.IsServiceAccountForComponent: "true", "any-other-label": "any-value"}}},
			metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		sa := serviceAccounts.Items[0]
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), sa.Name)
		assert.Equal(t, map[string]string{"azure.workload.identity/client-id": clientId}, sa.Annotations)
		assert.Equal(t, map[string]string{kube.RadixComponentLabel: componentName, kube.IsServiceAccountForComponent: "true", "azure.workload.identity/use": "true"}, sa.Labels)
	})

	t.Run("component removed, custom SA is garbage collected", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		appName, envName, componentName, clientId, anyOtherServiceAccountName := "any-app", "any-env", "any-component", "any-client-id", "any-other-serviceaccount"

		// A service account that must not be deleted
		_, err := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).Create(
			context.Background(),
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: anyOtherServiceAccountName, Labels: map[string]string{kube.RadixComponentLabel: "anything"}}},
			metav1.CreateOptions{})
		require.NoError(t, err)
		// Deploy component with Azure identity must create custom SA
		_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
				utils.NewDeployComponentBuilder().
					WithName("any-other-component").
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 3, len(serviceAccounts.Items), "Number of service accounts was not expected")

		// Redploy component without Azure identity should delete custom SA
		_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 2, len(serviceAccounts.Items), "Number of service accounts was not expected")
		assert.NotNil(t, getServiceAccountByName(utils.GetComponentServiceAccountName(componentName), serviceAccounts))
		assert.NotNil(t, getServiceAccountByName(anyOtherServiceAccountName, serviceAccounts))
	})

	t.Run("app with job use radix-job-scheduler SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, pointers.Ptr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

	})

	// Test
	t.Run("app from component to job and back to component", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)

		// Initial deployment, app is a component
		_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("comp1")).
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
		allDeployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(allDeployments.Items)
		assert.Equal(t, pointers.Ptr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

		// Change app to be a job
		_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().WithName("job1")).
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		allDeployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(),
			metav1.ListOptions{})
		expectedJobDeployments := getDeploymentsForRadixComponents(allDeployments.Items)
		assert.Equal(t, 1, len(expectedJobDeployments))
		assert.Equal(t, pointers.Ptr(true), expectedJobDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, expectedJobDeployments[0].Spec.Template.Spec.ServiceAccountName)
		expectedJobAuxDeployments := getDeploymentsForRadixJobAux(allDeployments.Items)
		assert.Equal(t, 1, len(expectedJobAuxDeployments))
		assert.Equal(t, pointers.Ptr(false), expectedJobAuxDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedJobAuxDeployments[0].Spec.Template.Spec.ServiceAccountName)

		// And change app back to a component
		_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("comp1")).
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		allDeployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.Background(), metav1.ListOptions{})
		expectedDeployments = getDeploymentsForRadixComponents(allDeployments.Items)
		assert.Equal(t, 1, len(expectedDeployments))
		assert.Equal(t, pointers.Ptr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})

	t.Run("webhook runs as radix-github-webhook SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("radix-github-webhook").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(context.Background(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, pointers.Ptr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixGithubWebhookServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

	})

	t.Run("radix-api runs as radix-api SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
		_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("radix-api").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-api", "test")).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("radix-api", "test")).List(context.Background(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, pointers.Ptr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixAPIServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})
}

func TestObjectSynced_MultiComponentWithSameName_ContainsOneComponent(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("test").
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithImage("anyimage").
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithImage("anotherimage").
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http")))
	require.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
	assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")

	services, _ := client.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
	expectedServices := getServicesForRadixComponents(&services.Items)
	assert.Equal(t, 1, len(expectedServices), "Number of services wasn't as expected")

	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Number of ingresses was not according to public components")
}

func TestConfigMap_IsGarbageCollected(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	appName := "app"
	comp1, comp2, job1, job2 := "comp1", "comp2", "job1", "job2"
	anyEnvironment := "test"
	namespace := utils.GetEnvironmentNamespace(appName, anyEnvironment)

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().WithName(comp1),
			utils.NewDeployComponentBuilder().WithName(comp2),
		).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName(job1),
			utils.NewDeployJobComponentBuilder().WithName(job2),
		),
	)
	require.NoError(t, err)

	cmNameMapper := func(cm *corev1.ConfigMap) string { return cm.Name }
	componentsEnvNames := func(names ...string) []string {
		return slice.Map(names, func(n string) string { return kube.GetEnvVarsConfigMapName(n) })
	}
	componentsEnvMetaNames := func(names ...string) []string {
		return slice.Map(names, func(n string) string { return kube.GetEnvVarsMetadataConfigMapName(n) })
	}

	// check that config maps with env vars and env vars metadata were created
	envVarCms, err := kubeUtil.ListEnvVarsConfigMaps(context.Background(), namespace)
	assert.NoError(t, err)
	assert.ElementsMatch(t, componentsEnvNames(comp1, comp2, job1, job2), slice.Map(envVarCms, cmNameMapper))
	envVarMetadataCms, err := kubeUtil.ListEnvVarsMetadataConfigMaps(context.Background(), namespace)
	assert.NoError(t, err)
	assert.ElementsMatch(t, componentsEnvMetaNames(comp1, comp2, job1, job2), slice.Map(envVarMetadataCms, cmNameMapper))

	// delete 2nd component
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().WithName(comp1),
		).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName(job1),
		),
	)
	require.NoError(t, err)

	// check that config maps were garbage collected for the component and job we just deleted
	envVarCms, err = kubeUtil.ListEnvVarsConfigMaps(context.Background(), namespace)
	assert.NoError(t, err)
	assert.ElementsMatch(t, componentsEnvNames(comp1, job1), slice.Map(envVarCms, cmNameMapper))
	envVarMetadataCms, err = kubeUtil.ListEnvVarsMetadataConfigMaps(context.Background(), namespace)
	assert.NoError(t, err)
	assert.ElementsMatch(t, componentsEnvMetaNames(comp1, job1), slice.Map(envVarMetadataCms, cmNameMapper))
}

func TestConfigMap_RetainDataBetweenSync(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	appName := "app"
	comp, job1, job2 := "comp", "job1", "job2"
	anyEnvironment := "test"
	namespace := utils.GetEnvironmentNamespace(appName, anyEnvironment)

	// Initial apply of RadixDeployment
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(comp).
				WithEnvironmentVariables(map[string]string{"COMPVAR1": "comp_original1", "COMPVAR2": "comp_original2"}),
		).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(job1).
				WithEnvironmentVariables(map[string]string{"JOB1VAR1": "job1_original1", "JOB1VAR2": "job1_original2"}),
			utils.NewDeployJobComponentBuilder().
				WithName(job2).
				WithEnvironmentVariables(map[string]string{"JOB2VAR1": "job2_original1", "JOB2VAR2": "job2_original2"}),
		),
	)
	require.NoError(t, err)

	// Check initial state of configmaps
	// comp configmaps
	varCm, _, varMeta, err := kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, comp)
	require.NoError(t, err)
	expectedData := map[string]string{"COMPVAR1": "comp_original1", "COMPVAR2": "comp_original2"}
	assert.Equal(t, expectedData, varCm.Data)
	assert.Empty(t, varMeta)
	// job1 configmaps
	varCm, _, varMeta, err = kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, job1)
	require.NoError(t, err)
	expectedData = map[string]string{"JOB1VAR1": "job1_original1", "JOB1VAR2": "job1_original2"}
	assert.Equal(t, expectedData, varCm.Data)
	assert.Empty(t, varMeta)
	// job2 configmaps
	varCm, _, varMeta, err = kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, job2)
	require.NoError(t, err)
	expectedData = map[string]string{"JOB2VAR1": "job2_original1", "JOB2VAR2": "job2_original2"}
	assert.Equal(t, expectedData, varCm.Data)
	assert.Empty(t, varMeta)

	// Update variables and metadata configmaps, the way radix-api will do a change from a user
	updateVariable := func(compName, varName, newVal, configValue string) error {
		varCm, varMetaCm, varMeta, err := kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, compName)
		if err != nil {
			return err
		}
		updatedVarCm := varCm.DeepCopy()
		updatedVarCm.Data[varName] = newVal
		varMeta[varName] = kube.EnvVarMetadata{RadixConfigValue: configValue}
		err = kubeUtil.ApplyConfigMap(context.Background(), namespace, varCm, updatedVarCm)
		if err != nil {
			return err
		}
		return kubeUtil.ApplyEnvVarsMetadataConfigMap(context.Background(), namespace, varMetaCm, varMeta)
	}
	// Change comp configmaps
	err = updateVariable(comp, "COMPVAR1", "comp_new1", "comp_original1")
	require.NoError(t, err)
	// Change job1 configmaps
	err = updateVariable(job1, "JOB1VAR1", "job1_new1", "job1_original1")
	require.NoError(t, err)
	// Change job2 configmaps
	err = updateVariable(job2, "JOB2VAR1", "job2_new1", "job2_original1")
	require.NoError(t, err)

	// Apply update of RadixDeployment - job2 is moved from jobs to components
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(comp).
				WithEnvironmentVariables(map[string]string{"COMPVAR1": "comp_original1", "COMPVAR2": "comp_original2"}),
			utils.NewDeployComponentBuilder().
				WithName(job2).
				WithEnvironmentVariables(map[string]string{"JOB2VAR1": "job2_original1", "JOB2VAR2": "job2_original2"}),
		).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(job1).
				WithEnvironmentVariables(map[string]string{"JOB1VAR1": "job1_original1", "JOB1VAR2": "job1_original2"}),
		),
	)
	require.NoError(t, err)

	// // Check state of configmaps is kept between syncs
	// // comp configmaps
	varCm, _, varMeta, err = kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, comp)
	require.NoError(t, err)
	expectedData = map[string]string{"COMPVAR1": "comp_new1", "COMPVAR2": "comp_original2"}
	assert.Equal(t, expectedData, varCm.Data)
	assert.Equal(t, map[string]kube.EnvVarMetadata{"COMPVAR1": {RadixConfigValue: "comp_original1"}}, varMeta)
	// job1 configmaps
	varCm, _, varMeta, err = kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, job1)
	require.NoError(t, err)
	expectedData = map[string]string{"JOB1VAR1": "job1_new1", "JOB1VAR2": "job1_original2"}
	assert.Equal(t, expectedData, varCm.Data)
	assert.Equal(t, map[string]kube.EnvVarMetadata{"JOB1VAR1": {RadixConfigValue: "job1_original1"}}, varMeta)
	// job2 configmaps
	varCm, _, varMeta, err = kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, job2)
	require.NoError(t, err)
	expectedData = map[string]string{"JOB2VAR1": "job2_new1", "JOB2VAR2": "job2_original2"}
	assert.Equal(t, expectedData, varCm.Data)
	assert.Equal(t, map[string]kube.EnvVarMetadata{"JOB2VAR1": {RadixConfigValue: "job2_original1"}}, varMeta)
}

func TestObjectSynced_NoEnvAndNoSecrets_ContainsDefaultEnvVariables(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyEnvironment := "test"
	commitId := string(uuid.NewUUID())

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("component").
				WithEnvironmentVariables(nil).
				WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commitId).
				WithSecrets(nil)))
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
		container := deployments.Items[0].Spec.Template.Spec.Containers[0]
		cm, _ := client.CoreV1().ConfigMaps(envNamespace).Get(context.Background(), kube.GetEnvVarsConfigMapName(container.Name), metav1.GetOptions{})

		templateSpecEnv := container.Env
		assert.Equal(t, 9, len(templateSpecEnv), "Should only have default environment variables")
		assert.True(t, envVariableByNameExist(defaults.ContainerRegistryEnvironmentVariable, templateSpecEnv))
		assert.True(t, envVariableByNameExist(defaults.RadixDNSZoneEnvironmentVariable, templateSpecEnv))
		assert.True(t, envVariableByNameExist(defaults.ClusternameEnvironmentVariable, templateSpecEnv))
		assert.True(t, envVariableByNameExist(defaults.RadixClusterTypeEnvironmentVariable, templateSpecEnv))
		assert.True(t, envVariableByNameExist(defaults.RadixAppEnvironmentVariable, templateSpecEnv))
		assert.True(t, envVariableByNameExist(defaults.RadixComponentEnvironmentVariable, templateSpecEnv))
		assert.True(t, envVariableByNameExist(defaults.RadixCommitHashEnvironmentVariable, templateSpecEnv))
		assert.True(t, envVariableByNameExist(defaults.RadixActiveClusterEgressIpsEnvironmentVariable, templateSpecEnv))
		assert.True(t, envVariableByNameExist(defaults.RadixCommitHashEnvironmentVariable, templateSpecEnv))
		assert.Equal(t, os.Getenv(defaults.ContainerRegistryEnvironmentVariable), getEnvVariableByName(defaults.ContainerRegistryEnvironmentVariable, templateSpecEnv, nil))
		assert.Equal(t, os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable), getEnvVariableByName(defaults.RadixDNSZoneEnvironmentVariable, templateSpecEnv, cm))
		assert.Equal(t, testClusterName, getEnvVariableByName(defaults.ClusternameEnvironmentVariable, templateSpecEnv, cm))
		assert.Equal(t, anyEnvironment, getEnvVariableByName(defaults.EnvironmentnameEnvironmentVariable, templateSpecEnv, cm))
		assert.Equal(t, "app", getEnvVariableByName(defaults.RadixAppEnvironmentVariable, templateSpecEnv, cm))
		assert.Equal(t, "component", getEnvVariableByName(defaults.RadixComponentEnvironmentVariable, templateSpecEnv, cm))
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 0, len(secrets.Items), "Should have no secrets")
	})
}

func TestObjectSynced_WithLabels_LabelsAppliedToDeployment(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeploymentWithComponentModifier(func(builder utils.DeployComponentBuilder) utils.DeployComponentBuilder {
			return builder.WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, "4faca8595c5283a9d0f17a623b9255a0d9866a2e")
		}).
			WithAppName("app").
			WithEnvironment("test").
			WithLabel("radix-branch", "master"))
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace("app", "test")

	t.Run("validate deploy labels", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, "master", deployments.Items[0].Annotations[kube.RadixBranchAnnotation])
		assert.Equal(t, "4faca8595c5283a9d0f17a623b9255a0d9866a2e", deployments.Items[0].Labels["radix-commit"])
	})
}

func TestObjectSynced_NotLatest_DeploymentIsIgnored(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()

	// Test
	now := time.Now().UTC()
	var firstUID, secondUID types.UID

	firstUID = "fda3d224-3115-11e9-b189-06c15a8f2fbb"
	secondUID = "5a8f2fbb-3115-11e9-b189-06c1fda3d224"

	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("firstdeployment").
		WithCreated(now).
		WithUID(firstUID).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http")))
	require.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("app1", "prod")
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, firstUID, deployments.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	services, _ := client.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, firstUID, services.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, firstUID, ingresses.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	time.Sleep(1 * time.Millisecond)
	// This is one second newer deployment
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("seconddeployment").
		WithCreated(now.Add(time.Second*time.Duration(1))).
		WithUID(secondUID).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http")))
	require.NoError(t, err)
	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	services, _ = client.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, secondUID, ingresses.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	// Re-apply the first  This should be ignored and cause an error as it is not the latest
	rdBuilder := utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("app1").
		WithEnvironment("prod").
		WithImageTag("firstdeployment").
		WithCreated(now).
		WithUID(firstUID).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithPublicPort("http"))

	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
	require.NoError(t, err)

	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	services, _ = client.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, secondUID, ingresses.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")
}

func Test_UpdateAndAddDeployment_DeploymentAnnotationIsCorrectlyUpdated(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Test first deployment
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("first_deployment").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("first").
				WithAlwaysPullImageOnDeploy(true),
			utils.NewDeployComponentBuilder().
				WithName("second").
				WithAlwaysPullImageOnDeploy(false)))
	require.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("anyapp1", "test")

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	firstDeployment := getDeploymentByName("first", deployments.Items)
	assert.Equal(t, "first_deployment", firstDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation])
	secondDeployment := getDeploymentByName("second", deployments.Items)
	assert.Empty(t, secondDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation])

	// Test second deployment
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("second_deployment").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("first").
				WithAlwaysPullImageOnDeploy(true),
			utils.NewDeployComponentBuilder().
				WithName("second").
				WithAlwaysPullImageOnDeploy(false)))
	require.NoError(t, err)
	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	firstDeployment = getDeploymentByName("first", deployments.Items)
	assert.Equal(t, "second_deployment", firstDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation])
	secondDeployment = getDeploymentByName("second", deployments.Items)
	assert.Empty(t, secondDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation])
}

func TestObjectUpdated_UpdatePort_IngressIsCorrectlyReconciled(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8080).
				WithAlwaysPullImageOnDeploy(true).
				WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithName("app2").
				WithPort("http", 8080).
				WithAlwaysPullImageOnDeploy(false).
				WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithName("app3").
				WithPort("http", 8080).
				WithPublicPort("http")))
	require.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace("anyapp1", "test")
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")

	time.Sleep(1 * time.Second)

	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8081).
				WithAlwaysPullImageOnDeploy(true).
				WithPublicPort("http")))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, int32(8081), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
}

func TestObjectUpdated_ZeroReplicasExistsAndNotSpecifiedReplicas_SetsDefaultReplicaCount(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithReplicas(test.IntPtr(0))))
	require.NoError(t, err)

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, int32(0), *deployments.Items[0].Spec.Replicas)

	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app")))
	require.NoError(t, err)
	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, int32(1), *deployments.Items[0].Spec.Replicas)
}

func TestObjectSynced_DeploymentReplicasSetAccordingToSpec(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1"),
			utils.NewDeployComponentBuilder().WithName("comp2").WithReplicas(pointers.Ptr(2)),
			utils.NewDeployComponentBuilder().WithName("comp3").WithReplicas(pointers.Ptr(4)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(5).WithMaxReplicas(10).Build()),
			utils.NewDeployComponentBuilder().WithName("comp4").WithReplicas(pointers.Ptr(6)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(5).WithMaxReplicas(10).Build()),
			utils.NewDeployComponentBuilder().WithName("comp5").WithReplicas(pointers.Ptr(11)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(5).WithMaxReplicas(10).Build()),
			utils.NewDeployComponentBuilder().WithName("comp6").WithReplicas(pointers.Ptr(0)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(5).WithMaxReplicas(10).Build()),
			utils.NewDeployComponentBuilder().WithName("comp7").WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(5).WithMaxReplicas(10).Build()),
		))
	require.NoError(t, err)
	comp1, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(1), *comp1.Spec.Replicas)
	comp2, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp2", metav1.GetOptions{})
	assert.Equal(t, int32(2), *comp2.Spec.Replicas)
	comp3, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp3", metav1.GetOptions{})
	assert.Equal(t, int32(5), *comp3.Spec.Replicas)
	comp4, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp4", metav1.GetOptions{})
	assert.Equal(t, int32(6), *comp4.Spec.Replicas)
	comp5, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp5", metav1.GetOptions{})
	assert.Equal(t, int32(10), *comp5.Spec.Replicas)
	comp6, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp6", metav1.GetOptions{})
	assert.Equal(t, int32(0), *comp6.Spec.Replicas)
	comp7, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp7", metav1.GetOptions{})
	assert.Equal(t, int32(5), *comp7.Spec.Replicas)
}

func TestObjectSynced_DeploymentReplicasFromCurrentDeploymentWhenHPAEnabled(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Initial sync creating deployments should use replicas from spec
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(4).Build()),
		))
	require.NoError(t, err)

	comp1, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(1), *comp1.Spec.Replicas)

	// Simulate HPA scaling up comp1 to 3 replicas
	comp1.Spec.Replicas = pointers.Ptr[int32](3)
	_, err = client.AppsV1().Deployments(envNamespace).Update(context.Background(), comp1, metav1.UpdateOptions{})
	require.NoError(t, err)
	// Resync existing RD should use replicas from current deployment for HPA enabled component
	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(4).Build()),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(3), *comp1.Spec.Replicas)

	// Resync new RD should use replicas from current deployment for HPA enabled component
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("deployment2").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(4).Build()),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(3), *comp1.Spec.Replicas)

	// Resync new RD with HPA removed should use replicas from RD spec
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("deployment3").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(1), *comp1.Spec.Replicas)
}

func TestObjectSynced_StopAndStartDeploymentWhenHPAEnabled(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Initial sync creating deployments should use replicas from spec
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(4).Build()),
		))
	require.NoError(t, err)

	comp1, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(2), *comp1.Spec.Replicas)

	// Resync existing RD with replicas 0 (stop) should set deployment replicas to 0
	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(0)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(1).WithMaxReplicas(4).Build()),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(0), *comp1.Spec.Replicas)

	// Resync existing RD with replicas set back to original value (start) should use replicas from spec
	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(2).WithMaxReplicas(4).Build()),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(2), *comp1.Spec.Replicas)

}

func TestObjectSynced_DeploymentRevisionHistoryLimit(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1"),
			utils.NewDeployComponentBuilder().WithName("comp2").WithSecretRefs(radixv1.RadixSecretRefs{AzureKeyVaults: []radixv1.RadixAzureKeyVault{{}}}),
		))
	require.NoError(t, err)
	comp1, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp1", metav1.GetOptions{})
	assert.Equal(t, pointers.Ptr(int32(10)), comp1.Spec.RevisionHistoryLimit, "Invalid default RevisionHistoryLimit")
	comp2, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), "comp2", metav1.GetOptions{})
	assert.Equal(t, pointers.Ptr(int32(0)), comp2.Spec.RevisionHistoryLimit, "Invalid RevisionHistoryLimit")
}

func TestObjectSynced_DeploymentsUsedByScheduledJobsMaintainHistoryLimit(t *testing.T) {
	type scenario struct {
		name                        string
		deploymentNames             []string
		deploymentsReferencedInJobs map[string][]string
		expectedDeploymentNames     []string
	}
	scenarios := []scenario{
		{name: "no jobs, no history cleanup",
			deploymentNames: []string{"d1", "d2"}, expectedDeploymentNames: []string{"d1", "d2"}},
		{name: "no jobs, rd, deleted by history cleanup",
			deploymentNames: []string{"d1", "d2", "d3"}, expectedDeploymentNames: []string{"d2", "d3"}},
		{name: "one oldest rd, referenced by a job, not deleted by history cleanup",
			deploymentNames: []string{"d1", "d2", "d3"}, deploymentsReferencedInJobs: map[string][]string{"d1": {"j1"}}, expectedDeploymentNames: []string{"d1", "d2", "d3"}},
		{name: "one intermediate rd, referenced by a job, not deleted by history cleanup",
			deploymentNames: []string{"d1", "d2", "d3"}, deploymentsReferencedInJobs: map[string][]string{"d2": {"j1"}}, expectedDeploymentNames: []string{"d2", "d3"}},
		{name: "multiple rds, referenced by multiple jobs, not deleted by history cleanup",
			deploymentNames: []string{"d1", "d2", "d3", "d4"}, deploymentsReferencedInJobs: map[string][]string{"d1": {"j1", "j2"}, "d2": {"j3", "j4"}}, expectedDeploymentNames: []string{"d1", "d2", "d3", "d4"}},
		{name: "multiple rds, referenced by multiple jobs, not referenced deleted by history cleanup",
			deploymentNames: []string{"d1", "d2", "d3", "d4"}, deploymentsReferencedInJobs: map[string][]string{"d1": {"j1", "j2"}, "d3": {"j3", "j4"}}, expectedDeploymentNames: []string{"d1", "d3", "d4"}},
		{name: "many rds, referenced by multiple jobs, not referenced deleted by history cleanup",
			deploymentNames: []string{"d1", "d2", "d3", "d4", "d5"}, deploymentsReferencedInJobs: map[string][]string{"d1": {"j1", "j2"}, "d3": {"j3", "j4"}}, expectedDeploymentNames: []string{"d1", "d3", "d4", "d5"}},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(tt *testing.T) {
			tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
			defer TeardownTest()
			envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

			radixApplication := utils.ARadixApplication()
			now := time.Now()
			timeShift := 1
			for _, deploymentName := range ts.deploymentNames {
				_, err := applyDeploymentWithModifiedSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.NewDeploymentBuilder().
					WithRadixApplication(radixApplication).
					WithDeploymentName(deploymentName).
					WithAppName("anyapp").
					WithCreated(now.Add(time.Duration(timeShift)*time.Minute)).
					WithEnvironment("test").
					WithJobComponents(
						utils.NewDeployJobComponentBuilder().WithName("job1"),
					), func(syncer DeploymentSyncer) {
					if s, ok := syncer.(*Deployment); ok {
						newcfg := *s.config
						newcfg.DeploymentSyncer.DeploymentHistoryLimit = 2
						s.config = &newcfg
					}
				})
				require.NoError(t, err)
				err = addRadixBatches(radixclient, envNamespace, deploymentName, ts.deploymentsReferencedInJobs[deploymentName])
				assert.NoError(tt, err)
				timeShift++
			}

			rdList, err := radixclient.RadixV1().RadixDeployments(envNamespace).List(context.Background(), metav1.ListOptions{})
			assert.NoError(tt, err)
			rbList, err := radixclient.RadixV1().RadixBatches(envNamespace).List(context.Background(), metav1.ListOptions{})
			assert.NoError(tt, err)
			assert.NotNil(tt, rbList)

			foundRdNames := slice.Map(rdList.Items, func(rd radixv1.RadixDeployment) string { return rd.GetName() })
			assert.True(tt, radixutils.EqualStringLists(ts.expectedDeploymentNames, foundRdNames), fmt.Sprintf("expected %v, got %v", ts.expectedDeploymentNames, foundRdNames))
		})
	}
}

func addRadixBatches(radixclient radixclient.Interface, envNamespace string, deploymentName string, jobNames []string) error {
	var errs []error
	for _, jobName := range jobNames {
		_, err := radixclient.RadixV1().RadixBatches(envNamespace).Create(context.Background(), &radixv1.RadixBatch{
			ObjectMeta: metav1.ObjectMeta{Name: deploymentName + jobName},
			Spec: radixv1.RadixBatchSpec{
				RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
					LocalObjectReference: radixv1.LocalObjectReference{Name: deploymentName},
					Job:                  jobName,
				},
				Jobs: []radixv1.RadixBatchJob{{Name: radixutils.RandString(5)}},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func TestObjectUpdated_MultipleReplicasExistsAndNotSpecifiedReplicas_SetsDefaultReplicaCount(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithReplicas(test.IntPtr(3))))
	require.NoError(t, err)

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, int32(3), *deployments.Items[0].Spec.Replicas)

	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app")))
	require.NoError(t, err)
	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, int32(1), *deployments.Items[0].Spec.Replicas)
}

func TestObjectUpdated_WithAppAliasRemoved_AliasIngressIsCorrectlyReconciled(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true)))
	require.NoError(t, err)
	// Test
	ingresses, _ := client.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("any-app-url-alias", ingresses), "App should have had an app alias ingress")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have another external alias")

	// Remove app alias from dev
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(false)))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have another external alias")
}

func TestObjectSynced_MultiComponentToOneComponent_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	componentThreeName := "componentThreeName"

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithReplicas(test.IntPtr(4)),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(test.IntPtr(0)),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 3000).
				WithPublicPort("http")))

	require.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
	assert.Equal(t, 3, len(expectedDeployments), "Number of deployments wasn't as expected")
	assert.Equal(t, componentOneName, deployments.Items[0].Name, "app deployment not there")

	// Remove components
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("").
				WithReplicas(test.IntPtr(0)).
				WithSecrets([]string{"a_secret"})))

	require.NoError(t, err)
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")
		assert.Equal(t, componentTwoName, deployments.Items[0].Name, "app deployment not there")
	})

	t.Run("validate service", func(t *testing.T) {
		t.Parallel()
		services, _ := client.CoreV1().Services(envNamespace).List(context.Background(), metav1.ListOptions{})
		expectedServices := getServicesForRadixComponents(&services.Items)
		assert.Equal(t, 1, len(expectedServices), "Number of services wasn't as expected")
	})

	t.Run("validate ingress", func(t *testing.T) {
		t.Parallel()
		ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 0, len(ingresses.Items), "Number of ingresses was not according to public components")
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 1, len(secrets.Items), "Number of secrets was not according to spec")
		assert.Equal(t, utils.GetComponentSecretName(componentTwoName), secrets.Items[0].GetName(), "Component secret is not as expected")
	})

	t.Run("validate service accounts", func(t *testing.T) {
		t.Parallel()
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("validate roles", func(t *testing.T) {
		t.Parallel()
		roles, _ := client.RbacV1().Roles(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.ElementsMatch(t, []string{"radix-app-adm-componentTwoName", "radix-app-reader-componentTwoName", "radix-app-externaldns-adm", "radix-app-externaldns-reader"}, getRoleNames(roles))
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.ElementsMatch(t, []string{"radix-app-adm-componentTwoName", "radix-app-reader-componentTwoName", "radix-app-externaldns-adm", "radix-app-externaldns-reader"}, getRoleBindingNames(rolebindings))
	})
}

func TestObjectSynced_PublicToNonPublic_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"

	// Test
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http")))
	require.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Both components should be public")

	// Remove public on component 2
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http"),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("")))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Only component 1 should be public")

	// Remove public on component 1
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort(""),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("")))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "No component should be public")
}

//nolint:staticcheck
func TestObjectSynced_PublicPort_OldPublic(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"

	// New publicPort exists, old public does not exist
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			//lint:ignore SA1019 backward compatilibity test
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("http").
				WithPublic(false)))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	assert.Equal(t, int32(80), ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)

	// New publicPort exists, old public exists (ignored)
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			//lint:ignore SA1019 backward compatilibity test
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("http").
				WithPublic(true)))

	assert.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	assert.Equal(t, int32(80), ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)

	// New publicPort does not exist, old public does not exist
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			//lint:ignore SA1019 backward compatilibity test
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("").
				WithPublic(false)))

	assert.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "Component should not be public")

	// New publicPort does not exist, old public exists (used)
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			//lint:ignore SA1019 backward compatilibity test
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("https").
				WithPublic(true)))

	assert.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	expectedIngresses := getIngressesForRadixComponents(&ingresses.Items)
	assert.Equal(t, 1, len(expectedIngresses), "Component should be public")
	actualPortValue := ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number
	assert.Equal(t, int32(443), actualPortValue)
}

func getIngressesForRadixComponents(ingresses *[]networkingv1.Ingress) []networkingv1.Ingress {
	var result []networkingv1.Ingress
	for _, ing := range *ingresses {
		if val, ok := ing.Labels[kube.RadixComponentLabel]; ok && val != "job" {
			result = append(result, ing)
		}
	}
	return result
}

func TestObjectUpdated_WithAllExternalAliasRemoved_ExternalAliasIngressIsCorrectlyReconciled(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "some.alias.com"})))
	require.NoError(t, err)
	// Test
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})

	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	// Remove app alias from dev
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http")))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})

	assert.Equal(t, 2, len(ingresses.Items), "External alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")
}

func TestObjectUpdated_WithOneExternalAliasRemovedOrModified_AllChangesProperlyReconciled(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "some.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "another.alias.com"}).
				WithSecrets([]string{"a_secret"})))
	require.NoError(t, err)
	// Test
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 4, len(ingresses.Items), "Environment should have four ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	externalAliasIngress := getIngressByName("some.alias.com", ingresses)
	assert.Equal(t, "some.alias.com", externalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8080), externalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number, "Correct service port")

	anotherExternalAliasIngress := getIngressByName("another.alias.com", ingresses)
	assert.Equal(t, "another.alias.com", anotherExternalAliasIngress.GetName(), "App should have had another external alias ingress")
	assert.Equal(t, "another.alias.com", anotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8080), anotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number, "Correct service port")

	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "some.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "yet.another.alias.com"}).
				WithSecrets([]string{"a_secret"})))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 4, len(ingresses.Items), "Environment should have four ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("yet.another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	externalAliasIngress = getIngressByName("some.alias.com", ingresses)
	assert.Equal(t, "some.alias.com", externalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), externalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number, "Correct service port")

	yetAnotherExternalAliasIngress := getIngressByName("yet.another.alias.com", ingresses)
	assert.Equal(t, "yet.another.alias.com", yetAnotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), yetAnotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number, "Correct service port")

	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "yet.another.alias.com"}).
				WithSecrets([]string{"a_secret"})))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("yet.another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	yetAnotherExternalAliasIngress = getIngressByName("yet.another.alias.com", ingresses)
	assert.Equal(t, "yet.another.alias.com", yetAnotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), yetAnotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number, "Correct service port")

	// Remove app alias from dev
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http")))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "External alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")
}

func TestFixedAliasIngress_ActiveCluster(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))

	// Current cluster is active cluster
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, radixDeployBuilder)
	require.NoError(t, err)
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Environment should have two ingresses")
	activeClusterIngress := getIngressByName(getActiveClusterIngressName(anyComponentName), ingresses)
	assert.False(t, strings.Contains(activeClusterIngress.Spec.Rules[0].Host, testClusterName))
	defaultIngress := getIngressByName(getDefaultIngressName(anyComponentName), ingresses)
	assert.True(t, strings.Contains(defaultIngress.Spec.Rules[0].Host, testClusterName))

	// Current cluster is not active cluster
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "newClusterName")
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, radixDeployBuilder)
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Environment should have one ingresses")
	assert.True(t, strings.Contains(ingresses.Items[0].Spec.Rules[0].Host, testClusterName))
}

func TestNewDeploymentStatus(t *testing.T) {
	anyApp := "any-app"
	anyEnv := "dev"
	anyComponentName := "frontend"

	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(anyApp).
		WithEnvironment(anyEnv).
		WithEmptyStatus().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))

	rd, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, radixDeployBuilder)
	require.NoError(t, err)
	assert.Equal(t, radixv1.DeploymentActive, rd.Status.Condition)
	assert.True(t, !rd.Status.ActiveFrom.IsZero())
	assert.True(t, rd.Status.ActiveTo.IsZero())

	time.Sleep(2 * time.Millisecond)

	radixDeployBuilder = utils.ARadixDeployment().
		WithAppName(anyApp).
		WithEnvironment(anyEnv).
		WithEmptyStatus().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))

	rd2, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, radixDeployBuilder)
	require.NoError(t, err)
	rd, _ = getUpdatedRD(radixclient, rd)

	assert.Equal(t, radixv1.DeploymentInactive, rd.Status.Condition)
	assert.Equal(t, rd.Status.ActiveTo, rd2.Status.ActiveFrom)

	assert.Equal(t, radixv1.DeploymentActive, rd2.Status.Condition)
	assert.True(t, !rd2.Status.ActiveFrom.IsZero())
}

func Test_AddMultipleNewDeployments_CorrectStatuses(t *testing.T) {
	anyApp := "any-app"
	anyEnv := "dev"
	anyComponentName := "frontend"
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	rd1 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient)

	time.Sleep(2 * time.Millisecond)
	rd2 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient)
	rd1, _ = getUpdatedRD(radixclient, rd1)

	assert.Equal(t, radixv1.DeploymentInactive, rd1.Status.Condition)
	assert.Equal(t, rd1.Status.ActiveTo, rd2.Status.ActiveFrom)
	assert.Equal(t, radixv1.DeploymentActive, rd2.Status.Condition)
	assert.True(t, !rd2.Status.ActiveFrom.IsZero())

	time.Sleep(3 * time.Millisecond)
	rd3 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient)
	rd1, _ = getUpdatedRD(radixclient, rd1)
	rd2, _ = getUpdatedRD(radixclient, rd2)

	assert.Equal(t, radixv1.DeploymentInactive, rd1.Status.Condition)
	assert.Equal(t, radixv1.DeploymentInactive, rd2.Status.Condition)
	assert.Equal(t, rd1.Status.ActiveTo, rd2.Status.ActiveFrom)
	assert.Equal(t, rd2.Status.ActiveTo, rd3.Status.ActiveFrom)
	assert.Equal(t, radixv1.DeploymentActive, rd3.Status.Condition)
	assert.True(t, !rd3.Status.ActiveFrom.IsZero())

	time.Sleep(4 * time.Millisecond)
	rd4 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient)
	rd1, _ = getUpdatedRD(radixclient, rd1)
	rd2, _ = getUpdatedRD(radixclient, rd2)
	rd3, _ = getUpdatedRD(radixclient, rd3)

	assert.Equal(t, radixv1.DeploymentInactive, rd1.Status.Condition)
	assert.Equal(t, radixv1.DeploymentInactive, rd2.Status.Condition)
	assert.Equal(t, radixv1.DeploymentInactive, rd3.Status.Condition)
	assert.Equal(t, rd1.Status.ActiveTo, rd2.Status.ActiveFrom)
	assert.Equal(t, rd2.Status.ActiveTo, rd3.Status.ActiveFrom)
	assert.Equal(t, rd3.Status.ActiveTo, rd4.Status.ActiveFrom)
	assert.Equal(t, radixv1.DeploymentActive, rd4.Status.Condition)
	assert.True(t, !rd4.Status.ActiveFrom.IsZero())
}

func getUpdatedRD(radixclient radixclient.Interface, rd *radixv1.RadixDeployment) (*radixv1.RadixDeployment, error) {
	return radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{ResourceVersion: rd.ResourceVersion})
}

func addRadixDeployment(anyApp string, anyEnv string, anyComponentName string, tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube, radixclient radixclient.Interface, kedaClient kedav2.Interface, prometheusclient prometheusclient.Interface, certClient *certfake.Clientset) *radixv1.RadixDeployment {
	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(anyApp).
		WithEnvironment(anyEnv).
		WithEmptyStatus().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))
	rd, _ := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, radixDeployBuilder)
	return rd
}

func TestObjectUpdated_RemoveOneSecret_SecretIsRemoved(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithSecrets([]string{"a_secret", "another_secret", "a_third_secret"})))
	require.NoError(t, err)
	secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Len(t, secrets.Items, 1)
	anyComponentSecret := getSecretByName(utils.GetComponentSecretName(anyComponentName), secrets)
	assert.NotNil(t, anyComponentSecret, "Component secret is not found")

	// Secret is initially empty but get filled with data from the API
	assert.Len(t, radixmaps.GetKeysFromByteMap(anyComponentSecret.Data), 0, "Component secret data is not as expected")

	// Will emulate that data is set from the API
	anySecretValue := "anySecretValue"
	secretData := make(map[string][]byte)
	secretData["a_secret"] = []byte(anySecretValue)
	secretData["another_secret"] = []byte(anySecretValue)
	secretData["a_third_secret"] = []byte(anySecretValue)

	anyComponentSecret.Data = secretData
	_, err = client.CoreV1().Secrets(envNamespace).Update(context.Background(), anyComponentSecret, metav1.UpdateOptions{})
	require.NoError(t, err)
	// Removing one secret from config and therefore from the deployment
	// should cause it to disappear
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithSecrets([]string{"a_secret", "a_third_secret"})))
	require.NoError(t, err)
	secrets, _ = client.CoreV1().Secrets(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Len(t, secrets.Items, 1)
	anyComponentSecret = getSecretByName(utils.GetComponentSecretName(anyComponentName), secrets)
	assert.True(t, radixutils.ArrayEqualElements([]string{"a_secret", "a_third_secret"}, radixmaps.GetKeysFromByteMap(anyComponentSecret.Data)), "Component secret data is not as expected")
}

func TestHistoryLimit_IsBroken_FixedAmountOfDeployments(t *testing.T) {
	anyAppName := "any-app"
	anyComponentName := "frontend"
	anyEnvironment := "dev"
	anyLimit := 3

	tu, client, kubeUtils, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Current cluster is active cluster
	deploymentHistoryLimitSetter := func(syncer DeploymentSyncer) {
		if s, ok := syncer.(*Deployment); ok {
			newcfg := *s.config
			newcfg.DeploymentSyncer.DeploymentHistoryLimit = anyLimit
			s.config = &newcfg
		}
	}
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	_, err := applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithDeploymentName("firstdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")),
		deploymentHistoryLimitSetter)
	require.NoError(t, err)
	_, err = applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithDeploymentName("seconddeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")),
		deploymentHistoryLimitSetter)
	require.NoError(t, err)
	_, err = applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithDeploymentName("thirddeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")),
		deploymentHistoryLimitSetter)
	require.NoError(t, err)
	_, err = applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithDeploymentName("fourthdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")),
		deploymentHistoryLimitSetter)
	require.NoError(t, err)
	deployments, _ := radixclient.RadixV1().RadixDeployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(deployments.Items), "Number of deployments should match limit")

	assert.False(t, radixDeploymentByNameExists("firstdeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("seconddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("thirddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fourthdeployment", deployments))

	_, err = applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithDeploymentName("fifthdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")),
		deploymentHistoryLimitSetter)
	require.NoError(t, err)
	deployments, _ = radixclient.RadixV1().RadixDeployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(deployments.Items), "Number of deployments should match limit")

	assert.False(t, radixDeploymentByNameExists("firstdeployment", deployments))
	assert.False(t, radixDeploymentByNameExists("seconddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("thirddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fourthdeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fifthdeployment", deployments))

	TeardownTest()
}

func TestMonitoringConfig(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	myAppName := "anyappname"
	myEnvName := "test"

	compNames := []string{
		"withMonitoringConfigAndEnabled",
		"withMonitoringEnabled",
		"withMonitoringConfigAndDisabled",
		"withMonitoringDisabled",
	}
	monitoringConfig := radixv1.MonitoringConfig{PortName: "monitoring", Path: "some/special/path"}
	ports := []radixv1.ComponentPort{
		{Name: "public", Port: 8080},
		{Name: monitoringConfig.PortName, Port: 9001},
		{Name: "super_secure_public_port", Port: 8443},
	}

	serviceMonitorTestFunc := func(t *testing.T, compName string, port radixv1.MonitoringConfig, serviceMonitor *monitoringv1.ServiceMonitor) {
		assert.Equal(t, port.PortName, serviceMonitor.Spec.Endpoints[0].Port)
		assert.Equal(t, port.Path, serviceMonitor.Spec.Endpoints[0].Path)
		assert.Equal(t, fmt.Sprintf("%s-%s-%s", myAppName, myEnvName, compName), serviceMonitor.Spec.JobLabel)
		assert.Len(t, serviceMonitor.Spec.NamespaceSelector.MatchNames, 1)
		assert.Equal(t, fmt.Sprintf("%s-%s", myAppName, myEnvName), serviceMonitor.Spec.NamespaceSelector.MatchNames[0])
		assert.Len(t, serviceMonitor.Spec.Selector.MatchLabels, 1)
		assert.Equal(t, compName, serviceMonitor.Spec.Selector.MatchLabels[kube.RadixComponentLabel])
	}

	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(myAppName).
		WithEnvironment(myEnvName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(compNames[0]).
				WithPorts(ports).
				WithMonitoring(true).
				WithMonitoringConfig(monitoringConfig),
			utils.NewDeployComponentBuilder().
				WithName(compNames[1]).
				WithPorts(ports).
				WithMonitoring(true),
			utils.NewDeployComponentBuilder().
				WithName(compNames[2]).
				WithPorts(ports).
				WithMonitoringConfig(monitoringConfig),
			utils.NewDeployComponentBuilder().
				WithName(compNames[3]).
				WithPorts(ports)))
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(myAppName, myEnvName)
	t.Run("validate service monitors", func(t *testing.T) {
		servicemonitors, _ := prometheusclient.MonitoringV1().ServiceMonitors(envNamespace).List(context.Background(), metav1.ListOptions{})
		assert.Equal(t, 2, len(servicemonitors.Items), "Number of service monitors was not as expected")
		assert.True(t, serviceMonitorByNameExists(compNames[0], servicemonitors), "compName[0] service monitor should exist")
		assert.True(t, serviceMonitorByNameExists(compNames[1], servicemonitors), "compNames[1] service monitor should exist")
		assert.False(t, serviceMonitorByNameExists(compNames[2], servicemonitors), "compNames[2] service monitor should NOT exist")
		assert.False(t, serviceMonitorByNameExists(compNames[3], servicemonitors), "compNames[3] service monitor should NOT exist")

		// serviceMonitor, monitoringConfig, should use monitoringConfig
		serviceMonitor := getServiceMonitorByName(compNames[0], servicemonitors)
		serviceMonitorTestFunc(t, compNames[0], monitoringConfig, serviceMonitor)

		// serviceMonitor, no monitoringConfig, should use first port
		serviceMonitor = getServiceMonitorByName(compNames[1], servicemonitors)
		serviceMonitorTestFunc(t, compNames[1], radixv1.MonitoringConfig{PortName: ports[0].Name}, serviceMonitor)

		// no serviceMonitor, monitoringConfig, should not exist
		serviceMonitor = getServiceMonitorByName(compNames[2], servicemonitors)
		assert.Nil(t, serviceMonitor)

		// no serviceMonitor, no monitoringConfig, should not exist
		serviceMonitor = getServiceMonitorByName(compNames[3], servicemonitors)
		assert.Nil(t, serviceMonitor)
	})
}

func TestObjectUpdated_UpdatePort_DeploymentPodPortSpecIsCorrect(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	var portTestFunc = func(portName string, portNumber int32, ports []corev1.ContainerPort) {
		port := getPortByName(portName, ports)
		assert.NotNil(t, port)
		assert.Equal(t, portNumber, port.ContainerPort)
	}

	// Initial build
	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("env").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("comp").
				WithPort("port1", 8001).
				WithPort("port2", 8002)).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName("job").
				WithSchedulerPort(numbers.Int32Ptr(8080))))
	require.NoError(t, err)

	deployments, _ := kubeclient.AppsV1().Deployments("app-env").List(context.Background(), metav1.ListOptions{})
	comp := getDeploymentByName("comp", deployments.Items)
	assert.Len(t, comp.Spec.Template.Spec.Containers[0].Ports, 2)
	portTestFunc("port1", 8001, comp.Spec.Template.Spec.Containers[0].Ports)
	portTestFunc("port2", 8002, comp.Spec.Template.Spec.Containers[0].Ports)
	job := getDeploymentByName("job", deployments.Items)
	assert.Len(t, job.Spec.Template.Spec.Containers[0].Ports, 1)
	portTestFunc("scheduler-port", 8080, job.Spec.Template.Spec.Containers[0].Ports)

	// Update ports
	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("env").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("comp").
				WithPort("port2", 9002)).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName("job").
				WithSchedulerPort(numbers.Int32Ptr(9090))))
	require.NoError(t, err)

	deployments, _ = kubeclient.AppsV1().Deployments("app-env").List(context.Background(), metav1.ListOptions{})
	comp = getDeploymentByName("comp", deployments.Items)
	assert.Len(t, comp.Spec.Template.Spec.Containers[0].Ports, 1)
	portTestFunc("port2", 9002, comp.Spec.Template.Spec.Containers[0].Ports)
	job = getDeploymentByName("job", deployments.Items)
	assert.Len(t, job.Spec.Template.Spec.Containers[0].Ports, 1)
	portTestFunc("scheduler-port", 9090, job.Spec.Template.Spec.Containers[0].Ports)
}

func TestUseGpuNode(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentName1 := "componentName1"
	componentName2 := "componentName2"
	componentName3 := "componentName3"
	componentName4 := "componentName4"
	jobComponentName := "jobComponentName"

	// Test
	nodeGpu1 := "nvidia-v100"
	nodeGpu2 := "nvidia-v100, nvidia-p100"
	nodeGpu3 := "nvidia-v100, nvidia-p100, -nvidia-k80"
	nodeGpu4 := "nvidia-p100, -nvidia-k80"
	rd, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName1).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithNodeGpu(nodeGpu1),
			utils.NewDeployComponentBuilder().
				WithName(componentName2).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithNodeGpu(nodeGpu2),
			utils.NewDeployComponentBuilder().
				WithName(componentName3).
				WithPort("http", 8082).
				WithPublicPort("http").
				WithNodeGpu(nodeGpu3),
			utils.NewDeployComponentBuilder().
				WithName(componentName4).
				WithPort("http", 8084).
				WithPublicPort("http")).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(jobComponentName).
				WithPort("http", 8085).
				WithNodeGpu(nodeGpu4)))

	assert.NoError(t, err)

	t.Run("has node with gpu1", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName1)
		assert.NotNil(t, component.Node)
		assert.Equal(t, nodeGpu1, component.Node.Gpu)
	})
	t.Run("has node with gpu2", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName2)
		assert.NotNil(t, component.Node)
		assert.Equal(t, nodeGpu2, component.Node.Gpu)
	})
	t.Run("has node with gpu3", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName3)
		assert.NotNil(t, component.Node)
		assert.Equal(t, nodeGpu3, component.Node.Gpu)
	})
	t.Run("has node with no gpu", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName4)
		assert.NotNil(t, component.Node)
		assert.Empty(t, component.Node.Gpu)
	})
	t.Run("job has node with gpu4", func(t *testing.T) {
		t.Parallel()
		jobComponent := rd.GetJobComponentByName(jobComponentName)
		assert.NotNil(t, jobComponent.Node)
		assert.Equal(t, nodeGpu4, jobComponent.Node.Gpu)
	})
}

func TestUseGpuNodeOnDeploy(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentName1 := "componentName1"
	componentName2 := "componentName2"
	componentName3 := "componentName3"
	jobComponentName := "jobComponentName"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	// Test
	gpuNvidiaV100 := "nvidia-v100"
	gpuNvidiaP100 := "nvidia-p100"
	gpuNvidiaK80 := "nvidia-k80"
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName1).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithNodeGpu(gpuNvidiaV100),
			utils.NewDeployComponentBuilder().
				WithName(componentName2).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithNodeGpu(fmt.Sprintf("%s, %s", gpuNvidiaV100, gpuNvidiaP100)),
			utils.NewDeployComponentBuilder().
				WithName(componentName3).
				WithPort("http", 8082).
				WithPublicPort("http").
				WithNodeGpu(fmt.Sprintf("%s, %s, -%s", gpuNvidiaV100, gpuNvidiaP100, gpuNvidiaK80))).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(jobComponentName).
				WithPort("http", 8085).
				WithNodeGpu(fmt.Sprintf("%s, -%s", gpuNvidiaP100, gpuNvidiaK80))))
	require.NoError(t, err)

	t.Run("has node with nvidia-v100", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName1, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.NotNil(t, affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
		nodeSelectorTerms := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		assert.Len(t, nodeSelectorTerms, 1)
		assert.Equal(t, corev1.NodeSelectorRequirement{
			Key:      kube.RadixGpuLabel,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{gpuNvidiaV100},
		}, nodeSelectorTerms[0].MatchExpressions[0])

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 1)
		assert.Equal(t, corev1.Toleration{Key: kube.RadixGpuCountLabel, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])
	})
	t.Run("has node with nvidia-v100, nvidia-p100", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName2, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.NotNil(t, affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
		nodeSelectorTerms := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		assert.Len(t, nodeSelectorTerms, 1)
		assert.Equal(t, corev1.NodeSelectorRequirement{
			Key:      kube.RadixGpuLabel,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{gpuNvidiaV100, gpuNvidiaP100},
		}, nodeSelectorTerms[0].MatchExpressions[0])

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 1)
		assert.Equal(t, corev1.Toleration{Key: kube.RadixGpuCountLabel, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])
	})
	t.Run("has node with nvidia-v100, nvidia-p100, not nvidia-k80", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName3, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.NotNil(t, affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
		nodeSelectorTerms := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		assert.Len(t, nodeSelectorTerms, 1)
		assert.Equal(t, corev1.NodeSelectorRequirement{
			Key:      kube.RadixGpuLabel,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{gpuNvidiaV100, gpuNvidiaP100},
		}, nodeSelectorTerms[0].MatchExpressions[0])

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 1)
		assert.Equal(t, corev1.Toleration{Key: kube.RadixGpuCountLabel, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])
	})
	t.Run("job has node with GPUs, it is JobScheduler component", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), jobComponentName, metav1.GetOptions{})
		expectedAffinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
				{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
				{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(radixv1.RuntimeArchitectureAmd64)}},
			}}}}},
			PodAntiAffinity: &corev1.PodAntiAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 1, PodAffinityTerm: corev1.PodAffinityTerm{TopologyKey: corev1.LabelHostname, LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: kube.RadixAppLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{anyAppName}},
				{Key: kube.RadixComponentLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{jobComponentName}},
			}}}}}},
		}
		assert.Equal(t, expectedAffinity, deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
}

func TestUseGpuNodeCount(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentName1 := "componentName1"
	componentName2 := "componentName2"
	componentName3 := "componentName3"
	componentName4 := "componentName4"
	componentName5 := "componentName5"
	componentName6 := "componentName6"
	jobComponentName := "jobComponentName"

	// Test
	nodeGpuCount1 := "1"
	nodeGpuCount10 := "10"
	nodeGpuCount0 := "0"
	nodeGpuCountMinus1 := "-1"
	nodeGpuCountInvalidTextValue := "invalid-count"
	rd, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName1).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithNodeGpuCount(nodeGpuCount1),
			utils.NewDeployComponentBuilder().
				WithName(componentName2).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithNodeGpuCount(nodeGpuCount10),
			utils.NewDeployComponentBuilder().
				WithName(componentName3).
				WithPort("http", 8082).
				WithPublicPort("http").
				WithNodeGpuCount(nodeGpuCount0),
			utils.NewDeployComponentBuilder().
				WithName(componentName4).
				WithPort("http", 8083).
				WithPublicPort("http").
				WithNodeGpuCount(nodeGpuCountMinus1),
			utils.NewDeployComponentBuilder().
				WithName(componentName5).
				WithPort("http", 8085).
				WithPublicPort("http").
				WithNodeGpuCount(nodeGpuCountInvalidTextValue),
			utils.NewDeployComponentBuilder().
				WithName(componentName6).
				WithPort("http", 8086).
				WithPublicPort("http")).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(jobComponentName).
				WithPort("http", 8087).
				WithNodeGpuCount(nodeGpuCount10)))
	require.NoError(t, err)

	t.Run("has node with gpu-count 1", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName1)
		assert.Equal(t, nodeGpuCount1, component.Node.GpuCount)
	})
	t.Run("has node with gpu-count 10", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName2)
		assert.Equal(t, nodeGpuCount10, component.Node.GpuCount)
	})
	t.Run("has node with gpu-count 0", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName3)
		assert.Equal(t, nodeGpuCount0, component.Node.GpuCount)
	})
	t.Run("has node with gpu-count -1", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName4)
		assert.Equal(t, nodeGpuCountMinus1, component.Node.GpuCount)
	})
	t.Run("has node with invalid value of gpu-count", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName5)
		assert.Equal(t, nodeGpuCountInvalidTextValue, component.Node.GpuCount)
	})
	t.Run("has node with no gpu-count", func(t *testing.T) {
		t.Parallel()
		component := rd.GetComponentByName(componentName6)
		assert.Empty(t, component.Node.GpuCount)
	})
	t.Run("job has node with gpu-count 10 ", func(t *testing.T) {
		t.Parallel()
		jobComponent := rd.GetJobComponentByName(jobComponentName)
		assert.NotNil(t, jobComponent.Node)
		assert.Equal(t, nodeGpuCount10, jobComponent.Node.GpuCount)
	})
}

func TestUseGpuNodeCountOnDeployment(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentName1 := "componentName1"
	componentName2 := "componentName2"
	componentName3 := "componentName3"
	componentName4 := "componentName4"
	componentName5 := "componentName5"
	componentName6 := "componentName6"
	jobComponentName := "jobComponentName"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)

	// Test
	gpuNvidiaV100 := "nvidia-v100"
	nodeGpuCount1 := "1"
	nodeGpuCount10 := "10"
	nodeGpuCount0 := "0"
	nodeGpuCountMinus1 := "-1"
	nodeGpuCountInvalidTextValue := "invalid-count"
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName1).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithNodeGpuCount(nodeGpuCount1),
			utils.NewDeployComponentBuilder().
				WithName(componentName2).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithNodeGpu(gpuNvidiaV100).
				WithNodeGpuCount(nodeGpuCount10),
			utils.NewDeployComponentBuilder().
				WithName(componentName3).
				WithPort("http", 8082).
				WithPublicPort("http").
				WithNodeGpu(gpuNvidiaV100).
				WithNodeGpuCount(nodeGpuCount0),
			utils.NewDeployComponentBuilder().
				WithName(componentName4).
				WithPort("http", 8083).
				WithPublicPort("http").
				WithNodeGpu(gpuNvidiaV100).
				WithNodeGpuCount(nodeGpuCountMinus1),
			utils.NewDeployComponentBuilder().
				WithName(componentName5).
				WithPort("http", 8085).
				WithPublicPort("http").
				WithNodeGpu(gpuNvidiaV100).
				WithNodeGpuCount(nodeGpuCountInvalidTextValue),
			utils.NewDeployComponentBuilder().
				WithName(componentName6).
				WithPort("http", 8086).
				WithPublicPort("http")).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(jobComponentName).
				WithPort("http", 8087).
				WithNodeGpu(gpuNvidiaV100).
				WithNodeGpuCount(nodeGpuCount10)))
	require.NoError(t, err)

	t.Run("missing node.gpu", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName1, metav1.GetOptions{})
		assert.Equal(t, getDefaultComponentAffinityBuilder(componentName1, anyAppName), deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0) // missing node.gpu
	})
	t.Run("has node with gpu-count 10", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName2, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.NotNil(t, affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
		nodeSelectorTerms := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		assert.Len(t, nodeSelectorTerms, 1)
		assert.Len(t, nodeSelectorTerms[0].MatchExpressions, 2)
		assert.Equal(t, corev1.NodeSelectorRequirement{
			Key:      kube.RadixGpuCountLabel,
			Operator: corev1.NodeSelectorOpGt,
			Values:   []string{"9"},
		}, nodeSelectorTerms[0].MatchExpressions[0])
		assert.Equal(t, corev1.NodeSelectorRequirement{
			Key:      kube.RadixGpuLabel,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{gpuNvidiaV100},
		}, nodeSelectorTerms[0].MatchExpressions[1])

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 1)
		assert.Equal(t, corev1.Toleration{Key: kube.RadixGpuCountLabel, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])
	})
	t.Run("has node with gpu-count 0", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName3, metav1.GetOptions{})
		assert.Equal(t, getDefaultComponentAffinityBuilder(componentName3, anyAppName), deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("has node with gpu-count -1", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName4, metav1.GetOptions{})
		assert.Equal(t, getDefaultComponentAffinityBuilder(componentName4, anyAppName), deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("has node with invalid value of gpu-count", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName5, metav1.GetOptions{})
		assert.Equal(t, getDefaultComponentAffinityBuilder(componentName5, anyAppName), deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("has node with no gpu-count", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName6, metav1.GetOptions{})
		assert.Equal(t, getDefaultComponentAffinityBuilder(componentName6, anyAppName), deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("job has node, but pod template of Job Scheduler does not have it", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), jobComponentName, metav1.GetOptions{})
		assert.Equal(t, getDefaultJobComponentAffinityBuilder(anyAppName, jobComponentName), deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
}

func TestUseGpuNodeWithGpuCountOnDeployment(t *testing.T) {
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentName := "componentName"
	jobComponentName := "jobComponentName"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)

	// Test
	gpuNvidiaV100 := "nvidia-v100"
	gpuNvidiaP100 := "nvidia-p100"
	gpuNvidiaK80 := "nvidia-k80"
	nodeGpuCount10 := "10"
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithNodeGpu(fmt.Sprintf("%s, %s, -%s", gpuNvidiaV100, gpuNvidiaP100, gpuNvidiaK80)).
				WithNodeGpuCount(nodeGpuCount10)).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(jobComponentName).
				WithPort("http", 8081).
				WithNodeGpu(fmt.Sprintf("%s, %s, -%s", gpuNvidiaV100, gpuNvidiaP100, gpuNvidiaK80)).
				WithNodeGpuCount(nodeGpuCount10)))
	require.NoError(t, err)

	t.Run("has node with gpu and gpu-count 10", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), componentName, metav1.GetOptions{})
		expectedAffinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
				{Key: kube.RadixGpuCountLabel, Operator: corev1.NodeSelectorOpGt, Values: []string{"9"}},
				{Key: kube.RadixGpuLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{gpuNvidiaV100, gpuNvidiaP100}},
				{Key: kube.RadixGpuLabel, Operator: corev1.NodeSelectorOpNotIn, Values: []string{gpuNvidiaK80}},
			}}}}},
			PodAntiAffinity: &corev1.PodAntiAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 1, PodAffinityTerm: corev1.PodAffinityTerm{TopologyKey: corev1.LabelHostname, LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: kube.RadixAppLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{anyAppName}},
				{Key: kube.RadixComponentLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{componentName}},
			}}}}}},
		}
		assert.Equal(t, expectedAffinity, deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 1)
		assert.Equal(t, corev1.Toleration{Key: kube.RadixGpuCountLabel, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])
	})
	t.Run("job has node, but pod template of Job Scheduler does not have it", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), jobComponentName, metav1.GetOptions{})
		expectedAffinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
				{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
				{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(radixv1.RuntimeArchitectureAmd64)}},
			}}}}},
			PodAntiAffinity: &corev1.PodAntiAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 1, PodAffinityTerm: corev1.PodAffinityTerm{TopologyKey: corev1.LabelHostname, LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: kube.RadixAppLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{anyAppName}},
				{Key: kube.RadixComponentLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{jobComponentName}},
			}}}}}},
		}
		assert.Equal(t, expectedAffinity, deployment.Spec.Template.Spec.Affinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
}

func Test_JobScheduler_ObjectsGarbageCollected(t *testing.T) {
	defer TeardownTest()
	type theoryData struct {
		name             string
		builder          utils.DeploymentBuilder
		expectedJobs     []string
		expectedSecrets  []string
		expectedServices []string
	}

	testTheory := func(theory *theoryData) {
		addJob := func(client kubernetes.Interface, name, namespace, componentName string, isJobSchedulerJob bool) {
			labels := map[string]string{"item-in-test": "true"}

			if strings.TrimSpace(componentName) != "" {
				labels[kube.RadixComponentLabel] = componentName
			}

			if isJobSchedulerJob {
				labels[kube.RadixJobTypeLabel] = kube.RadixJobTypeJobSchedule
			}

			_, err := client.BatchV1().Jobs(namespace).Create(context.Background(),
				&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
					},
				},
				metav1.CreateOptions{})
			require.NoError(t, err)
		}

		addSecret := func(client kubernetes.Interface, name, namespace, componentName string) {
			labels := map[string]string{"item-in-test": "true", kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule}

			if strings.TrimSpace(componentName) != "" {
				labels[kube.RadixComponentLabel] = componentName
			}

			_, err := client.CoreV1().Secrets(namespace).Create(context.Background(),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
					},
				},
				metav1.CreateOptions{})
			require.NoError(t, err)
		}

		addService := func(client kubernetes.Interface, name, namespace, componentName string) {
			labels := map[string]string{"item-in-test": "true", kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule}

			if strings.TrimSpace(componentName) != "" {
				labels[kube.RadixComponentLabel] = componentName
			}

			_, err := client.CoreV1().Services(namespace).Create(context.Background(),
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
					},
				},
				metav1.CreateOptions{})
			require.NoError(t, err)
		}

		t.Run(theory.name, func(t *testing.T) {
			t.Parallel()
			tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)

			// Add jobs used in test
			addJob(client, "dev-job-job1", "app-dev", "job", true)
			addJob(client, "dev-job-job2", "app-dev", "job", true)
			addJob(client, "dev-nonscheduler-job", "app-dev", "job", false)
			addJob(client, "dev-compute-job1", "app-dev", "compute", true)
			addJob(client, "dev-nonscheduler-compute", "app-dev", "compute", false)
			addJob(client, "prod-job-job1", "app-prod", "job", true)

			// Add secrets used in test
			addSecret(client, "dev-job-secret1", "app-dev", "job")
			addSecret(client, "dev-job-secret2", "app-dev", "job")
			addSecret(client, "dev-compute-secret1", "app-dev", "compute")
			addSecret(client, "non-job-secret1", "app-dev", "")
			addSecret(client, "prod-job-secret1", "app-prod", "job")

			// Add services used in test
			addService(client, "dev-job-service1", "app-dev", "job")
			addService(client, "dev-job-service2", "app-dev", "job")
			addService(client, "dev-compute-service1", "app-dev", "compute")
			addService(client, "non-job-service1", "app-dev", "")
			addService(client, "prod-job-service1", "app-prod", "job")

			if _, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, theory.builder); err != nil {
				assert.FailNow(t, fmt.Sprintf("error apply deployment: %v", err))
			}

			// Verify expected jobs
			kubeJobs, _ := client.BatchV1().Jobs("").List(context.Background(), metav1.ListOptions{LabelSelector: "item-in-test=true"})
			assert.Equal(t, len(theory.expectedJobs), len(kubeJobs.Items), "incorrect number of jobs")
			for _, job := range kubeJobs.Items {
				assert.Contains(t, theory.expectedJobs, job.Name, fmt.Sprintf("expected job %s", job.Name))
			}

			// Verify expected secrets
			kubeSecrets, _ := client.CoreV1().Secrets("").List(context.Background(), metav1.ListOptions{LabelSelector: "item-in-test=true"})
			assert.Equal(t, len(theory.expectedSecrets), len(kubeSecrets.Items), "incorrect number of secrets")
			for _, secret := range kubeSecrets.Items {
				assert.Contains(t, theory.expectedSecrets, secret.Name, fmt.Sprintf("expected secrets %s", secret.Name))
			}

			// Verify expected services
			kubeServices, _ := client.CoreV1().Services("").List(context.Background(), metav1.ListOptions{LabelSelector: "item-in-test=true"})
			assert.Equal(t, len(theory.expectedSecrets), len(kubeServices.Items), "incorrect number of services")
			for _, service := range kubeServices.Items {
				assert.Contains(t, theory.expectedServices, service.Name, fmt.Sprintf("expected service %s", service.Name))
			}
		})

	}

	// RD in dev environment with no jobs or components
	deploy := utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("dev").
		WithJobComponents().
		WithComponents()

	testTheory(&theoryData{
		name:             "deployment in dev with no jobs or components",
		builder:          deploy,
		expectedJobs:     []string{"dev-nonscheduler-job", "dev-nonscheduler-compute", "prod-job-job1"},
		expectedSecrets:  []string{"prod-job-secret1", "non-job-secret1"},
		expectedServices: []string{"prod-job-service1", "non-job-service1"},
	})

	// RD in dev environment with job named 'other' and no components
	deploy = utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("dev").
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("other"),
		).
		WithComponents()

	testTheory(&theoryData{
		name:             "deployment in dev with job named 'other' and no components",
		builder:          deploy,
		expectedJobs:     []string{"dev-nonscheduler-job", "dev-nonscheduler-compute", "prod-job-job1"},
		expectedSecrets:  []string{"prod-job-secret1", "non-job-secret1"},
		expectedServices: []string{"prod-job-service1", "non-job-service1"},
	})

	// RD in dev environment with no jobs and component named 'job'
	deploy = utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("dev").
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("job").WithPorts([]radixv1.ComponentPort{{Name: "http", Port: 8000}}),
		)

	testTheory(&theoryData{
		name:             "deployment in dev with no jobs and component named 'job'",
		builder:          deploy,
		expectedJobs:     []string{"dev-nonscheduler-job", "dev-nonscheduler-compute", "prod-job-job1"},
		expectedSecrets:  []string{"prod-job-secret1", "non-job-secret1"},
		expectedServices: []string{"prod-job-service1", "non-job-service1"},
	})

	// RD in dev environment with job named 'job' and no components
	deploy = utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("dev").
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("job").WithPorts([]radixv1.ComponentPort{{Name: "http", Port: 8000}}),
		).
		WithComponents()

	testTheory(&theoryData{
		name:             "deployment in dev with job named 'job' and no components",
		builder:          deploy,
		expectedJobs:     []string{"dev-job-job1", "dev-job-job2", "dev-nonscheduler-job", "dev-nonscheduler-compute", "prod-job-job1"},
		expectedSecrets:  []string{"dev-job-secret1", "dev-job-secret2", "prod-job-secret1", "non-job-secret1"},
		expectedServices: []string{"dev-job-service1", "dev-job-service2", "prod-job-service1", "non-job-service1"},
	})

	// RD in dev environment with job named 'compute' and no components
	deploy = utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("dev").
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("compute").WithPorts([]radixv1.ComponentPort{{Name: "http", Port: 8000}}),
		).
		WithComponents()

	testTheory(&theoryData{
		name:             "deployment in dev with job named 'compute' and no components",
		builder:          deploy,
		expectedJobs:     []string{"dev-compute-job1", "dev-nonscheduler-job", "dev-nonscheduler-compute", "prod-job-job1"},
		expectedSecrets:  []string{"dev-compute-secret1", "prod-job-secret1", "non-job-secret1"},
		expectedServices: []string{"dev-compute-service1", "prod-job-service1", "non-job-service1"},
	})

	// RD in prod environment with jobs named 'compute' and 'job' and no components
	deploy = utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("prod").
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("job").WithPorts([]radixv1.ComponentPort{{Name: "http", Port: 8000}}),
			utils.NewDeployJobComponentBuilder().WithName("compute").WithPorts([]radixv1.ComponentPort{{Name: "http", Port: 8000}}),
		).
		WithComponents()

	testTheory(&theoryData{
		name:             "deployment in prod with jobs named 'compute' and 'job' and no components",
		builder:          deploy,
		expectedJobs:     []string{"dev-job-job1", "dev-job-job2", "dev-nonscheduler-job", "dev-compute-job1", "dev-nonscheduler-compute", "prod-job-job1"},
		expectedSecrets:  []string{"dev-job-secret1", "dev-job-secret2", "dev-compute-secret1", "prod-job-secret1", "non-job-secret1"},
		expectedServices: []string{"dev-job-service1", "dev-job-service2", "dev-compute-service1", "prod-job-service1", "non-job-service1"},
	})
}

func Test_IngressAnnotations_Called(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, _, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	defer os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithComponent(utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithDNSAppAlias(true)).BuildRD()
	_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	annotations1 := ingress.NewMockAnnotationProvider(ctrl)
	annotations1.EXPECT().GetAnnotations(&rd.Spec.Components[0], rd.Namespace).Times(3).Return(map[string]string{"foo": "x"}, nil)
	annotations2 := ingress.NewMockAnnotationProvider(ctrl)
	annotations2.EXPECT().GetAnnotations(&rd.Spec.Components[0], rd.Namespace).Times(3).Return(map[string]string{"bar": "y", "baz": "z"}, nil)

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, []ingress.AnnotationProvider{annotations1, annotations2}, nil, &config.Config{})
	err = syncer.OnSync(context.Background())
	require.NoError(t, err)
	ingresses, _ := kubeclient.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	assert.Len(t, ingresses.Items, 3)
	expected := map[string]string{"bar": "y", "baz": "z", "foo": "x"}

	for _, ingress := range ingresses.Items {
		assert.Equal(t, expected, ingress.GetAnnotations())
	}
}

func Test_IngressAnnotations_ReturnError(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, _, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithComponent(utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http")).BuildRD()
	_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	annotations1 := ingress.NewMockAnnotationProvider(ctrl)
	annotations1.EXPECT().GetAnnotations(&rd.Spec.Components[0], "app-dev").Times(1).Return(nil, errors.New("any error"))

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, []ingress.AnnotationProvider{annotations1}, nil, &config.Config{})
	err = syncer.OnSync(context.Background())
	assert.Error(t, err)
}

func Test_AuxiliaryResourceManagers_Called(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, _, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithComponent(utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http")).BuildRD()
	_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	auxResource := NewMockAuxiliaryResourceManager(ctrl)
	auxResource.EXPECT().GarbageCollect(gomock.Any()).Times(1).Return(nil)
	auxResource.EXPECT().Sync(gomock.Any()).Times(1).Return(nil)

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, nil, []AuxiliaryResourceManager{auxResource}, &config.Config{})
	err = syncer.OnSync(context.Background())
	assert.NoError(t, err)
}

func Test_AuxiliaryResourceManagers_Sync_ReturnErr(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, _, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithComponent(utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http")).BuildRD()
	_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	auxErr := errors.New("an error")
	auxResource := NewMockAuxiliaryResourceManager(ctrl)
	auxResource.EXPECT().GarbageCollect(gomock.Any()).Times(1).Return(nil)
	auxResource.EXPECT().Sync(gomock.Any()).Times(1).Return(auxErr)

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, nil, []AuxiliaryResourceManager{auxResource}, &config.Config{})
	err = syncer.OnSync(context.Background())
	assert.Contains(t, err.Error(), auxErr.Error())
}

func Test_AuxiliaryResourceManagers_GarbageCollect_ReturnErr(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, _, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithComponent(utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http")).BuildRD()
	_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	auxErr := errors.New("an error")
	auxResource := NewMockAuxiliaryResourceManager(ctrl)
	auxResource.EXPECT().GarbageCollect(gomock.Any()).Times(1).Return(auxErr)
	auxResource.EXPECT().Sync(gomock.Any()).Times(0)

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, nil, []AuxiliaryResourceManager{auxResource}, &config.Config{})
	err = syncer.OnSync(context.Background())
	assert.Contains(t, err.Error(), auxErr.Error())
}

func Test_ComponentSynced_VolumeAndMounts(t *testing.T) {
	appName, environment, compName := "app", "dev", "comp"
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.NewDeploymentBuilder().
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithAppName(appName).
					WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName)),
			).
			WithAppName(appName).
			WithEnvironment(environment).
			WithComponent(
				utils.NewDeployComponentBuilder().
					WithName(compName).
					WithVolumeMounts(
						radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlob, Name: "blob", Container: "blobcontainer", Path: "blobpath"},
						radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blobcsi", Storage: "blobcsistorage", Path: "blobcsipath"},
						radixv1.RadixVolumeMount{Type: radixv1.MountTypeAzureFileCsiAzure, Name: "filecsi", Storage: "filecsistorage", Path: "filecsipath"},
					),
			),
	)
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(appName, environment)
	deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.Background(), compName, metav1.GetOptions{})
	require.NotNil(t, deployment)
	assert.Len(t, deployment.Spec.Template.Spec.Volumes, 3, "incorrect number of volumes")
	assert.Len(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, 3, "incorrect number of volumemounts")
}

func Test_JobSynced_VolumeAndMounts(t *testing.T) {
	appName, environment, jobName := "app", "dev", "job"
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.NewDeploymentBuilder().
			WithRadixApplication(
				utils.ARadixApplication().
					WithAppName(appName).
					WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName)),
			).
			WithAppName(appName).
			WithEnvironment(environment).
			WithJobComponent(
				utils.NewDeployJobComponentBuilder().
					WithName(jobName).
					WithVolumeMounts(
						radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlob, Name: "blob", Container: "blobcontainer", Path: "blobpath"},
						radixv1.RadixVolumeMount{Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Name: "blobcsi", Storage: "blobcsistorage", Path: "blobcsipath"},
						radixv1.RadixVolumeMount{Type: radixv1.MountTypeAzureFileCsiAzure, Name: "filecsi", Storage: "filecsistorage", Path: "filecsipath"},
					),
			),
	)
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(appName, environment)
	deploymentList, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: radixlabels.ForJobAuxObject(jobName).String()})
	require.Len(t, deploymentList.Items, 1)
	deployment := deploymentList.Items[0]
	assert.Len(t, deployment.Spec.Template.Spec.Volumes, 3, "incorrect number of volumes")
	assert.Len(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, 0, "incorrect number of volumemounts")
}

func Test_ComponentSynced_SecretRefs(t *testing.T) {
	appName, environment, compName := "app", "dev", "comp"
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.NewDeploymentBuilder().
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithAppName(appName).
					WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName)),
			).
			WithAppName(appName).
			WithEnvironment(environment).
			WithComponent(
				utils.NewDeployComponentBuilder().
					WithName(compName).
					WithSecretRefs(
						radixv1.RadixSecretRefs{AzureKeyVaults: []radixv1.RadixAzureKeyVault{
							{Name: "kv1", Path: radixutils.StringPtr("/mnt/kv1"), Items: []radixv1.RadixAzureKeyVaultItem{{Name: "secret", EnvVar: "SECRET1"}}},
							{Name: "kv2", Path: radixutils.StringPtr("/mnt/kv2"), Items: []radixv1.RadixAzureKeyVaultItem{{Name: "secret", EnvVar: "SECRET2"}}},
						}},
					),
			),
	)
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(appName, environment)
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	require.NotNil(t, deployments)
	require.Len(t, deployments.Items, 1)
	require.Equal(t, compName, deployments.Items[0].Name)
	assert.Len(t, deployments.Items[0].Spec.Template.Spec.Volumes, 2, "incorrect number of volumes")
	assert.Len(t, deployments.Items[0].Spec.Template.Spec.Containers[0].VolumeMounts, 2, "incorrect number of volumemounts")
	assert.True(t, envVariableByNameExistOnDeployment("SECRET1", compName, deployments.Items), "SECRET1 does not exist")
	assert.True(t, envVariableByNameExistOnDeployment("SECRET2", compName, deployments.Items), "SECRET2 does not exist")
}

func Test_JobSynced_SecretRefs(t *testing.T) {
	appName, environment, jobName := "app", "dev", "job"
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.NewDeploymentBuilder().
			WithRadixApplication(
				utils.NewRadixApplicationBuilder().
					WithAppName(appName).
					WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName)),
			).
			WithAppName(appName).
			WithEnvironment(environment).
			WithJobComponent(
				utils.NewDeployJobComponentBuilder().
					WithName(jobName).
					WithSecretRefs(
						radixv1.RadixSecretRefs{AzureKeyVaults: []radixv1.RadixAzureKeyVault{
							{Name: "kv1", Path: radixutils.StringPtr("/mnt/kv1"), Items: []radixv1.RadixAzureKeyVaultItem{{Name: "secret"}}},
							{Name: "kv2", Path: radixutils.StringPtr("/mnt/kv2"), Items: []radixv1.RadixAzureKeyVaultItem{{Name: "secret"}}},
						}},
					),
			),
	)
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(appName, environment)
	allDeployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	jobDeployments := getDeploymentsForRadixComponents(allDeployments.Items)
	jobAuxDeployments := getDeploymentsForRadixJobAux(allDeployments.Items)
	require.Len(t, jobDeployments, 1)
	require.Len(t, jobAuxDeployments, 1)
	require.Equal(t, "job", jobDeployments[0].Name)
	require.Equal(t, "job-aux", jobAuxDeployments[0].Name)
	assert.Len(t, jobAuxDeployments[0].Spec.Template.Spec.Volumes, 2, "number of volumes")
	assert.Len(t, jobAuxDeployments[0].Spec.Template.Spec.Containers[0].VolumeMounts, 2, "number of volume mounts")
	assert.False(t, envVariableByNameExistOnDeployment("SECRET1", jobName, jobDeployments), "SECRET1 exist")
	assert.False(t, envVariableByNameExistOnDeployment("SECRET2", jobName, jobDeployments), "SECRET2 exist")
}

func Test_RestartJobManager_RestartsAuxDeployment(t *testing.T) {
	appName, environment, jobName := "app", "dev", "job"
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	applicationBuilder := utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))
	jobComponentBuilder := utils.NewDeployJobComponentBuilder().WithName(jobName)
	_, err := ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.NewDeploymentBuilder().
			WithRadixApplication(
				applicationBuilder,
			).
			WithAppName(appName).
			WithEnvironment(environment).
			WithJobComponent(
				jobComponentBuilder,
			),
	)
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(appName, environment)
	allDeployments, _ := client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	jobDeployments := getDeploymentsForRadixComponents(allDeployments.Items)
	jobAuxDeployments := getDeploymentsForRadixJobAux(allDeployments.Items)
	require.Len(t, jobDeployments, 1)
	require.Len(t, jobAuxDeployments, 1)
	require.Equal(t, "job", jobDeployments[0].Name)
	require.Equal(t, "job-aux", jobAuxDeployments[0].Name)
	restartTimestamp := "some-time-stamp"
	require.False(t, slice.Any(jobDeployments[0].Spec.Template.Spec.Containers[0].Env, func(envVar corev1.EnvVar) bool {
		return envVar.Name == defaults.RadixRestartEnvironmentVariable && envVar.Value == restartTimestamp
	}), "Not found restart env var in the job deployment")
	require.False(t, slice.Any(jobAuxDeployments[0].Spec.Template.Spec.Containers[0].Env, func(envVar corev1.EnvVar) bool {
		return envVar.Name == defaults.RadixRestartEnvironmentVariable && envVar.Value == restartTimestamp
	}), "Not found restart env var in the job aux deployment")

	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.NewDeploymentBuilder().
			WithRadixApplication(applicationBuilder).
			WithAppName(appName).
			WithEnvironment(environment).
			WithJobComponent(jobComponentBuilder.WithEnvironmentVariable(defaults.RadixRestartEnvironmentVariable, restartTimestamp)),
	)
	require.NoError(t, err)

	allDeployments, _ = client.AppsV1().Deployments(envNamespace).List(context.Background(), metav1.ListOptions{})
	jobDeployments = getDeploymentsForRadixComponents(allDeployments.Items)
	jobAuxDeployments = getDeploymentsForRadixJobAux(allDeployments.Items)
	require.Len(t, jobDeployments, 1)
	require.Len(t, jobAuxDeployments, 1)
	require.Equal(t, "job", jobDeployments[0].Name)
	require.True(t, slice.Any(jobDeployments[0].Spec.Template.Spec.Containers[0].Env, func(envVar corev1.EnvVar) bool {
		return envVar.Name == defaults.RadixRestartEnvironmentVariable && envVar.Value == restartTimestamp
	}), "Not found restart env var in the job deployment")
	require.True(t, slice.Any(jobAuxDeployments[0].Spec.Template.Spec.Containers[0].Env, func(envVar corev1.EnvVar) bool {
		return envVar.Name == defaults.RadixRestartEnvironmentVariable && envVar.Value == restartTimestamp
	}), "Not found restart env var in the job aux deployment")
}

func TestRadixBatch_IsGarbageCollected(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	appName := "app"
	anyEnvironment := "test"
	namespace := utils.GetEnvironmentNamespace(appName, anyEnvironment)

	batchFactory := func(name, componentName string) *radixv1.RadixBatch {
		var lbl labels.Set
		if len(componentName) > 0 {
			lbl = radixlabels.ForComponentName(componentName)
		}

		return &radixv1.RadixBatch{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: lbl,
			},
		}
	}

	_, err := radixclient.RadixV1().RadixBatches(namespace).Create(context.Background(), batchFactory("batch1", "job1"), metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixBatches(namespace).Create(context.Background(), batchFactory("batch2", "job1"), metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixBatches(namespace).Create(context.Background(), batchFactory("batch3", "job2"), metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixBatches(namespace).Create(context.Background(), batchFactory("batch4", "job2"), metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixBatches(namespace).Create(context.Background(), batchFactory("batch5", "job3"), metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixBatches(namespace).Create(context.Background(), batchFactory("batch6", "job4"), metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixBatches(namespace).Create(context.Background(), batchFactory("batch7", ""), metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixBatches("other-ns").Create(context.Background(), batchFactory("batch8", "job1"), metav1.CreateOptions{})
	require.NoError(t, err)

	// Test
	_, err = ApplyDeploymentWithSync(tu, client, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(anyEnvironment).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().WithName("job1"),
			utils.NewDeployJobComponentBuilder().WithName("job2"),
		),
	)
	require.NoError(t, err)
	actualBatches, _ := radixclient.RadixV1().RadixBatches("").List(context.Background(), metav1.ListOptions{})
	actualBatchNames := slice.Map(actualBatches.Items, func(batch radixv1.RadixBatch) string { return batch.GetName() })
	expectedBatchNames := []string{"batch1", "batch2", "batch3", "batch4", "batch7", "batch8"}
	assert.ElementsMatch(t, expectedBatchNames, actualBatchNames)
}

func Test_ExternalDNS_Legacy_ResourcesMigrated(t *testing.T) {
	appName, envName, compName := "anyapp", "anyenv", "anycomp"
	fqdnManual, fqdnAutomation := "app1.example.com", "app2.example.com"
	certDataManual, keyDataManual, certDataAutomation, keyDataAutomation := "manualcert", "manualkey", "automationcert", "automationkey"
	ns := utils.GetEnvironmentNamespace(appName, envName)

	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	t.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	defer TeardownTest()

	// Setup legacy secrets
	legacyManualSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fqdnManual,
			Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: compName, kube.RadixExternalAliasLabel: "true"},
		},
		Data: map[string][]byte{corev1.TLSPrivateKeyKey: []byte(keyDataManual), corev1.TLSCertKey: []byte(certDataManual)},
	}
	_, err := kubeclient.CoreV1().Secrets(ns).Create(context.Background(), &legacyManualSecret, metav1.CreateOptions{})
	require.NoError(t, err)
	legacyAutomationSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fqdnAutomation,
			Labels:      map[string]string{"controller.cert-manager.io/fao": "true"},
			Annotations: map[string]string{"anyannotation": "anyvalue"},
		},
		Data: map[string][]byte{corev1.TLSPrivateKeyKey: []byte(keyDataAutomation), corev1.TLSCertKey: []byte(certDataAutomation)},
	}
	_, err = kubeclient.CoreV1().Secrets(ns).Create(context.Background(), &legacyAutomationSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	// Setup legacy certificate
	legacyCert := cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fqdnAutomation,
			Labels:          map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: compName, kube.RadixExternalAliasLabel: "true"},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Name: fqdnAutomation}},
		},
	}
	_, err = certClient.CertmanagerV1().Certificates(ns).Create(context.Background(), &legacyCert, metav1.CreateOptions{})
	require.NoError(t, err)

	// Setup legacy ingresses
	legacyAutomationIngress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fqdnAutomation,
			Labels:      map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: compName, kube.RadixExternalAliasLabel: "true"},
			Annotations: map[string]string{"radix.equinor.com/external-dns-use-certificate-automation": "true", "cert-manager.io/cluster-issuer": "any", "cert-manager.io/duration": "any", "cert-manager.io/renew-before": "any"},
		},
	}
	_, err = kubeclient.NetworkingV1().Ingresses(ns).Create(context.Background(), &legacyAutomationIngress, metav1.CreateOptions{})
	require.NoError(t, err)

	legacyManualIngress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fqdnManual,
			Labels:      map[string]string{kube.RadixAppLabel: appName, kube.RadixComponentLabel: compName, kube.RadixExternalAliasLabel: "true"},
			Annotations: map[string]string{"radix.equinor.com/external-dns-use-certificate-automation": "false"},
		},
	}
	_, err = kubeclient.NetworkingV1().Ingresses(ns).Create(context.Background(), &legacyManualIngress, metav1.CreateOptions{})
	require.NoError(t, err)

	// Apply RD
	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient,
		utils.ARadixDeployment().
			WithAppName(appName).
			WithEnvironment(envName).
			WithComponents(utils.NewDeployComponentBuilder().WithName(compName).WithPublicPort("http").WithExternalDNS(
				radixv1.RadixDeployExternalDNS{FQDN: fqdnManual, UseCertificateAutomation: false},
				radixv1.RadixDeployExternalDNS{FQDN: fqdnAutomation, UseCertificateAutomation: true},
			)).
			WithJobComponents(),
	)
	require.NoError(t, err)

	// Manual secret: changed labels and annotations
	manualSecret, err := kubeclient.CoreV1().Secrets(ns).Get(context.Background(), fqdnManual, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{kube.RadixAppLabel: appName, kube.RadixExternalAliasFQDNLabel: fqdnManual}, manualSecret.Labels)
	assert.Empty(t, manualSecret.Annotations)
	assert.Equal(t, []byte(keyDataManual), manualSecret.Data[corev1.TLSPrivateKeyKey])
	assert.Equal(t, []byte(certDataManual), manualSecret.Data[corev1.TLSCertKey])

	// Automation secret: retains original labels and annotations as these are controlled by the Certificate resource
	automationSecret, err := kubeclient.CoreV1().Secrets(ns).Get(context.Background(), fqdnAutomation, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, legacyAutomationSecret.Labels, automationSecret.Labels)
	assert.Equal(t, legacyAutomationSecret.Annotations, automationSecret.Annotations)
	assert.Equal(t, []byte(keyDataAutomation), automationSecret.Data[corev1.TLSPrivateKeyKey])
	assert.Equal(t, []byte(certDataAutomation), automationSecret.Data[corev1.TLSCertKey])

	// Automation ingress: annotations removed
	automationIngress, err := kubeclient.NetworkingV1().Ingresses(ns).Get(context.Background(), fqdnAutomation, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, automationIngress.Annotations)

	// Manual ingress: annotations removed
	manualIngress, err := kubeclient.NetworkingV1().Ingresses(ns).Get(context.Background(), fqdnManual, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, manualIngress.Annotations)

	// Automation automationIngress: annotations removed
	cert, err := certClient.CertmanagerV1().Certificates(ns).Get(context.Background(), fqdnAutomation, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{kube.RadixAppLabel: appName, kube.RadixExternalAliasFQDNLabel: fqdnAutomation}, cert.Labels)
	assert.Empty(t, cert.OwnerReferences)
	expectedCertSpec := cmv1.CertificateSpec{
		DNSNames:    []string{fqdnAutomation},
		Duration:    &metav1.Duration{Duration: testConfig.CertificateAutomation.Duration},
		RenewBefore: &metav1.Duration{Duration: testConfig.CertificateAutomation.RenewBefore},
		IssuerRef: v1.ObjectReference{
			Name:  testConfig.CertificateAutomation.ClusterIssuer,
			Kind:  "ClusterIssuer",
			Group: "cert-manager.io",
		},
		SecretName: fqdnAutomation,
		SecretTemplate: &cmv1.CertificateSecretTemplate{
			Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixExternalAliasFQDNLabel: fqdnAutomation},
		},
		PrivateKey: &cmv1.CertificatePrivateKey{RotationPolicy: cmv1.RotationPolicyAlways},
	}
	assert.Equal(t, expectedCertSpec, cert.Spec)
}

func Test_ExternalDNS_ContainsAllResources(t *testing.T) {
	appName, envName := "anyapp", "anyenv"
	fqdnManual1, fqdnManual2 := "foo1.example.com", "foo2.example.com"
	fqdnAutomation1, fqdnAutomation2 := "bar1.example.com", "bar2.example.com"
	adminGroups, readerGroups := []string{"adm1", "adm2"}, []string{"rdr1", "rdr2"}
	ns := utils.GetEnvironmentNamespace(appName, envName)

	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()

	rrBuilder := utils.NewRegistrationBuilder().WithName(appName).WithAdGroups(adminGroups).WithReaderAdGroups(readerGroups)
	raBuilder := utils.NewRadixApplicationBuilder().WithAppName(appName).WithRadixRegistration(rrBuilder)
	rdBuilder := utils.NewDeploymentBuilder().
		WithRadixApplication(raBuilder).
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(utils.NewDeployComponentBuilder().WithName("comp").WithExternalDNS(
			radixv1.RadixDeployExternalDNS{FQDN: fqdnManual1, UseCertificateAutomation: false},
			radixv1.RadixDeployExternalDNS{FQDN: fqdnManual2, UseCertificateAutomation: false},
			radixv1.RadixDeployExternalDNS{FQDN: fqdnAutomation1, UseCertificateAutomation: true},
			radixv1.RadixDeployExternalDNS{FQDN: fqdnAutomation2, UseCertificateAutomation: true},
		)).
		WithJobComponents()

	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
	require.NoError(t, err)

	// Non-automation secrets
	secrets, _ := kubeclient.CoreV1().Secrets(ns).List(context.Background(), metav1.ListOptions{})
	require.ElementsMatch(t, []string{fqdnManual1, fqdnManual2}, getSecretNames(secrets))
	assertSecret := func(fqdn string) {
		secret := getSecretByName(fqdn, secrets)
		assert.Equal(t, map[string]string{kube.RadixAppLabel: appName, kube.RadixExternalAliasFQDNLabel: fqdn}, secret.Labels)
		assert.Equal(t, corev1.SecretTypeTLS, secret.Type)
		assert.Equal(t, map[string][]byte{corev1.TLSCertKey: nil, corev1.TLSPrivateKeyKey: nil}, secret.Data)
	}
	assertSecret(fqdnManual1)
	assertSecret(fqdnManual2)

	// Certificate
	certs, _ := certClient.CertmanagerV1().Certificates(ns).List(context.Background(), metav1.ListOptions{})
	require.ElementsMatch(t, []string{fqdnAutomation1, fqdnAutomation2}, slice.Map(certs.Items, func(c cmv1.Certificate) string { return c.Name }))
	assertCert := func(fqdn string) {
		cert, _ := slice.FindFirst(certs.Items, func(c cmv1.Certificate) bool { return c.Name == fqdn })
		assert.Equal(t, map[string]string{kube.RadixAppLabel: appName, kube.RadixExternalAliasFQDNLabel: fqdn}, cert.Labels)
		assert.Empty(t, cert.OwnerReferences)
		expectedCertSpec := cmv1.CertificateSpec{
			DNSNames:    []string{fqdn},
			Duration:    &metav1.Duration{Duration: testConfig.CertificateAutomation.Duration},
			RenewBefore: &metav1.Duration{Duration: testConfig.CertificateAutomation.RenewBefore},
			IssuerRef: v1.ObjectReference{
				Name:  testConfig.CertificateAutomation.ClusterIssuer,
				Kind:  "ClusterIssuer",
				Group: "cert-manager.io",
			},
			SecretName: fqdn,
			SecretTemplate: &cmv1.CertificateSecretTemplate{
				Labels: map[string]string{kube.RadixAppLabel: appName, kube.RadixExternalAliasFQDNLabel: fqdn},
			},
			PrivateKey: &cmv1.CertificatePrivateKey{RotationPolicy: cmv1.RotationPolicyAlways},
		}
		assert.Equal(t, expectedCertSpec, cert.Spec)
	}
	assertCert(fqdnAutomation1)
	assertCert(fqdnAutomation2)

	// Roles
	roles, _ := kubeclient.RbacV1().Roles(ns).List(context.Background(), metav1.ListOptions{})
	require.Subset(t, getRoleNames(roles), []string{"radix-app-externaldns-adm", "radix-app-externaldns-reader"})
	expectedAdmRules := []rbacv1.PolicyRule{{Verbs: []string{"get", "list", "watch", "update", "patch", "delete"}, APIGroups: []string{""}, Resources: []string{"secrets"}, ResourceNames: []string{fqdnManual1, fqdnManual2}}}
	assert.Equal(t, expectedAdmRules, getRoleByName("radix-app-externaldns-adm", roles).Rules)
	expectedReaderRules := []rbacv1.PolicyRule{{Verbs: []string{"get", "list", "watch"}, Resources: []string{"secrets"}, APIGroups: []string{""}, ResourceNames: []string{fqdnManual1, fqdnManual2}}}
	assert.Equal(t, expectedReaderRules, getRoleByName("radix-app-externaldns-reader", roles).Rules)

	// RoleBindings
	roleBindings, _ := kubeclient.RbacV1().RoleBindings(ns).List(context.Background(), metav1.ListOptions{})
	require.Subset(t, getRoleBindingNames(roleBindings), []string{"radix-app-externaldns-adm", "radix-app-externaldns-reader"})
	assert.Equal(t, rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Name: "radix-app-externaldns-adm", Kind: "Role"}, getRoleBindingByName("radix-app-externaldns-adm", roleBindings).RoleRef)
	assert.ElementsMatch(t,
		slice.Map(adminGroups, func(group string) rbacv1.Subject {
			return rbacv1.Subject{
				Kind: "Group", APIGroup: rbacv1.GroupName, Name: group, Namespace: "",
			}
		}),
		getRoleBindingByName("radix-app-externaldns-adm", roleBindings).Subjects,
	)
	assert.Equal(t, rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Name: "radix-app-externaldns-reader", Kind: "Role"}, getRoleBindingByName("radix-app-externaldns-reader", roleBindings).RoleRef)
	assert.ElementsMatch(t,
		slice.Map(readerGroups, func(group string) rbacv1.Subject {
			return rbacv1.Subject{
				Kind: "Group", APIGroup: rbacv1.GroupName, Name: group, Namespace: "",
			}
		}),
		getRoleBindingByName("radix-app-externaldns-reader", roleBindings).Subjects,
	)
}

func Test_ExternalDNS_RetainSecretData(t *testing.T) {
	appName, envName, fqdn := "anyapp", "anyenv", "app1.example.com"
	keyData, certData := "keydata", "certdata"
	ns := utils.GetEnvironmentNamespace(appName, envName)

	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()

	rdBuilder := utils.ARadixDeployment().
		WithDeploymentName("rd-init").
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(utils.NewDeployComponentBuilder().WithName("comp").
			WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: false})).
		WithJobComponents()

	// Init deployment and set TLS secret data
	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
	require.NoError(t, err)
	tlsSecret, err := kubeclient.CoreV1().Secrets(ns).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)
	tlsSecret.Data[corev1.TLSCertKey] = []byte(certData)
	tlsSecret.Data[corev1.TLSPrivateKeyKey] = []byte(keyData)
	_, err = kubeclient.CoreV1().Secrets(ns).Update(context.Background(), tlsSecret, metav1.UpdateOptions{})
	require.NoError(t, err)

	type testSpec struct {
		componentName string
		useAutomation bool
	}

	tests := []testSpec{
		{componentName: "comp", useAutomation: true},
		{componentName: "comp", useAutomation: false},
		{componentName: "comp2", useAutomation: true},
		{componentName: "comp3", useAutomation: false},
	}
	expectedTlsData := map[string][]byte{corev1.TLSCertKey: []byte(certData), corev1.TLSPrivateKeyKey: []byte(keyData)}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %v", i), func(t *testing.T) {
			rdBuilder.
				WithDeploymentName(fmt.Sprintf("rd%v", i)).
				WithComponents(utils.NewDeployComponentBuilder().WithName(test.componentName).
					WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: test.useAutomation}),
				)
			_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
			require.NoError(t, err)
			tlsSecret, err := kubeclient.CoreV1().Secrets(ns).Get(context.Background(), fqdn, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, expectedTlsData, tlsSecret.Data)
			_, err = certClient.CertmanagerV1().Certificates(ns).Get(context.Background(), fqdn, metav1.GetOptions{})
			if test.useAutomation {
				assert.NoError(t, err)
			} else {
				assert.True(t, kubeerrors.IsNotFound(err))
			}
		})
	}
}

func Test_ExternalDNS_CertificateDurationAndRenewBefore_MinValue(t *testing.T) {
	fqdn := "any.example.com"
	_, kubeclient, kubeUtil, radixclient, _, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithComponent(
		utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: true}),
	).BuildRD()
	_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
	require.NoError(t, err)

	// Duration and RenewBefore not below min values
	cfg := &config.Config{
		CertificateAutomation: certificateconfig.AutomationConfig{
			ClusterIssuer: "anyissuer",
			Duration:      10000 * time.Hour,
			RenewBefore:   1000 * time.Hour,
		}}

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, nil, nil, cfg)
	require.NoError(t, syncer.OnSync(context.Background()))
	cert, _ := certClient.CertmanagerV1().Certificates("app-dev").Get(context.Background(), fqdn, metav1.GetOptions{})
	assert.Equal(t, cfg.CertificateAutomation.Duration, cert.Spec.Duration.Duration)
	assert.Equal(t, cfg.CertificateAutomation.RenewBefore, cert.Spec.RenewBefore.Duration)

	// Duration below min value
	cfg = &config.Config{
		CertificateAutomation: certificateconfig.AutomationConfig{
			ClusterIssuer: "anyissuer",
			Duration:      2159 * time.Hour,
			RenewBefore:   1000 * time.Hour,
		}}

	syncer = NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, nil, nil, cfg)
	require.NoError(t, syncer.OnSync(context.Background()))
	cert, _ = certClient.CertmanagerV1().Certificates("app-dev").Get(context.Background(), fqdn, metav1.GetOptions{})
	assert.Equal(t, 2160*time.Hour, cert.Spec.Duration.Duration)
	assert.Equal(t, cfg.CertificateAutomation.RenewBefore, cert.Spec.RenewBefore.Duration)

	// RenewBefore below min value
	cfg = &config.Config{
		CertificateAutomation: certificateconfig.AutomationConfig{
			ClusterIssuer: "anyissuer",
			Duration:      10000 * time.Hour,
			RenewBefore:   359 * time.Hour,
		}}

	syncer = NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, nil, nil, cfg)
	require.NoError(t, syncer.OnSync(context.Background()))
	cert, _ = certClient.CertmanagerV1().Certificates("app-dev").Get(context.Background(), fqdn, metav1.GetOptions{})
	assert.Equal(t, cfg.CertificateAutomation.Duration, cert.Spec.Duration.Duration)
	assert.Equal(t, 360*time.Hour, cert.Spec.RenewBefore.Duration)
}

func Test_ExternalDNS_ClusterIssuerNotSet(t *testing.T) {
	fqdn := "any.example.com"
	_, kubeclient, kubeUtil, radixclient, _, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()
	rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithComponent(
		utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: true}),
	).BuildRD()
	_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
	require.NoError(t, err)

	// Duration and RenewBefore not below min values
	cfg := &config.Config{
		CertificateAutomation: certificateconfig.AutomationConfig{
			Duration:    10000 * time.Hour,
			RenewBefore: 1000 * time.Hour,
		}}

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, rr, rd, nil, nil, cfg)
	assert.ErrorContains(t, syncer.OnSync(context.Background()), "cluster issuer not set in certificate automation config")
}

func Test_ExternalDNS_GarbageCollectResourceNoLongerInSpec(t *testing.T) {
	appName, envName := "anyapp", "anyenv"
	ns := utils.GetEnvironmentNamespace(appName, envName)

	tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, _, certClient := SetupTest(t)
	defer TeardownTest()

	rdBuilder := utils.ARadixDeployment().
		WithDeploymentName("rd1").
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp").WithExternalDNS(
				radixv1.RadixDeployExternalDNS{FQDN: "app1.example.com", UseCertificateAutomation: false},
				radixv1.RadixDeployExternalDNS{FQDN: "app2.example.com", UseCertificateAutomation: false},
				radixv1.RadixDeployExternalDNS{FQDN: "app3.example.com", UseCertificateAutomation: true},
				radixv1.RadixDeployExternalDNS{FQDN: "app4.example.com", UseCertificateAutomation: true},
			),
		).
		WithJobComponents()
	_, err := ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
	require.NoError(t, err)

	certs, _ := certClient.CertmanagerV1().Certificates(ns).List(context.Background(), metav1.ListOptions{})
	require.Len(t, certs.Items, 2)
	// Simulate that cert-manager creates Certificate secrets
	for _, cert := range certs.Items {
		_, err := kubeclient.CoreV1().Secrets(ns).Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: cert.Spec.SecretName, Labels: cert.Spec.SecretTemplate.Labels},
			Type:       corev1.SecretTypeTLS,
		}, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	secrets, _ := kubeclient.CoreV1().Secrets(ns).List(context.Background(), metav1.ListOptions{})
	require.ElementsMatch(t, []string{"app1.example.com", "app2.example.com", "app3.example.com", "app4.example.com"}, getSecretNames(secrets))

	// Remove app2 and app4 should remove respective TLS secrets and app4 certificate
	rdBuilder.WithDeploymentName("rd2").WithComponents(
		utils.NewDeployComponentBuilder().WithName("comp").WithExternalDNS(
			radixv1.RadixDeployExternalDNS{FQDN: "app1.example.com", UseCertificateAutomation: false},
			radixv1.RadixDeployExternalDNS{FQDN: "app3.example.com", UseCertificateAutomation: true},
		),
	)
	_, err = ApplyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, rdBuilder)
	require.NoError(t, err)
	certs, _ = certClient.CertmanagerV1().Certificates(ns).List(context.Background(), metav1.ListOptions{})
	require.Len(t, certs.Items, 1)
	assert.Equal(t, "app3.example.com", certs.Items[0].Name)
	secrets, _ = kubeclient.CoreV1().Secrets(ns).List(context.Background(), metav1.ListOptions{})
	require.ElementsMatch(t, []string{"app1.example.com", "app3.example.com"}, getSecretNames(secrets))
}

func Test_Deployment_ImagePullSecrets(t *testing.T) {
	tests := map[string]struct {
		rdImagePullSecrets        []corev1.LocalObjectReference
		defaultRegistryAuthSecret string
		expectedImagePullSecrets  []corev1.LocalObjectReference
	}{
		"none defined => empty": {
			rdImagePullSecrets:        nil,
			defaultRegistryAuthSecret: "",
			expectedImagePullSecrets:  nil,
		},
		"rd defined": {
			rdImagePullSecrets:        []corev1.LocalObjectReference{{Name: "secret1"}, {Name: "secret2"}},
			defaultRegistryAuthSecret: "",
			expectedImagePullSecrets:  []corev1.LocalObjectReference{{Name: "secret1"}, {Name: "secret2"}},
		},
		"default defined": {
			rdImagePullSecrets:        nil,
			defaultRegistryAuthSecret: "default-auth",
			expectedImagePullSecrets:  []corev1.LocalObjectReference{{Name: "default-auth"}},
		},
		"both defined": {
			rdImagePullSecrets:        []corev1.LocalObjectReference{{Name: "secret1"}, {Name: "secret2"}},
			defaultRegistryAuthSecret: "default-auth",
			expectedImagePullSecrets:  []corev1.LocalObjectReference{{Name: "secret1"}, {Name: "secret2"}, {Name: "default-auth"}},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, kubeclient, kubeUtil, radixclient, _, promClient, _, certClient := SetupTest(t)
			defer TeardownTest()

			rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
			rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithImagePullSecrets(test.rdImagePullSecrets).
				WithComponents(utils.NewDeployComponentBuilder().WithName("comp")).
				WithJobComponents(utils.NewDeployJobComponentBuilder().WithName("job")).
				BuildRD()
			_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
			require.NoError(t, err)

			cfg := &config.Config{
				RegistryConfig: registry.RegistryConfig{DefaultAuthSecret: test.defaultRegistryAuthSecret},
			}

			syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, promClient, certClient, rr, rd, nil, nil, cfg)
			err = syncer.OnSync(context.Background())
			require.NoError(t, err)
			compDeployment, err := kubeclient.AppsV1().Deployments("app-dev").Get(context.Background(), "comp", metav1.GetOptions{})
			require.NoError(t, err)
			assert.ElementsMatch(t, test.expectedImagePullSecrets, compDeployment.Spec.Template.Spec.ImagePullSecrets, "comp deployment")
			jobDeployment, err := kubeclient.AppsV1().Deployments("app-dev").Get(context.Background(), "job", metav1.GetOptions{})
			require.NoError(t, err)
			assert.ElementsMatch(t, test.expectedImagePullSecrets, jobDeployment.Spec.Template.Spec.ImagePullSecrets, "job component")
		})
	}

}

func parseQuantity(value string) resource.Quantity {
	q, _ := resource.ParseQuantity(value)
	return q
}

func applyDeploymentWithSyncForTestEnv(testEnv *testEnvProps, deploymentBuilder utils.DeploymentBuilder) (*radixv1.RadixDeployment, error) {
	return ApplyDeploymentWithSync(testEnv.testUtil, testEnv.kubeclient, testEnv.kubeUtil, testEnv.radixclient, testEnv.kedaClient, testEnv.prometheusclient, testEnv.certClient, deploymentBuilder)
}

func ApplyDeploymentWithSync(tu *test.Utils, kubeclient kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, kedaClient kedav2.Interface, prometheusclient prometheusclient.Interface, certClient *certfake.Clientset, deploymentBuilder utils.DeploymentBuilder) (*radixv1.RadixDeployment, error) {
	return applyDeploymentWithModifiedSync(tu, kubeclient, kubeUtil, radixclient, kedaClient, prometheusclient, certClient, deploymentBuilder, func(syncer DeploymentSyncer) {})
}

func applyDeploymentWithModifiedSync(tu *test.Utils, kubeclient kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, kedaClient kedav2.Interface, prometheusclient prometheusclient.Interface, certClient *certfake.Clientset, deploymentBuilder utils.DeploymentBuilder, modifySyncer func(syncer DeploymentSyncer)) (*radixv1.RadixDeployment, error) {

	rd, err := tu.ApplyDeployment(context.Background(), deploymentBuilder)
	if err != nil {
		return nil, err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	deploymentSyncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, certClient, radixRegistration, rd, nil, nil, &testConfig)
	modifySyncer(deploymentSyncer)
	err = deploymentSyncer.OnSync(context.Background())
	if err != nil {
		return nil, err
	}

	updatedRD, err := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.Background(), rd.GetName(), metav1.GetOptions{})
	return updatedRD, err
}

func applyDeploymentUpdateWithSync(tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, kedaClient kedav2.Interface, prometheusclient prometheusclient.Interface, certClient *certfake.Clientset, deploymentBuilder utils.DeploymentBuilder) error {
	rd, err := tu.ApplyDeploymentUpdate(deploymentBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(context.Background(), rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment := NewDeploymentSyncer(client, kubeUtil, radixclient, prometheusclient, certClient, radixRegistration, rd, nil, nil, &testConfig)
	err = deployment.OnSync(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func envVariableByNameExistOnDeployment(name, deploymentName string, deployments []appsv1.Deployment) bool {
	return envVariableByNameExist(name, getContainerByName(deploymentName, getDeploymentByName(deploymentName, deployments).Spec.Template.Spec.Containers).Env)
}

func getEnvVariableByNameOnDeployment(kubeclient kubernetes.Interface, name, deploymentName string, deployments []appsv1.Deployment) string {
	deployment := getDeploymentByName(deploymentName, deployments)
	container := getContainerByName(deploymentName, deployment.Spec.Template.Spec.Containers)
	envVarsConfigMapName := kube.GetEnvVarsConfigMapName(container.Name)
	cm, _ := kubeclient.CoreV1().ConfigMaps(deployment.Namespace).Get(context.Background(), envVarsConfigMapName, metav1.GetOptions{})
	return getEnvVariableByName(name, container.Env, cm)
}

func radixDeploymentByNameExists(name string, deployments *radixv1.RadixDeploymentList) bool {
	return getRadixDeploymentByName(name, deployments) != nil
}

func getRadixDeploymentByName(name string, deployments *radixv1.RadixDeploymentList) *radixv1.RadixDeployment {
	for _, deployment := range deployments.Items {
		if deployment.Name == name {
			return &deployment
		}
	}

	return nil
}

func deploymentByNameExists(name string, deployments []appsv1.Deployment) bool {
	return getDeploymentByName(name, deployments) != nil
}

func getDeploymentByName(name string, deployments []appsv1.Deployment) *appsv1.Deployment {
	for _, deployment := range deployments {
		if deployment.GetName() == name {
			return &deployment
		}
	}

	return nil
}

func getContainerByName(name string, containers []corev1.Container) *corev1.Container {
	for _, container := range containers {
		if container.Name == name {
			return &container
		}
	}

	return nil
}

func envVariableByNameExist(name string, envVars []corev1.EnvVar) bool {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return true
		}
	}

	return false
}

func getEnvVariableByName(name string, envVars []corev1.EnvVar, envVarsConfigMap *corev1.ConfigMap) string {
	for _, envVar := range envVars {
		if envVar.Name != name {
			continue
		}
		if envVar.ValueFrom == nil {
			return envVar.Value
		}
		value := envVarsConfigMap.Data[envVar.ValueFrom.ConfigMapKeyRef.Key]
		return value
	}

	return ""
}

func serviceMonitorByNameExists(name string, serviceMonitors *monitoringv1.ServiceMonitorList) bool {
	return getServiceMonitorByName(name, serviceMonitors) != nil
}

func getServiceMonitorByName(name string, serviceMonitors *monitoringv1.ServiceMonitorList) *monitoringv1.ServiceMonitor {
	for _, serviceMonitor := range serviceMonitors.Items {
		if serviceMonitor.Name == name {
			return serviceMonitor
		}
	}

	return nil
}

func getServiceByName(name string, services *corev1.ServiceList) *corev1.Service {
	for _, service := range services.Items {
		if service.Name == name {
			return &service
		}
	}

	return nil
}

func getServiceAccountByName(name string, serviceAccounts *corev1.ServiceAccountList) *corev1.ServiceAccount {
	for _, serviceAccount := range serviceAccounts.Items {
		if serviceAccount.Name == name {
			return &serviceAccount
		}
	}

	return nil
}

func getIngressByName(name string, ingresses *networkingv1.IngressList) *networkingv1.Ingress {
	for _, ingress := range ingresses.Items {
		if ingress.Name == name {
			return &ingress
		}
	}

	return nil
}

func ingressByNameExists(name string, ingresses *networkingv1.IngressList) bool {
	return getIngressByName(name, ingresses) != nil
}

func getRoleByName(name string, roles *rbacv1.RoleList) *rbacv1.Role {
	for _, role := range roles.Items {
		if role.Name == name {
			return &role
		}
	}

	return nil
}

func getRoleNames(roles *rbacv1.RoleList) []string {
	if roles == nil {
		return nil
	}
	return slice.Map(roles.Items, func(r rbacv1.Role) string { return r.GetName() })
}

func getSecretByName(name string, secrets *corev1.SecretList) *corev1.Secret {
	for _, secret := range secrets.Items {
		if secret.Name == name {
			return &secret
		}
	}

	return nil
}

func secretByNameExists(name string, secrets *corev1.SecretList) bool {
	return getSecretByName(name, secrets) != nil
}

func getSecretNames(secrets *corev1.SecretList) []string {
	if secrets == nil {
		return nil
	}
	return slice.Map(secrets.Items, func(s corev1.Secret) string { return s.Name })
}

func getRoleBindingByName(name string, roleBindings *rbacv1.RoleBindingList) *rbacv1.RoleBinding {
	for _, roleBinding := range roleBindings.Items {
		if roleBinding.Name == name {
			return &roleBinding
		}
	}

	return nil
}

func getRoleBindingNames(roleBindings *rbacv1.RoleBindingList) []string {
	if roleBindings == nil {
		return nil
	}
	return slice.Map(roleBindings.Items, func(r rbacv1.RoleBinding) string { return r.GetName() })
}

func getPortByName(name string, ports []corev1.ContainerPort) *corev1.ContainerPort {
	for _, port := range ports {
		if port.Name == name {
			return &port
		}
	}
	return nil
}

func getDefaultComponentAffinityBuilder(componentName string, appName string) *corev1.Affinity {
	return &corev1.Affinity{NodeAffinity: getLinuxAmd64NodeAffinity(), PodAntiAffinity: getComponentPodAntiAffinity(appName, componentName)}
}

func getDefaultJobComponentAffinityBuilder(appName string, jobComponentName string) *corev1.Affinity {
	return &corev1.Affinity{NodeAffinity: getLinuxAmd64NodeAffinity(), PodAntiAffinity: getComponentPodAntiAffinity(appName, jobComponentName)}
}

func getComponentPodAntiAffinity(anyAppName string, componentName string) *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 1, PodAffinityTerm: corev1.PodAffinityTerm{TopologyKey: corev1.LabelHostname, LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: kube.RadixAppLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{anyAppName}},
		{Key: kube.RadixComponentLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{componentName}},
	}}}}}}
}

func getLinuxAmd64NodeAffinity() *corev1.NodeAffinity {
	return &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorArchitecture}},
	}}}}}
}
