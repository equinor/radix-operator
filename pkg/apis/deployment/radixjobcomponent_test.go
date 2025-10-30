// nolint:staticcheck // SA1019: Ignore linting deprecated fields
package deployment

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_GetRadixJobComponents_BuildAllJobComponents(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job1").
				WithSchedulerPort(8888).
				WithPayloadPath(utils.StringPtr("/path/to/payload")),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{
		ra:              ra,
		env:             "any",
		componentImages: make(pipeline.DeployComponentImages),
	}
	jobs, err := cfg.JobComponents(context.Background())
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
	assert.Equal(t, "job1", jobs[0].Name)
	assert.Equal(t, int32(8888), jobs[0].SchedulerPort)
	assert.Equal(t, "/path/to/payload", jobs[0].Payload.Path)
	assert.Equal(t, "job2", jobs[1].Name)
	assert.Zero(t, jobs[1].SchedulerPort)
	assert.Nil(t, jobs[1].Payload)
}

func Test_GetRadixJobComponentsWithNode_BuildAllJobComponents(t *testing.T) {
	gpu := "any-gpu"
	gpuCount := "12"
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job1").
				WithSchedulerPort(8888).
				WithPayloadPath(utils.StringPtr("/path/to/payload")).
				WithNode(radixv1.RadixNode{Gpu: gpu, GpuCount: gpuCount}),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{
		ra:              ra,
		env:             "any",
		componentImages: make(pipeline.DeployComponentImages),
	}
	jobs, err := cfg.JobComponents(context.Background())
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
	assert.Equal(t, gpu, jobs[0].Node.Gpu)
	assert.Equal(t, gpuCount, jobs[0].Node.GpuCount)
	assert.Empty(t, jobs[1].Node.Gpu)
	assert.Empty(t, jobs[1].Node.GpuCount)
}

func Test_GetRadixJobComponents_EnvironmentVariables(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job").
				WithCommonEnvironmentVariable("COMMON1", "common1").
				WithCommonEnvironmentVariable("COMMON2", "common2").
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithEnvironmentVariable("JOB1", "job1").WithEnvironmentVariable("COMMON1", "override1"),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2").
						WithEnvironmentVariable("COMMON2", "override2"),
				),
		).BuildRA()

	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	cfg := jobComponentsBuilder{
		ra:              ra,
		env:             "env1",
		componentImages: make(pipeline.DeployComponentImages),
		defaultEnvVars:  envVarsMap,
	}
	jobComponents, err := cfg.JobComponents(context.Background())
	require.NoError(t, err)
	jobComponent := jobComponents[0]
	assert.Len(t, jobComponent.EnvironmentVariables, 5)
	assert.Equal(t, "override1", jobComponent.EnvironmentVariables["COMMON1"])
	assert.Equal(t, "common2", jobComponent.EnvironmentVariables["COMMON2"])
	assert.Equal(t, "job1", jobComponent.EnvironmentVariables["JOB1"])
	assert.Equal(t, "anycommit", jobComponent.EnvironmentVariables[defaults.RadixCommitHashEnvironmentVariable])
	assert.Equal(t, "anytag", jobComponent.EnvironmentVariables[defaults.RadixGitTagsEnvironmentVariable])
}

func Test_GetRadixJobComponents_Monitoring(t *testing.T) {
	monitoringConfig := radixv1.MonitoringConfig{
		PortName: "monitor",
		Path:     "/api/monitor",
	}

	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job_1").
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithMonitoring(pointers.Ptr(true)),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2"),
				),
			utils.AnApplicationJobComponent().
				WithName("job_2").
				WithMonitoringConfig(monitoringConfig).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithMonitoring(pointers.Ptr(true)),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2"),
				),
		).BuildRA()

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(pipeline.DeployComponentImages)}
	jobComponents, err := cfg.JobComponents(context.Background())
	require.NoError(t, err)
	job := jobComponents[0]
	assert.True(t, job.Monitoring)
	assert.Empty(t, job.MonitoringConfig.PortName)
	assert.Empty(t, job.MonitoringConfig.Path)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(pipeline.DeployComponentImages)}
	jobComponents, err = cfg.JobComponents(context.Background())
	require.NoError(t, err)
	job = jobComponents[0]
	assert.False(t, job.Monitoring)
	assert.Empty(t, job.MonitoringConfig.PortName)
	assert.Empty(t, job.MonitoringConfig.Path)

	cfg = jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(pipeline.DeployComponentImages)}
	jobComponents, err = cfg.JobComponents(context.Background())
	require.NoError(t, err)
	job = jobComponents[1]
	assert.True(t, job.Monitoring)
	assert.Equal(t, monitoringConfig.PortName, job.MonitoringConfig.PortName)
	assert.Equal(t, monitoringConfig.Path, job.MonitoringConfig.Path)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(pipeline.DeployComponentImages)}
	jobComponents, err = cfg.JobComponents(context.Background())
	require.NoError(t, err)
	job = jobComponents[1]
	assert.False(t, job.Monitoring)
	assert.Equal(t, monitoringConfig.PortName, job.MonitoringConfig.PortName)
	assert.Equal(t, monitoringConfig.Path, job.MonitoringConfig.Path)
}

