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

	radixutils "github.com/equinor/radix-common/utils"
	radixmaps "github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config"
	certificateconfig "github.com/equinor/radix-operator/pkg/apis/config/certificate"
	"github.com/equinor/radix-operator/pkg/apis/config/deployment"
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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const testClusterName = "AnyClusterName"
const dnsZone = "dev.radix.equinor.com"
const anyContainerRegistry = "any.container.registry"
const testEgressIps = "0.0.0.0"

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

func setupTest(t *testing.T) (*test.Utils, *kubefake.Clientset, *kube.Kube, *radixfake.Clientset, *prometheusfake.Clientset, *secretproviderfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	prometheusClient := prometheusfake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(
		kubeclient,
		radixClient,
		secretProviderClient,
	)
	handlerTestUtils := test.NewTestUtils(kubeclient, radixClient, secretProviderClient)
	err := handlerTestUtils.CreateClusterPrerequisites(testClusterName, testEgressIps, "anysubid")
	require.NoError(t, err)
	return &handlerTestUtils, kubeclient, kubeUtil, radixClient, prometheusClient, secretProviderClient
}

func teardownTest() {
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
	defer teardownTest()
	commitId := string(uuid.NewUUID())
	const componentNameApp = "app"
	for _, componentsExist := range []bool{true, false} {
		testScenario := utils.TernaryString(componentsExist, "Updating deployment", "Creating deployment")

		tu, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		defer teardownTest()
		_ = os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")

		t.Run("Test Suite", func(t *testing.T) {
			aRadixRegistrationBuilder := utils.ARadixRegistration()
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
							WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "updated_some.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "updated_another.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "updated_external.alias.com", UseCertificateAutomation: true}).
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
				_, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, existingRadixDeploymentBuilder)
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
							WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: "some.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "another.alias.com"}, radixv1.RadixDeployExternalDNS{FQDN: "external.alias.com", UseCertificateAutomation: true}).
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
				_, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, aRadixDeploymentBuilder)
				assert.NoError(t, err)
			}

			envNamespace := utils.GetEnvironmentNamespace(appName, environment)

			t.Run(fmt.Sprintf("%s: validate deploy", testScenario), func(t *testing.T) {
				t.Parallel()
				allDeployments, _ := kubeclient.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
				deployments := getDeploymentsForRadixComponents(allDeployments.Items)
				assert.Equal(t, 3, len(deployments), "Number of deployments wasn't as expected")
				assert.Equal(t, componentNameApp, getDeploymentByName(componentNameApp, deployments).Name, "app deployment not there")

				if !componentsExist {
					assert.Equal(t, int32(4), *getDeploymentByName(componentNameApp, deployments).Spec.Replicas, "number of replicas was unexpected")

				} else {
					assert.Equal(t, int32(2), *getDeploymentByName(componentNameApp, deployments).Spec.Replicas, "number of replicas was unexpected")

				}

				pdbs, _ := kubeclient.PolicyV1().PodDisruptionBudgets(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.Equal(t, 1, len(pdbs.Items))
				assert.Equal(t, "app", pdbs.Items[0].Spec.Selector.MatchLabels[kube.RadixComponentLabel])
				assert.Equal(t, int32(1), pdbs.Items[0].Spec.MinAvailable.IntVal)

				assert.Equal(t, 13, len(getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Env), "number of environment variables was unexpected for component. It should contain default and custom")
				assert.Equal(t, anyContainerRegistry, getEnvVariableByNameOnDeployment(kubeclient, defaults.ContainerRegistryEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, dnsZone, getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixDNSZoneEnvironmentVariable, componentNameApp, deployments))
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

				expectedStartegy := appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
						MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
					},
				}
				for _, deployment := range deployments {
					assert.Equal(t, expectedStartegy, deployment.Spec.Strategy)
				}
			})

			t.Run(fmt.Sprintf("%s: validate hpa", testScenario), func(t *testing.T) {
				t.Parallel()
				hpas, _ := kubeclient.AutoscalingV2().HorizontalPodAutoscalers(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.Equal(t, 0, len(hpas.Items), "Number of horizontal pod autoscaler wasn't as expected")
			})

			t.Run(fmt.Sprintf("%s: validate service", testScenario), func(t *testing.T) {
				t.Parallel()
				services, _ := kubeclient.CoreV1().Services(envNamespace).List(context.TODO(), metav1.ListOptions{})
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
				secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})

				if !componentsExist {
					assert.Equal(t, 5, len(secrets.Items), "Number of secrets was not according to spec")
				} else {
					assert.Equal(t, 3, len(secrets.Items), "Number of secrets was not according to spec")
				}

				componentSecretName := utils.GetComponentSecretName(componentNameRadixQuote)
				assert.True(t, secretByNameExists(componentSecretName, secrets), "Component secret is not as expected")

				// Exists due to external DNS, even though this is not active cluster
				if !componentsExist {
					assert.True(t, secretByNameExists("some.alias.com", secrets), "TLS certificate for external alias is not properly defined")
					assert.True(t, secretByNameExists("another.alias.com", secrets), "TLS certificate for second external alias is not properly defined")
				} else {
					assert.True(t, secretByNameExists("updated_some.alias.com", secrets), "TLS certificate for external alias is not properly defined")
					assert.True(t, secretByNameExists("updated_another.alias.com", secrets), "TLS certificate for second external alias is not properly defined")
				}

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
				serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
			})

			t.Run(fmt.Sprintf("%s: validate roles", testScenario), func(t *testing.T) {
				t.Parallel()
				roles, _ := kubeclient.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.ElementsMatch(t, []string{"radix-app-adm-radixquote", "radix-app-adm-app", "radix-app-reader-radixquote", "radix-app-reader-app"}, getRoleNames(roles))
			})

			t.Run(fmt.Sprintf("%s validate rolebindings", testScenario), func(t *testing.T) {
				t.Parallel()
				rolebindings, _ := kubeclient.RbacV1().RoleBindings(envNamespace).List(context.TODO(), metav1.ListOptions{})

				assert.ElementsMatch(t, []string{"radix-app-adm-radixquote", "radix-app-adm-app", "radix-app-reader-radixquote", "radix-app-reader-app"}, getRoleBindingNames(rolebindings))
				assert.Equal(t, 1, len(getRoleBindingByName("radix-app-adm-radixquote", rolebindings).Subjects), "Number of rolebinding subjects was not as expected")
			})

			t.Run(fmt.Sprintf("%s: validate networkpolicy", testScenario), func(t *testing.T) {
				t.Parallel()
				np, _ := kubeclient.NetworkingV1().NetworkPolicies(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.Equal(t, 4, len(np.Items), "Number of networkpolicy was not expected")
			})
		})
	}
}

