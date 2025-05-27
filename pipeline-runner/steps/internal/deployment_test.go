package internal

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
						WithReplicas(pointers.Ptr(4)),
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
						WithVolumeMounts([]radixv1.RadixVolumeMount{
							{
								Type:      radixv1.MountTypeBlobFuse2FuseCsiAzure,
								Container: "some-container",
								Path:      "some-path",
							},
							{
								Type:    radixv1.MountTypeBlobFuse2FuseCsiAzure,
								Storage: "some-storage",
								Path:    "some-path",
								GID:     "1000",
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
		expectedGitCommitHash        string
		expectedGitTags              string
		alwaysPullImageOnDeploy      bool
	}{
		{"prod", 4, "db-prod", "1234", "128Mi", "500m", "64Mi", "250m", 0, "jfkewki8273", "tag1 tag2 tag3", true},
		{"dev", 3, "db-dev", "9876", "64Mi", "250m", "32Mi", "125m", 2, "plksmfnwi2309", "radixv1 v2 radixv1.1", true},
	}

	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: "anyImagePath"}

	for _, testcase := range testScenarios {
		t.Run(testcase.environment, func(t *testing.T) {
			pipelineInfo := &model.PipelineInfo{
				RadixApplication:                 ra,
				DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{testcase.environment: componentImages},
				PipelineArguments: model.PipelineArguments{
					JobName:  "anyjob",
					ImageTag: "anyimageTag",
					Branch:   "anybranch",
				},
			}
			rd, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: testcase.environment}, testcase.expectedGitCommitHash, testcase.expectedGitTags, "anyhash", "anybuildsecrethash")
			require.NoError(t, err)

			assert.Equal(t, testcase.expectedReplicas, *rd.Spec.Components[0].Replicas, "Number of replicas wasn't as expected")
			assert.Equal(t, testcase.expectedDbHost, rd.Spec.Components[0].EnvironmentVariables["DB_HOST"])
			assert.Equal(t, testcase.expectedDbPort, rd.Spec.Components[0].EnvironmentVariables["DB_PORT"])
			assert.Equal(t, testcase.expectedGitCommitHash, rd.Spec.Components[0].EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
			assert.Equal(t, testcase.expectedGitTags, rd.Spec.Components[0].EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])
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

func TestConstructForTargetEnvironments_PicksTheCorrectReplicas(t *testing.T) {
	const (
		envName1       = "env1"
		envName2       = "env2"
		componentName1 = "component1"
	)
	type scenario struct {
		name                         string
		componentReplicas            *int
		environment1Replicas         *int
		environment2Replicas         *int
		expectedEnvironment1Replicas *int
		expectedEnvironment2Replicas *int
	}
	scenarios := []scenario{
		{name: "No replicas defined", componentReplicas: nil, environment1Replicas: nil, environment2Replicas: nil, expectedEnvironment1Replicas: nil, expectedEnvironment2Replicas: nil},
		{name: "Env1 replica set", componentReplicas: nil, environment1Replicas: pointers.Ptr(2), environment2Replicas: nil, expectedEnvironment1Replicas: pointers.Ptr(2), expectedEnvironment2Replicas: nil},
		{name: "Two environments replicas set", componentReplicas: nil, environment1Replicas: pointers.Ptr(2), environment2Replicas: pointers.Ptr(3), expectedEnvironment1Replicas: pointers.Ptr(2), expectedEnvironment2Replicas: pointers.Ptr(3)},
		{name: "One environment gets replicas from a component", componentReplicas: pointers.Ptr(4), environment1Replicas: nil, environment2Replicas: pointers.Ptr(3), expectedEnvironment1Replicas: pointers.Ptr(4), expectedEnvironment2Replicas: pointers.Ptr(3)},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			ra := utils.ARadixApplication().
				WithEnvironment(envName1, "main").
				WithEnvironment(envName2, "dev").
				WithComponents(
					utils.AnApplicationComponent().
						WithName(componentName1).
						WithReplicas(ts.componentReplicas).
						WithEnvironmentConfigs(
							utils.AnEnvironmentConfig().
								WithEnvironment(envName1).
								WithReplicas(ts.environment1Replicas),
							utils.AnEnvironmentConfig().
								WithEnvironment(envName2).
								WithReplicas(ts.environment2Replicas))).
				BuildRA()

			pipelineInfo := &model.PipelineInfo{
				RadixApplication:                 ra,
				DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{envName1: make(pipeline.DeployComponentImages), envName2: make(pipeline.DeployComponentImages)},
				PipelineArguments: model.PipelineArguments{
					JobName:  "anyjob",
					ImageTag: "anyimageTag",
					Branch:   "anybranch",
				},
			}
			rdEnv1, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: envName1}, "anycommit", "anytag", "anyhash", "anybuildsecrethash")
			require.NoError(t, err)
			assert.Equal(t, ts.expectedEnvironment1Replicas, rdEnv1.Spec.Components[0].Replicas, "Environment 1 Number of replicas wasn't as expected")

			rdEnv2, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: envName2}, "anycommit", "anytag", "anyhash", "anybuildsecrethash")
			require.NoError(t, err)
			assert.Equal(t, ts.expectedEnvironment2Replicas, rdEnv2.Spec.Components[0].Replicas, "Environment 2 Number of replicas wasn't as expected")
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

	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: "anyImagePath"}

	pipelineInfo := &model.PipelineInfo{
		RadixApplication:                 ra,
		DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{"dev": componentImages, "prod": componentImages},
		PipelineArguments: model.PipelineArguments{
			JobName:  "anyjob",
			ImageTag: "anyimageTag",
			Branch:   "anybranch",
		},
	}
	rd, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: "dev"}, "anycommit", "anytag", "anyhash", "anybuildsecrethash")
	require.NoError(t, err)

	t.Log(rd.Spec.Components[0].Name)
	assert.True(t, rd.Spec.Components[0].AlwaysPullImageOnDeploy)
	t.Log(rd.Spec.Components[1].Name)
	assert.True(t, rd.Spec.Components[1].AlwaysPullImageOnDeploy)
	t.Log(rd.Spec.Components[2].Name)
	assert.False(t, rd.Spec.Components[2].AlwaysPullImageOnDeploy)

	rd, err = ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: "prod"}, "anycommit", "anytag", "anyhash", "anybuildsecrethash")
	require.NoError(t, err)

	t.Log(rd.Spec.Components[0].Name)
	assert.False(t, rd.Spec.Components[0].AlwaysPullImageOnDeploy)
	t.Log(rd.Spec.Components[1].Name)
	assert.False(t, rd.Spec.Components[1].AlwaysPullImageOnDeploy)
}