func Test_GetRadixJobComponents_Identity(t *testing.T) {
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

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			const envName = "anyenv"
			jobComponent := utils.AnApplicationJobComponent().WithName("anyjob").WithIdentity(scenario.commonConfig)
			if scenario.configureEnvironment {
				jobComponent = jobComponent.WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().WithEnvironment(envName).WithIdentity(scenario.environmentConfig),
				)
			}
			ra := utils.ARadixApplication().WithJobComponents(jobComponent).BuildRA()
			sut := jobComponentsBuilder{ra: ra, env: envName, componentImages: make(pipeline.DeployComponentImages)}
			jobs, err := sut.JobComponents(context.Background())
			require.NoError(t, err)
			assert.Equal(t, scenario.expected, jobs[0].Identity)
		})
	}
}

func Test_GetRadixJobComponents_ImageTagName(t *testing.T) {
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["job"] = pipeline.DeployComponentImage{Build: false, ImagePath: "img:{imageTagName}"}
	componentImages["job2"] = pipeline.DeployComponentImage{ImagePath: "job2:tag"}

	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job").
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithImageTagName("master"),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2").
						WithImageTagName("release"),
				),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{ra: ra, env: "env2", componentImages: componentImages}
	jobs, err := cfg.JobComponents(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "img:release", jobs[0].Image)
	assert.Equal(t, "job2:tag", jobs[1].Image)
}

func Test_GetRadixJobComponents_NodeName(t *testing.T) {
	compGpu := "comp gpu"
	compGpuCount := "10"
	envGpu1 := "env1 gpu"
	envGpuCount1 := "20"
	envGpuCount2 := "30"
	envGpu3 := "env3 gpu"
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job").
				WithNode(radixv1.RadixNode{Gpu: compGpu, GpuCount: compGpuCount}).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithNode(radixv1.RadixNode{Gpu: envGpu1, GpuCount: envGpuCount1}),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2").
						WithNode(radixv1.RadixNode{GpuCount: envGpuCount2}),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env3").
						WithNode(radixv1.RadixNode{Gpu: envGpu3}),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env4"),
				),
		).BuildRA()

	t.Run("override job gpu and gpu-count with environment gpu and gpu-count", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env1"}
		jobs, err := cfg.JobComponents(context.Background())
		require.NoError(t, err)
		assert.Equal(t, envGpu1, jobs[0].Node.Gpu)
		assert.Equal(t, envGpuCount1, jobs[0].Node.GpuCount)
	})
	t.Run("override job gpu-count with environment gpu-count", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env2"}
		jobs, err := cfg.JobComponents(context.Background())
		require.NoError(t, err)
		assert.Equal(t, compGpu, jobs[0].Node.Gpu)
		assert.Equal(t, envGpuCount2, jobs[0].Node.GpuCount)
	})
	t.Run("override job gpu with environment gpu", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env3"}
		jobs, err := cfg.JobComponents(context.Background())
		require.NoError(t, err)
		assert.Equal(t, envGpu3, jobs[0].Node.Gpu)
		assert.Equal(t, compGpuCount, jobs[0].Node.GpuCount)
	})
	t.Run("do not override job gpu or gpu-count with environment gpu or gpu-count", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env4"}
		jobs, err := cfg.JobComponents(context.Background())
		require.NoError(t, err)
		assert.Equal(t, compGpu, jobs[0].Node.Gpu)
		assert.Equal(t, compGpuCount, jobs[0].Node.GpuCount)
	})
}

func Test_GetRadixJobComponents_Resources(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job").
				WithCommonResource(map[string]string{
					"memory": "100Mi",
					"cpu":    "150m",
				}, map[string]string{
					"memory": "200Mi",
					"cpu":    "250m",
				}).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithResource(map[string]string{
							"memory": "1100Mi",
							"cpu":    "1150m",
						}, map[string]string{
							"memory": "1200Mi",
							"cpu":    "1250m",
						}),
				),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(pipeline.DeployComponentImages)}
	jobs, err := cfg.JobComponents(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "1100Mi", jobs[0].Resources.Requests["memory"])
	assert.Equal(t, "1150m", jobs[0].Resources.Requests["cpu"])
	assert.Equal(t, "1200Mi", jobs[0].Resources.Limits["memory"])
	assert.Equal(t, "1250m", jobs[0].Resources.Limits["cpu"])
	assert.Empty(t, jobs[1].Resources)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(pipeline.DeployComponentImages)}
	jobs, err = cfg.JobComponents(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "100Mi", jobs[0].Resources.Requests["memory"])
	assert.Equal(t, "150m", jobs[0].Resources.Requests["cpu"])
	assert.Equal(t, "200Mi", jobs[0].Resources.Limits["memory"])
	assert.Equal(t, "250m", jobs[0].Resources.Limits["cpu"])
}

func Test_GetRadixJobComponents_Ports(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job").
				WithPort("http", 8080).
				WithPort("metrics", 9000),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(pipeline.DeployComponentImages)}
	jobs, err := cfg.JobComponents(context.Background())
	require.NoError(t, err)
	assert.Len(t, jobs[0].Ports, 2)
	portMap := make(map[string]radixv1.ComponentPort)
	for _, p := range jobs[0].Ports {
		portMap[p.Name] = p
	}
	assert.Equal(t, int32(8080), portMap["http"].Port)
	assert.Equal(t, int32(9000), portMap["metrics"].Port)
	assert.Empty(t, jobs[1].Ports)
}