func TestObjectSynced_MultiJob_ContainsAllElements(t *testing.T) {
	const jobSchedulerImage = "radix-job-scheduler:latest"
	defer teardownTest()
	commitId := string(uuid.NewUUID())

	for _, jobsExist := range []bool{false, true} {
		testScenario := utils.TernaryString(jobsExist, "Updating deployment", "Creating deployment")

		tu, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")
		os.Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, jobSchedulerImage)

		t.Run("Test Suite", func(t *testing.T) {
			aRadixRegistrationBuilder := utils.ARadixRegistration()
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
							WithAlwaysPullImageOnDeploy(false),
					).
					WithComponents()
				_, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, existingRadixDeploymentBuilder)
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
							WithAlwaysPullImageOnDeploy(false),
						utils.NewDeployJobComponentBuilder().
							WithName(jobName2).WithSchedulerPort(&schedulerPortCreate),
					).
					WithComponents()

				// Test
				_, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, aRadixDeploymentBuilder)
				assert.NoError(t, err)
			}

			envNamespace := utils.GetEnvironmentNamespace(appName, environment)

			t.Run(fmt.Sprintf("%s: validate deploy", testScenario), func(t *testing.T) {
				allDeployments, _ := kubeclient.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
				deployments := getDeploymentsForRadixComponents(allDeployments.Items)
				jobAuxDeployments := getDeploymentsForRadixJobAux(allDeployments.Items)
				if jobsExist {
					assert.Equal(t, 1, len(deployments), "Number of deployments wasn't as expected")
					assert.Equal(t, 1, len(jobAuxDeployments), "Unexpected job aux deployments component amount")

				} else {
					assert.Equal(t, 2, len(deployments), "Number of deployments wasn't as expected")
					assert.Equal(t, 2, len(jobAuxDeployments), "Unexpected job aux deployments component amount")
				}

				assert.Equal(t, jobName, getDeploymentByName(jobName, deployments).Name, "app deployment not there")
				assert.Equal(t, int32(1), *getDeploymentByName(jobName, deployments).Spec.Replicas, "number of replicas was unexpected")

				envVars := getContainerByName(jobName, getDeploymentByName(jobName, deployments).Spec.Template.Spec.Containers).Env
				assert.Equal(t, 14, len(envVars), "number of environment variables was unexpected for component. It should contain default and custom")
				assert.Equal(t, "a_value", getEnvVariableByNameOnDeployment(kubeclient, "a_variable", jobName, deployments))
				assert.Equal(t, anyContainerRegistry, getEnvVariableByNameOnDeployment(kubeclient, defaults.ContainerRegistryEnvironmentVariable, jobName, deployments))
				assert.Equal(t, dnsZone, getEnvVariableByNameOnDeployment(kubeclient, defaults.RadixDNSZoneEnvironmentVariable, jobName, deployments))
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
				}
			})

			t.Run(fmt.Sprintf("%s: validate hpa", testScenario), func(t *testing.T) {
				hpas, _ := kubeclient.AutoscalingV2().HorizontalPodAutoscalers(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.Equal(t, 0, len(hpas.Items), "Number of horizontal pod autoscaler wasn't as expected")
			})

			t.Run(fmt.Sprintf("%s: validate service", testScenario), func(t *testing.T) {
				services, _ := kubeclient.CoreV1().Services(envNamespace).List(context.TODO(), metav1.ListOptions{})
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
				secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})

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
				serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
			})

			t.Run(fmt.Sprintf("%s: validate roles", testScenario), func(t *testing.T) {
				t.Parallel()
				roles, _ := kubeclient.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.ElementsMatch(t, []string{"radix-app-adm-job", "radix-app-reader-job"}, getRoleNames(roles))
			})

			t.Run(fmt.Sprintf("%s validate rolebindings", testScenario), func(t *testing.T) {
				rolebindings, _ := kubeclient.RbacV1().RoleBindings(envNamespace).List(context.TODO(), metav1.ListOptions{})
				assert.ElementsMatch(t, []string{"radix-app-adm-job", "radix-app-reader-job", defaults.RadixJobSchedulerRoleName}, getRoleBindingNames(rolebindings))
				assert.Equal(t, 1, len(getRoleBindingByName("radix-app-adm-job", rolebindings).Subjects), "Number of rolebinding subjects was not as expected")
			})

			t.Run(fmt.Sprintf("%s: validate networkpolicy", testScenario), func(t *testing.T) {
				np, _ := kubeclient.NetworkingV1().NetworkPolicies(envNamespace).List(context.TODO(), metav1.ListOptions{})
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
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
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

func TestObjectSynced_MultiComponent_ActiveCluster_ContainsAllAliasesAndSupportingObjects(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
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
	assert.Equal(t, map[string]string{kube.RadixExternalDNSUseCertificateAutomationAnnotation: "false"}, externalDNS1.Annotations)
	assert.Equal(t, "external1.alias.com", externalDNS1.Spec.Rules[0].Host, "App should have an external alias")

	externalDNS2 := getIngressByName("external2.alias.com", ingresses)
	assert.Equal(t, int32(8080), externalDNS2.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, externalDNS2.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", externalDNS2.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Empty(t, externalDNS2.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", externalDNS2.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, map[string]string{kube.RadixExternalDNSUseCertificateAutomationAnnotation: "false"}, externalDNS2.Annotations)
	assert.Equal(t, "external2.alias.com", externalDNS2.Spec.Rules[0].Host, "App should have an external alias")

	externalDNS3 := getIngressByName("external3.alias.com", ingresses)
	assert.Equal(t, int32(8080), externalDNS3.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
	assert.Empty(t, externalDNS3.Labels[kube.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", externalDNS3.Labels[kube.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Empty(t, externalDNS3.Labels[kube.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", externalDNS3.Labels[kube.RadixComponentLabel], "Ingress should have the corresponding component")
	expectedAnnotations := map[string]string{
		kube.RadixExternalDNSUseCertificateAutomationAnnotation: "true",
		"cert-manager.io/cluster-issuer":                        testConfig.CertificateAutomation.ClusterIssuer,
		"cert-manager.io/duration":                              testConfig.CertificateAutomation.Duration.String(),
		"cert-manager.io/renew-before":                          testConfig.CertificateAutomation.RenewBefore.String(),
	}
	assert.Equal(t, expectedAnnotations, externalDNS3.Annotations)
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

	roles, _ := client.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.ElementsMatch(t, []string{"radix-app-adm-app", "radix-app-reader-app"}, getRoleNames(roles))

	appAdmAppRole := getRoleByName("radix-app-adm-app", roles)
	assert.Equal(t, "secrets", appAdmAppRole.Rules[0].Resources[0], "Expected role radix-app-adm-app should be able to access secrets")
	assert.ElementsMatch(t, []string{"external1.alias.com", "external2.alias.com"}, appAdmAppRole.Rules[0].ResourceNames)

	secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.True(t, secretByNameExists("external1.alias.com", secrets), "TLS certificate for external alias is not properly defined")
	assert.True(t, secretByNameExists("external2.alias.com", secrets), "TLS certificate for second external alias is not properly defined")

	assert.Equal(t, corev1.SecretType(corev1.SecretTypeTLS), getSecretByName("external1.alias.com", secrets).Type, "TLS certificate for external alias is not properly defined type")
	assert.Equal(t, corev1.SecretType(corev1.SecretTypeTLS), getSecretByName("external2.alias.com", secrets).Type, "TLS certificate for external alias is not properly defined type")

	rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.ElementsMatch(t, []string{"radix-app-adm-app", "radix-app-reader-app"}, getRoleBindingNames(rolebindings))
}

func TestObjectSynced_ServiceAccountSettingsAndRbac(t *testing.T) {
	defer teardownTest()
	// Test
	t.Run("app with component use default SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})

	t.Run("app with component using identity use custom SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		appName, envName, componentName, clientId, newClientId := "any-app", "any-env", "any-component", "any-client-id", "new-client-id"

		// Deploy component with Azure identity must create custom SA
		_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		sa := serviceAccounts.Items[0]
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), sa.Name)
		assert.Equal(t, map[string]string{"azure.workload.identity/client-id": clientId}, sa.Annotations)
		assert.Equal(t, map[string]string{kube.RadixComponentLabel: componentName, kube.IsServiceAccountForComponent: "true", "azure.workload.identity/use": "true"}, sa.Labels)

		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
		assert.Equal(t, "true", expectedDeployments[0].Spec.Template.Labels["azure.workload.identity/use"])

		// Deploy component with new Azure identity must update SA
		_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: newClientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		sa = serviceAccounts.Items[0]
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), sa.Name)
		assert.Equal(t, map[string]string{"azure.workload.identity/client-id": newClientId}, sa.Annotations)
		assert.Equal(t, map[string]string{kube.RadixComponentLabel: componentName, kube.IsServiceAccountForComponent: "true", "azure.workload.identity/use": "true"}, sa.Labels)

		deployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments = getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
		assert.Equal(t, "true", expectedDeployments[0].Spec.Template.Labels["azure.workload.identity/use"])

		// Redploy component without Azure identity should delete custom SA
		_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(utils.NewDeployComponentBuilder().WithName(componentName)).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")

		deployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments = getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
		_, hasLabel := expectedDeployments[0].Spec.Template.Labels["azure.workload.identity/use"]
		assert.False(t, hasLabel)
	})

	t.Run("app with component using identity fails if SA exist with missing is-service-account-for-component label", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		appName, envName, componentName, clientId := "any-app", "any-env", "any-component", "any-client-id"
		_, err := client.CoreV1().ServiceAccounts("any-app-any-env").Create(
			context.Background(),
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: utils.GetComponentServiceAccountName(componentName), Labels: map[string]string{kube.RadixComponentLabel: componentName}}},
			metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		appName, envName, componentName, clientId := "any-app", "any-env", "any-component", "any-client-id"
		_, err := client.CoreV1().ServiceAccounts("any-app-any-env").Create(
			context.Background(),
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: utils.GetComponentServiceAccountName(componentName), Labels: map[string]string{kube.RadixComponentLabel: componentName, kube.IsServiceAccountForComponent: "true", "any-other-label": "any-value"}}},
			metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		sa := serviceAccounts.Items[0]
		assert.Equal(t, utils.GetComponentServiceAccountName(componentName), sa.Name)
		assert.Equal(t, map[string]string{"azure.workload.identity/client-id": clientId}, sa.Annotations)
		assert.Equal(t, map[string]string{kube.RadixComponentLabel: componentName, kube.IsServiceAccountForComponent: "true", "azure.workload.identity/use": "true"}, sa.Labels)
	})

	t.Run("component removed, custom SA is garbage collected", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		appName, envName, componentName, clientId, anyOtherServiceAccountName := "any-app", "any-env", "any-component", "any-client-id", "any-other-serviceaccount"

		// A service account that must not be deleted
		_, err := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).Create(
			context.Background(),
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: anyOtherServiceAccountName, Labels: map[string]string{kube.RadixComponentLabel: "anything"}}},
			metav1.CreateOptions{})
		require.NoError(t, err)
		// Deploy component with Azure identity must create custom SA
		_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 3, len(serviceAccounts.Items), "Number of service accounts was not expected")

		// Redploy component without Azure identity should delete custom SA
		_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(componentName).
					WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: clientId}}),
			).
			WithJobComponents().
			WithAppName(appName).
			WithEnvironment(envName))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace(appName, envName)).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 2, len(serviceAccounts.Items), "Number of service accounts was not expected")
		assert.NotNil(t, getServiceAccountByName(utils.GetComponentServiceAccountName(componentName), serviceAccounts))
		assert.NotNil(t, getServiceAccountByName(anyOtherServiceAccountName, serviceAccounts))
	})

	t.Run("app with job use radix-job-scheduler SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, utils.BoolPtr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

	})

	// Test
	t.Run("app from component to job and back to component", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)

		// Initial deployment, app is a component
		_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("comp1")).
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
		allDeployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(allDeployments.Items)
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

		// Change app to be a job
		_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().WithName("job1")).
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		allDeployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(),
			metav1.ListOptions{})
		expectedJobDeployments := getDeploymentsForRadixComponents(allDeployments.Items)
		assert.Equal(t, 1, len(expectedJobDeployments))
		assert.Equal(t, utils.BoolPtr(true), expectedJobDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, expectedJobDeployments[0].Spec.Template.Spec.ServiceAccountName)
		expectedJobAuxDeployments := getDeploymentsForRadixJobAux(allDeployments.Items)
		assert.Equal(t, 1, len(expectedJobAuxDeployments))
		assert.Equal(t, utils.BoolPtr(false), expectedJobAuxDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedJobAuxDeployments[0].Spec.Template.Spec.ServiceAccountName)

		// And change app back to a component
		_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("comp1")).
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		allDeployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments = getDeploymentsForRadixComponents(allDeployments.Items)
		assert.Equal(t, 1, len(expectedDeployments))
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaultServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})

	t.Run("webhook runs as radix-github-webhook SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("radix-github-webhook").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, utils.BoolPtr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixGithubWebhookServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

	})

	t.Run("radix-api runs as radix-api SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
		_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("radix-api").
			WithEnvironment("test"))
		require.NoError(t, err)
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-api", "test")).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("radix-api", "test")).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, utils.BoolPtr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixAPIServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})
}

