package deployment

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	kubeUtils "github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils/maps"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusclient "github.com/coreos/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/coreos/prometheus-operator/pkg/client/versioned/fake"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kube "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const clusterName = "AnyClusterName"
const dnsZone = "dev.radix.equinor.com"
const anyContainerRegistry = "any.container.registry"

func setupTest() (*test.Utils, kube.Interface, *kubeUtils.Kube, radixclient.Interface, prometheusclient.Interface) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	kubeUtil, _ := kubeUtils.New(kubeclient, radixclient)

	handlerTestUtils := test.NewTestUtils(kubeclient, radixclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, anyContainerRegistry)
	return &handlerTestUtils, kubeclient, kubeUtil, radixclient, prometheusclient
}

func teardownTest() {
	// Cleanup setup
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
	os.Unsetenv(defaults.ActiveClusternameEnvironmentVariable)
	os.Unsetenv(defaults.DeploymentsHistoryLimitEnvironmentVariable)
}

func TestObjectSynced_MultiComponent_ContainsAllElements(t *testing.T) {
	for _, componentsExist := range []bool{true, false} {
		testScenario := utils.TernaryString(componentsExist, "Updating deployment", "Creating deployment")

		tu, kubeclient, kubeUtil, radixclient, prometheusclient := setupTest()
		os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")

		t.Run("Test Suite", func(t *testing.T) {
			aRadixRegistrationBuilder := utils.ARadixRegistration().
				WithMachineUser(true)
			aRadixApplicationBuilder := utils.ARadixApplication().
				WithRadixRegistration(aRadixRegistrationBuilder)
			environment := "test"
			appName := "edcradix"
			componentNameApp := "app"
			componentNameRedis := "redis"
			componentNameRadixQuote := "radixquote"
			outdatedSecret := "outdatedSecret"
			remainingSecret := "remainingSecret"
			addingSecret := "addingSecret"
			blobVolumeName := "blob_volume_1"

			if componentsExist {
				// Update component
				existingRadixDeploymentBuilder := utils.ARadixDeployment().
					WithRadixApplication(aRadixApplicationBuilder).
					WithAppName(appName).
					WithImageTag("old_axmz8").
					WithEnvironment(environment).
					WithJobComponents().
					WithComponents(
						utils.NewDeployComponentBuilder().
							WithImage("old_radixdev.azurecr.io/radix-loadbalancer-html-app:1igdh").
							WithName(componentNameApp).
							WithPort("http", 8081).
							WithPublicPort("http").
							WithDNSAppAlias(true).
							WithDNSExternalAlias("updated_some.alias.com").
							WithDNSExternalAlias("updated_another.alias.com").
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
					WithComponents(
						utils.NewDeployComponentBuilder().
							WithImage("radixdev.azurecr.io/radix-loadbalancer-html-app:1igdh").
							WithName(componentNameApp).
							WithPort("http", 8080).
							WithPublicPort("http").
							WithDNSAppAlias(true).
							WithDNSExternalAlias("some.alias.com").
							WithDNSExternalAlias("another.alias.com").
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
							WithVolumeMounts([]v1.RadixVolumeMount{
								{
									Type:      v1.MountTypeBlob,
									Name:      blobVolumeName,
									Container: "some-container",
									Path:      "some-path",
								},
							}).
							WithSecrets([]string{outdatedSecret, remainingSecret}))

				// Test
				_, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, aRadixDeploymentBuilder)
				assert.NoError(t, err)
			}

			envNamespace := utils.GetEnvironmentNamespace(appName, environment)

			t.Run(fmt.Sprintf("%s: validate deploy", testScenario), func(t *testing.T) {
				t.Parallel()
				deployments, _ := kubeclient.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
				expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)
				assert.Equal(t, 3, len(expectedDeployments), "Number of deployments wasn't as expected")
				assert.Equal(t, componentNameApp, getDeploymentByName(componentNameApp, deployments).Name, "app deployment not there")

				if !componentsExist {
					assert.Equal(t, int32(4), *getDeploymentByName(componentNameApp, deployments).Spec.Replicas, "number of replicas was unexpected")
				} else {
					assert.Equal(t, int32(2), *getDeploymentByName(componentNameApp, deployments).Spec.Replicas, "number of replicas was unexpected")
				}

				assert.Equal(t, 11, len(getContainerByName(componentNameApp, getDeploymentByName(componentNameApp, deployments).Spec.Template.Spec.Containers).Env), "number of environment variables was unexpected for component. It should contain default and custom")
				assert.Equal(t, anyContainerRegistry, getEnvVariableByNameOnDeployment(defaults.ContainerRegistryEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, dnsZone, getEnvVariableByNameOnDeployment(defaults.RadixDNSZoneEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, "AnyClusterName", getEnvVariableByNameOnDeployment(defaults.ClusternameEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, environment, getEnvVariableByNameOnDeployment(defaults.EnvironmentnameEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, "app-edcradix-test.AnyClusterName.dev.radix.equinor.com", getEnvVariableByNameOnDeployment(defaults.PublicEndpointEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, "app-edcradix-test.AnyClusterName.dev.radix.equinor.com", getEnvVariableByNameOnDeployment(defaults.CanonicalEndpointEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, appName, getEnvVariableByNameOnDeployment(defaults.RadixAppEnvironmentVariable, componentNameApp, deployments))
				assert.Equal(t, componentNameApp, getEnvVariableByNameOnDeployment(defaults.RadixComponentEnvironmentVariable, componentNameApp, deployments))

				if !componentsExist {
					assert.Equal(t, "(8080)", getEnvVariableByNameOnDeployment(defaults.RadixPortsEnvironmentVariable, componentNameApp, deployments))
				} else {
					assert.Equal(t, "(8081)", getEnvVariableByNameOnDeployment(defaults.RadixPortsEnvironmentVariable, componentNameApp, deployments))
				}

				assert.Equal(t, "(http)", getEnvVariableByNameOnDeployment(defaults.RadixPortNamesEnvironmentVariable, componentNameApp, deployments))
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

				assert.Equal(t, 10, len(getContainerByName(componentNameRedis, getDeploymentByName(componentNameRedis, deployments).Spec.Template.Spec.Containers).Env), "number of environment variables was unexpected for component. It should contain default and custom")
				assert.True(t, envVariableByNameExistOnDeployment("a_variable", componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.ContainerRegistryEnvironmentVariable, componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixDNSZoneEnvironmentVariable, componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.ClusternameEnvironmentVariable, componentNameRedis, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.EnvironmentnameEnvironmentVariable, componentNameRedis, deployments))

				if !componentsExist {
					assert.Equal(t, "3001", getEnvVariableByNameOnDeployment("a_variable", componentNameRedis, deployments))
				} else {
					assert.Equal(t, "3002", getEnvVariableByNameOnDeployment("a_variable", componentNameRedis, deployments))
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

				volumesExist := len(spec.Template.Spec.Volumes) > 0
				volumeMountsExist := len(spec.Template.Spec.Containers[0].VolumeMounts) > 0
				if !componentsExist {
					assert.True(t, volumesExist, "expected existing volumes")
					assert.True(t, volumeMountsExist, "expected existing volume mounts")
				} else {
					assert.False(t, volumesExist, "unexpected existing volumes")
					assert.False(t, volumeMountsExist, "unexpected existing volume mounts")
				}
			})

			t.Run(fmt.Sprintf("%s: validate hpa", testScenario), func(t *testing.T) {
				t.Parallel()
				hpas, _ := kubeclient.AutoscalingV1().HorizontalPodAutoscalers(envNamespace).List(metav1.ListOptions{})
				assert.Equal(t, 0, len(hpas.Items), "Number of horizontal pod autoscaler wasn't as expected")
			})

			t.Run(fmt.Sprintf("%s: validate service", testScenario), func(t *testing.T) {
				t.Parallel()
				services, _ := kubeclient.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
				expectedServices := getServicesForRadixComponents(&services.Items)
				assert.Equal(t, 3, len(expectedServices), "Number of services wasn't as expected")
				assert.True(t, serviceByNameExists(componentNameApp, services), "app service not there")
				assert.True(t, serviceByNameExists(componentNameRedis, services), "redis service not there")
				assert.True(t, serviceByNameExists(componentNameRadixQuote, services), "radixquote service not there")
			})

			t.Run(fmt.Sprintf("%s: validate secrets", testScenario), func(t *testing.T) {
				t.Parallel()
				secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})

				if !componentsExist {
					assert.Equal(t, 4, len(secrets.Items), "Number of secrets was not according to spec")
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
				if !componentsExist {
					assert.True(t, blobFuseSecretExists, "expected volume mount secret")
				} else {
					assert.False(t, blobFuseSecretExists, "unexpected volume mount secrets")
				}
			})

			t.Run(fmt.Sprintf("%s: validate service accounts", testScenario), func(t *testing.T) {
				t.Parallel()
				serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(envNamespace).List(metav1.ListOptions{})
				assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
			})

			t.Run(fmt.Sprintf("%s: validate roles", testScenario), func(t *testing.T) {
				t.Parallel()
				roles, _ := kubeclient.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})

				assert.Equal(t, 2, len(roles.Items), "Number of roles was not expected")
				assert.True(t, roleByNameExists("radix-app-adm-radixquote", roles), "Expected role radix-app-adm-radixquote to be there to access secret")

				// Exists due to external DNS, even though this is not acive cluster
				assert.True(t, roleByNameExists("radix-app-adm-app", roles), "Expected role radix-app-adm-frontend to be there to access secrets for TLS certificates")
			})

			t.Run(fmt.Sprintf("%s validate rolebindings", testScenario), func(t *testing.T) {
				t.Parallel()
				rolebindings, _ := kubeclient.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
				assert.Equal(t, 2, len(rolebindings.Items), "Number of rolebindings was not expected")

				assert.True(t, roleBindingByNameExists("radix-app-adm-radixquote", rolebindings), "Expected rolebinding radix-app-adm-radixquote to be there to access secret")
				assert.Equal(t, 2, len(getRoleBindingByName("radix-app-adm-radixquote", rolebindings).Subjects), "Number of rolebinding subjects was not as expected")
				assert.Equal(t, "edcradix-machine-user", getRoleBindingByName("radix-app-adm-radixquote", rolebindings).Subjects[1].Name)

				// Exists due to external DNS, even though this is not acive cluster
				assert.True(t, roleBindingByNameExists("radix-app-adm-app", rolebindings), "Expected rolebinding radix-app-adm-app to be there to access secrets for TLS certificates")
			})

			t.Run(fmt.Sprintf("%s: validate networkpolicy", testScenario), func(t *testing.T) {
				t.Parallel()
				np, _ := kubeclient.NetworkingV1().NetworkPolicies(envNamespace).List(metav1.ListOptions{})
				assert.Equal(t, 1, len(np.Items), "Number of networkpolicy was not expected")
			})
		})
		teardownTest()
	}
}

func TestObjectSynced_MultiJob_ContainsAllElements(t *testing.T) {
	const jobSchedulerImage = "job-scheduler:latest"

	for _, jobsExist := range []bool{false, true} {
		testScenario := utils.TernaryString(jobsExist, "Updating deployment", "Creating deployment")

		tu, kubeclient, kubeUtil, radixclient, prometheusclient := setupTest()
		os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "AnotherClusterName")
		os.Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, jobSchedulerImage)

		t.Run("Test Suite", func(t *testing.T) {
			aRadixRegistrationBuilder := utils.ARadixRegistration().
				WithMachineUser(true)
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
							WithPort("http", 3002).
							WithEnvironmentVariable("a_variable", "a_value").
							WithMonitoring(true).
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
							WithPort("http", 3002).
							WithEnvironmentVariable("a_variable", "a_value").
							WithMonitoring(true).
							WithResource(map[string]string{
								"memory": "65Mi",
								"cpu":    "251m",
							}, map[string]string{
								"memory": "129Mi",
								"cpu":    "501m",
							}).
							WithVolumeMounts([]v1.RadixVolumeMount{
								{
									Type:      v1.MountTypeBlob,
									Name:      blobVolumeName,
									Container: "some-container",
									Path:      "some-path",
								}},
							).
							WithSchedulerPort(&schedulerPortCreate).
							WithPayloadPath(&payloadPath).
							WithSecrets([]string{outdatedSecret, remainingSecret}).
							WithAlwaysPullImageOnDeploy(false),
						utils.NewDeployJobComponentBuilder().
							WithName(jobName2),
					).
					WithComponents()

				// Test
				_, err := applyDeploymentWithSync(tu, kubeclient, kubeUtil, radixclient, prometheusclient, aRadixDeploymentBuilder)
				assert.NoError(t, err)
			}

			envNamespace := utils.GetEnvironmentNamespace(appName, environment)

			t.Run(fmt.Sprintf("%s: validate deploy", testScenario), func(t *testing.T) {
				t.Parallel()
				deployments, _ := kubeclient.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
				expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)

				if jobsExist {
					assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")
				} else {
					assert.Equal(t, 2, len(expectedDeployments), "Number of deployments wasn't as expected")
				}

				assert.Equal(t, jobName, getDeploymentByName(jobName, deployments).Name, "app deployment not there")
				assert.Equal(t, int32(1), *getDeploymentByName(jobName, deployments).Spec.Replicas, "number of replicas was unexpected")

				envVars := getContainerByName(jobName, getDeploymentByName(jobName, deployments).Spec.Template.Spec.Containers).Env
				assert.Equal(t, 10, len(envVars), "number of environment variables was unexpected for component. It should contain default and custom")
				assert.Equal(t, anyContainerRegistry, getEnvVariableByNameOnDeployment(defaults.ContainerRegistryEnvironmentVariable, jobName, deployments))
				assert.Equal(t, dnsZone, getEnvVariableByNameOnDeployment(defaults.RadixDNSZoneEnvironmentVariable, jobName, deployments))
				assert.Equal(t, "AnyClusterName", getEnvVariableByNameOnDeployment(defaults.ClusternameEnvironmentVariable, jobName, deployments))
				assert.Equal(t, environment, getEnvVariableByNameOnDeployment(defaults.EnvironmentnameEnvironmentVariable, jobName, deployments))
				assert.Equal(t, appName, getEnvVariableByNameOnDeployment(defaults.RadixAppEnvironmentVariable, jobName, deployments))
				assert.Equal(t, jobName, getEnvVariableByNameOnDeployment(defaults.RadixComponentEnvironmentVariable, jobName, deployments))
				assert.Equal(t, "("+defaults.RadixJobSchedulerPortName+")", getEnvVariableByNameOnDeployment(defaults.RadixPortNamesEnvironmentVariable, jobName, deployments))
				assert.True(t, envVariableByNameExistOnDeployment(defaults.RadixCommitHashEnvironmentVariable, jobName, deployments))

				if jobsExist {
					assert.Equal(t, "("+fmt.Sprint(schedulerPortUpdate)+")", getEnvVariableByNameOnDeployment(defaults.RadixPortsEnvironmentVariable, jobName, deployments))
				} else {
					assert.Equal(t, "("+fmt.Sprint(schedulerPortCreate)+")", getEnvVariableByNameOnDeployment(defaults.RadixPortsEnvironmentVariable, jobName, deployments))
				}

				if jobsExist {
					assert.Equal(t, "deploy-update", getEnvVariableByNameOnDeployment(defaults.RadixDeploymentEnvironmentVariable, jobName, deployments))
				} else {
					assert.Equal(t, "deploy-create", getEnvVariableByNameOnDeployment(defaults.RadixDeploymentEnvironmentVariable, jobName, deployments))
				}
			})

			t.Run(fmt.Sprintf("%s: validate hpa", testScenario), func(t *testing.T) {
				t.Parallel()
				hpas, _ := kubeclient.AutoscalingV1().HorizontalPodAutoscalers(envNamespace).List(metav1.ListOptions{})
				assert.Equal(t, 0, len(hpas.Items), "Number of horizontal pod autoscaler wasn't as expected")
			})

			t.Run(fmt.Sprintf("%s: validate service", testScenario), func(t *testing.T) {
				t.Parallel()
				services, _ := kubeclient.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
				expectedServices := getServicesForRadixComponents(&services.Items)

				if jobsExist {
					assert.Equal(t, 1, len(expectedServices), "Number of services wasn't as expected")
				} else {
					assert.Equal(t, 2, len(expectedServices), "Number of services wasn't as expected")
				}

				assert.True(t, serviceByNameExists(jobName, services), "app service not there")
			})

			t.Run(fmt.Sprintf("%s: validate secrets", testScenario), func(t *testing.T) {
				t.Parallel()
				secrets, _ := kubeclient.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})

				if !jobsExist {
					assert.Equal(t, 2, len(secrets.Items), "Number of secrets was not according to spec")
				} else {
					assert.Equal(t, 1, len(secrets.Items), "Number of secrets was not according to spec")
				}

				jobSecretName := utils.GetComponentSecretName(jobName)
				assert.True(t, secretByNameExists(jobSecretName, secrets), "Job secret is not as expected")

				blobFuseSecretExists := secretByNameExists(defaults.GetBlobFuseCredsSecretName(jobName, blobVolumeName), secrets)
				if !jobsExist {
					assert.True(t, blobFuseSecretExists, "expected volume mount secret")
				} else {
					assert.False(t, blobFuseSecretExists, "unexpected volume mount secrets")
				}
			})

			t.Run(fmt.Sprintf("%s: validate service accounts", testScenario), func(t *testing.T) {
				t.Parallel()
				serviceAccounts, _ := kubeclient.CoreV1().ServiceAccounts(envNamespace).List(metav1.ListOptions{})
				assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
			})

			t.Run(fmt.Sprintf("%s: validate roles", testScenario), func(t *testing.T) {
				t.Parallel()
				roles, _ := kubeclient.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})

				assert.Equal(t, 1, len(roles.Items), "Number of roles was not expected")
			})

			t.Run(fmt.Sprintf("%s validate rolebindings", testScenario), func(t *testing.T) {
				t.Parallel()
				rolebindings, _ := kubeclient.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
				assert.Equal(t, 2, len(rolebindings.Items), "Number of rolebindings was not expected")

				assert.True(t, roleBindingByNameExists("radix-app-adm-job", rolebindings), "Expected rolebinding radix-app-adm-radixquote to be there to access secret")
				assert.Equal(t, 2, len(getRoleBindingByName("radix-app-adm-job", rolebindings).Subjects), "Number of rolebinding subjects was not as expected")
				assert.Equal(t, "edcradix-machine-user", getRoleBindingByName("radix-app-adm-job", rolebindings).Subjects[1].Name)

				// Exists due to being job-scheduler
				assert.True(t, roleBindingByNameExists(defaults.RadixJobSchedulerRoleName, rolebindings), "Expected rolebinding radix-job-scheduler to be there to access secrets for TLS certificates")
			})

			t.Run(fmt.Sprintf("%s: validate networkpolicy", testScenario), func(t *testing.T) {
				t.Parallel()
				np, _ := kubeclient.NetworkingV1().NetworkPolicies(envNamespace).List(metav1.ListOptions{})
				assert.Equal(t, 1, len(np.Items), "Number of networkpolicy was not expected")
			})
		})
		teardownTest()
	}
}