func Test_GetRadixJobComponents_Secrets(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job").
				WithSecrets("SECRET1", "SECRET2"),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(pipeline.DeployComponentImages)}
	jobs, err := cfg.JobComponents(context.Background())
	require.NoError(t, err)
	assert.Len(t, jobs[0].Secrets, 2)
	assert.ElementsMatch(t, []string{"SECRET1", "SECRET2"}, jobs[0].Secrets)

	assert.Empty(t, jobs[1].Secrets)
}

func Test_GetRadixJobComponents_TimeLimitSeconds(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("this job name does get set").
				WithSchedulerPort(8888).
				WithTimeLimitSeconds(numbers.Int64Ptr(200)).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithTimeLimitSeconds(numbers.Int64Ptr(100)),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2").
						WithEnvironmentVariable("COMMON2", "override2"),
				),
		).BuildRA()

	cfgEnv1 := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(pipeline.DeployComponentImages)}
	cfgEnv2 := jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(pipeline.DeployComponentImages)}
	env1Job, err := cfgEnv1.JobComponents(context.Background())
	require.NoError(t, err)
	env2Job, err := cfgEnv2.JobComponents(context.Background())
	require.NoError(t, err)
	assert.Equal(t, numbers.Int64Ptr(100), env1Job[0].TimeLimitSeconds)
	assert.Equal(t, numbers.Int64Ptr(200), env2Job[0].TimeLimitSeconds)
}

func Test_GetRadixJobComponents_BackoffLimit(t *testing.T) {
	devEnvName, prodEnvName := "dev", "prod"
	scenarios := []struct {
		name                        string
		defaultBackoffLimit         *int32
		envDevBackoffLimit          *int32
		expectedEnvDevBackoffLimit  *int32
		expectedEnvProdBackoffLimit *int32
	}{
		{name: "expect dev and prod nil", defaultBackoffLimit: nil, envDevBackoffLimit: nil, expectedEnvDevBackoffLimit: nil, expectedEnvProdBackoffLimit: nil},
		{name: "expect dev and prod from default", defaultBackoffLimit: numbers.Int32Ptr(5), envDevBackoffLimit: nil, expectedEnvDevBackoffLimit: numbers.Int32Ptr(5), expectedEnvProdBackoffLimit: numbers.Int32Ptr(5)},
		{name: "expect dev from envconfig and prod nil", defaultBackoffLimit: nil, envDevBackoffLimit: numbers.Int32Ptr(5), expectedEnvDevBackoffLimit: numbers.Int32Ptr(5), expectedEnvProdBackoffLimit: nil},
		{name: "expect dev from envconfig and prod from default", defaultBackoffLimit: numbers.Int32Ptr(5), envDevBackoffLimit: numbers.Int32Ptr(10), expectedEnvDevBackoffLimit: numbers.Int32Ptr(10), expectedEnvProdBackoffLimit: numbers.Int32Ptr(5)},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			ra := utils.NewRadixApplicationBuilder().
				WithJobComponents(
					utils.NewApplicationJobComponentBuilder().
						WithName("anyjob").
						WithBackoffLimit(scenario.defaultBackoffLimit).
						WithEnvironmentConfigs(
							utils.NewJobComponentEnvironmentBuilder().
								WithEnvironment(devEnvName).
								WithBackoffLimit(scenario.envDevBackoffLimit),
						),
				).
				BuildRA()

			devBuilder := jobComponentsBuilder{ra: ra, env: devEnvName, componentImages: pipeline.DeployComponentImages{}}
			devJobs, err := devBuilder.JobComponents(context.Background())
			require.NoError(t, err, "devJobs build error")
			require.Len(t, devJobs, 1, "devJobs length")
			assert.Equal(t, scenario.expectedEnvDevBackoffLimit, devJobs[0].BackoffLimit, "devJobs backoffLimit")

			prodBuilder := jobComponentsBuilder{ra: ra, env: prodEnvName, componentImages: pipeline.DeployComponentImages{}}
			prodJobs, err := prodBuilder.JobComponents(context.Background())
			require.NoError(t, err, "prodJobs build error")
			require.Len(t, prodJobs, 1, "prodJobs length")
			assert.Equal(t, scenario.expectedEnvProdBackoffLimit, prodJobs[0].BackoffLimit, "prodJobs bacoffLimit")
		})
	}
}

