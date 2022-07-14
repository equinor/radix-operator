package steps

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"testing"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	anyClusterName       = "AnyClusterName"
	anyContainerRegistry = "any.container.registry"
	anyAppName           = "any-app"
	anyJobName           = "any-job-name"
	anyImageTag          = "anytag"
	anyCommitID          = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"
	anyGitTags           = "some tags go here"
	egressIps            = "0.0.0.0"
)

// FakeNamespaceWatcher Unit tests doesn't handle muliti-threading well
type FakeNamespaceWatcher struct {
}

// WaitFor Waits for namespace to appear
func (watcher FakeNamespaceWatcher) WaitFor(namespace string) error {
	return nil
}

func TestDeploy_BranchIsNotMapped_ShouldSkip(t *testing.T) {
	kubeclient, kubeUtil, radixclient, _, env := setupTest(t)

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

	cli := NewDeployStep(FakeNamespaceWatcher{})
	cli.Init(kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, rr, env)

	applicationConfig, _ := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra)
	branchIsMapped, targetEnvs := applicationConfig.IsThereAnythingToDeploy(anyNoMappedBranch)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   anyNoMappedBranch,
			CommitID: anyCommitID,
		},
		TargetEnvironments: targetEnvs,
		BranchIsMapped:     branchIsMapped,
	}

	err := cli.Run(pipelineInfo)
	assert.Error(t, err)

}

func TestDeploy_PromotionSetup_ShouldCreateNamespacesForAllBranchesIfNotExtists(t *testing.T) {
	kubeclient, kubeUtil, radixclient, _, env := setupTest(t)

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
						WithReplicas(test.IntPtr(4)),
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
						WithRunAsNonRoot(true).
						WithReplicas(test.IntPtr(4))),
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

	// Prometheus doesn´t contain any fake
	cli := NewDeployStep(FakeNamespaceWatcher{})
	cli.Init(kubeclient, radixclient, kubeUtil, &monitoring.Clientset{}, rr, env)

	applicationConfig, _ := application.NewApplicationConfig(kubeclient, kubeUtil, radixclient, rr, ra)
	branchIsMapped, targetEnvs := applicationConfig.IsThereAnythingToDeploy("master")

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   "master",
			CommitID: anyCommitID,
		},
		BranchIsMapped:     branchIsMapped,
		TargetEnvironments: targetEnvs,
		GitCommitHash:      anyCommitID,
		GitTags:            anyGitTags,
	}

	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	pipelineInfo.SetApplicationConfig(applicationConfig)
	pipelineInfo.SetGitAttributes(gitCommitHash, gitTags)
	err := cli.Run(pipelineInfo)
	rds, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").List(context.TODO(), metav1.ListOptions{})

	t.Run("validate deploy", func(t *testing.T) {
		assert.NoError(t, err)
		assert.True(t, len(rds.Items) > 0)
	})

	rdNameDev := rds.Items[0].Name

	t.Run("validate deployment exist in only the namespace of the modified branch", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.TODO(), rdNameDev, metav1.GetOptions{})
		assert.NotNil(t, rdDev)

		rdProd, _ := radixclient.RadixV1().RadixDeployments("any-app-prod").Get(context.TODO(), rdNameDev, metav1.GetOptions{})
		assert.Nil(t, rdProd)
	})

	t.Run("validate deployment environment variables", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.TODO(), rdNameDev, metav1.GetOptions{})
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
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.TODO(), rdNameDev, metav1.GetOptions{})

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
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.TODO(), rdNameDev, metav1.GetOptions{})
		assert.True(t, rdDev.Spec.Components[0].DNSAppAlias)
		assert.False(t, rdDev.Spec.Components[1].DNSAppAlias)
	})

	t.Run("validate resources", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.TODO(), rdNameDev, metav1.GetOptions{})

		fmt.Print(rdDev.Spec.Components[0].Resources)
		fmt.Print(rdDev.Spec.Components[1].Resources)
		assert.NotNil(t, rdDev.Spec.Components[1].Resources)
		assert.Equal(t, "128Mi", rdDev.Spec.Components[1].Resources.Limits["memory"])
		assert.Equal(t, "500m", rdDev.Spec.Components[1].Resources.Limits["cpu"])
	})

	t.Run("Validate run as non root", func(t *testing.T) {
		rdDev, _ := radixclient.RadixV1().RadixDeployments("any-app-dev").Get(context.TODO(), rdNameDev, metav1.GetOptions{})

		assert.True(t, rdDev.Spec.Components[0].RunAsNonRoot)
		assert.False(t, rdDev.Spec.Components[1].RunAsNonRoot)
	})
}
