package steps_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	commonTest "github.com/equinor/radix-operator/pkg/apis/test"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

const (
	anyAppName  = "any-app"
	anyJobName  = "any-job-name"
	anyImageTag = "anytag"
	anyCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"
	anyGitTags  = "some tags go here"
)

func setupTest(t *testing.T) (*kubernetes.Clientset, *kube.Kube, *radix.Clientset, commonTest.Utils) {
	// Setup
	kubeclient := kubernetes.NewSimpleClientset()
	radixclient := radix.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	testUtils := commonTest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := testUtils.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	require.NoError(t, err)
	kubeUtil, _ := kube.New(kubeclient, radixclient, kedaClient, secretproviderclient)

	return kubeclient, kubeUtil, radixclient, testUtils
}

// FakeNamespaceWatcher Unit tests doesn't handle multi-threading well
type FakeNamespaceWatcher struct {
}

// FakeRadixDeploymentWatcher Unit tests doesn't handle multi-threading well
type FakeRadixDeploymentWatcher struct {
}

// WaitFor Waits for namespace to appear
func (watcher FakeNamespaceWatcher) WaitFor(_ context.Context, _ string) error {
	return nil
}

// WaitFor Waits for radix deployment gets active
func (watcher FakeRadixDeploymentWatcher) WaitForActive(_, _ string) error {
	return nil
}

func TestDeploy_BranchIsNotMapped_ShouldSkip(t *testing.T) {
	kubeclient, kubeUtil, radixclient, _ := setupTest(t)

	anyBranch := "master"
	anyEnvironment := "dev"
	anyComponentName := "app"
	anyNoMappedBranch := "feature"

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, anyBranch).
		WithComponents(
			utils.AnApplicationComponent().
				WithName(anyComponentName)).
		BuildRA()

	cli := steps.NewDeployStep(FakeNamespaceWatcher{}, FakeRadixDeploymentWatcher{})
	cli.Init(kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, rr)

	targetEnvs := application.GetTargetEnvironments(anyNoMappedBranch, ra)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   anyNoMappedBranch,
			CommitID: anyCommitID,
		},
		TargetEnvironments: targetEnvs,
	}

	err := cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)
	radixJobList, err := radixclient.RadixV1().RadixJobs(utils.GetAppNamespace(anyAppName)).List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Empty(t, radixJobList.Items)
}