func getServicesForRadixComponents(services *[]corev1.Service) []corev1.Service {
	var result []corev1.Service
	for _, svc := range *services {
		if _, ok := svc.Labels["radix-component"]; ok {
			result = append(result, svc)
		}
	}
	return result
}

func getDeploymentsForRadixComponents(deployments *[]appsv1.Deployment) []appsv1.Deployment {
	var result []appsv1.Deployment
	for _, depl := range *deployments {
		if _, ok := depl.Labels["radix-component"]; ok {
			result = append(result, depl)
		}
	}
	return result
}

func TestObjectSynced_MultiComponent_NonActiveCluster_ContainsOnlyClusterSpecificIngresses(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
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
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com"),
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

	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Only cluster specific ingresses for the two public components should appear")
	assert.Truef(t, ingressByNameExists("app", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("radixquote", ingresses), "Cluster specific ingress for public component should exist")

	appIngress := getIngressByName("app", ingresses)
	assert.Equal(t, int32(8080), appIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", appIngress.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", appIngress.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", appIngress.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", appIngress.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")

	quoteIngress := getIngressByName("radixquote", ingresses)
	assert.Equal(t, int32(3000), quoteIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", quoteIngress.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", quoteIngress.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", quoteIngress.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "radixquote", quoteIngress.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")

	teardownTest()

}

func TestObjectSynced_MultiComponent_ActiveCluster_ContainsAllAliasesAndSupportingObjects(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

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
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com"),
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

	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 7, len(ingresses.Items), "Number of ingresses was not according to public components, app alias and number of external aliases")
	assert.Truef(t, ingressByNameExists("app", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("radixquote", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("edcradix-url-alias", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have an external alias")
	assert.Truef(t, ingressByNameExists("another.alias.com", ingresses), "App should have another external alias")
	assert.Truef(t, ingressByNameExists("app-active-cluster-url-alias", ingresses), "App should have another external alias")
	assert.Truef(t, ingressByNameExists("radixquote-active-cluster-url-alias", ingresses), "Radixquote should have had an ingress")

	appAlias := getIngressByName("edcradix-url-alias", ingresses)
	assert.Equal(t, int32(8080), appAlias.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "true", appAlias.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", appAlias.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", appAlias.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", appAlias.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "edcradix.app.dev.radix.equinor.com", appAlias.Spec.Rules[0].Host, "App should have an external alias")

	externalAlias := getIngressByName("some.alias.com", ingresses)
	assert.Equal(t, int32(8080), externalAlias.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", externalAlias.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", externalAlias.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", externalAlias.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", externalAlias.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "some.alias.com", externalAlias.Spec.Rules[0].Host, "App should have an external alias")

	anotherExternalAlias := getIngressByName("another.alias.com", ingresses)
	assert.Equal(t, int32(8080), anotherExternalAlias.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", anotherExternalAlias.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "true", anotherExternalAlias.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "false", anotherExternalAlias.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", anotherExternalAlias.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")
	assert.Equal(t, "another.alias.com", anotherExternalAlias.Spec.Rules[0].Host, "App should have an external alias")

	appActiveClusterIngress := getIngressByName("app-active-cluster-url-alias", ingresses)
	assert.Equal(t, int32(8080), appActiveClusterIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", appActiveClusterIngress.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", appActiveClusterIngress.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "true", appActiveClusterIngress.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "app", appActiveClusterIngress.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")

	quoteActiveClusterIngress := getIngressByName("radixquote-active-cluster-url-alias", ingresses)
	assert.Equal(t, int32(3000), quoteActiveClusterIngress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")
	assert.Equal(t, "false", quoteActiveClusterIngress.Labels[kubeUtils.RadixAppAliasLabel], "Ingress should not be an app alias")
	assert.Equal(t, "false", quoteActiveClusterIngress.Labels[kubeUtils.RadixExternalAliasLabel], "Ingress should not be an external app alias")
	assert.Equal(t, "true", quoteActiveClusterIngress.Labels[kubeUtils.RadixActiveClusterAliasLabel], "Ingress should not be an active cluster alias")
	assert.Equal(t, "radixquote", quoteActiveClusterIngress.Labels[kubeUtils.RadixComponentLabel], "Ingress should have the corresponding component")

	roles, _ := client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.True(t, roleByNameExists("radix-app-adm-app", roles), "Expected role radix-app-adm-app to be there to access secrets for TLS certificates")

	appAdmAppRole := getRoleByName("radix-app-adm-app", roles)
	assert.Equal(t, "secrets", appAdmAppRole.Rules[0].Resources[0], "Expected role radix-app-adm-app should be able to access secrets")
	assert.Equal(t, "some.alias.com", appAdmAppRole.Rules[0].ResourceNames[0], "Expected role should be able to access TLS certificate for external alias")
	assert.Equal(t, "another.alias.com", appAdmAppRole.Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for second external alias")

	secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	assert.True(t, secretByNameExists("some.alias.com", secrets), "TLS certificate for external alias is not properly defined")
	assert.True(t, secretByNameExists("another.alias.com", secrets), "TLS certificate for second external alias is not properly defined")

	assert.Equal(t, corev1.SecretType("kubernetes.io/tls"), getSecretByName("some.alias.com", secrets).Type, "TLS certificate for external alias is not properly defined type")
	assert.Equal(t, corev1.SecretType("kubernetes.io/tls"), getSecretByName("another.alias.com", secrets).Type, "TLS certificate for external alias is not properly defined type")

	rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
	assert.True(t, roleBindingByNameExists("radix-app-adm-app", rolebindings), "Expected rolebinding radix-app-adm-app to be there to access secrets for TLS certificates")

	teardownTest()
}

func TestObjectSynced_ServiceAccountSettingsAndRbac(t *testing.T) {
	// Test
	t.Run("app with component use default SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, "", expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})

	t.Run("app with job use custom SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)
		assert.Equal(t, utils.BoolPtr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

	})

	// Test
	t.Run("app from component to job and back to component", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

		// Initial deployment, app is a component
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, "", expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

		// Change app to be a job
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().WithName("app")).
			WithAppName("any-other-app").
			WithEnvironment("test"))

		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		expectedDeployments = getDeploymentsForRadixComponents(&deployments.Items)
		assert.Equal(t, 1, len(expectedDeployments))
		assert.Equal(t, utils.BoolPtr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixJobSchedulerServiceName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

		// And change app back to a component
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithComponents(
				utils.NewDeployComponentBuilder().WithName("app")).
			WithJobComponents().
			WithAppName("any-other-app").
			WithEnvironment("test"))

		serviceAccounts, _ = client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ = client.AppsV1().Deployments(utils.GetEnvironmentNamespace("any-other-app", "test")).List(metav1.ListOptions{})
		expectedDeployments = getDeploymentsForRadixComponents(&deployments.Items)
		assert.Equal(t, 1, len(expectedDeployments))
		assert.Equal(t, utils.BoolPtr(false), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, "", expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})

	t.Run("webhook runs custom SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("radix-github-webhook").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("radix-github-webhook", "test")).List(metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)
		assert.Equal(t, utils.BoolPtr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixGithubWebhookServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)

	})

	t.Run("radix-api runs custom SA", func(t *testing.T) {
		tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
		applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithJobComponents().
			WithAppName("radix-api").
			WithEnvironment("test"))

		serviceAccounts, _ := client.CoreV1().ServiceAccounts(utils.GetEnvironmentNamespace("radix-api", "test")).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(serviceAccounts.Items), "Number of service accounts was not expected")
		deployments, _ := client.AppsV1().Deployments(utils.GetEnvironmentNamespace("radix-api", "test")).List(metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)
		assert.Equal(t, utils.BoolPtr(true), expectedDeployments[0].Spec.Template.Spec.AutomountServiceAccountToken)
		assert.Equal(t, defaults.RadixAPIServiceAccountName, expectedDeployments[0].Spec.Template.Spec.ServiceAccountName)
	})

	teardownTest()
}

func TestObjectSynced_MultiComponentWithSameName_ContainsOneComponent(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)
	assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")

	services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	expectedServices := getServicesForRadixComponents(&services.Items)
	assert.Equal(t, 1, len(expectedServices), "Number of services wasn't as expected")

	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Number of ingresses was not according to public components")

	teardownTest()
}

func TestObjectSynced_NoEnvAndNoSecrets_ContainsDefaultEnvVariables(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
	anyEnvironment := "test"

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("component").
				WithEnvironmentVariables(nil).
				WithSecrets(nil)))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 7, len(deployments.Items[0].Spec.Template.Spec.Containers[0].Env), "Should only have default environment variables")
		assert.Equal(t, defaults.ContainerRegistryEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, defaults.RadixDNSZoneEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Name)
		assert.Equal(t, defaults.ClusternameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[2].Name)
		assert.Equal(t, anyContainerRegistry, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[0].Value)
		assert.Equal(t, dnsZone, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[1].Value)
		assert.Equal(t, clusterName, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[2].Value)
		assert.Equal(t, defaults.EnvironmentnameEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[3].Name)
		assert.Equal(t, anyEnvironment, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[3].Value)
		assert.Equal(t, defaults.RadixAppEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[4].Name)
		assert.Equal(t, "app", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[4].Value)
		assert.Equal(t, defaults.RadixComponentEnvironmentVariable, deployments.Items[0].Spec.Template.Spec.Containers[0].Env[5].Name)
		assert.Equal(t, "component", deployments.Items[0].Spec.Template.Spec.Containers[0].Env[5].Value)
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(secrets.Items), "Should have no secrets")
	})

	teardownTest()
}