func Test_GetRadixJobComponents_Notifications(t *testing.T) {
	type scenarioSpec struct {
		name                 string
		commonConfig         *radixv1.Notifications
		configureEnvironment bool
		environmentConfig    *radixv1.Notifications
		expected             *radixv1.Notifications
	}

	scenarios := []scenarioSpec{
		{name: "nil when commonConfig and environmentConfig is empty", commonConfig: &radixv1.Notifications{}, configureEnvironment: true, environmentConfig: &radixv1.Notifications{}, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: &radixv1.Notifications{}, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is nil", commonConfig: &radixv1.Notifications{}, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is not set", commonConfig: &radixv1.Notifications{}, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8080")}, configureEnvironment: true, environmentConfig: &radixv1.Notifications{}, expected: &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8080")}},
		{name: "use commonConfig when environmentConfig.Webhook is empty", commonConfig: &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8080")}, configureEnvironment: true, environmentConfig: &radixv1.Notifications{Webhook: nil}, expected: &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8080")}},
		{name: "override non-empty commonConfig with environmentConfig.Webhook", commonConfig: &radixv1.Notifications{Webhook: pointers.Ptr("http://api:8080")}, configureEnvironment: true, environmentConfig: &radixv1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}, expected: &radixv1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}},
		{name: "override empty commonConfig with environmentConfig", commonConfig: &radixv1.Notifications{}, configureEnvironment: true, environmentConfig: &radixv1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}, expected: &radixv1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}},
		{name: "override empty commonConfig.Webhook with environmentConfig", commonConfig: &radixv1.Notifications{Webhook: nil}, configureEnvironment: true, environmentConfig: &radixv1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}, expected: &radixv1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			const envName = "anyenv"
			jobComponent := utils.AnApplicationJobComponent().WithName("anyjob").WithNotifications(scenario.commonConfig)
			if scenario.configureEnvironment {
				jobComponent = jobComponent.WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().WithEnvironment(envName).WithNotifications(scenario.environmentConfig),
				)
			}
			ra := utils.ARadixApplication().WithJobComponents(jobComponent).BuildRA()
			sut := jobComponentsBuilder{ra: ra, env: envName, componentImages: make(pipeline.DeployComponentImages)}
			jobs, err := sut.JobComponents(context.Background())
			require.NoError(t, err)
			assert.Equal(t, scenario.expected, jobs[0].Notifications)
		})
	}
}

func Test_GetRadixJobComponents_FailurePolicy(t *testing.T) {

	tests := map[string]struct {
		commonConfig         *radixv1.RadixJobComponentFailurePolicy
		configureEnvironment bool
		environmentConfig    *radixv1.RadixJobComponentFailurePolicy
		expected             *radixv1.RadixJobComponentFailurePolicy
	}{
		"nil when common and environment is nil": {
			commonConfig:         nil,
			configureEnvironment: true,
			environmentConfig:    nil,
			expected:             nil,
		},
		"nil when common is nil and environment not set": {
			commonConfig:         nil,
			configureEnvironment: false,
			expected:             nil,
		},
		"use common when environment is nil": {
			commonConfig: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{1, 2, 3}}},
				},
			},
			configureEnvironment: true,
			environmentConfig:    nil,
			expected: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{1, 2, 3}}},
				},
			},
		},
		"use common when environment not set": {
			commonConfig: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{1, 2, 3}}},
				},
			},
			configureEnvironment: false,
			expected: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{1, 2, 3}}},
				},
			},
		},
		"use environment when common is nil": {
			commonConfig:         nil,
			configureEnvironment: true,
			environmentConfig: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{1, 2, 3}}},
				},
			},
			expected: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{1, 2, 3}}},
				},
			},
		},
		"use environment when both common and environment is set": {
			commonConfig: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{1, 2, 3}}},
					{Action: radixv1.RadixJobComponentFailurePolicyActionIgnore, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpNotIn, Values: []int32{4, 5, 6}}},
				},
			},
			configureEnvironment: true,
			environmentConfig: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionCount, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{7, 8}}},
				},
			},
			expected: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionCount, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{7, 8}}},
				},
			},
		},
		"use environment when environment empty and common is set": {
			commonConfig: &radixv1.RadixJobComponentFailurePolicy{
				Rules: []radixv1.RadixJobComponentFailurePolicyRule{
					{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{1, 2, 3}}},
					{Action: radixv1.RadixJobComponentFailurePolicyActionIgnore, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpNotIn, Values: []int32{4, 5, 6}}},
				},
			},
			configureEnvironment: true,
			environmentConfig:    &radixv1.RadixJobComponentFailurePolicy{},
			expected:             &radixv1.RadixJobComponentFailurePolicy{},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			const envName = "anyenv"
			jobComponent := utils.AnApplicationJobComponent().WithName("anyjob").WithFailurePolicy(test.commonConfig)
			if test.configureEnvironment {
				jobComponent = jobComponent.WithEnvironmentConfigs(
					utils.AJobComponentEnvironmentConfig().WithEnvironment(envName).WithFailurePolicy(test.environmentConfig),
				)
			}
			ra := utils.ARadixApplication().WithJobComponents(jobComponent).BuildRA()
			sut := jobComponentsBuilder{ra: ra, env: envName, componentImages: make(pipeline.DeployComponentImages)}
			jobs, err := sut.JobComponents(context.Background())
			require.NoError(t, err)
			assert.Equal(t, test.expected, jobs[0].FailurePolicy)
		})
	}
}