func TestConstructForTargetEnvironment_GetCommitID(t *testing.T) {
	ra := utils.ARadixApplication().
		WithEnvironment("prod", "dev").
		WithComponents(utils.AnApplicationComponent().WithName("app")).
		BuildRA()

	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: "anyImagePath"}

	pipelineInfo := &model.PipelineInfo{
		RadixApplication:                 ra,
		DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{"dev": componentImages},
		PipelineArguments: model.PipelineArguments{
			JobName:  "anyjob",
			ImageTag: "anyimageTag",
			Branch:   "anybranch",
		},
	}
	rd, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: "dev"}, "commit-abc", "anytags", "anyhash", "anybuildsecrethash")
	require.NoError(t, err)

	assert.Equal(t, "commit-abc", rd.ObjectMeta.Labels[kube.RadixCommitLabel])
}

func TestConstructForTargetEnvironment_GetCommitsToDeploy(t *testing.T) {
	raBuilder := utils.ARadixApplication().
		WithEnvironment("prod", "dev").
		WithComponents(
			utils.AnApplicationComponent().WithName("comp1").WithImage("comp1-image:tag1"),
			utils.AnApplicationComponent().WithName("comp2").WithImage("comp2-image:tag1"),
		).
		WithJobComponents(
			utils.AnApplicationJobComponent().WithName("job1").WithImage("job1-image:tag1"),
			utils.AnApplicationJobComponent().WithName("job2").WithImage("job2-image:tag1"),
		)
	schedulerPort := pointers.Ptr(int32(8080))
	const (
		commit1 = "commit1"
		commit2 = "commit2"
		gitTag1 = "git-tag1"
		gitTag2 = "git-tag2"
	)
	rdBuilder := utils.NewDeploymentBuilder().
		WithRadixApplication(raBuilder).WithAppName("anyapp").
		WithEnvironment("dev").
		WithComponent(utils.NewDeployComponentBuilder().WithName("comp1").WithImage("comp1-image:tag1").
			WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commit1).
			WithEnvironmentVariable(defaults.RadixGitTagsEnvironmentVariable, gitTag1)).
		WithComponent(utils.NewDeployComponentBuilder().WithName("comp2").WithImage("comp2-image:tag1").
			WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commit1).
			WithEnvironmentVariable(defaults.RadixGitTagsEnvironmentVariable, gitTag1)).
		WithJobComponent(utils.NewDeployJobComponentBuilder().WithName("job1").WithImage("job1-image:tag1").WithSchedulerPort(schedulerPort).
			WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commit1).
			WithEnvironmentVariable(defaults.RadixGitTagsEnvironmentVariable, gitTag1)).
		WithJobComponent(utils.NewDeployJobComponentBuilder().WithName("job2").WithImage("job2-image:tag1").WithSchedulerPort(schedulerPort).
			WithEnvironmentVariable(defaults.RadixCommitHashEnvironmentVariable, commit1).
			WithEnvironmentVariable(defaults.RadixGitTagsEnvironmentVariable, gitTag1))
	activeRadixDeployment := rdBuilder.BuildRD()
	ra := rdBuilder.GetApplicationBuilder().BuildRA()

	componentImages := make(pipeline.DeployComponentImages)
	componentImages["comp1"] = pipeline.DeployComponentImage{ImagePath: "comp1-image:tag2"}
	componentImages["comp2"] = pipeline.DeployComponentImage{ImagePath: "comp2-image:tag2"}
	componentImages["job1"] = pipeline.DeployComponentImage{ImagePath: "job1-image:tag2"}
	componentImages["job2"] = pipeline.DeployComponentImage{ImagePath: "job2-image:tag2"}

	t.Run("deploy only specific components", func(t *testing.T) {
		pipelineInfo := &model.PipelineInfo{
			RadixApplication:                 ra,
			DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{"dev": componentImages},
			PipelineArguments: model.PipelineArguments{
				JobName:            "anyjob",
				ImageTag:           "anyimageTag",
				Branch:             "anybranch",
				ComponentsToDeploy: []string{"comp1", "job1"},
			},
		}
		rd, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: "dev", ActiveRadixDeployment: activeRadixDeployment}, commit2, gitTag2, "anyhash", "anybuildsecrethash")
		require.NoError(t, err)

		comp1, ok := slice.FindFirst(rd.Spec.Components, func(component radixv1.RadixDeployComponent) bool { return component.GetName() == "comp1" })
		require.True(t, ok)
		assert.Equal(t, "comp1-image:tag2", comp1.GetImage())
		assert.Equal(t, commit2, comp1.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, gitTag2, comp1.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable])

		comp2, ok := slice.FindFirst(rd.Spec.Components, func(component radixv1.RadixDeployComponent) bool { return component.GetName() == "comp2" })
		require.True(t, ok)
		assert.Equal(t, "comp2-image:tag1", comp2.GetImage())
		assert.Equal(t, commit1, comp2.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, gitTag1, comp2.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable])

		job1, ok := slice.FindFirst(rd.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.GetName() == "job1" })
		require.True(t, ok)
		assert.Equal(t, "job1-image:tag2", job1.GetImage())
		assert.Equal(t, commit2, job1.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, gitTag2, job1.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable])

		job2, ok := slice.FindFirst(rd.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.GetName() == "job2" })
		require.True(t, ok)
		assert.Equal(t, "job2-image:tag1", job2.GetImage())
		assert.Equal(t, commit1, job2.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, gitTag1, job2.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable])
	})

	t.Run("deploy all components", func(t *testing.T) {
		pipelineInfo := &model.PipelineInfo{
			RadixApplication:                 ra,
			DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{"dev": componentImages},
			PipelineArguments: model.PipelineArguments{
				JobName:  "anyjob",
				ImageTag: "anyimageTag",
				Branch:   "anybranch",
			},
		}
		rd, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: "dev", ActiveRadixDeployment: activeRadixDeployment}, commit2, gitTag2, "anyhash", "anybuildsecrethash")
		require.NoError(t, err)

		comp1, ok := slice.FindFirst(rd.Spec.Components, func(component radixv1.RadixDeployComponent) bool { return component.GetName() == "comp1" })
		require.True(t, ok)
		assert.Equal(t, "comp1-image:tag2", comp1.GetImage())
		assert.Equal(t, commit2, comp1.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, gitTag2, comp1.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable])

		comp2, ok := slice.FindFirst(rd.Spec.Components, func(component radixv1.RadixDeployComponent) bool { return component.GetName() == "comp2" })
		require.True(t, ok)
		assert.Equal(t, "comp2-image:tag2", comp2.GetImage())
		assert.Equal(t, commit2, comp2.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, gitTag2, comp2.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable])

		job1, ok := slice.FindFirst(rd.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.GetName() == "job1" })
		require.True(t, ok)
		assert.Equal(t, "job1-image:tag2", job1.GetImage())
		assert.Equal(t, commit2, job1.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, gitTag2, job1.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable])

		job2, ok := slice.FindFirst(rd.Spec.Jobs, func(job radixv1.RadixDeployJobComponent) bool { return job.GetName() == "job2" })
		require.True(t, ok)
		assert.Equal(t, "job2-image:tag2", job2.GetImage())
		assert.Equal(t, commit2, job2.GetEnvironmentVariables()[defaults.RadixCommitHashEnvironmentVariable])
		assert.Equal(t, gitTag2, job2.GetEnvironmentVariables()[defaults.RadixGitTagsEnvironmentVariable])
	})
}