func TestObjectSynced_WithLabels_LabelsAppliedToDeployment(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName("app").
		WithEnvironment("test").
		WithLabel("radix-branch", "master").
		WithLabel("radix-commit", "4faca8595c5283a9d0f17a623b9255a0d9866a2e"))

	envNamespace := utils.GetEnvironmentNamespace("app", "test")

	t.Run("validate deploy labels", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, "master", deployments.Items[0].Annotations[kubeUtils.RadixBranchAnnotation])
		assert.Equal(t, "4faca8595c5283a9d0f17a623b9255a0d9866a2e", deployments.Items[0].Labels["radix-commit"])
	})

	teardownTest()
}

func TestObjectSynced_NotLatest_DeploymentIsIgnored(t *testing.T) {
	// Setup
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Test
	now := time.Now().UTC()
	var firstUID, secondUID types.UID

	firstUID = "fda3d224-3115-11e9-b189-06c15a8f2fbb"
	secondUID = "5a8f2fbb-3115-11e9-b189-06c1fda3d224"

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	envNamespace := utils.GetEnvironmentNamespace("app1", "prod")
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, deployments.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, services.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, firstUID, ingresses.Items[0].OwnerReferences[0].UID, "First RD didn't take effect")

	time.Sleep(1 * time.Millisecond)
	// This is one second newer deployment
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	deployments, _ = client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	services, _ = client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Second RD didn't take effect")

	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
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

	applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, rdBuilder)

	deployments, _ = client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, deployments.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	services, _ = client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, services.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, secondUID, ingresses.Items[0].OwnerReferences[0].UID, "Should still be second RD which is the effective in the namespace")

	teardownTest()
}