func TestObjectSynced_MultiComponentWithSameName_ContainsOneComponent(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
	assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")

	services, _ := client.CoreV1().Services(envNamespace).List(context.TODO(), metav1.ListOptions{})
	expectedServices := getServicesForRadixComponents(&services.Items)
	assert.Equal(t, 1, len(expectedServices), "Number of services wasn't as expected")

	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Number of ingresses was not according to public components")
}

func TestConfigMap_IsGarbageCollected(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	anyEnvironment := "test"
	namespace := utils.GetEnvironmentNamespace(appName, anyEnvironment)

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("somecomponentname").
				WithEnvironmentVariables(nil).
				WithSecrets(nil),
			utils.NewDeployComponentBuilder().
				WithName(componentName).
				WithEnvironmentVariables(nil).
				WithSecrets(nil)),
	)
	require.NoError(t, err)

	// check that config maps with env vars and env vars metadata were created
	envVarCm, err := kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsConfigMapName(componentName))
	assert.NoError(t, err)
	envVarMetadataCm, err := kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
	assert.NoError(t, err)
	assert.NotNil(t, envVarCm)
	assert.NotNil(t, envVarMetadataCm)
	envVarCms, err := kubeUtil.ListEnvVarsConfigMaps(namespace)
	assert.NoError(t, err)
	assert.Len(t, envVarCms, 2)
	envVarMetadataCms, err := kubeUtil.ListEnvVarsMetadataConfigMaps(namespace)
	assert.NoError(t, err)
	assert.Len(t, envVarMetadataCms, 2)

	// delete 2nd component
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("somecomponentname").
				WithEnvironmentVariables(nil).
				WithSecrets(nil)),
	)
	require.NoError(t, err)

	// check that config maps were garbage collected for the component we just deleted
	envVarCm, err = kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsConfigMapName(componentName))
	assert.Error(t, err)
	envVarMetadataCm, err = kubeUtil.GetConfigMap(namespace, kube.GetEnvVarsMetadataConfigMapName(componentName))
	assert.Error(t, err)
	assert.Nil(t, envVarCm)
	assert.Nil(t, envVarMetadataCm)
	envVarCms, err = kubeUtil.ListEnvVarsConfigMaps(namespace)
	assert.NoError(t, err)
	assert.Len(t, envVarCms, 1)
	envVarMetadataCms, err = kubeUtil.ListEnvVarsMetadataConfigMaps(namespace)
	assert.NoError(t, err)
	assert.Len(t, envVarMetadataCms, 1)
}

func TestObjectSynced_NoEnvAndNoSecrets_ContainsDefaultEnvVariables(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	anyEnvironment := "test"
	commitId := string(uuid.NewUUID())

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
		container := deployments.Items[0].Spec.Template.Spec.Containers[0]
		cm, _ := client.CoreV1().ConfigMaps(envNamespace).Get(context.TODO(), kube.GetEnvVarsConfigMapName(container.Name), metav1.GetOptions{})

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
		assert.Equal(t, anyContainerRegistry, getEnvVariableByName(defaults.ContainerRegistryEnvironmentVariable, templateSpecEnv, nil))
		assert.Equal(t, dnsZone, getEnvVariableByName(defaults.RadixDNSZoneEnvironmentVariable, templateSpecEnv, cm))
		assert.Equal(t, testClusterName, getEnvVariableByName(defaults.ClusternameEnvironmentVariable, templateSpecEnv, cm))
		assert.Equal(t, anyEnvironment, getEnvVariableByName(defaults.EnvironmentnameEnvironmentVariable, templateSpecEnv, cm))
		assert.Equal(t, "app", getEnvVariableByName(defaults.RadixAppEnvironmentVariable, templateSpecEnv, cm))
		assert.Equal(t, "component", getEnvVariableByName(defaults.RadixComponentEnvironmentVariable, templateSpecEnv, cm))
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 0, len(secrets.Items), "Should have no secrets")
	})
}