func TestGetRadixJobComponentsForEnv_ImageWithImageTagName(t *testing.T) {
	const (
		dynamicImageName1 = "custom-image-name1:{imageTagName}"
		dynamicImageName2 = "custom-image-name2:{imageTagName}"
		staticImageName1  = "custom-image-name1:latest"
		staticImageName2  = "custom-image-name2:latest"
		environment       = "dev"
	)
	type scenario struct {
		name                           string
		componentImages                map[string]string
		externalImageTagNames          map[string]string // map[component-name]image-tag
		componentImageTagNames         map[string]string // map[component-name]image-tag
		environmentConfigImageTagNames map[string]string // map[component-name]image-tag
		expectedJobComponentImage      map[string]string // map[component-name]image
		expectedError                  error
	}
	componentName1 := "componentA"
	componentName2 := "componentB"
	scenarios := []scenario{
		{
			name: "image has no tagName",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: staticImageName2,
			},
			expectedJobComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: staticImageName2,
			},
		},
		{
			name: "image has tagName, but no tags provided",
			componentImages: map[string]string{
				componentName1: dynamicImageName1,
				componentName2: staticImageName2,
			},
			expectedError: errorMissingExpectedDynamicImageTagName(componentName1),
		},
		{
			name: "with component image-tags",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: dynamicImageName2,
			},
			componentImageTagNames: map[string]string{
				componentName2: "tag-component-b",
			},
			expectedJobComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: "custom-image-name2:tag-component-b",
			},
		},
		{
			name: "with environment image-tags",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: dynamicImageName2,
			},
			environmentConfigImageTagNames: map[string]string{
				componentName2: "tag-component-b",
			},
			expectedJobComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: "custom-image-name2:tag-component-b",
			},
		},
		{
			name: "with environment overriding image-tags",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: dynamicImageName2,
			},
			componentImageTagNames: map[string]string{
				componentName2: "tag-component-b",
			},
			environmentConfigImageTagNames: map[string]string{
				componentName2: "tag-component-env-b",
			},
			expectedJobComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: "custom-image-name2:tag-component-env-b",
			},
		},
		{
			name: "external image-tags is used when missing component env imageTagName",
			componentImages: map[string]string{
				componentName1: staticImageName1,
				componentName2: dynamicImageName2,
			},
			externalImageTagNames: map[string]string{
				componentName2: "external-tag-component-b",
			},
			expectedJobComponentImage: map[string]string{
				componentName1: staticImageName1,
				componentName2: "custom-image-name2:external-tag-component-b",
			},
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			componentImages := make(pipeline.DeployComponentImages)
			var componentBuilders []utils.RadixApplicationJobComponentBuilder
			for _, jobComponentName := range []string{componentName1, componentName2} {
				componentImages[jobComponentName] = pipeline.DeployComponentImage{ImagePath: ts.componentImages[jobComponentName], ImageTagName: ts.externalImageTagNames[jobComponentName]}
				componentBuilder := utils.NewApplicationJobComponentBuilder()
				componentBuilder.WithName(jobComponentName).WithImage(ts.componentImages[jobComponentName]).WithImageTagName(ts.componentImageTagNames[jobComponentName]).
					WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(environment).WithImageTagName(ts.environmentConfigImageTagNames[jobComponentName]))
				componentBuilders = append(componentBuilders, componentBuilder)
			}

			ra := utils.ARadixApplication().WithEnvironment(environment, "master").WithJobComponents(componentBuilders...).BuildRA()

			deployJobComponents, err := NewJobComponentsBuilder(ra, environment, componentImages, make(radixv1.EnvVarsMap), nil).JobComponents(context.Background())
			if err != nil && ts.expectedError == nil {
				assert.Fail(t, fmt.Sprintf("unexpected error %v", err))
				return
			}
			if err == nil && ts.expectedError != nil {
				assert.Fail(t, fmt.Sprintf("missing an expected error %s", ts.expectedError))
				return
			}
			if err != nil && err.Error() != ts.expectedError.Error() {
				assert.Fail(t, fmt.Sprintf("expected error '%s', but got '%s'", ts.expectedError, err.Error()))
				return
			}
			if ts.expectedError != nil {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, 2, len(deployJobComponents))
			assert.Equal(t, ts.expectedJobComponentImage[deployJobComponents[0].Name], deployJobComponents[0].Image)
		})
	}
}

func TestGetRadixJobComponentsForEnv_ReadOnlyFileSystem(t *testing.T) {
	const (
		environment = "dev"
	)
	// Test cases with different values for ReadOnlyFileSystem
	testCases := []struct {
		name                  string
		readOnlyFileSystem    *bool
		readOnlyFileSystemEnv *bool

		expectedReadOnlyFile *bool
	}{
		{"No configuration set", nil, nil, nil},
		{"Env controls when readOnlyFileSystem is nil, set to true", nil, pointers.Ptr(true), pointers.Ptr(true)},
		{"Env controls when readOnlyFileSystem is nil, set to false", nil, pointers.Ptr(false), pointers.Ptr(false)},
		{"readOnlyFileSystem set to true, no env config", pointers.Ptr(true), nil, pointers.Ptr(true)},
		{"Both readOnlyFileSystem and monitoringEnv set to true", pointers.Ptr(true), pointers.Ptr(true), pointers.Ptr(true)},
		{"Env overrides to false when both is set", pointers.Ptr(true), pointers.Ptr(false), pointers.Ptr(false)},
		{"readOnlyFileSystem set to false, no env config", pointers.Ptr(false), nil, pointers.Ptr(false)},
		{"Env overrides to true when both is set", pointers.Ptr(false), pointers.Ptr(true), pointers.Ptr(true)},
		{"Both readOnlyFileSystem and monitoringEnv set to false", pointers.Ptr(false), pointers.Ptr(false), pointers.Ptr(false)},
	}

	for _, ts := range testCases {
		t.Run(ts.name, func(t *testing.T) {
			componentImages := make(pipeline.DeployComponentImages)
			var componentBuilders []utils.RadixApplicationJobComponentBuilder

			componentBuilder := utils.NewApplicationJobComponentBuilder().
				WithName("jobComponentName").
				WithReadOnlyFileSystem(ts.readOnlyFileSystem).
				WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().
					WithEnvironment(environment).
					WithReadOnlyFileSystem(ts.readOnlyFileSystemEnv))
			componentBuilders = append(componentBuilders, componentBuilder)

			ra := utils.ARadixApplication().WithEnvironment(environment, "master").WithJobComponents(componentBuilders...).BuildRA()

			deployComponents, err := NewJobComponentsBuilder(ra, environment, componentImages, make(radixv1.EnvVarsMap), nil).JobComponents(context.Background())
			assert.NoError(t, err)
			deployComponent, exists := slice.FindFirst(deployComponents, func(component radixv1.RadixDeployJobComponent) bool {
				return component.Name == "jobComponentName"
			})
			require.True(t, exists)

			assert.Equal(t, ts.expectedReadOnlyFile, deployComponent.ReadOnlyFileSystem)

		})
	}
}