func TestObjectUpdated_UpdatePort_IngressIsCorrectlyReconciled_DeploymentAnnotationIsCorrectlyUpdated(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
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

	envNamespace := utils.GetEnvironmentNamespace("anyapp1", "test")
	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(8080), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")

	deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	firstDeploymentUpdateTime := deployments.Items[0].Spec.Template.Annotations["radix-update-time"]
	assert.NotEqual(t, "", firstDeploymentUpdateTime)
	assert.Empty(t, deployments.Items[1].Spec.Template.Annotations["radix-update-time"])
	assert.Empty(t, deployments.Items[2].Spec.Template.Annotations["radix-update-time"])

	time.Sleep(1 * time.Second)

	applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp1").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithPort("http", 8081).
				WithAlwaysPullImageOnDeploy(true).
				WithPublicPort("http")))

	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(8081), ingresses.Items[0].Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal, "Port was unexpected")

	deployments, _ = client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	secondDeploymentUpdateTime := deployments.Items[0].Spec.Template.Annotations["radix-update-time"]
	assert.NotEqual(t, "", secondDeploymentUpdateTime)
	assert.NotEqual(t, firstDeploymentUpdateTime, secondDeploymentUpdateTime)
	assert.True(t, firstDeploymentUpdateTime < secondDeploymentUpdateTime)

	teardownTest()
}

func TestObjectUpdated_ZeroReplicasExistsAndNotSpecifiedReplicas_SetsDefaultReplicaCount(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithReplicas(test.IntPtr(0))))

	time.Sleep(1 * time.Second)
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(0), *deployments.Items[0].Spec.Replicas)

	applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app")))

	deployments, _ = client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(1), *deployments.Items[0].Spec.Replicas)

	teardownTest()
}

func TestObjectUpdated_MultipleReplicasExistsAndNotSpecifiedReplicas_SetsDefaultReplicaCount(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	envNamespace := utils.GetEnvironmentNamespace("anyapp", "test")

	// Test
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app").
				WithReplicas(test.IntPtr(3))))

	time.Sleep(1 * time.Second)
	deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(3), *deployments.Items[0].Spec.Replicas)

	applyDeploymentUpdateWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithDeploymentName("a_deployment_name").
		WithAppName("anyapp").
		WithEnvironment("test").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("app")))

	deployments, _ = client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, int32(1), *deployments.Items[0].Spec.Replicas)

	teardownTest()
}

func TestObjectUpdated_WithAppAliasRemoved_AliasIngressIsCorrectlyReconciled(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true)))

	// Test
	ingresses, _ := client.NetworkingV1beta1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("any-app-url-alias", ingresses), "App should have had an app alias ingress")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have another external alias")

	// Remove app alias from dev
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(false)))

	ingresses, _ = client.NetworkingV1beta1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "Cluster specific ingress for public component should exist")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have another external alias")

	teardownTest()
}

func TestObjectSynced_MultiComponentToOneComponent_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

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
				WithPublicPort("http").
				WithSecrets([]string{"a_secret"})))

	assert.NoError(t, err)

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
				WithReplicas(test.IntPtr(0))))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	t.Run("validate deploy", func(t *testing.T) {
		t.Parallel()
		deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
		expectedDeployments := getDeploymentsForRadixComponents(&deployments.Items)
		assert.Equal(t, 1, len(expectedDeployments), "Number of deployments wasn't as expected")
		assert.Equal(t, componentTwoName, deployments.Items[0].Name, "app deployment not there")
	})

	t.Run("validate service", func(t *testing.T) {
		t.Parallel()
		services, _ := client.CoreV1().Services(envNamespace).List(metav1.ListOptions{})
		expectedServices := getServicesForRadixComponents(&services.Items)
		assert.Equal(t, 1, len(expectedServices), "Number of services wasn't as expected")
	})

	t.Run("validate ingress", func(t *testing.T) {
		t.Parallel()
		ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(ingresses.Items), "Number of ingresses was not according to public components")
	})

	t.Run("validate secrets", func(t *testing.T) {
		t.Parallel()
		secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(secrets.Items), "Number of secrets was not according to spec")
		assert.Equal(t, utils.GetComponentSecretName(componentThreeName), secrets.Items[0].GetName(), "Component secret is not as expected")
	})

	t.Run("validate service accounts", func(t *testing.T) {
		t.Parallel()
		serviceAccounts, _ := client.CoreV1().ServiceAccounts(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(serviceAccounts.Items), "Number of service accounts was not expected")
	})

	t.Run("validate rolebindings", func(t *testing.T) {
		t.Parallel()
		rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 0, len(rolebindings.Items), "Number of rolebindings was not expected")
	})

	teardownTest()
}

func TestObjectSynced_PublicToNonPublic_HandlesChange(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

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

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
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

	assert.NoError(t, err)
	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
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

	assert.NoError(t, err)

	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "No component should be public")

	teardownTest()
}

