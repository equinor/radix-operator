package steps

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	application "github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

var ExpectedNoCacheCmd = "/usr/bin/buildah login --username ${BUILDAH_USERNAME} --password ${BUILDAH_PASSWORD} radixdev.azurecr.io && /usr/bin/buildah build --storage-driver=vfs --isolation=chroot --jobs 0 --file /app/Dockerfile --build-arg RADIX_GIT_COMMIT_HASH=\"${RADIX_GIT_COMMIT_HASH}\" --build-arg RADIX_GIT_TAGS=\"${RADIX_GIT_TAGS}\" --build-arg BRANCH=\"${BRANCH}\" --build-arg TARGET_ENVIRONMENTS=\"${TARGET_ENVIRONMENTS}\" --tag radixdev.azurecr.io/any-app-web-component:anytag --tag radixdev.azurecr.io/any-app-:development-anytag --tag radixdev.azurecr.io/any-app-:weekly-42-anytag /app/ && /usr/bin/buildah push --storage-driver=vfs radixdev.azurecr.io/any-app-web-component:anytag && /usr/bin/buildah push --storage-driver=vfs radixdev.azurecr.io/any-app-:development-anytag && /usr/bin/buildah push --storage-driver=vfs radixdev.azurecr.io/any-app-:weekly-42-anytag"
var ExpectedWithCacheCmd = "/usr/bin/buildah login --username ${BUILDAH_USERNAME} --password ${BUILDAH_PASSWORD} radixdev.azurecr.io && /usr/bin/buildah login --username ${BUILDAH_CACHE_USERNAME} --password ${BUILDAH_CACHE_PASSWORD} radixdevcache.azurecr.io && /usr/bin/buildah build --storage-driver=vfs --isolation=chroot --jobs 0 --file /app/Dockerfile --build-arg RADIX_GIT_COMMIT_HASH=\"${RADIX_GIT_COMMIT_HASH}\" --build-arg RADIX_GIT_TAGS=\"${RADIX_GIT_TAGS}\" --build-arg BRANCH=\"${BRANCH}\" --build-arg TARGET_ENVIRONMENTS=\"${TARGET_ENVIRONMENTS}\" --layers --cache-to=radixdevcache.azurecr.io/any-app-web-component --cache-from=radixdevcache.azurecr.io/any-app-web-component --tag radixdev.azurecr.io/any-app-web-component:anytag --tag radixdev.azurecr.io/any-app-:development-anytag --tag radixdev.azurecr.io/any-app-:weekly-42-anytag /app/ && /usr/bin/buildah push --storage-driver=vfs radixdev.azurecr.io/any-app-web-component:anytag && /usr/bin/buildah push --storage-driver=vfs radixdev.azurecr.io/any-app-:development-anytag && /usr/bin/buildah push --storage-driver=vfs radixdev.azurecr.io/any-app-:weekly-42-anytag"

func TestBuild_Buildah_withoutCache(t *testing.T) {
	kubeclient, kube, radixclient, _ := setupTest(t)

	anyBranch := "master"
	anyEnvironment := "dev"
	anyComponentName := "app"

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, anyBranch).
		WithBuildKit(pointers.Ptr(true)).
		WithBuildCache(pointers.Ptr(false)).
		WithComponents(
			utils.AnApplicationComponent().
				WithName(anyComponentName).
				WithSourceFolder("/app/src").
				WithDockerfileName("Dockerfile")).
		BuildRA()

	// Prometheus doesn´t contain any fake
	cli := NewBuildStep()
	cli.Init(kubeclient, radixclient, kube, &monitoring.Clientset{}, rr)

	applicationConfig, _ := application.NewApplicationConfig(kubeclient, kube, radixclient, rr, ra)
	branchIsMapped, targetEnvs := applicationConfig.IsThereAnythingToDeploy(anyBranch)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Debug: true,

			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   anyBranch,
			CommitID: anyCommitID,

			ContainerRegistry:      "radixdev.azurecr.io",
			CacheContainerRegistry: "radixdevcache.azurecr.io",
			Clustername:            "weekly-42",
			Clustertype:            "development",
			ImageBuilder:           "radix-image-builder:master-latest",

			Builder: model.Builder{
				ResourcesLimitsMemory:   "500M",
				ResourcesRequestsCPU:    "200m",
				ResourcesRequestsMemory: "500M",
			},
		},
		TargetEnvironments: targetEnvs,
		BranchIsMapped:     branchIsMapped,
		ComponentImages: map[string]pipeline.ComponentImage{
			"main": {
				Build:         true,
				ContainerName: "web-component",
				Context:       "/app/",
				Dockerfile:    "Dockerfile",
				ImagePath:     utils.GetImagePath("radixdev.azurecr.io", ra.Name, "web-component", "anytag"),
				ImageTagName:  "anytag",
			},
		},
		GitCommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RadixApplication: ra,
	}

	job, err := createACRBuildJob(rr, pipelineInfo, make([]corev1.EnvVar, 0))
	require.NoError(t, err)

	assert.Equal(t, 1, len(job.Spec.Template.Spec.Containers))

	buildah := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, 3, len(job.Spec.Template.Spec.Containers[0].Command))
	command := buildah.Command
	assert.Equal(t, []string{"/bin/bash", "-c"}, command[:2])

	cmd := buildah.Command[2]
	assert.NotEqual(t, "", cmd)

	assert.Equal(t, ExpectedNoCacheCmd, cmd)
}