func Test_GetRadixJobComponentAndEnv_Monitoring(t *testing.T) {
	componentName := "comp"
	env := "dev"
	anyImagePath := "imagepath"
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	// Test cases with different values for Monitoring
	testCases := []struct {
		description   string
		monitoring    *bool
		monitoringEnv *bool

		expectedMonitoring bool
	}{
		{"No configuration set", nil, nil, false},
		{"Env controls when monitoring is nil, set to true", nil, pointers.Ptr(true), true},
		{"Env controls when monitoring is nil, set to false", nil, pointers.Ptr(false), false},
		{"monitoring set to true, no env config", pointers.Ptr(true), nil, true},
		{"Both monitoring and monitoringEnv set to true", pointers.Ptr(true), pointers.Ptr(true), true},
		{"Env overrides to false when both is set", pointers.Ptr(true), pointers.Ptr(false), false},
		{"monitoring set to false, no env config", pointers.Ptr(false), nil, false},
		{"Env overrides to true when both is set", pointers.Ptr(false), pointers.Ptr(true), true},
		{"Both monitoring and monitoringEnv set to false", pointers.Ptr(false), pointers.Ptr(false), false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			ra := utils.ARadixApplication().
				WithJobComponents(
					utils.NewApplicationJobComponentBuilder().
						WithName(componentName).
						WithMonitoring(testCase.monitoring).
						WithEnvironmentConfigs(
							utils.AJobComponentEnvironmentConfig().
								WithEnvironment(env).
								WithMonitoring(testCase.monitoringEnv),
							utils.AJobComponentEnvironmentConfig().
								WithEnvironment("prod").
								WithMonitoring(pointers.Ptr(false)),
						)).BuildRA()

			deployComponents, _ := NewJobComponentsBuilder(ra, env, componentImages, envVarsMap, nil).JobComponents(context.Background())
			deployComponent, exists := slice.FindFirst(deployComponents, func(component radixv1.RadixDeployJobComponent) bool {
				return component.Name == componentName
			})
			require.True(t, exists)
			assert.Equal(t, testCase.expectedMonitoring, deployComponent.Monitoring)
		})
	}
}