func TestConstructForTargetEnvironment_PicksTheCorrectEnvironmentConfig(t *testing.T) {
	ra := utils.ARadixApplication().
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithAlwaysPullImageOnDeploy(true).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("DB_HOST", "db-prod").
						WithEnvironmentVariable("DB_PORT", "1234").
						WithResource(map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}, map[string]string{
							"memory": "128Mi",
							"cpu":    "500m",
						}).
						WithReplicas(test.IntPtr(4)),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("DB_HOST", "db-dev").
						WithEnvironmentVariable("DB_PORT", "9876").
						WithResource(map[string]string{
							"memory": "32Mi",
							"cpu":    "125m",
						}, map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}).
						WithVolumeMounts([]v1.RadixVolumeMount{
							{
								Type:      v1.MountTypeBlob,
								Container: "some-container",
								Path:      "some-path",
							},
						}).
						WithReplicas(test.IntPtr(3)))).
		BuildRA()

	var testScenarios = []struct {
		environment                  string
		expectedReplicas             int
		expectedDbHost               string
		expectedDbPort               string
		expectedMemoryLimit          string
		expectedCPULimit             string
		expectedMemoryRequest        string
		expectedCPURequest           string
		expectedNumberOfVolumeMounts int
		alwaysPullImageOnDeploy      bool
	}{
		{"prod", 4, "db-prod", "1234", "128Mi", "500m", "64Mi", "250m", 0, true},
		{"dev", 3, "db-dev", "9876", "64Mi", "250m", "32Mi", "125m", 1, true},
	}

	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: "anyImage", ImagePath: "anyImagePath"}

	for _, testcase := range testScenarios {
		t.Run(testcase.environment, func(t *testing.T) {

			rd, _ := ConstructForTargetEnvironment(ra, "anyjob", "anyimageTag", "anybranch", "anycommit", componentImages, testcase.environment)

			assert.Equal(t, testcase.expectedReplicas, *rd.Spec.Components[0].Replicas, "Number of replicas wasn't as expected")
			assert.Equal(t, testcase.expectedDbHost, rd.Spec.Components[0].EnvironmentVariables["DB_HOST"])
			assert.Equal(t, testcase.expectedDbPort, rd.Spec.Components[0].EnvironmentVariables["DB_PORT"])
			assert.Equal(t, testcase.expectedMemoryLimit, rd.Spec.Components[0].Resources.Limits["memory"])
			assert.Equal(t, testcase.expectedCPULimit, rd.Spec.Components[0].Resources.Limits["cpu"])
			assert.Equal(t, testcase.expectedMemoryRequest, rd.Spec.Components[0].Resources.Requests["memory"])
			assert.Equal(t, testcase.expectedCPURequest, rd.Spec.Components[0].Resources.Requests["cpu"])
			assert.Equal(t, testcase.expectedCPURequest, rd.Spec.Components[0].Resources.Requests["cpu"])
			assert.Equal(t, testcase.alwaysPullImageOnDeploy, rd.Spec.Components[0].AlwaysPullImageOnDeploy)
			assert.Equal(t, testcase.expectedNumberOfVolumeMounts, len(rd.Spec.Components[0].VolumeMounts))
		})
	}

}

func TestConstructForTargetEnvironment_AlwaysPullImageOnDeployOverride(t *testing.T) {
	ra := utils.ARadixApplication().
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithAlwaysPullImageOnDeploy(false).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithAlwaysPullImageOnDeploy(true).
						WithReplicas(test.IntPtr(3)),
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithAlwaysPullImageOnDeploy(false).
						WithReplicas(test.IntPtr(3))),
			utils.AnApplicationComponent().
				WithName("app1").
				WithAlwaysPullImageOnDeploy(true).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithAlwaysPullImageOnDeploy(true).
						WithReplicas(test.IntPtr(3)),
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithAlwaysPullImageOnDeploy(false).
						WithReplicas(test.IntPtr(3))),
			utils.AnApplicationComponent().
				WithName("app2").
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithReplicas(test.IntPtr(3)))).
		BuildRA()

	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["app"] = pipeline.ComponentImage{ImageName: "anyImage", ImagePath: "anyImagePath"}

	rd, _ := ConstructForTargetEnvironment(ra, "anyjob", "anyimageTag", "anybranch", "anycommit", componentImages, "dev")

	t.Log(rd.Spec.Components[0].Name)
	assert.True(t, rd.Spec.Components[0].AlwaysPullImageOnDeploy)
	t.Log(rd.Spec.Components[1].Name)
	assert.True(t, rd.Spec.Components[1].AlwaysPullImageOnDeploy)
	t.Log(rd.Spec.Components[2].Name)
	assert.False(t, rd.Spec.Components[2].AlwaysPullImageOnDeploy)

	rd, _ = ConstructForTargetEnvironment(ra, "anyjob", "anyimageTag", "anybranch", "anycommit", componentImages, "prod")

	t.Log(rd.Spec.Components[0].Name)
	assert.False(t, rd.Spec.Components[0].AlwaysPullImageOnDeploy)
	t.Log(rd.Spec.Components[1].Name)
	assert.False(t, rd.Spec.Components[1].AlwaysPullImageOnDeploy)
}

func TestObjectSynced_PublicPort_OldPublic(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"

	// New publicPort exists, old public does not exist
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("http").
				WithPublic(false)))

	assert.NoError(t, err)
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	assert.Equal(t, 80, ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntValue())

	// New publicPort exists, old public exists (ignored)
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("http").
				WithPublic(true)))

	assert.NoError(t, err)
	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Component should be public")
	assert.Equal(t, 80, ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntValue())

	// New publicPort does not exist, old public does not exist
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("").
				WithPublic(false)))

	assert.NoError(t, err)
	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(ingresses.Items), "Component should not be public")

	// New publicPort does not exist, old public exists (used)
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("https", 443).
				WithPort("http", 80).
				WithPublicPort("https").
				WithPublic(true)))

	assert.NoError(t, err)
	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	expectedIngresses := getIngressesForRadixComponents(&ingresses.Items)
	assert.Equal(t, 1, len(expectedIngresses), "Component should be public")
	actualPortValue := ingresses.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntValue()
	assert.Equal(t, 443, actualPortValue)

	teardownTest()
}

func getIngressesForRadixComponents(ingresses *[]networkingv1beta1.Ingress) []networkingv1beta1.Ingress {
	var result []networkingv1beta1.Ingress
	for _, ing := range *ingresses {
		if val, ok := ing.Labels["radix-component"]; ok && val != "job" {
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

	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com")))

	// Test
	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	roles, _ := client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	rolebindings, _ := client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})

	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	assert.Equal(t, 1, len(roles.Items), "Environment should have one role for TLS cert")
	assert.True(t, roleByNameExists("radix-app-adm-frontend", roles), "Expected role radix-app-adm-frontend to be there to access secrets for TLS certificates")

	assert.Equal(t, 1, len(rolebindings.Items), "Environment should have one rolebinding for TLS cert")
	assert.True(t, roleBindingByNameExists("radix-app-adm-frontend", rolebindings), "Expected rolebinding radix-app-adm-app to be there to access secrets for TLS certificates")

	assert.Equal(t, 1, len(secrets.Items), "Environment should have one secret for TLS cert")
	assert.True(t, secretByNameExists("some.alias.com", secrets), "TLS certificate for external alias is not properly defined")

	// Remove app alias from dev
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithJobComponents().
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http")))

	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	secrets, _ = client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	rolebindings, _ = client.RbacV1().RoleBindings(envNamespace).List(metav1.ListOptions{})

	assert.Equal(t, 2, len(ingresses.Items), "External alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	assert.Equal(t, 0, len(rolebindings.Items), "Role should have been removed")
	assert.Equal(t, 0, len(rolebindings.Items), "Rolebinding should have been removed")
	assert.Equal(t, 0, len(secrets.Items), "Secret should have been removed")

}

func TestObjectUpdated_WithOneExternalAliasRemovedOrModified_AllChangesPropelyReconciled(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Setup
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com").
				WithSecrets([]string{"a_secret"})))

	// Test
	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 4, len(ingresses.Items), "Environment should have four ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	externalAliasIngress := getIngressByName("some.alias.com", ingresses)
	assert.Equal(t, "some.alias.com", externalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8080), externalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	anotherExternalAliasIngress := getIngressByName("another.alias.com", ingresses)
	assert.Equal(t, "another.alias.com", anotherExternalAliasIngress.GetName(), "App should have had another external alias ingress")
	assert.Equal(t, "another.alias.com", anotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8080), anotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	roles, _ := client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "some.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for external alias")
	assert.Equal(t, "another.alias.com", roles.Items[0].Rules[0].ResourceNames[2], "Expected role should be able to access TLS certificate for second external alias")

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("yet.another.alias.com").
				WithSecrets([]string{"a_secret"})))

	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 4, len(ingresses.Items), "Environment should have four ingresses")
	assert.Truef(t, ingressByNameExists("some.alias.com", ingresses), "App should have had an external alias ingress")
	assert.Truef(t, ingressByNameExists("yet.another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	externalAliasIngress = getIngressByName("some.alias.com", ingresses)
	assert.Equal(t, "some.alias.com", externalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), externalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	yetAnotherExternalAliasIngress := getIngressByName("yet.another.alias.com", ingresses)
	assert.Equal(t, "yet.another.alias.com", yetAnotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), yetAnotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	roles, _ = client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "some.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for external alias")
	assert.Equal(t, "yet.another.alias.com", roles.Items[0].Rules[0].ResourceNames[2], "Expected role should be able to access TLS certificate for second external alias")

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8081).
				WithPublicPort("http").
				WithDNSExternalAlias("yet.another.alias.com").
				WithSecrets([]string{"a_secret"})))

	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 3, len(ingresses.Items), "Environment should have three ingresses")
	assert.Truef(t, ingressByNameExists("yet.another.alias.com", ingresses), "App should have had another external alias ingress")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	yetAnotherExternalAliasIngress = getIngressByName("yet.another.alias.com", ingresses)
	assert.Equal(t, "yet.another.alias.com", yetAnotherExternalAliasIngress.Spec.Rules[0].Host, "App should have an external alias")
	assert.Equal(t, int32(8081), yetAnotherExternalAliasIngress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntVal, "Correct service port")

	roles, _ = client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(roles.Items[0].Rules[0].ResourceNames))
	assert.Equal(t, "yet.another.alias.com", roles.Items[0].Rules[0].ResourceNames[1], "Expected role should be able to access TLS certificate for second external alias")

	// Remove app alias from dev
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http")))

	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "External alias ingress should have been removed")
	assert.Truef(t, ingressByNameExists("frontend-active-cluster-url-alias", ingresses), "App should have active cluster alias")
	assert.Truef(t, ingressByNameExists("frontend", ingresses), "App should have cluster specific alias")

	roles, _ = client.RbacV1().Roles(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 0, len(roles.Items), "Role should have been removed")

}