func Test_ConstructForTargetEnvironment_Identity(t *testing.T) {
	type scenarioSpec struct {
		name                 string
		commonConfig         *radixv1.Identity
		configureEnvironment bool
		environmentConfig    *radixv1.Identity
		expected             *radixv1.Identity
	}

	scenarios := []scenarioSpec{
		{name: "nil when commonConfig and environmentConfig is empty", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is nil", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is not set", commonConfig: &radixv1.Identity{}, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "use commonConfig when environmentConfig.Azure is empty", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "override non-empty commonConfig with environmentConfig.Azure", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig with environmentConfig", commonConfig: &radixv1.Identity{}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig.Azure with environmentConfig", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{}}, configureEnvironment: true, environmentConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "transform clientId with curly to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "{11111111-2222-3333-4444-555555555555}"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId with urn:uuid to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "urn:uuid:11111111-2222-3333-4444-555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId without dashes to standard format", commonConfig: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111222233334444555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
	}

	componentTest := func(scenario scenarioSpec, t *testing.T) {
		const envName = "anyenv"
		component := utils.AnApplicationComponent().WithName("anycomponent").WithIdentity(scenario.commonConfig)
		if scenario.configureEnvironment {
			component = component.WithEnvironmentConfigs(
				utils.AnEnvironmentConfig().WithEnvironment(envName).WithIdentity(scenario.environmentConfig),
			)
		}
		ra := utils.ARadixApplication().WithComponents(component).BuildRA()
		pipelineInfo := &model.PipelineInfo{
			RadixApplication:                 ra,
			DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{envName: make(pipeline.DeployComponentImages)},
			PipelineArguments: model.PipelineArguments{
				JobName:  "anyjob",
				ImageTag: "anyimage",
				Branch:   "anybranch",
			},
		}

		rd, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: envName}, "anycommit", "anytags", "anyhash", "anybuildsecrethash")
		require.NoError(t, err)
		assert.Equal(t, scenario.expected, rd.Spec.Components[0].Identity)
	}
	jobTest := func(scenario scenarioSpec, t *testing.T) {
		const envName = "anyenv"
		job := utils.AnApplicationJobComponent().WithName("anyjob").WithIdentity(scenario.commonConfig)
		if scenario.configureEnvironment {
			job = job.WithEnvironmentConfigs(
				utils.AJobComponentEnvironmentConfig().WithEnvironment(envName).WithIdentity(scenario.environmentConfig),
			)
		}
		ra := utils.ARadixApplication().WithJobComponents(job).BuildRA()
		pipelineInfo := &model.PipelineInfo{
			RadixApplication:                 ra,
			DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{envName: make(pipeline.DeployComponentImages)},
			PipelineArguments: model.PipelineArguments{
				JobName:  "anyjob",
				ImageTag: "anyimage",
				Branch:   "anybranch",
			},
		}
		rd, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: envName}, "anycommit", "anytags", "anyhash", "anybuildsecrethash")
		require.NoError(t, err)
		assert.Equal(t, scenario.expected, rd.Spec.Jobs[0].Identity)
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			componentTest(scenario, t)
			jobTest(scenario, t)
		})
	}
}