func TestBuild_Buildah_WithCache(t *testing.T) {
	kubeclient, kube, radixclient, _ := setupTest(t)

	anyBranch := "master"
	anyEnvironment := "dev"
	anyComponentName := "app"

	rr := utils.ARadixRegistration().
		WithName(anyAppName).
		BuildRR()

	ra := utils.NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment(anyEnvironment, anyBranch).
		WithBuildKit(pointers.Ptr(true)).
		WithBuildCache(pointers.Ptr(true)).
		WithComponents(
			utils.AnApplicationComponent().
				WithName(anyComponentName).
				WithSourceFolder("/app/src").
				WithDockerfileName("Dockerfile")).
		BuildRA()

	// Prometheus doesn´t contain any fake
	cli := NewBuildStep()
	cli.Init(kubeclient, radixclient, kube, &monitoring.Clientset{}, rr)

	applicationConfig, _ := application.NewApplicationConfig(kubeclient, kube, radixclient, rr, ra)
	branchIsMapped, targetEnvs := applicationConfig.IsThereAnythingToDeploy(anyBranch)

	pipelineInfo := &model.PipelineInfo{
		PipelineArguments: model.PipelineArguments{
			Debug: true,

			JobName:  anyJobName,
			ImageTag: anyImageTag,
			Branch:   anyBranch,
			CommitID: anyCommitID,

			ContainerRegistry:      "radixdev.azurecr.io",
			CacheContainerRegistry: "radixdevcache.azurecr.io",
			Clustername:            "weekly-42",
			Clustertype:            "development",
			ImageBuilder:           "radix-image-builder:master-latest",

			Builder: model.Builder{
				ResourcesLimitsMemory:   "500M",
				ResourcesRequestsCPU:    "200m",
				ResourcesRequestsMemory: "500M",
			},
		},
		TargetEnvironments: targetEnvs,
		BranchIsMapped:     branchIsMapped,
		ComponentImages: map[string]pipeline.ComponentImage{
			"main": {
				Build:         true,
				ContainerName: "web-component",
				Context:       "/app/",
				Dockerfile:    "Dockerfile",
				ImagePath:     utils.GetImagePath("radixdev.azurecr.io", ra.Name, "web-component", "anytag"),
				ImageTagName:  "anytag",
			},
		},
		GitCommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RadixApplication: ra,
	}

	job, err := createACRBuildJob(rr, pipelineInfo, make([]corev1.EnvVar, 0))
	require.NoError(t, err)

	assert.Equal(t, 1, len(job.Spec.Template.Spec.Containers))

	buildah := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, 3, len(job.Spec.Template.Spec.Containers[0].Command))
	command := buildah.Command
	assert.Equal(t, []string{"/bin/bash", "-c"}, command[:2])

	cmd := buildah.Command[2]
	assert.NotEqual(t, "", cmd)

	assert.Equal(t, ExpectedWithCacheCmd, cmd)
}