func TestFixedAliasIngress_ActiveCluster(t *testing.T) {
	anyAppName := "any-app"
	anyEnvironment := "dev"
	anyComponentName := "frontend"
	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	radixDeployBuilder := utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http"))

	// Current cluster is active cluster
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)

	ingresses, _ := client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 2, len(ingresses.Items), "Environment should have two ingresses")
	assert.False(t, strings.Contains(ingresses.Items[0].Spec.Rules[0].Host, clusterName))
	assert.True(t, strings.Contains(ingresses.Items[1].Spec.Rules[0].Host, clusterName))

	// Current cluster is not active cluster
	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, "newClusterName")
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)
	ingresses, _ = client.NetworkingV1beta1().Ingresses(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, 1, len(ingresses.Items), "Environment should have one ingresses")
	assert.True(t, strings.Contains(ingresses.Items[0].Spec.Rules[0].Host, clusterName))

	teardownTest()
}

func TestNewDeploymentStatus(t *testing.T) {
	anyApp := "any-app"
	anyEnv := "dev"
	anyComponentName := "frontend"

	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

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
	assert.Equal(t, v1.DeploymentActive, rd.Status.Condition)
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

	rd2, _ := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, radixDeployBuilder)
	rd, _ = getUpdatedRD(radixclient, rd)

	assert.Equal(t, v1.DeploymentInactive, rd.Status.Condition)
	assert.Equal(t, rd.Status.ActiveTo, rd2.Status.ActiveFrom)

	assert.Equal(t, v1.DeploymentActive, rd2.Status.Condition)
	assert.True(t, !rd2.Status.ActiveFrom.IsZero())
}

func Test_AddMultipleNewDeployments_CorrectStatuses(t *testing.T) {
	anyApp := "any-app"
	anyEnv := "dev"
	anyComponentName := "frontend"
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	rd1 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, prometheusclient)

	time.Sleep(2 * time.Millisecond)
	rd2 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, prometheusclient)
	rd1, _ = getUpdatedRD(radixclient, rd1)

	assert.Equal(t, v1.DeploymentInactive, rd1.Status.Condition)
	assert.Equal(t, rd1.Status.ActiveTo, rd2.Status.ActiveFrom)
	assert.Equal(t, v1.DeploymentActive, rd2.Status.Condition)
	assert.True(t, !rd2.Status.ActiveFrom.IsZero())

	time.Sleep(3 * time.Millisecond)
	rd3 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, prometheusclient)
	rd1, _ = getUpdatedRD(radixclient, rd1)
	rd2, _ = getUpdatedRD(radixclient, rd2)

	assert.Equal(t, v1.DeploymentInactive, rd1.Status.Condition)
	assert.Equal(t, v1.DeploymentInactive, rd2.Status.Condition)
	assert.Equal(t, rd1.Status.ActiveTo, rd2.Status.ActiveFrom)
	assert.Equal(t, rd2.Status.ActiveTo, rd3.Status.ActiveFrom)
	assert.Equal(t, v1.DeploymentActive, rd3.Status.Condition)
	assert.True(t, !rd3.Status.ActiveFrom.IsZero())

	time.Sleep(4 * time.Millisecond)
	rd4 := addRadixDeployment(anyApp, anyEnv, anyComponentName, tu, client, kubeUtil, radixclient, prometheusclient)
	rd1, _ = getUpdatedRD(radixclient, rd1)
	rd2, _ = getUpdatedRD(radixclient, rd2)
	rd3, _ = getUpdatedRD(radixclient, rd3)

	assert.Equal(t, v1.DeploymentInactive, rd1.Status.Condition)
	assert.Equal(t, v1.DeploymentInactive, rd2.Status.Condition)
	assert.Equal(t, v1.DeploymentInactive, rd3.Status.Condition)
	assert.Equal(t, rd1.Status.ActiveTo, rd2.Status.ActiveFrom)
	assert.Equal(t, rd2.Status.ActiveTo, rd3.Status.ActiveFrom)
	assert.Equal(t, rd3.Status.ActiveTo, rd4.Status.ActiveFrom)
	assert.Equal(t, v1.DeploymentActive, rd4.Status.Condition)
	assert.True(t, !rd4.Status.ActiveFrom.IsZero())
}

func getUpdatedRD(radixclient radixclient.Interface, rd *v1.RadixDeployment) (*v1.RadixDeployment, error) {
	return radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(rd.GetName(), metav1.GetOptions{ResourceVersion: rd.ResourceVersion})
}

func addRadixDeployment(anyApp string, anyEnv string, anyComponentName string, tu *test.Utils, client kube.Interface, kubeUtil *kubeUtils.Kube, radixclient radixclient.Interface, prometheusclient prometheusclient.Interface) *v1.RadixDeployment {
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

	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Setup
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com").
				WithSecrets([]string{"a_secret", "another_secret", "a_third_secret"})))

	secrets, _ := client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	anyComponentSecret := secrets.Items[0]
	assert.Equal(t, utils.GetComponentSecretName(anyComponentName), anyComponentSecret.GetName(), "Component secret is not as expected")

	// Secret is initially empty but get filled with data from the API
	assert.Equal(t, []string{}, maps.GetKeysFromByteMap(anyComponentSecret.Data), "Component secret data is not as expected")

	// Will emulate that data is set from the API
	anySecretValue := "anySecretValue"
	secretData := make(map[string][]byte)
	secretData["a_secret"] = []byte(anySecretValue)
	secretData["another_secret"] = []byte(anySecretValue)
	secretData["a_third_secret"] = []byte(anySecretValue)

	anyComponentSecret.Data = secretData
	client.CoreV1().Secrets(envNamespace).Update(&anyComponentSecret)

	// Removing one secret from config and therefor from the deployment
	// should cause it to disappear
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(anyComponentName).
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSExternalAlias("some.alias.com").
				WithDNSExternalAlias("another.alias.com").
				WithSecrets([]string{"a_secret", "a_third_secret"})))

	secrets, _ = client.CoreV1().Secrets(envNamespace).List(metav1.ListOptions{})
	anyComponentSecret = secrets.Items[0]
	assert.True(t, utils.ArrayEqualElements([]string{"a_secret", "a_third_secret"}, maps.GetKeysFromByteMap(anyComponentSecret.Data)), "Component secret data is not as expected")
}

