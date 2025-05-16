package commithash

import (
	"context"
	"slices"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetLastSuccessfulEnvironmentDeployCommits(t *testing.T) {
	const appName = "anApp"

	scenarios := map[string]struct {
		environment                string
		envRadixDeploymentBuilders map[string][]utils.DeploymentBuilder
		expectedCommitInfo         RadixDeploymentCommit
	}{
		"no deployments - no commits": {
			environment:                "dev",
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{},
			expectedCommitInfo:         RadixDeploymentCommit{},
		},
		"deployment is not active - no commit": {
			environment: "dev",
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithCondition(v1.DeploymentInactive),
				},
			},
			expectedCommitInfo: RadixDeploymentCommit{},
		},
		"active deployment with commit label - its commit": {
			environment: "dev",
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithLabel(kube.RadixCommitLabel, "commit1").
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedCommitInfo: RadixDeploymentCommit{RadixDeploymentName: "rd1", CommitHash: "commit1"},
		},
		"active deployment with commit annotations - its commit": {
			environment: "dev",
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedCommitInfo: RadixDeploymentCommit{RadixDeploymentName: "rd1", CommitHash: "commit1"},
		},
		"active deployment with commit annotations and label - commit from annotation": {
			environment: "dev",
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "annotationcommit",
						}).
						WithLabel(kube.RadixCommitLabel, "labelcommit").
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedCommitInfo: RadixDeploymentCommit{RadixDeploymentName: "rd1", CommitHash: "annotationcommit"},
		},
		"multiple deployments - commit from active": {
			environment: "dev",
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).WithCondition(v1.DeploymentInactive),
					getRadixDeploymentBuilderFor("dev", "rd3").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit3",
						}).WithCondition(v1.DeploymentActive),
					getRadixDeploymentBuilderFor("dev", "rd2").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit2",
						}).WithCondition(v1.DeploymentInactive),
				},
			},
			expectedCommitInfo: RadixDeploymentCommit{RadixDeploymentName: "rd3", CommitHash: "commit3"},
		},
		"active deployment in other environment - no commit": {
			environment: "dev",
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"qa": {
					getRadixDeploymentBuilderFor("qa", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedCommitInfo: RadixDeploymentCommit{},
		},
	}

	for testName, scenario := range scenarios {
		t.Run(testName, func(t *testing.T) {
			provider := prepareTestGetLastSuccessfulEnvironmentDeployCommits(t, appName, scenario.environment, scenario.envRadixDeploymentBuilders)
			commits, err := provider.GetLastCommitHashForEnvironment(context.Background(), appName, scenario.environment)
			require.NoError(t, err)
			assert.Equal(t, scenario.expectedCommitInfo, commits)
		})
	}
}

func getRadixDeploymentBuilderFor(env string, deploymentName string) utils.DeploymentBuilder {
	return utils.ARadixDeployment().
		WithEnvironment(env).
		WithDeploymentName(deploymentName)
}

func prepareTestGetLastSuccessfulEnvironmentDeployCommits(t *testing.T, appName string, environment string, rdBuilders map[string][]utils.DeploymentBuilder) Provider {
	kubeUtil, err := kube.New(kubeclientfake.NewSimpleClientset(), radixclientfake.NewSimpleClientset(), nil, nil)
	require.NoError(t, err)

	envs := []string{environment}
	envs = append(envs, maps.Keys(rdBuilders)...)
	slices.Sort(envs)
	envs = slices.Compact(envs)

	for _, env := range envs {
		environmentNamespace := utils.GetEnvironmentNamespace(appName, env)
		_, err := kubeUtil.KubeClient().CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: environmentNamespace}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err.Error())
		}
		if radixDeploymentBuilders, ok := rdBuilders[env]; ok {
			for _, radixDeploymentBuilder := range radixDeploymentBuilders {
				_, err := kubeUtil.RadixClient().RadixV1().RadixDeployments(environmentNamespace).
					Create(context.Background(), radixDeploymentBuilder.WithAppName(appName).BuildRD(), metav1.CreateOptions{})
				if err != nil {
					t.Fatal(err.Error())
				}
			}
		}
	}
	return NewProvider(kubeUtil)
}