func TestObjectSynced_WithLabels_LabelsAppliedToDeployment(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient,
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
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, "master", deployments.Items[0].Annotations[kube.RadixBranchAnnotation])
		assert.Equal(t, "4faca8595c5283a9d0f17a623b9255a0d9866a2e", deployments.Items[0].Labels["radix-commit"])
	})
}

func TestObjectSynced_NotLatest_DeploymentIsIgnored(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()

	// Test
	now := time.Now().UTC()
	var firstUID, secondUID types.UID

	firstUID = "fda3d224-3115-11e9-b189-06c15a8f2fbb"
	secondUID = "5a8f2fbb-3115-11e9-b189-06c1fda3d224"

	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, firstUID, deployments.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	services, _ := client.CoreV1().Services(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, firstUID, services.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, firstUID, ingresses.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	time.Sleep(1 * time.Millisecond)
	// This is one second newer deployment
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	services, _ = client.CoreV1().Services(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
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

	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, rdBuilder)
	require.NoError(t, err)

	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	services, _ = client.CoreV1().Services(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, secondUID, ingresses.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")
}

func Test_UpdateAndAddDeployment_DeploymentAnnotationIsCorrectlyUpdated(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Test first deployment
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	firstDeployment := getDeploymentByName("first", deployments.Items)
	assert.Equal(t, "first_deployment", firstDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation])
	secondDeployment := getDeploymentByName("second", deployments.Items)
	assert.Empty(t, secondDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation])

	// Test second deployment
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	firstDeployment = getDeploymentByName("first", deployments.Items)
	assert.Equal(t, "second_deployment", firstDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation])
	secondDeployment = getDeploymentByName("second", deployments.Items)
	assert.Empty(t, secondDeployment.Spec.Template.Annotations[kube.RadixDeploymentNameAnnotation])
}

func TestObjectUpdated_UpdatePort_IngressIsCorrectlyReconciled(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")

	time.Sleep(1 * time.Second)

	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, int32(8081), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number, "Port was unexpected")
}

func TestObjectUpdated_ZeroReplicasExistsAndNotSpecifiedReplicas_SetsDefaultReplicaCount(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithReplicas(test.IntPtr(0))))
	require.NoError(t, err)

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, int32(0), *deployments.Items[0].Spec.Replicas)

	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app")))
	require.NoError(t, err)
	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, int32(1), *deployments.Items[0].Spec.Replicas)
}

func TestObjectSynced_DeploymentReplicasSetAccordingToSpec(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1"),
			utils.NewDeployComponentBuilder().WithName("comp2").WithReplicas(pointers.Ptr(2)),
			utils.NewDeployComponentBuilder().WithName("comp3").WithReplicas(pointers.Ptr(4)).WithHorizontalScaling(pointers.Ptr(int32(5)), int32(10), nil, nil),
			utils.NewDeployComponentBuilder().WithName("comp4").WithReplicas(pointers.Ptr(6)).WithHorizontalScaling(pointers.Ptr(int32(5)), int32(10), nil, nil),
			utils.NewDeployComponentBuilder().WithName("comp5").WithReplicas(pointers.Ptr(11)).WithHorizontalScaling(pointers.Ptr(int32(5)), int32(10), nil, nil),
			utils.NewDeployComponentBuilder().WithName("comp6").WithReplicas(pointers.Ptr(0)).WithHorizontalScaling(pointers.Ptr(int32(5)), int32(10), nil, nil),
			utils.NewDeployComponentBuilder().WithName("comp7").WithHorizontalScaling(pointers.Ptr(int32(5)), int32(10), nil, nil),
		))
	require.NoError(t, err)
	comp1, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(1), *comp1.Spec.Replicas)
	comp2, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp2", metav1.GetOptions{})
	assert.Equal(t, int32(2), *comp2.Spec.Replicas)
	comp3, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp3", metav1.GetOptions{})
	assert.Equal(t, int32(5), *comp3.Spec.Replicas)
	comp4, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp4", metav1.GetOptions{})
	assert.Equal(t, int32(6), *comp4.Spec.Replicas)
	comp5, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp5", metav1.GetOptions{})
	assert.Equal(t, int32(10), *comp5.Spec.Replicas)
	comp6, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp6", metav1.GetOptions{})
	assert.Equal(t, int32(0), *comp6.Spec.Replicas)
	comp7, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp7", metav1.GetOptions{})
	assert.Equal(t, int32(5), *comp7.Spec.Replicas)
}

func TestObjectSynced_DeploymentReplicasFromCurrentDeploymentWhenHPAEnabled(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Initial sync creating deployments should use replicas from spec
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(pointers.Ptr(int32(1)), int32(4), nil, nil),
		))
	require.NoError(t, err)

	comp1, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(1), *comp1.Spec.Replicas)

	// Simulate HPA scaling up comp1 to 3 replicas
	comp1.Spec.Replicas = pointers.Ptr[int32](3)
	_, err = client.AppsV1().Deployments(envNamespace).Update(context.Background(), comp1, metav1.UpdateOptions{})
	require.NoError(t, err)
	// Resync existing RD should use replicas from current deployment for HPA enabled component
	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(pointers.Ptr(int32(1)), int32(4), nil, nil),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(3), *comp1.Spec.Replicas)

	// Resync new RD should use replicas from current deployment for HPA enabled component
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("deployment2").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(pointers.Ptr(int32(1)), int32(4), nil, nil),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(3), *comp1.Spec.Replicas)

	// Resync new RD with HPA removed should use replicas from RD spec
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("deployment3").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(1), *comp1.Spec.Replicas)
}

func TestObjectSynced_StopAndStartDeploymentWhenHPAEnabled(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Initial sync creating deployments should use replicas from spec
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(pointers.Ptr(int32(2)), int32(4), nil, nil),
		))
	require.NoError(t, err)

	comp1, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(2), *comp1.Spec.Replicas)

	// Resync existing RD with replicas 0 (stop) should set deployment replicas to 0
	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(0)).WithHorizontalScaling(pointers.Ptr(int32(1)), int32(4), nil, nil),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(0), *comp1.Spec.Replicas)

	// Resync existing RD with replicas set back to original value (start) should use replicas from spec
	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("deployment1").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1").WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(pointers.Ptr(int32(2)), int32(4), nil, nil),
		))
	require.NoError(t, err)

	comp1, _ = client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, int32(2), *comp1.Spec.Replicas)

}

func TestObjectSynced_DeploymentRevisionHistoryLimit(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().WithName("comp1"),
			utils.NewDeployComponentBuilder().WithName("comp2").WithSecretRefs(radixv1.RadixSecretRefs{AzureKeyVaults: []radixv1.RadixAzureKeyVault{{}}}),
		))
	require.NoError(t, err)
	comp1, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp1", metav1.GetOptions{})
	assert.Equal(t, pointers.Ptr(int32(10)), comp1.Spec.RevisionHistoryLimit, "Invalid default RevisionHistoryLimit")
	comp2, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), "comp2", metav1.GetOptions{})
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
			tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
			defer teardownTest()
			envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

			radixApplication := utils.ARadixApplication()
			now := time.Now()
			timeShift := 1
			for _, deploymentName := range ts.deploymentNames {
				_, err := applyDeploymentWithModifiedSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.NewDeploymentBuilder().
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

			rdList, err := radixclient.RadixV1().RadixDeployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
			assert.NoError(tt, err)
			rbList, err := radixclient.RadixV1().RadixBatches(envNamespace).List(context.TODO(), metav1.ListOptions{})
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
		_, err := radixclient.RadixV1().RadixBatches(envNamespace).Create(context.TODO(), &radixv1.RadixBatch{
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
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithReplicas(test.IntPtr(3))))
	require.NoError(t, err)

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, int32(3), *deployments.Items[0].Spec.Replicas)

	err = applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app")))
	require.NoError(t, err)
	deployments, _ = client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, int32(1), *deployments.Items[0].Spec.Replicas)
}