func Test_GetRadixJobComponents_VolumeMounts(t *testing.T) {
	testCases := map[string]struct {
		componentVolumeMounts   []radixv1.RadixVolumeMount
		environmentVolumeMounts []radixv1.RadixVolumeMount
		expectedVolumeMounts    []radixv1.RadixVolumeMount
	}{
		"Path": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", Path: "comp1"},
				{Name: "vol-common-override", Path: "comp2"},
				{Name: "vol-comp", Path: "comp3"},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override"},
				{Name: "vol-common-override", Path: "env1"},
				{Name: "vol-env", Path: "env2"},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", Path: "comp1"},
				{Name: "vol-common-override", Path: "env1"},
				{Name: "vol-comp", Path: "comp3"},
				{Name: "vol-env", Path: "env2"},
			},
		},
		"Deprecated: Storage": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", Storage: "comp1"},
				{Name: "vol-common-override", Storage: "comp2"},
				{Name: "vol-comp", Storage: "comp3"},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override"},
				{Name: "vol-common-override", Storage: "env1"},
				{Name: "vol-env", Storage: "env2"},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", Storage: "comp1"},
				{Name: "vol-common-override", Storage: "env1"},
				{Name: "vol-comp", Storage: "comp3"},
				{Name: "vol-env", Storage: "env2"},
			},
		},
		"Deprecated: UID": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", UID: "comp1"},
				{Name: "vol-common-override", UID: "comp2"},
				{Name: "vol-comp", UID: "comp3"},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override"},
				{Name: "vol-common-override", UID: "env1"},
				{Name: "vol-env", UID: "env2"},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", UID: "comp1"},
				{Name: "vol-common-override", UID: "env1"},
				{Name: "vol-comp", UID: "comp3"},
				{Name: "vol-env", UID: "env2"},
			},
		},
		"Deprecated: GID": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", GID: "comp1"},
				{Name: "vol-common-override", GID: "comp2"},
				{Name: "vol-comp", GID: "comp3"},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override"},
				{Name: "vol-common-override", GID: "env1"},
				{Name: "vol-env", GID: "env2"},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", GID: "comp1"},
				{Name: "vol-common-override", GID: "env1"},
				{Name: "vol-comp", GID: "comp3"},
				{Name: "vol-env", GID: "env2"},
			},
		},
		"Deprecated: RequestsStorage": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", RequestsStorage: resource.MustParse("1G")},
				{Name: "vol-common-override", RequestsStorage: resource.MustParse("2G")},
				{Name: "vol-comp", RequestsStorage: resource.MustParse("3G")},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override"},
				{Name: "vol-common-override", RequestsStorage: resource.MustParse("1M")},
				{Name: "vol-env", RequestsStorage: resource.MustParse("2M")},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", RequestsStorage: resource.MustParse("1G")},
				{Name: "vol-common-override", RequestsStorage: resource.MustParse("1M")},
				{Name: "vol-comp", RequestsStorage: resource.MustParse("3G")},
				{Name: "vol-env", RequestsStorage: resource.MustParse("2M")},
			},
		},
		"Deprecated: AccessMode": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", AccessMode: "comp1"},
				{Name: "vol-common-override", AccessMode: "comp2"},
				{Name: "vol-comp", AccessMode: "comp3"},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override"},
				{Name: "vol-common-override", AccessMode: "env1"},
				{Name: "vol-env", AccessMode: "env2"},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", AccessMode: "comp1"},
				{Name: "vol-common-override", AccessMode: "env1"},
				{Name: "vol-comp", AccessMode: "comp3"},
				{Name: "vol-env", AccessMode: "env2"},
			},
		},
		"Blobfuse2: nil handling": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override"},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override"},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
			},
		},
		"Blobfuse2: Protocol": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Protocol: "env2"}},
			},
		},
		"Blobfuse2: Container": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: "env2"}},
			},
		},
		"Blobfuse2: GID": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{GID: "env2"}},
			},
		},
		"Blobfuse2: UID": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UID: "env2"}},
			},
		},
		"Blobfuse2: RequestsStorage": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("1G")}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("2G")}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("3G")}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("1M")}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("2M")}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("1G")}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("1M")}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("3G")}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{RequestsStorage: resource.MustParse("2M")}},
			},
		},
		"Blobfuse2: AccessMode": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AccessMode: "env2"}},
			},
		},
		"Blobfuse2: UseAdls": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(true)}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(false)}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(true)}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(false)}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(true)}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(false)}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(true)}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(false)}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(true)}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(false)}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(false)}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(true)}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(true)}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAdls: pointers.Ptr(false)}},
			},
		},
		"Blobfuse2: UseAzureIdentity": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(true)}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(false)}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(true)}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(false)}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(true)}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(false)}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(true)}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(false)}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(true)}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(false)}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(false)}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(true)}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(true)}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{UseAzureIdentity: pointers.Ptr(false)}},
			},
		},
		"Blobfuse2: StorageAccount": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StorageAccount: "env2"}},
			},
		},
		"Blobfuse2: ResourceGroup": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{ResourceGroup: "env2"}},
			},
		},
		"Blobfuse2: SubscriptionId": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{SubscriptionId: "env2"}},
			},
		},
		"Blobfuse2: TenantId": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "comp2"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "comp3"}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "env1"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "env2"}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "comp1"}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "env1"}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "comp3"}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{TenantId: "env2"}},
			},
		},
		"Blobfuse2: CacheMode": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("comp1")}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("comp2")}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("comp3")}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("env1")}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("env2")}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("comp1")}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("env1")}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("comp3")}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{CacheMode: pointers.Ptr[radixv1.BlobFuse2CacheMode]("env2")}},
			},
		},
		"Blobfuse2.AttributeCacheOptions: nil handling": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
			},
		},
		"Blobfuse2.AttributeCacheOptions: Timeout": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](2)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](3)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](10)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](20)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](10)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](3)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{AttributeCacheOptions: &radixv1.BlobFuse2AttributeCacheOptions{Timeout: pointers.Ptr[uint32](20)}}},
			},
		},
		"Blobfuse2.FileCacheOptions: nil handling": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
			},
		},
		"Blobfuse2.FileCacheOptions: Timeout": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](2)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](3)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](10)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](20)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](10)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](3)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{FileCacheOptions: &radixv1.BlobFuse2FileCacheOptions{Timeout: pointers.Ptr[uint32](20)}}},
			},
		},
		"Blobfuse2.StreamingOptions: nil handling": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
			},
		},
		"Blobfuse2.StreamingOptions: Enabled": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(true)}}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(true)}}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(true)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{}}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(true)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(true)}}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(true)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(true)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{StreamingOptions: &radixv1.BlobFuse2StreamingOptions{Enabled: pointers.Ptr(false)}}},
			},
		},

		"Blobfuse2.BlockCacheOptions: nil handling": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
			},
		},
		"Blobfuse2.BlockCacheOptions: BlockSize": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](2)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](3)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](10)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](20)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](10)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](3)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{BlockSize: pointers.Ptr[uint32](20)}}},
			},
		},
		"Blobfuse2.BlockCacheOptions: PoolSize": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](2)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](3)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](10)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](20)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](10)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](3)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PoolSize: pointers.Ptr[uint32](20)}}},
			},
		},
		"Blobfuse2.BlockCacheOptions: DiskSize": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](2)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](3)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](10)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](20)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](10)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](3)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskSize: pointers.Ptr[uint32](20)}}},
			},
		},
		"Blobfuse2.BlockCacheOptions: DiskTimeout": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](2)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](3)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](10)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](20)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](10)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](3)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{DiskTimeout: pointers.Ptr[uint32](20)}}},
			},
		},
		"Blobfuse2.BlockCacheOptions: PrefetchCount": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](2)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](3)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](10)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](20)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](10)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](3)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchCount: pointers.Ptr[uint32](20)}}},
			},
		},
		"Blobfuse2.BlockCacheOptions: Parallelism": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](2)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](3)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](10)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](20)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](1)}}},
				{Name: "vol-common-override", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](10)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](3)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{Parallelism: pointers.Ptr[uint32](20)}}},
			},
		},
		"Blobfuse2.BlockCacheOptions: PrefetchOnOpen": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(true)}}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(false)}}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(true)}}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(false)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(true)}}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{}}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(false)}}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(true)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(false)}}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "vol-common-no-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(true)}}},
				{Name: "vol-common-no-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(false)}}},
				{Name: "vol-common-override-true", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(false)}}},
				{Name: "vol-common-override-false", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(true)}}},
				{Name: "vol-comp", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(true)}}},
				{Name: "vol-env", BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{BlockCacheOptions: &radixv1.BlobFuse2BlockCacheOptions{PrefetchOnOpen: pointers.Ptr(false)}}},
			},
		},
		"EmptyDir": {
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage-common", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("1M")}},
				{Name: "storage-comp", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("2M")}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage-common", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("3M")}},
				{Name: "storage-env", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("4M")}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage-common", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("3M")}},
				{Name: "storage-comp", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("2M")}},
				{Name: "storage-env", EmptyDir: &radixv1.RadixEmptyDirVolumeMount{SizeLimit: resource.MustParse("4M")}},
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			const (
				env           = "any-env"
				componentName = "any-comp"
			)
			ra := utils.ARadixApplication().
				WithJobComponents(
					utils.NewApplicationJobComponentBuilder().
						WithName(componentName).
						WithVolumeMounts(testCase.componentVolumeMounts).
						WithEnvironmentConfigs(
							utils.AJobComponentEnvironmentConfig().
								WithEnvironment(env).
								WithVolumeMounts(testCase.environmentVolumeMounts),
						)).BuildRA()

			deployComponents, _ := NewJobComponentsBuilder(ra, env, nil, nil, nil).JobComponents(context.Background())
			deployComponent, exists := slice.FindFirst(deployComponents, func(component radixv1.RadixDeployJobComponent) bool {
				return component.Name == componentName
			})
			require.True(t, exists)
			assert.Equal(t, testCase.expectedVolumeMounts, deployComponent.VolumeMounts)
		})
	}
}