func TestHistoryLimit_IsBroken_FixedAmountOfDeployments(t *testing.T) {
	anyAppName := "any-app"
	anyComponentName := "frontend"
	anyEnvironment := "dev"
	anyLimit := 3

	tu, client, kubeUtils, radixclient, prometheusclient := setupTest()

	// Current cluster is active cluster
	os.Setenv(defaults.DeploymentsHistoryLimitEnvironmentVariable, strconv.Itoa(anyLimit))

	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironment)
	applyDeploymentWithSync(tu, client, kubeUtils, radixclient, prometheusclient,
		utils.ARadixDeployment().
			WithDeploymentName("firstdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	applyDeploymentWithSync(tu, client, kubeUtils, radixclient, prometheusclient,
		utils.ARadixDeployment().
			WithDeploymentName("seconddeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	applyDeploymentWithSync(tu, client, kubeUtils, radixclient, prometheusclient,
		utils.ARadixDeployment().
			WithDeploymentName("thirddeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	applyDeploymentWithSync(tu, client, kubeUtils, radixclient, prometheusclient,
		utils.ARadixDeployment().
			WithDeploymentName("fourthdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	deployments, _ := radixclient.RadixV1().RadixDeployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(deployments.Items), "Number of deployments should match limit")

	assert.False(t, radixDeploymentByNameExists("firstdeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("seconddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("thirddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fourthdeployment", deployments))

	applyDeploymentWithSync(tu, client, kubeUtils, radixclient, prometheusclient,
		utils.ARadixDeployment().
			WithDeploymentName("fifthdeployment").
			WithAppName(anyAppName).
			WithEnvironment(anyEnvironment).
			WithComponents(
				utils.NewDeployComponentBuilder().
					WithName(anyComponentName).
					WithPort("http", 8080).
					WithPublicPort("http")))

	deployments, _ = radixclient.RadixV1().RadixDeployments(envNamespace).List(metav1.ListOptions{})
	assert.Equal(t, anyLimit, len(deployments.Items), "Number of deployments should match limit")

	assert.False(t, radixDeploymentByNameExists("firstdeployment", deployments))
	assert.False(t, radixDeploymentByNameExists("seconddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("thirddeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fourthdeployment", deployments))
	assert.True(t, radixDeploymentByNameExists("fifthdeployment", deployments))

	teardownTest()
}

func TestObjectUpdated_WithIngressConfig_AnnotationIsPutOnIngresses(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	// Setup
	client.CoreV1().ConfigMaps(corev1.NamespaceDefault).Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressConfigurationMap,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string]string{
			"ingressConfiguration": testIngressConfiguration,
		},
	})

	os.Setenv(defaults.ActiveClusternameEnvironmentVariable, clusterName)
	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName("any-app").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithIngressConfiguration("non-existing")))

	applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName("any-app-2").
		WithEnvironment("dev").
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true).
				WithIngressConfiguration("socket")))

	// Test
	ingresses, _ := client.NetworkingV1beta1().Ingresses(utils.GetEnvironmentNamespace("any-app", "dev")).List(metav1.ListOptions{})
	appAliasIngress := getIngressByName("any-app-url-alias", ingresses)
	clusterSpecificIngress := getIngressByName("frontend", ingresses)
	activeClusterIngress := getIngressByName("frontend-active-cluster-url-alias", ingresses)
	assert.Equal(t, 2, len(appAliasIngress.ObjectMeta.Annotations))
	assert.Equal(t, 2, len(clusterSpecificIngress.ObjectMeta.Annotations))
	assert.Equal(t, 2, len(activeClusterIngress.ObjectMeta.Annotations))

	ingresses, _ = client.NetworkingV1beta1().Ingresses(utils.GetEnvironmentNamespace("any-app-2", "dev")).List(metav1.ListOptions{})
	appAliasIngress = getIngressByName("any-app-2-url-alias", ingresses)
	clusterSpecificIngress = getIngressByName("frontend", ingresses)
	activeClusterIngress = getIngressByName("frontend-active-cluster-url-alias", ingresses)
	assert.Equal(t, 5, len(appAliasIngress.ObjectMeta.Annotations))
	assert.Equal(t, 5, len(clusterSpecificIngress.ObjectMeta.Annotations))
	assert.Equal(t, 5, len(activeClusterIngress.ObjectMeta.Annotations))

}

func TestHPAConfig(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

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
				WithHorizontalScaling(&minReplicas, maxReplicas),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas)))

	assert.NoError(t, err)

	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	t.Run("validate hpas", func(t *testing.T) {
		hpas, _ := client.AutoscalingV1().HorizontalPodAutoscalers(envNamespace).List(metav1.ListOptions{})
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
				WithHorizontalScaling(&minReplicas, maxReplicas),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1)).
				WithHorizontalScaling(&minReplicas, maxReplicas),
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("http", 6379).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1))))

	assert.NoError(t, err)

	t.Run("validate hpas after reconfiguration", func(t *testing.T) {
		hpas, _ := client.AutoscalingV1().HorizontalPodAutoscalers(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(hpas.Items), "Number of horizontal pod autoscalers wasn't as expected")
		assert.False(t, hpaByNameExists(componentOneName, hpas), "componentOneName horizontal pod autoscaler should not exist")
		assert.True(t, hpaByNameExists(componentTwoName, hpas), "componentTwoName horizontal pod autoscaler should exist")
		assert.False(t, hpaByNameExists(componentThreeName, hpas), "componentThreeName horizontal pod autoscaler should not exist")
		assert.Equal(t, int32(2), *getHPAByName(componentTwoName, hpas).Spec.MinReplicas, "componentTwoName horizontal pod autoscaler config is incorrect")
	})

}

func TestNonRootOverrideFlag(t *testing.T) {

	for _, runAsNonRoot := range []bool{true, false} {
		componentName := utils.TernaryString(runAsNonRoot, "root-component", "non-root-component")
		tu, client, kubeUtil, radixclient, prometheusclient := setupTest()
		_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
			WithAppName("radix-root-example").
			WithEnvironment("test").
			WithComponent(utils.NewDeployComponentBuilder().
				WithRunAsNonRoot(runAsNonRoot).
				WithPort("http", 8080).
				WithName(componentName).
				WithPublicPort("http").
				WithReplicas(test.IntPtr(1))))

		assert.NoError(t, err)

		t.Run(fmt.Sprintf("%s has correct security context", componentName), func(t *testing.T) {
			envNamespace := utils.GetEnvironmentNamespace("radix-root-example", "test")
			deployments, _ := client.AppsV1().Deployments(envNamespace).List(metav1.ListOptions{})
			expectedSecurityContext := getSecurityContextForPod(runAsNonRoot)
			assert.Equal(t, expectedSecurityContext, getDeploymentByName(componentName, deployments).Spec.Template.Spec.SecurityContext)
			assert.Equal(t, runAsNonRoot, *getContainerByName(componentName, getDeploymentByName(componentName, deployments).Spec.Template.Spec.Containers).SecurityContext.RunAsNonRoot)
		})

	}

}

func TestMonitoringConfig(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

	anyAppName := "anyappname"
	anyEnvironmentName := "test"
	componentOneName := "componentOneName"
	componentTwoName := "componentTwoName"
	componentThreeName := "componentThreeName"

	// Test
	_, err := applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("httpa", 8000).
				WithMonitoring(true),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("httpb", 8001).
				WithMonitoring(true)))

	assert.NoError(t, err)

	serviceMonitorTestFunc := func(t *testing.T, componentName, portName string, serviceMonitor *monitoringv1.ServiceMonitor) {
		assert.Equal(t, portName, serviceMonitor.Spec.Endpoints[0].Port)
		assert.Equal(t, fmt.Sprintf("%s-%s-%s", anyAppName, anyEnvironmentName, componentName), serviceMonitor.Spec.JobLabel)
		assert.Len(t, serviceMonitor.Spec.NamespaceSelector.MatchNames, 1)
		assert.Equal(t, fmt.Sprintf("%s-%s", anyAppName, anyEnvironmentName), serviceMonitor.Spec.NamespaceSelector.MatchNames[0])
		assert.Len(t, serviceMonitor.Spec.Selector.MatchLabels, 1)
		assert.Equal(t, componentName, serviceMonitor.Spec.Selector.MatchLabels[kubeUtils.RadixComponentLabel])
	}

	envNamespace := utils.GetEnvironmentNamespace(anyAppName, anyEnvironmentName)
	t.Run("validate service montitors", func(t *testing.T) {
		servicemonitors, _ := prometheusclient.MonitoringV1().ServiceMonitors(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 2, len(servicemonitors.Items), "Number of service monitors was not as expected")
		assert.True(t, serviceMonitorByNameExists(componentOneName, servicemonitors), "componentTwoName service monitor should exist")
		assert.True(t, serviceMonitorByNameExists(componentTwoName, servicemonitors), "componentTwoName service monitor should exist")

		serviceMonitor := getServiceMonitorByName(componentOneName, servicemonitors)
		serviceMonitorTestFunc(t, componentOneName, "httpa", serviceMonitor)
		serviceMonitor = getServiceMonitorByName(componentTwoName, servicemonitors)
		serviceMonitorTestFunc(t, componentTwoName, "httpb", serviceMonitor)
	})

	// Test - rename port names
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentOneName).
				WithPort("metrica", 8000).
				WithMonitoring(true),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("metricb", 8001).
				WithMonitoring(true)))

	assert.NoError(t, err)

	t.Run("validate service monitoring after renaming port", func(t *testing.T) {
		servicemonitors, _ := prometheusclient.MonitoringV1().ServiceMonitors(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 2, len(servicemonitors.Items), "Number of service monitors was not as expected")
		assert.True(t, serviceMonitorByNameExists(componentOneName, servicemonitors), "componentTwoName service monitor should exist")
		assert.True(t, serviceMonitorByNameExists(componentTwoName, servicemonitors), "componentTwoName service monitor should exist")

		serviceMonitor := getServiceMonitorByName(componentOneName, servicemonitors)
		serviceMonitorTestFunc(t, componentOneName, "metrica", serviceMonitor)
		serviceMonitor = getServiceMonitorByName(componentTwoName, servicemonitors)
		serviceMonitorTestFunc(t, componentTwoName, "metricb", serviceMonitor)
	})

	// Test - disable monitoring for component two, remove component one, add component three
	_, err = applyDeploymentWithSync(tu, client, kubeUtil, radixclient, prometheusclient, utils.ARadixDeployment().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironmentName).
		WithComponents(
			utils.NewDeployComponentBuilder().
				WithName(componentThreeName).
				WithPort("metricx", 8000).
				WithMonitoring(true),
			utils.NewDeployComponentBuilder().
				WithName(componentTwoName).
				WithPort("metricb", 8001).
				WithMonitoring(false)))

	assert.NoError(t, err)

	t.Run("validate hpas after deleting component one and adding component three", func(t *testing.T) {
		servicemonitors, _ := prometheusclient.MonitoringV1().ServiceMonitors(envNamespace).List(metav1.ListOptions{})
		assert.Equal(t, 1, len(servicemonitors.Items), "Number of service monitors was not as expected")
		assert.False(t, serviceMonitorByNameExists(componentOneName, servicemonitors), "componentOneName service monitor not should exist")
		assert.False(t, serviceMonitorByNameExists(componentTwoName, servicemonitors), "componentTwoName service monitor not should exist")
		assert.True(t, serviceMonitorByNameExists(componentThreeName, servicemonitors), "componentThreeName service monitor should exist")

		serviceMonitor := getServiceMonitorByName(componentThreeName, servicemonitors)
		serviceMonitorTestFunc(t, componentThreeName, "metricx", serviceMonitor)
	})
}