func TestObjectUpdated_WithAppAliasRemoved_AliasIngressIsCorrectlyReconciled(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ := client.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("any-app-url-alias", ingresses), "App should have had an app alias ingress")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have another external alias")

	// Remove app alias from dev
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(false)))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have another external alias")
}

func TestObjectSynced_MultiComponentToOneComponent_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
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

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
	assert.Equal(t, 3, len(expectedDeployments), "Number of deployments wasn't as expected")
	assert.Equal(t, componentOneName, deployments.Items[0].Name, "app deployment not there")

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
				WithReplicas(test.IntPtr(0)).
				WithSecrets([]string{"a_secret"})))

	require.NoError(t, err)
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(deployments.Items)
		assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")
		assert.Equal(t, componentTwoName, deployments.Items[0].Name, "app deployment not there")
	})

	t.Run("validate service", func(t *testing.T) {
		t.Parallel()
		services, _ := client.CoreV1().Services(envNamespace).List(context.TODO(), metav1.ListOptions{})
		expectedServices := getServicesForRadixComponents(&services.Items)
		assert.Equal(t, 1, len(expectedServices), "Number of services wasn't as expected")
	})

	t.Run("validate ingress", func(t *testing.T) {
		t.Parallel()
		ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 0, len(ingresses.Items), "Number of ingresses was not according to public components")
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(secrets.Items), "Number of secrets was not according to spec")
		assert.Equal(t, utils.GetComponentSecretName(componentTwoName), secrets.Items[0].GetName(), "Component secret is not as expected")
	})

	t.Run("validate service accounts", func(t *testing.T) {
		t.Parallel()
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("validate roles", func(t *testing.T) {
		t.Parallel()
		roles, _ := client.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.ElementsMatch(t, []string{"radix-app-adm-componentTwoName", "radix-app-reader-componentTwoName"}, getRoleNames(roles))
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.ElementsMatch(t, []string{"radix-app-adm-componentTwoName", "radix-app-reader-componentTwoName"}, getRoleBindingNames(rolebindings))
	})
}

func TestObjectSynced_PublicToNonPublic_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Both components should be public")

	// Remove public on component 2
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Only component 1 should be public")

	// Remove public on component 1
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "No component should be public")
}

//nolint:staticcheck
func TestObjectSynced_PublicPort_OldPublic(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"

	// New publicPort exists, old public does not exist
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	assert.Equal(t, int32(80), ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)

	// New publicPort exists, old public exists (ignored)
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	assert.Equal(t, int32(80), ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)

	// New publicPort does not exist, old public does not exist
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "Component should not be public")

	// New publicPort does not exist, old public exists (used)
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
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

	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	roles, _ := client.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
	rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(context.TODO(), metav1.ListOptions{})

	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	assert.ElementsMatch(t, []string{"radix-app-adm-frontend", "radix-app-reader-frontend"}, getRoleNames(roles))
	assert.ElementsMatch(t, []string{"radix-app-adm-frontend", "radix-app-reader-frontend"}, getRoleBindingNames(rolebindings))

	assert.Equal(t, 1, len(secrets.Items), "Environment should have one secret for TLS cert")
	assert.True(t, secretByNameExists("some.alias.com", secrets), "TLS certificate for external alias is not properly defined")

	// Remove app alias from dev
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http")))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	secrets, _ = client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	rolebindings, _ = client.RbacV1().RoleBindings(envNamespace).List(context.TODO(), metav1.ListOptions{})

	assert.Equal(t, 2, len(ingresses.Items), "External alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	assert.Equal(t, 0, len(rolebindings.Items), "Role should have been removed")
	assert.Equal(t, 0, len(rolebindings.Items), "Rolebinding should have been removed")
	assert.Equal(t, 0, len(secrets.Items), "Secret should have been removed")

}

func TestObjectUpdated_WithOneExternalAliasRemovedOrModified_AllChangesProperlyReconciled(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
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

	roles, _ := client.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 3, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "some.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for external alias")
	assert.Equal(t, "another.alias.com", roles.Items[0].Rules[0].ResourceNames[2], "Expected role should be able to access TLS certificate for second external alias")

	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
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

	roles, _ = client.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 3, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "some.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for external alias")
	assert.Equal(t, "yet.another.alias.com", roles.Items[0].Rules[0].ResourceNames[2], "Expected role should be able to access TLS certificate for second external alias")

	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("yet.another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	yetAnotherExternalAliasIngress = getIngressByName("yet.another.alias.com", ingresses)
	assert.Equal(t, "yet.another.alias.com", yetAnotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), yetAnotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number, "Correct service port")

	roles, _ = client.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 2, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "yet.another.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for second external alias")

	// Remove app alias from dev
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http")))
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "External alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	roles, _ = client.RbacV1().Roles(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 0, len(roles.Items), "Role should have been removed")

}

func TestFixedAliasIngress_ActiveCluster(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)
	require.NoError(t, err)
	ingresses, _ := client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Environment should have two ingresses")
	activeClusterIngress := getIngressByName(getActiveClusterIngressName(anyComponentName), ingresses)
	assert.False(t, strings.Contains(activeClusterIngress.Spec.Rules[0].Host, testClusterName))
	defaultIngress := getIngressByName(getDefaultIngressName(anyComponentName), ingresses)
	assert.True(t, strings.Contains(defaultIngress.Spec.Rules[0].Host, testClusterName))

	// Current cluster is not active cluster
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "newClusterName")
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)
	require.NoError(t, err)
	ingresses, _ = client.NetworkingV1().Ingresses(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Environment should have one ingresses")
	assert.True(t, strings.Contains(ingresses.Items[0].Spec.Rules[0].Host, testClusterName))
}

func TestNewDeploymentStatus(t *testing.T) {
	anyApp := "any-app"
	anyEnv := "dev"
	anyComponentName := "frontend"

	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(anyApp).
		WithEnvironment(anyEnv).
		WithEmptyStatus().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))

	rd, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)
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

	rd2, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)
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
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	rd1 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, prometheusclient)

	time.Sleep(2 * time.Millisecond)
	rd2 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, prometheusclient)
	rd1, _ = getUpdatedRD(radixclient, rd1)

	assert.Equal(t, radixv1.DeploymentInactive, rd1.Status.Condition)
	assert.Equal(t, rd1.Status.ActiveTo, rd2.Status.ActiveFrom)
	assert.Equal(t, radixv1.DeploymentActive, rd2.Status.Condition)
	assert.True(t, !rd2.Status.ActiveFrom.IsZero())

	time.Sleep(3 * time.Millisecond)
	rd3 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, prometheusclient)
	rd1, _ = getUpdatedRD(radixclient, rd1)
	rd2, _ = getUpdatedRD(radixclient, rd2)

	assert.Equal(t, radixv1.DeploymentInactive, rd1.Status.Condition)
	assert.Equal(t, radixv1.DeploymentInactive, rd2.Status.Condition)
	assert.Equal(t, rd1.Status.ActiveTo, rd2.Status.ActiveFrom)
	assert.Equal(t, rd2.Status.ActiveTo, rd3.Status.ActiveFrom)
	assert.Equal(t, radixv1.DeploymentActive, rd3.Status.Condition)
	assert.True(t, !rd3.Status.ActiveFrom.IsZero())

	time.Sleep(4 * time.Millisecond)
	rd4 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, prometheusclient)
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
	return radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.TODO(), rd.GetName(), metav1.GetOptions{ResourceVersion: rd.ResourceVersion})
}