func TestDeploy_PromotionSetup_ShouldCreateNamespacesForAllBranchesIfNotExists(t *testing.T) {
	kubeclient, kubeUtil, radixclient, _ := setupTest(t)

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	certificateVerification := v1.VerificationTypeOptional

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "").
		WithDNSAppAlias("dev", "app").
		WithComponents(
			utils.AnApplicationComponent().
				WithName("app").
				WithPublicPort("http").
				WithPort("http", 8080).
				WithAuthentication(
					&v1.Authentication{
						ClientCertificate: &v1.ClientCertificate{
							PassCertificateToUpstream: utils.BoolPtr(true),
						},
					},
				).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithReplicas(commonTest.IntPtr(4)),
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithAuthentication(
							&v1.Authentication{
								ClientCertificate: &v1.ClientCertificate{
									Verification:              &certificateVerification,
									PassCertificateToUpstream: utils.BoolPtr(false),
								},
							},
						).
						WithReplicas(commonTest.IntPtr(4))),
			utils.AnApplicationComponent().
				WithName("redis").
				WithPublicPort("").
				WithPort("http", 6379).
				WithAuthentication(
					&v1.Authentication{
						ClientCertificate: &v1.ClientCertificate{
							PassCertificateToUpstream: utils.BoolPtr(true),
						},
					},
				).
				WithEnvironmentConfigs(
					utils.AnEnvironmentConfig().
						WithEnvironment("dev").
						WithEnvironmentVariable("DB_HOST", "db-dev").
						WithEnvironmentVariable("DB_PORT", "1234").
						WithResource(map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}, map[string]string{
							"memory": "128Mi",
							"cpu":    "500m",
						}),
					utils.AnEnvironmentConfig().
						WithEnvironment("prod").
						WithEnvironmentVariable("DB_HOST", "db-prod").
						WithEnvironmentVariable("DB_PORT", "9876").
						WithResource(map[string]string{
							"memory": "64Mi",
							"cpu":    "250m",
						}, map[string]string{
							"memory": "128Mi",
							"cpu":    "500m",
						}),
					utils.AnEnvironmentConfig().
						WithEnvironment("no-existing-env").
						WithEnvironmentVariable("DB_HOST", "db-prod").
						WithEnvironmentVariable("DB_PORT", "9876"))).
		BuildRA()

	// Prometheus don´t contain any fake
	cli := steps.NewDeployStep(FakeNamespaceWatcher{}, FakeRadixDeploymentWatcher{})
	cli.Init(kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, rr)

	dnsConfig := dnsalias.DNSConfig{
		DNSZone:               "dev.radix.equinor.com",
		ReservedAppDNSAliases: dnsalias.AppReservedDNSAlias{"api": "radix-api"},
		ReservedDNSAliases:    []string{"grafana"},
	}
	applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, &dnsConfig)
	targetEnvs := application.GetTargetEnvironments("master", ra)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   "master",
			CommitID: anyCommitID,
		},
		TargetEnvironments: targetEnvs,
		GitCommitHash:      anyCommitID,
		GitTags:            anyGitTags,
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err := cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)
	rds, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").List(context.Background(), metav1.ListOptions{})

	t.Run("validate deploy", func(t *testing.T) {
		assert.NoError(t, err)
		assert.True(t, len(rds.Items) > 0)
	})

	rdNameDev := rds.Items[0].Name

	t.Run("validate deployment exist in only the namespace of the modified branch", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.Background(), rdNameDev, metav1.GetOptions{})
		assert.NotNil(t, rdDev)

		rdProd, _ := radixclient.RadixV1().RadixDeployments("any-app-prod").Get(context.Background(), rdNameDev, metav1.GetOptions{})
		assert.Nil(t, rdProd)
	})

	t.Run("validate deployment environment variables", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.Background(), rdNameDev, metav1.GetOptions{})
		assert.Equal(t, 2, len(rdDev.Spec.Components))
		assert.Equal(t, 4, len(rdDev.Spec.Components[1].EnvironmentVariables))
		assert.Equal(t, "db-dev", rdDev.Spec.Components[1].EnvironmentVariables["DB_HOST"])
		assert.Equal(t, "1234", rdDev.Spec.Components[1].EnvironmentVariables["DB_PORT"])
		assert.Equal(t, anyCommitID, rdDev.Spec.Components[1].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, anyGitTags, rdDev.Spec.Components[1].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])
		assert.NotEmpty(t, rdDev.Annotations[kube.RadixBranchAnnotation])
		assert.NotEmpty(t, rdDev.Labels[kube.RadixCommitLabel])
		assert.NotEmpty(t, rdDev.Labels["radix-job-name"])
		assert.Equal(t, "master", rdDev.Annotations[kube.RadixBranchAnnotation])
		assert.Equal(t, anyCommitID, rdDev.Labels[kube.RadixCommitLabel])
		assert.Equal(t, anyJobName, rdDev.Labels["radix-job-name"])
	})

	t.Run("validate authentication variable", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.Background(), rdNameDev, metav1.GetOptions{})

		x0 := &v1.Authentication{
			ClientCertificate: &v1.ClientCertificate{
				Verification:              &certificateVerification,
				PassCertificateToUpstream: utils.BoolPtr(false),
			},
		}

		x1 := &v1.Authentication{
			ClientCertificate: &v1.ClientCertificate{
				PassCertificateToUpstream: utils.BoolPtr(true),
			},
		}

		assert.NotNil(t, rdDev.Spec.Components[0].Authentication)
		assert.NotNil(t, rdDev.Spec.Components[0].Authentication.ClientCertificate)
		assert.Equal(t, x0, rdDev.Spec.Components[0].Authentication)

		assert.NotNil(t, rdDev.Spec.Components[1].Authentication)
		assert.NotNil(t, rdDev.Spec.Components[1].Authentication.ClientCertificate)
		assert.Equal(t, x1, rdDev.Spec.Components[1].Authentication)
	})

	t.Run("validate dns app alias", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.Background(), rdNameDev, metav1.GetOptions{})
		assert.True(t, rdDev.Spec.Components[0].DNSAppAlias)
		assert.False(t, rdDev.Spec.Components[1].DNSAppAlias)
	})

	t.Run("validate resources", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.Background(), rdNameDev, metav1.GetOptions{})

		fmt.Print(rdDev.Spec.Components[0].Resources)
		fmt.Print(rdDev.Spec.Components[1].Resources)
		assert.NotNil(t, rdDev.Spec.Components[1].Resources)
		assert.Equal(t, "128Mi", rdDev.Spec.Components[1].Resources.Limits["memory"])
		assert.Equal(t, "500m", rdDev.Spec.Components[1].Resources.Limits["cpu"])
	})

}

