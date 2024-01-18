package internal

import (
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
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
								Type:      radixv1.MountTypeBlob,
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

			envVarsMap := make(radixv1.EnvVarsMap)
			envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = testcase.expectedGitCommitHash
			envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = testcase.expectedGitTags

			rd, err := ConstructForTargetEnvironment(ra, nil, "anyjob", "anyimageTag", "anybranch", componentImages, testcase.environment, envVarsMap, "anyhash", "anybuildsecrethash", nil)
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

	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	rd, err := ConstructForTargetEnvironment(ra, nil, "anyjob", "anyimageTag", "anybranch", componentImages, "dev", envVarsMap, "anyhash", "anybuildsecrethash", nil)
	require.NoError(t, err)

	t.Log(rd.Spec.Components[0].Name)
	assert.True(t, rd.Spec.Components[0].AlwaysPullImageOnDeploy)
	t.Log(rd.Spec.Components[1].Name)
	assert.True(t, rd.Spec.Components[1].AlwaysPullImageOnDeploy)
	t.Log(rd.Spec.Components[2].Name)
	assert.False(t, rd.Spec.Components[2].AlwaysPullImageOnDeploy)

	rd, err = ConstructForTargetEnvironment(ra, nil, "anyjob", "anyimageTag", "anybranch", componentImages, "prod", envVarsMap, "anyhash", "anybuildsecrethash", nil)
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

	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "commit-abc"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	rd, err := ConstructForTargetEnvironment(ra, nil, "anyjob", "anyimageTag", "anybranch", componentImages, "dev", envVarsMap, "anyhash", "anybuildsecrethash", nil)
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

	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = commit2
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = gitTag2

	t.Run("deploy only specific components", func(t *testing.T) {
		rd, err := ConstructForTargetEnvironment(ra, activeRadixDeployment, "anyjob", "anyimageTag", "anybranch", componentImages, "dev", envVarsMap, "anyhash", "anybuildsecrethash",
			[]string{"comp1", "job1"})
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
		rd, err := ConstructForTargetEnvironment(ra, activeRadixDeployment, "anyjob", "anyimageTag", "anybranch", componentImages, "dev", envVarsMap, "anyhash", "anybuildsecrethash",
			nil)
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

	t.Run("deploy all components", func(t *testing.T) {
		_, err := ConstructForTargetEnvironment(ra, activeRadixDeployment, "anyjob", "anyimageTag", "anybranch", componentImages, "dev", envVarsMap, "anyhash", "anybuildsecrethash",
			[]string{"not-existing-comp", "comp1"})
		assert.EqualError(t, err, "not all components of jobs requested for deployment found")
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
		rd, err := ConstructForTargetEnvironment(ra, nil, "anyjob", "anyimage", "anybranch", make(pipeline.DeployComponentImages), envName, make(radixv1.EnvVarsMap), "anyhash", "anybuildsecrethash", nil)
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
		rd, err := ConstructForTargetEnvironment(ra, nil, "anyjob", "anyimage", "anybranch", make(pipeline.DeployComponentImages), envName, make(radixv1.EnvVarsMap), "anyhash", "anybuildsecrethash", nil)
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