func addRadixDeployment(anyApp string, anyEnv string, anyComponentName string, tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube, radixclient radixclient.Interface, prometheusclient prometheusclient.Interface) *radixv1.RadixDeployment {
	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(anyApp).
		WithEnvironment(anyEnv).
		WithEmptyStatus().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))
	rd, _ := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)
	return rd
}

func TestObjectUpdated_RemoveOneSecret_SecretIsRemoved(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Setup
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithSecrets([]string{"a_secret", "another_secret", "a_third_secret"})))
	require.NoError(t, err)
	secrets, _ := client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
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
	_, err = client.CoreV1().Secrets(envNamespace).Update(context.TODO(), anyComponentSecret, metav1.UpdateOptions{})
	require.NoError(t, err)
	// Removing one secret from config and therefore from the deployment
	// should cause it to disappear
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithSecrets([]string{"a_secret", "a_third_secret"})))
	require.NoError(t, err)
	secrets, _ = client.CoreV1().Secrets(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Len(t, secrets.Items, 1)
	anyComponentSecret = getSecretByName(utils.GetComponentSecretName(anyComponentName), secrets)
	assert.True(t, radixutils.ArrayEqualElements([]string{"a_secret", "a_third_secret"}, radixmaps.GetKeysFromByteMap(anyComponentSecret.Data)), "Component secret data is not as expected")
}

func TestObjectUpdated_ExternalDNS_EnableAutomation_DeleteAndRecreateResources(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	fqdn := "some.alias.com"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	defer teardownTest()

	// Initial sync with UseCertificateAutomation false
	rd1 := utils.ARadixDeployment().WithDeploymentName("rd1").
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("anycomp").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: false}),
		)
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, rd1)
	require.NoError(t, err)
	_, err = client.NetworkingV1().Ingresses(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)
	_, err = client.CoreV1().Secrets(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)

	// New sync with UseCertificateAutomation true
	var m mock.Mock
	client.Fake.PrependReactor("*", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() == "delete" && slice.Any([]string{"ingresses", "secrets"}, func(r string) bool { return r == action.GetResource().Resource }) {
			deleteAction := action.(kubetesting.DeleteAction)
			m.MethodCalled(deleteAction.GetVerb(), deleteAction.GetResource().Resource, deleteAction.GetNamespace(), deleteAction.GetName())
		}
		return false, nil, nil
	})
	m.On("delete", "ingresses", envNamespace, fqdn).Times(1)
	m.On("delete", "secrets", envNamespace, fqdn).Times(1)

	rd2 := utils.ARadixDeployment().WithDeploymentName("rd2").
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("anycomp").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: true}),
		)
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, rd2)
	require.NoError(t, err)
	m.AssertExpectations(t)
	ingress, err := client.NetworkingV1().Ingresses(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)
	expectedAnnotations := map[string]string{
		kube.RadixExternalDNSUseCertificateAutomationAnnotation: "true",
		"cert-manager.io/cluster-issuer":                        testConfig.CertificateAutomation.ClusterIssuer,
		"cert-manager.io/duration":                              testConfig.CertificateAutomation.Duration.String(),
		"cert-manager.io/renew-before":                          testConfig.CertificateAutomation.RenewBefore.String(),
	}
	assert.Equal(t, expectedAnnotations, ingress.Annotations)
	_, err = client.CoreV1().Secrets(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	assert.True(t, kubeerrors.IsNotFound(err))
}

func TestObjectUpdated_ExternalDNS_DisableAutomation_DeleteIngressResetSecret(t *testing.T) {
	anyAppName, anyEnvironment, anyComponent := "any-app", "dev", "anyComp"
	fqdn := "some.alias.com"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	defer teardownTest()

	// Initial sync with UseCertificateAutomation true
	rd1 := utils.ARadixDeployment().WithDeploymentName("rd1").
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponent).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: true}),
		)
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, rd1)
	require.NoError(t, err)
	_, err = client.NetworkingV1().Ingresses(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = client.CoreV1().Secrets(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.True(t, kubeerrors.IsNotFound(err))
	// Create the TLS secret that cert-manager would normally created
	tlsSecret := &corev1.Secret{
		Type:       corev1.SecretTypeTLS,
		ObjectMeta: metav1.ObjectMeta{Name: fqdn, Namespace: envNamespace},
		Data: map[string][]byte{
			corev1.TLSPrivateKeyKey: []byte("anykey"),
			corev1.TLSCertKey:       []byte("anycert"),
		},
	}
	_, err = client.CoreV1().Secrets(envNamespace).Create(context.Background(), tlsSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	// New sync with UseCertificateAutomation false
	var m mock.Mock
	client.Fake.PrependReactor("*", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() == "delete" && slice.Any([]string{"ingresses", "secrets"}, func(r string) bool { return r == action.GetResource().Resource }) {
			deleteAction := action.(kubetesting.DeleteAction)
			m.MethodCalled(deleteAction.GetVerb(), deleteAction.GetResource().Resource, deleteAction.GetNamespace(), deleteAction.GetName())
		}
		return false, nil, nil
	})
	m.On("delete", "ingresses", envNamespace, fqdn).Times(1)

	rd2 := utils.ARadixDeployment().WithDeploymentName("rd2").
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponent).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: false}),
		)
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, rd2)
	require.NoError(t, err)
	m.AssertExpectations(t)
	ingress, err := client.NetworkingV1().Ingresses(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{kube.RadixExternalDNSUseCertificateAutomationAnnotation: "false"}, ingress.Annotations)
	secret, err := client.CoreV1().Secrets(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string][]byte{corev1.TLSCertKey: nil, corev1.TLSPrivateKeyKey: nil}, secret.Data)
	assert.Equal(t, map[string]string{kube.RadixAppLabel: anyAppName, kube.RadixComponentLabel: anyComponent, kube.RadixExternalAliasLabel: "true"}, secret.Labels)
}

func TestObjectUpdated_ExternalDNS_TLSSecretDataRetainedBetweenSync(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	fqdn := "some.alias.com"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	tlsKey, tlsCert := []byte("anytlskey"), []byte("anytlscert")
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)
	defer teardownTest()

	// Initial sync
	rd1 := utils.ARadixDeployment().WithDeploymentName("rd1").
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("anycomp").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: false}),
		)
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, rd1)
	require.NoError(t, err)
	_, err = client.NetworkingV1().Ingresses(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)
	secret, err := client.CoreV1().Secrets(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)
	secret.Data[corev1.TLSPrivateKeyKey] = tlsKey
	secret.Data[corev1.TLSCertKey] = tlsCert
	secret, err = client.CoreV1().Secrets(envNamespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Equal(t, tlsKey, secret.Data[corev1.TLSPrivateKeyKey])
	require.Equal(t, tlsCert, secret.Data[corev1.TLSCertKey])

	// New RD with same external DNS
	rd2 := utils.ARadixDeployment().WithDeploymentName("rd2").
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("anycomp").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: fqdn, UseCertificateAutomation: false}),
		)
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, rd2)
	require.NoError(t, err)
	_, err = client.NetworkingV1().Ingresses(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	assert.NoError(t, err)
	secret, err = client.CoreV1().Secrets(envNamespace).Get(context.Background(), fqdn, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, tlsKey, secret.Data[corev1.TLSPrivateKeyKey])
	assert.Equal(t, tlsCert, secret.Data[corev1.TLSCertKey])
}