func TestDeploy_SetCommitID_whenSet(t *testing.T) {
	kubeclient, kubeUtil, radixclient, _ := setupTest(t)

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	// Prometheus don´t contain any fake
	cli := steps.NewDeployStep(FakeNamespaceWatcher{}, FakeRadixDeploymentWatcher{})
	cli.Init(kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, rr)

	applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra, nil)

	const commitID = "222ca8595c5283a9d0f17a623b9255a0d9866a2e"

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   "master",
			CommitID: anyCommitID,
		},
		TargetEnvironments: []string{"master"},
		GitCommitHash:      commitID,
		GitTags:            "",
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err := cli.Run(context.Background(), pipelineInfo)
	require.NoError(t, err)
	rds, err := radixclient.RadixV1().RadixDeployments("any-app-dev").List(context.Background(), metav1.ListOptions{})

	assert.NoError(t, err)
	require.Len(t, rds.Items, 1)
	rd := rds.Items[0]
	assert.Equal(t, commitID, rd.ObjectMeta.Labels[kube.RadixCommitLabel])
}

func TestDeploy_WaitActiveDeployment(t *testing.T) {
	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	envName := "dev"
	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(envName, "master").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	type scenario struct {
		name                     string
		watcherError             error
		expectedRadixDeployments int
	}
	scenarios := []scenario{
		{name: "No fail", watcherError: nil, expectedRadixDeployments: 1},
		{name: "Watch fails", watcherError: errors.New("some error"), expectedRadixDeployments: 0},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(tt *testing.T) {
			kubeclient, kubeUtil, radixClient, _ := setupTest(tt)
			ctrl := gomock.NewController(tt)
			radixDeploymentWatcher := watcher.NewMockRadixDeploymentWatcher(ctrl)
			cli := steps.NewDeployStep(FakeNamespaceWatcher{}, radixDeploymentWatcher)
			cli.Init(kubeclient, radixClient, kubeUtil, &monitoring.Clientset{}, rr)

			applicationConfig := application.NewApplicationConfig(kubeclient, kubeUtil, radixClient, rr, ra, nil)

			pipelineInfo := &model.PipelineInfo{
				PipelineArguments: model.PipelineArguments{
					JobName:  anyJobName,
					ImageTag: anyImageTag,
					Branch:   "master",
				},
				TargetEnvironments: []string{envName},
			}

			pipelineInfo.SetApplicationConfig(applicationConfig)
			namespace := utils.GetEnvironmentNamespace(anyAppName, envName)
			radixDeploymentWatcher.EXPECT().
				WaitForActive(namespace, radixDeploymentNameMatcher{envName: envName, imageTag: anyImageTag}).
				Return(ts.watcherError)
			err := cli.Run(context.Background(), pipelineInfo)
			if ts.watcherError == nil {
				assert.NoError(tt, err)
			} else {
				assert.EqualError(tt, err, ts.watcherError.Error())
			}
			rdList, err := radixClient.RadixV1().RadixDeployments(namespace).List(context.Background(), metav1.ListOptions{})
			assert.NoError(tt, err)
			assert.Len(tt, rdList.Items, ts.expectedRadixDeployments, "Invalid expected RadixDeployment-s count")
		})
	}
}

type radixDeploymentNameMatcher struct {
	envName  string
	imageTag string
}

func (m radixDeploymentNameMatcher) Matches(name interface{}) bool {
	rdName, ok := name.(string)
	return ok && strings.HasPrefix(rdName, m.String())
}
func (m radixDeploymentNameMatcher) String() string {
	return fmt.Sprintf("%s-%s", m.envName, m.imageTag)
}