func Test_ConstructForTargetEnvironment_BuildKitAnnotations(t *testing.T) {
	type scenarioSpec struct {
		build                               *radixv1.BuildSpec
		overrideUseBuildCache               *bool
		refreshBuildCache                   *bool
		expectedAnnotationUseBuildKit       string
		expectedAnnotationUseBuildCache     string
		expectedAnnotationRefreshBuildCache string
	}

	scenarios := map[string]scenarioSpec{
		"Build empty": {
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build empty, overrideUseBuildCache true": {
			overrideUseBuildCache:               pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build empty, overrideUseBuildCache false": {
			overrideUseBuildCache:               pointers.Ptr(false),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build empty, overrideUseBuildCache true, refreshBuildCache false": {
			overrideUseBuildCache:               pointers.Ptr(true),
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build empty, overrideUseBuildCache false, refreshBuildCache true": {
			overrideUseBuildCache:               pointers.Ptr(false),
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build empty, refreshBuildCache false": {
			overrideUseBuildCache:               pointers.Ptr(true),
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build empty, refreshBuildCache true": {
			overrideUseBuildCache:               pointers.Ptr(false),
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build not empty": {
			build:                               &radixv1.BuildSpec{},
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build not empty, overrideUseBuildCache true": {
			build:                               &radixv1.BuildSpec{},
			overrideUseBuildCache:               pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build not empty, overrideUseBuildCache false": {
			build:                               &radixv1.BuildSpec{},
			overrideUseBuildCache:               pointers.Ptr(false),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build not empty, overrideUseBuildCache true, refreshBuildCache false": {
			build:                               &radixv1.BuildSpec{},
			overrideUseBuildCache:               pointers.Ptr(true),
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build not empty, overrideUseBuildCache false, refreshBuildCache true": {
			build:                               &radixv1.BuildSpec{},
			overrideUseBuildCache:               pointers.Ptr(false),
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build not empty, refreshBuildCache false": {
			build:                               &radixv1.BuildSpec{},
			overrideUseBuildCache:               pointers.Ptr(true),
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"Build not empty, refreshBuildCache true": {
			build:                               &radixv1.BuildSpec{},
			overrideUseBuildCache:               pointers.Ptr(false),
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "false",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"UseBuildKit, implicit UseBuildCache": {
			build:                               &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true)},
			expectedAnnotationUseBuildKit:       "true",
			expectedAnnotationUseBuildCache:     "true",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"UseBuildKit, explicite UseBuildCache": {
			build:                               &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true), UseBuildCache: pointers.Ptr(true)},
			expectedAnnotationUseBuildKit:       "true",
			expectedAnnotationUseBuildCache:     "true",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"UseBuildKit, implicit UseBuildCache, explicite no refreshBuildCache": {
			build:                               &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true)},
			refreshBuildCache:                   pointers.Ptr(false),
			expectedAnnotationUseBuildKit:       "true",
			expectedAnnotationUseBuildCache:     "true",
			expectedAnnotationRefreshBuildCache: "false",
		},
		"UseBuildKit, explicite UseBuildCache, explicite refreshBuildCache": {
			build:                               &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true), UseBuildCache: pointers.Ptr(true)},
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "true",
			expectedAnnotationUseBuildCache:     "true",
			expectedAnnotationRefreshBuildCache: "true",
		},
		"UseBuildKit, explicite no UseBuildCache, explicite refreshBuildCache": {
			build:                               &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true), UseBuildCache: pointers.Ptr(false)},
			refreshBuildCache:                   pointers.Ptr(true),
			expectedAnnotationUseBuildKit:       "true",
			expectedAnnotationUseBuildCache:     "false",
			expectedAnnotationRefreshBuildCache: "true",
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			const envName = "anyenv"
			builder := utils.ARadixApplication()
			if scenario.build != nil {
				builder = builder.WithBuildKit(scenario.build.UseBuildKit).WithBuildCache(scenario.build.UseBuildCache)
			}
			ra := builder.BuildRA()
			pipelineInfo := &model.PipelineInfo{
				RadixApplication:                 ra,
				DeployEnvironmentComponentImages: pipeline.DeployEnvironmentComponentImages{envName: make(pipeline.DeployComponentImages)},
				PipelineArguments: model.PipelineArguments{
					JobName:               "anyjob",
					ImageTag:              "anyimage",
					Branch:                "anybranch",
					OverrideUseBuildCache: scenario.overrideUseBuildCache,
					RefreshBuildCache:     scenario.refreshBuildCache,
				},
			}

			rd, err := ConstructForTargetEnvironment(context.Background(), pipelineInfo, model.TargetEnvironment{Environment: envName}, "anycommit", "anytags", "anyhash", "anybuildsecrethash")
			require.NoError(t, err)
			assert.Equal(t, scenario.expectedAnnotationUseBuildKit, rd.Annotations[kube.RadixUseBuildKit], "UseBuildKit annotation not as expected")
			assert.Equal(t, scenario.expectedAnnotationUseBuildCache, rd.Annotations[kube.RadixUseBuildCache], "UseBuildCache annotation not as expected")
			assert.Equal(t, scenario.expectedAnnotationRefreshBuildCache, rd.Annotations[kube.RadixRefreshBuildCache], "RefreshBuildCache annotation not as expected")
		})
	}
}