func TestHistoryLimit_IsBroken_FixedAmountOfDeployments(t *testing.T) {
	anyAppName := "any-app"
	anyComponentName := "frontend"
	anyEnvironment := "dev"
	anyLimit := 3

	tu, client, kubeUtils, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Current cluster is active cluster
	deploymentHistoryLimitSetter := func(syncer DeploymentSyncer) {
		if s, ok := syncer.(*Deployment); ok {
			newcfg := *s.config
			newcfg.DeploymentSyncer.DeploymentHistoryLimit = anyLimit
			s.config = &newcfg
		}
	}
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	_, err := applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, prometheusclient,
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
	_, err = applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, prometheusclient,
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
	_, err = applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, prometheusclient,
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
	_, err = applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, prometheusclient,
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
	deployments, _ := radixclient.RadixV1().RadixDeployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(deployments.Items), "Number of deployments should match limit")

	assert.False(t, radixDeploymentByNameExists("firstdeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("seconddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("thirddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fourthdeployment", deployments))

	_, err = applyDeploymentWithModifiedSync(tu, client, kubeUtils, radixclient, prometheusclient,
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
	deployments, _ = radixclient.RadixV1().RadixDeployments(envNamespace).List(context.TODO(), metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(deployments.Items), "Number of deployments should match limit")

	assert.False(t, radixDeploymentByNameExists("firstdeployment", deployments))
	assert.False(t, radixDeploymentByNameExists("seconddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("thirddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fourthdeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fifthdeployment", deployments))

	teardownTest()
}

func TestHPAConfig(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	componentThreeName := "componentThreeName"
	minReplicas := int32(2)
	maxReplicas := int32(4)

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(0)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil)))
	require.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	t.Run("validate hpas", func(t *testing.T) {
		hpas, _ := client.AutoscalingV2().HorizontalPodAutoscalers(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 2, len(hpas.Items), "Number of horizontal pod autoscalers wasn't as expected")
		assert.False(t, hpaByNameExists(componentOneName, hpas), "componentOneName horizontal pod autoscaler should not exist")
		assert.True(t, hpaByNameExists(componentTwoName, hpas), "componentTwoName horizontal pod autoscaler should exist")
		assert.True(t, hpaByNameExists(componentThreeName, hpas), "componentThreeName horizontal pod autoscaler should exist")
		assert.Equal(t, int32(2), *getHPAByName(componentTwoName, hpas).Spec.MinReplicas, "componentTwoName horizontal pod autoscaler config is incorrect")
	})

	// Test - remove HPA from component three
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(0)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas, nil, nil),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1))))
	require.NoError(t, err)

	t.Run("validate hpas after reconfiguration", func(t *testing.T) {
		hpas, _ := client.AutoscalingV2().HorizontalPodAutoscalers(envNamespace).List(context.TODO(), metav1.ListOptions{})
		assert.Equal(t, 1, len(hpas.Items), "Number of horizontal pod autoscalers wasn't as expected")
		assert.False(t, hpaByNameExists(componentOneName, hpas), "componentOneName horizontal pod autoscaler should not exist")
		assert.True(t, hpaByNameExists(componentTwoName, hpas), "componentTwoName horizontal pod autoscaler should exist")
		assert.False(t, hpaByNameExists(componentThreeName, hpas), "componentThreeName horizontal pod autoscaler should not exist")
		assert.Equal(t, int32(2), *getHPAByName(componentTwoName, hpas).Spec.MinReplicas, "componentTwoName horizontal pod autoscaler config is incorrect")
	})

}

func TestMonitoringConfig(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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

	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
		servicemonitors, _ := prometheusclient.MonitoringV1().ServiceMonitors(envNamespace).List(context.TODO(), metav1.ListOptions{})
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
	tu, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	var portTestFunc = func(portName string, portNumber int32, ports []corev1.ContainerPort) {
		port := getPortByName(portName, ports)
		assert.NotNil(t, port)
		assert.Equal(t, portNumber, port.ContainerPort)
	}

	// Initial build
	_, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	deployments, _ := kubeclient.AppsV1().Deployments("app-env").List(context.TODO(), metav1.ListOptions{})
	comp := getDeploymentByName("comp", deployments.Items)
	assert.Len(t, comp.Spec.Template.Spec.Containers[0].Ports, 2)
	portTestFunc("port1", 8001, comp.Spec.Template.Spec.Containers[0].Ports)
	portTestFunc("port2", 8002, comp.Spec.Template.Spec.Containers[0].Ports)
	job := getDeploymentByName("job", deployments.Items)
	assert.Len(t, job.Spec.Template.Spec.Containers[0].Ports, 1)
	portTestFunc("scheduler-port", 8080, job.Spec.Template.Spec.Containers[0].Ports)

	// Update ports
	_, err = applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	deployments, _ = kubeclient.AppsV1().Deployments("app-env").List(context.TODO(), metav1.ListOptions{})
	comp = getDeploymentByName("comp", deployments.Items)
	assert.Len(t, comp.Spec.Template.Spec.Containers[0].Ports, 1)
	portTestFunc("port2", 9002, comp.Spec.Template.Spec.Containers[0].Ports)
	job = getDeploymentByName("job", deployments.Items)
	assert.Len(t, job.Spec.Template.Spec.Containers[0].Ports, 1)
	portTestFunc("scheduler-port", 9090, job.Spec.Template.Spec.Containers[0].Ports)
}

func TestUseGpuNode(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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
	rd, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentName1 := "componentName1"
	componentName2 := "componentName2"
	componentName3 := "componentName3"
	componentName4 := "componentName4"
	jobComponentName1 := "jobComponentName1"
	jobComponentName2 := "jobComponentName2"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	// Test
	gpuNvidiaV100 := "nvidia-v100"
	gpuNvidiaP100 := "nvidia-p100"
	gpuNvidiaK80 := "nvidia-k80"
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
				WithNodeGpu(fmt.Sprintf("%s, %s, -%s", gpuNvidiaV100, gpuNvidiaP100, gpuNvidiaK80)),
			utils.NewDeployComponentBuilder().
				WithName(componentName4).
				WithPort("http", 8084).
				WithPublicPort("http")).
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(jobComponentName1).
				WithPort("http", 8085),
			utils.NewDeployJobComponentBuilder().
				WithName(jobComponentName2).
				WithPort("http", 8085).
				WithNodeGpu(fmt.Sprintf("%s, -%s", gpuNvidiaP100, gpuNvidiaK80))))
	require.NoError(t, err)

	t.Run("has node with nvidia-v100", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName1, metav1.GetOptions{})
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
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName2, metav1.GetOptions{})
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
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName3, metav1.GetOptions{})
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
	t.Run("has node with no gpu", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName4, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("job has no node GPU, it is JobScheduler component", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), jobComponentName1, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("job has node with GPUs, it is JobScheduler component", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), jobComponentName2, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
}

func TestUseGpuNodeCount(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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
	rd, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName1, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)
		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0) // missing node.gpu
	})
	t.Run("has node with gpu-count 10", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName2, metav1.GetOptions{})
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
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName3, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("has node with gpu-count -1", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName4, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("has node with invalid value of gpu-count", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName5, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("has node with no gpu-count", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName6, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
	t.Run("job has node, but pod template of Job Scheduler does not have it", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), jobComponentName, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
}

func TestUseGpuNodeWithGpuCountOnDeployment(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), componentName, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.NotNil(t, affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
		nodeSelectorTerms := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		assert.Len(t, nodeSelectorTerms, 1)
		assert.Len(t, nodeSelectorTerms[0].MatchExpressions, 3)
		assert.Equal(t, corev1.NodeSelectorRequirement{
			Key:      kube.RadixGpuCountLabel,
			Operator: corev1.NodeSelectorOpGt,
			Values:   []string{"9"},
		}, nodeSelectorTerms[0].MatchExpressions[0])
		assert.Equal(t, corev1.NodeSelectorRequirement{
			Key:      kube.RadixGpuLabel,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{gpuNvidiaV100, gpuNvidiaP100},
		}, nodeSelectorTerms[0].MatchExpressions[1])
		assert.Equal(t, corev1.NodeSelectorRequirement{
			Key:      kube.RadixGpuLabel,
			Operator: corev1.NodeSelectorOpNotIn,
			Values:   []string{gpuNvidiaK80},
		}, nodeSelectorTerms[0].MatchExpressions[2])

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 1)
		assert.Equal(t, corev1.Toleration{Key: kube.RadixGpuCountLabel, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}, tolerations[0])
	})
	t.Run("job has node, but pod template of Job Scheduler does not have it", func(t *testing.T) {
		t.Parallel()
		deployment, _ := client.AppsV1().Deployments(envNamespace).Get(context.TODO(), jobComponentName, metav1.GetOptions{})
		affinity := deployment.Spec.Template.Spec.Affinity
		assert.NotNil(t, affinity)
		assert.Nil(t, affinity.NodeAffinity)

		tolerations := deployment.Spec.Template.Spec.Tolerations
		assert.Len(t, tolerations, 0)
	})
}