func Test_JobCompopnentBuilder_Runtime(t *testing.T) {
	jobComponentBuilder := utils.NewApplicationJobComponentBuilder().
		WithName("anyjob").
		WithRuntime(&radixv1.Runtime{Architecture: "commonarch"}).
		WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().
			WithEnvironment("dev").
			WithRuntime(&radixv1.Runtime{Architecture: "devarch"}))

	ra := utils.ARadixApplication().
		WithEnvironmentNoBranch("dev").
		WithEnvironmentNoBranch("prod").
		WithJobComponents(jobComponentBuilder).BuildRA()

	tests := map[string]struct {
		env             string
		deployImages    pipeline.DeployComponentImages
		expectedRuntime *radixv1.Runtime
	}{
		"dev:nil when deployImages is nil": {
			env:             "dev",
			deployImages:    nil,
			expectedRuntime: nil,
		},
		"dev:nil when job not defined in deployImages": {
			env:             "dev",
			deployImages:    pipeline.DeployComponentImages{"otherjob": {Runtime: &radixv1.Runtime{Architecture: "otherjobarch"}}},
			expectedRuntime: nil,
		},
		"dev:runtime from deployImage job when defined": {
			env:             "dev",
			deployImages:    pipeline.DeployComponentImages{"anyjob": {Runtime: &radixv1.Runtime{Architecture: "anjobarch"}}},
			expectedRuntime: &radixv1.Runtime{Architecture: "anjobarch"},
		},
		"prod:nil when deployImages is nil": {
			env:             "prod",
			deployImages:    nil,
			expectedRuntime: nil,
		},
		"prod:nil when job not defined in deployImages": {
			env:             "prod",
			deployImages:    pipeline.DeployComponentImages{"otherjob": {Runtime: &radixv1.Runtime{Architecture: "otherjobarch"}}},
			expectedRuntime: nil,
		},
		"prod:runtime from deployImage job when defined": {
			env:             "prod",
			deployImages:    pipeline.DeployComponentImages{"anyjob": {Runtime: &radixv1.Runtime{Architecture: "anjobarch"}}},
			expectedRuntime: &radixv1.Runtime{Architecture: "anjobarch"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			deployJobComponents, err := NewJobComponentsBuilder(ra, test.env, test.deployImages, make(radixv1.EnvVarsMap), nil).JobComponents(context.Background())
			require.NoError(t, err)
			require.Len(t, deployJobComponents, 1)
			deployJobComponent := deployJobComponents[0]
			actualRuntime := deployJobComponent.Runtime
			if test.expectedRuntime == nil {
				assert.Nil(t, actualRuntime)
			} else {
				assert.Equal(t, test.expectedRuntime, actualRuntime)
			}
		})
	}
}