func TestObjectUpdated_UpdatePort_DeploymentPodPortSpecIsCorrect(t *testing.T) {
	tu, kubeclient, kubeUtil, radixclient, prometheusclient := setupTest()

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

	assert.Nil(t, err)
	deployments, _ := kubeclient.AppsV1().Deployments("app-env").List(metav1.ListOptions{})
	comp := getDeploymentByName("comp", deployments)
	assert.Len(t, comp.Spec.Template.Spec.Containers[0].Ports, 2)
	portTestFunc("port1", 8001, comp.Spec.Template.Spec.Containers[0].Ports)
	portTestFunc("port2", 8002, comp.Spec.Template.Spec.Containers[0].Ports)
	job := getDeploymentByName("job", deployments)
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

	assert.Nil(t, err)
	deployments, _ = kubeclient.AppsV1().Deployments("app-env").List(metav1.ListOptions{})
	comp = getDeploymentByName("comp", deployments)
	assert.Len(t, comp.Spec.Template.Spec.Containers[0].Ports, 1)
	portTestFunc("port2", 9002, comp.Spec.Template.Spec.Containers[0].Ports)
	job = getDeploymentByName("job", deployments)
	assert.Len(t, job.Spec.Template.Spec.Containers[0].Ports, 1)
	portTestFunc("scheduler-port", 9090, job.Spec.Template.Spec.Containers[0].Ports)
}

func TestUseGpuNode(t *testing.T) {
	tu, client, kubeUtil, radixclient, prometheusclient := setupTest()

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

func parseQuantity(value string) resource.Quantity {
	q, _ := resource.ParseQuantity(value)
	return q
}

func applyDeploymentWithSync(tu *test.Utils, kubeclient kube.Interface, kubeUtil *kubeUtils.Kube,
	radixclient radixclient.Interface, prometheusclient prometheusclient.Interface, deploymentBuilder utils.DeploymentBuilder) (*v1.RadixDeployment, error) {
	rd, err := tu.ApplyDeployment(deploymentBuilder)
	if err != nil {
		return nil, err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	deployment, err := NewDeployment(kubeclient, kubeUtil, radixclient, prometheusclient, radixRegistration, rd)
	err = deployment.OnSync()
	if err != nil {
		return nil, err
	}

	updatedRD, err := radixclient.RadixV1().RadixDeployments(rd.GetNamespace()).Get(rd.GetName(), metav1.GetOptions{})
	return updatedRD, err
}

func applyDeploymentUpdateWithSync(tu *test.Utils, client kube.Interface, kubeUtil *kubeUtils.Kube,
	radixclient radixclient.Interface, prometheusclient prometheusclient.Interface, deploymentBuilder utils.DeploymentBuilder) error {
	rd, err := tu.ApplyDeploymentUpdate(deploymentBuilder)
	if err != nil {
		return err
	}

	radixRegistration, err := radixclient.RadixV1().RadixRegistrations().Get(rd.Spec.AppName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment, err := NewDeployment(client, kubeUtil, radixclient, prometheusclient, radixRegistration, rd)
	err = deployment.OnSync()
	if err != nil {
		return err
	}

	return nil
}

func envVariableByNameExistOnDeployment(name, deploymentName string, deployments *appsv1.DeploymentList) bool {
	return envVariableByNameExist(name, getContainerByName(deploymentName, getDeploymentByName(deploymentName, deployments).Spec.Template.Spec.Containers).Env)
}

func getEnvVariableByNameOnDeployment(name, deploymentName string, deployments *appsv1.DeploymentList) string {
	return getEnvVariableByName(name, getContainerByName(deploymentName, getDeploymentByName(deploymentName, deployments).Spec.Template.Spec.Containers).Env)
}

func radixDeploymentByNameExists(name string, deployments *v1.RadixDeploymentList) bool {
	return getRadixDeploymentByName(name, deployments) != nil
}

func getRadixDeploymentByName(name string, deployments *v1.RadixDeploymentList) *v1.RadixDeployment {
	for _, deployment := range deployments.Items {
		if deployment.Name == name {
			return &deployment
		}
	}

	return nil
}

func deploymentByNameExists(name string, deployments *appsv1.DeploymentList) bool {
	return getDeploymentByName(name, deployments) != nil
}

func getDeploymentByName(name string, deployments *appsv1.DeploymentList) *appsv1.Deployment {
	for _, deployment := range deployments.Items {
		if deployment.Name == name {
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

func getEnvVariableByName(name string, envVars []corev1.EnvVar) string {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return envVar.Value
		}
	}

	return ""
}

func hpaByNameExists(name string, hpas *autoscalingv1.HorizontalPodAutoscalerList) bool {
	for _, hpa := range hpas.Items {
		if hpa.Name == name {
			return true
		}
	}

	return false
}

func getHPAByName(name string, hpas *autoscalingv1.HorizontalPodAutoscalerList) *autoscalingv1.HorizontalPodAutoscaler {
	for _, hpa := range hpas.Items {
		if hpa.Name == name {
			return &hpa
		}
	}

	return nil
}

func serviceMonitorByNameExists(name string, serviceMonitors *monitoringv1.ServiceMonitorList) bool {
	for _, serviceMonitor := range serviceMonitors.Items {
		if serviceMonitor.Name == name {
			return true
		}
	}

	return false
}

func getServiceMonitorByName(name string, serviceMonitors *monitoringv1.ServiceMonitorList) *monitoringv1.ServiceMonitor {
	for _, serviceMonitor := range serviceMonitors.Items {
		if serviceMonitor.Name == name {
			return serviceMonitor
		}
	}

	return nil
}

func serviceByNameExists(name string, services *corev1.ServiceList) bool {
	for _, service := range services.Items {
		if service.Name == name {
			return true
		}
	}

	return false
}

func getIngressByName(name string, ingresses *networkingv1beta1.IngressList) *networkingv1beta1.Ingress {
	for _, ingress := range ingresses.Items {
		if ingress.Name == name {
			return &ingress
		}
	}

	return nil
}

func ingressByNameExists(name string, ingresses *networkingv1beta1.IngressList) bool {
	ingress := getIngressByName(name, ingresses)
	if ingress != nil {
		return true
	}

	return false
}

func getRoleByName(name string, roles *rbacv1.RoleList) *rbacv1.Role {
	for _, role := range roles.Items {
		if role.Name == name {
			return &role
		}
	}

	return nil
}

func roleByNameExists(name string, roles *rbacv1.RoleList) bool {
	role := getRoleByName(name, roles)
	if role != nil {
		return true
	}

	return false
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
	secret := getSecretByName(name, secrets)
	if secret != nil {
		return true
	}

	return false
}

func getRoleBindingByName(name string, roleBindings *rbacv1.RoleBindingList) *rbacv1.RoleBinding {
	for _, roleBinding := range roleBindings.Items {
		if roleBinding.Name == name {
			return &roleBinding
		}
	}

	return nil
}

func roleBindingByNameExists(name string, roleBindings *rbacv1.RoleBindingList) bool {
	role := getRoleBindingByName(name, roleBindings)
	if role != nil {
		return true
	}

	return false
}

func getPortByName(name string, ports []corev1.ContainerPort) *corev1.ContainerPort {
	for _, port := range ports {
		if port.Name == name {
			return &port
		}
	}
	return nil
}