func Test_JobScheduler_ObjectsGarbageCollected(t *testing.T) {
	defer teardownTest()
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

			_, err := client.BatchV1().Jobs(namespace).Create(context.TODO(),
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

			_, err := client.CoreV1().Secrets(namespace).Create(context.TODO(),
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

			_, err := client.CoreV1().Services(namespace).Create(context.TODO(),
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
			tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)

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

			if _, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, theory.builder); err != nil {
				assert.FailNow(t, fmt.Sprintf("error apply deployment: %v", err))
			}

			// Verify expected jobs
			kubeJobs, _ := client.BatchV1().Jobs("").List(context.TODO(), metav1.ListOptions{LabelSelector: "item-in-test=true"})
			assert.Equal(t, len(theory.expectedJobs), len(kubeJobs.Items), "incorrect number of jobs")
			for _, job := range kubeJobs.Items {
				assert.Contains(t, theory.expectedJobs, job.Name, fmt.Sprintf("expected job %s", job.Name))
			}

			// Verify expected secrets
			kubeSecrets, _ := client.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{LabelSelector: "item-in-test=true"})
			assert.Equal(t, len(theory.expectedSecrets), len(kubeSecrets.Items), "incorrect number of secrets")
			for _, secret := range kubeSecrets.Items {
				assert.Contains(t, theory.expectedSecrets, secret.Name, fmt.Sprintf("expected secrets %s", secret.Name))
			}

			// Verify expected services
			kubeServices, _ := client.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{LabelSelector: "item-in-test=true"})
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
	_, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, rr, rd, []ingress.AnnotationProvider{annotations1, annotations2}, nil, &config.Config{})
	err = syncer.OnSync()
	require.NoError(t, err)
	ingresses, _ := kubeclient.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	assert.Len(t, ingresses.Items, 3)
	expected := map[string]string{"bar": "y", "baz": "z", "foo": "x"}

	for _, ingress := range ingresses.Items {
		assert.Equal(t, expected, ingress.GetAnnotations())
	}
}

func Test_IngressAnnotations_ReturnError(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, rr, rd, []ingress.AnnotationProvider{annotations1}, nil, &config.Config{})
	err = syncer.OnSync()
	assert.Error(t, err)
}

func Test_AuxiliaryResourceManagers_Called(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	rr := utils.NewRegistrationBuilder().WithName("app").BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName("app").WithEnvironment("dev").WithComponent(utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http")).BuildRD()
	_, err := radixclient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = radixclient.RadixV1().RadixDeployments("app-dev").Create(context.Background(), rd, metav1.CreateOptions{})
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	auxResource := NewMockAuxiliaryResourceManager(ctrl)
	auxResource.EXPECT().GarbageCollect().Times(1).Return(nil)
	auxResource.EXPECT().Sync().Times(1).Return(nil)

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, rr, rd, nil, []AuxiliaryResourceManager{auxResource}, &config.Config{})
	err = syncer.OnSync()
	assert.NoError(t, err)
}

func Test_AuxiliaryResourceManagers_Sync_ReturnErr(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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
	auxResource.EXPECT().GarbageCollect().Times(1).Return(nil)
	auxResource.EXPECT().Sync().Times(1).Return(auxErr)

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, rr, rd, nil, []AuxiliaryResourceManager{auxResource}, &config.Config{})
	err = syncer.OnSync()
	assert.Contains(t, err.Error(), auxErr.Error())
}

func Test_AuxiliaryResourceManagers_GarbageCollect_ReturnErr(t *testing.T) {
	_, kubeclient, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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
	auxResource.EXPECT().GarbageCollect().Times(1).Return(auxErr)
	auxResource.EXPECT().Sync().Times(0)

	syncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, rr, rd, nil, []AuxiliaryResourceManager{auxResource}, &config.Config{})
	err = syncer.OnSync()
	assert.Contains(t, err.Error(), auxErr.Error())
}

func Test_ComponentSynced_VolumeAndMounts(t *testing.T) {
	appName, environment, compName := "app", "dev", "comp"
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient,
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
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient,
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
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient,
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
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, testClusterName)

	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient,
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

func TestRadixBatch_IsGarbageCollected(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient, _ := setupTest(t)
	defer teardownTest()
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
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

func parseQuantity(value string) resource.Quantity {
	q, _ := resource.ParseQuantity(value)
	return q
}

func applyDeploymentWithSyncForTestEnv(testEnv *testEnvProps, deploymentBuilder utils.DeploymentBuilder) (*radixv1.RadixDeployment, error) {
	return applyDeploymentWithSync(testEnv.testUtil, testEnv.kubeclient, testEnv.kubeUtil, testEnv.radixclient, testEnv.prometheusclient, deploymentBuilder)
}

func applyDeploymentWithSync(tu *test.Utils, kubeclient kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, prometheusclient prometheusclient.Interface, deploymentBuilder utils.DeploymentBuilder) (*radixv1.RadixDeployment, error) {
	return applyDeploymentWithModifiedSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, deploymentBuilder, func(syncer DeploymentSyncer) {})
}

func applyDeploymentWithModifiedSync(tu *test.Utils, kubeclient kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, prometheusclient prometheusclient.Interface, deploymentBuilder utils.DeploymentBuilder, modifySyncer func(syncer DeploymentSyncer)) (*radixv1.RadixDeployment, error) {

	rd, err := tu.ApplyDeployment(deploymentBuilder)
	if err != nil {
		return nil, err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(context.TODO(), rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	deploymentSyncer := NewDeploymentSyncer(kubeclient, kubeUtil, radixclient, prometheusclient, radixRegistration, rd /*testTenantId, testKubernetesApiPort, testRadixDeploymentHistoryLimit,*/, nil, nil, &testConfig)
	modifySyncer(deploymentSyncer)
	err = deploymentSyncer.OnSync()
	if err != nil {
		return nil, err
	}

	updatedRD, err := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
	return updatedRD, err
}

func applyDeploymentUpdateWithSync(tu *test.Utils, client kubernetes.Interface, kubeUtil *kube.Kube,
	radixclient radixclient.Interface, prometheusclient prometheusclient.Interface, deploymentBuilder utils.DeploymentBuilder) error {
	rd, err := tu.ApplyDeploymentUpdate(deploymentBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(context.TODO(), rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment := NewDeploymentSyncer(client, kubeUtil, radixclient, prometheusclient, radixRegistration, rd /*testTenantId, testKubernetesApiPort, testRadixDeploymentHistoryLimit,*/, nil, nil, &testConfig)
	err = deployment.OnSync()
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
	cm, _ := kubeclient.CoreV1().ConfigMaps(deployment.Namespace).Get(context.TODO(), envVarsConfigMapName, metav1.GetOptions{})
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

func hpaByNameExists(name string, hpas *autoscalingv2.HorizontalPodAutoscalerList) bool {
	return getHPAByName(name, hpas) != nil
}

func getHPAByName(name string, hpas *autoscalingv2.HorizontalPodAutoscalerList) *autoscalingv2.HorizontalPodAutoscaler {
	for _, hpa := range hpas.Items {
		if hpa.Name == name {
			return &hpa
		}
	}

	return nil
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
