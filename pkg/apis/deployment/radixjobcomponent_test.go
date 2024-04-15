package deployment

import (
	"fmt"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

func Test_GetRadixJobComponents_BuildAllJobComponents(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job1").
				WithSchedulerPort(pointers.Ptr[int32](8888)).
				WithPayloadPath(utils.StringPtr("/path/to/payload")),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{
		ra:              ra,
		env:             "any",
		componentImages: make(pipeline.DeployComponentImages),
	}
	jobs, err := cfg.JobComponents()
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
	assert.Equal(t, "job1", jobs[0].Name)
	assert.Equal(t, pointers.Ptr[int32](8888), jobs[0].SchedulerPort)
	assert.Equal(t, "/path/to/payload", jobs[0].Payload.Path)
	assert.Equal(t, "job2", jobs[1].Name)
	assert.Nil(t, jobs[1].SchedulerPort)
	assert.Nil(t, jobs[1].Payload)
}

func Test_GetRadixJobComponentsWithNode_BuildAllJobComponents(t *testing.T) {
	gpu := "any-gpu"
	gpuCount := "12"
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job1").
				WithSchedulerPort(pointers.Ptr[int32](8888)).
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
	jobs, err := cfg.JobComponents()
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
	jobComponents, err := cfg.JobComponents()
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
	jobComponents, err := cfg.JobComponents()
	require.NoError(t, err)
	job := jobComponents[0]
	assert.True(t, job.Monitoring)
	assert.Empty(t, job.MonitoringConfig.PortName)
	assert.Empty(t, job.MonitoringConfig.Path)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(pipeline.DeployComponentImages)}
	jobComponents, err = cfg.JobComponents()
	require.NoError(t, err)
	job = jobComponents[0]
	assert.False(t, job.Monitoring)
	assert.Empty(t, job.MonitoringConfig.PortName)
	assert.Empty(t, job.MonitoringConfig.Path)

	cfg = jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(pipeline.DeployComponentImages)}
	jobComponents, err = cfg.JobComponents()
	require.NoError(t, err)
	job = jobComponents[1]
	assert.True(t, job.Monitoring)
	assert.Equal(t, monitoringConfig.PortName, job.MonitoringConfig.PortName)
	assert.Equal(t, monitoringConfig.Path, job.MonitoringConfig.Path)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(pipeline.DeployComponentImages)}
	jobComponents, err = cfg.JobComponents()
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
			jobs, err := sut.JobComponents()
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
	jobs, err := cfg.JobComponents()
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
		jobs, err := cfg.JobComponents()
		require.NoError(t, err)
		assert.Equal(t, envGpu1, jobs[0].Node.Gpu)
		assert.Equal(t, envGpuCount1, jobs[0].Node.GpuCount)
	})
	t.Run("override job gpu-count with environment gpu-count", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env2"}
		jobs, err := cfg.JobComponents()
		require.NoError(t, err)
		assert.Equal(t, compGpu, jobs[0].Node.Gpu)
		assert.Equal(t, envGpuCount2, jobs[0].Node.GpuCount)
	})
	t.Run("override job gpu with environment gpu", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env3"}
		jobs, err := cfg.JobComponents()
		require.NoError(t, err)
		assert.Equal(t, envGpu3, jobs[0].Node.Gpu)
		assert.Equal(t, compGpuCount, jobs[0].Node.GpuCount)
	})
	t.Run("do not override job gpu or gpu-count with environment gpu or gpu-count", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env4"}
		jobs, err := cfg.JobComponents()
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
	jobs, err := cfg.JobComponents()
	require.NoError(t, err)
	assert.Equal(t, "1100Mi", jobs[0].Resources.Requests["memory"])
	assert.Equal(t, "1150m", jobs[0].Resources.Requests["cpu"])
	assert.Equal(t, "1200Mi", jobs[0].Resources.Limits["memory"])
	assert.Equal(t, "1250m", jobs[0].Resources.Limits["cpu"])
	assert.Empty(t, jobs[1].Resources)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(pipeline.DeployComponentImages)}
	jobs, err = cfg.JobComponents()
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
	jobs, err := cfg.JobComponents()
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
	jobs, err := cfg.JobComponents()
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
				WithSchedulerPort(numbers.Int32Ptr(8888)).
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
	env1Job, err := cfgEnv1.JobComponents()
	require.NoError(t, err)
	env2Job, err := cfgEnv2.JobComponents()
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
			devJobs, err := devBuilder.JobComponents()
			require.NoError(t, err, "devJobs build error")
			require.Len(t, devJobs, 1, "devJobs length")
			assert.Equal(t, scenario.expectedEnvDevBackoffLimit, devJobs[0].BackoffLimit, "devJobs backoffLimit")

			prodBuilder := jobComponentsBuilder{ra: ra, env: prodEnvName, componentImages: pipeline.DeployComponentImages{}}
			prodJobs, err := prodBuilder.JobComponents()
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
			jobs, err := sut.JobComponents()
			require.NoError(t, err)
			assert.Equal(t, scenario.expected, jobs[0].Notifications)
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

			deployJobComponents, err := NewJobComponentsBuilder(ra, environment, componentImages, make(radixv1.EnvVarsMap), nil).JobComponents()
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
		{"Env controls when readOnlyFileSystem is nil, set to true", nil, utils.BoolPtr(true), utils.BoolPtr(true)},
		{"Env controls when readOnlyFileSystem is nil, set to false", nil, utils.BoolPtr(false), utils.BoolPtr(false)},
		{"readOnlyFileSystem set to true, no env config", utils.BoolPtr(true), nil, utils.BoolPtr(true)},
		{"Both readOnlyFileSystem and monitoringEnv set to true", utils.BoolPtr(true), utils.BoolPtr(true), utils.BoolPtr(true)},
		{"Env overrides to false when both is set", utils.BoolPtr(true), utils.BoolPtr(false), utils.BoolPtr(false)},
		{"readOnlyFileSystem set to false, no env config", utils.BoolPtr(false), nil, utils.BoolPtr(false)},
		{"Env overrides to true when both is set", utils.BoolPtr(false), utils.BoolPtr(true), utils.BoolPtr(true)},
		{"Both readOnlyFileSystem and monitoringEnv set to false", utils.BoolPtr(false), utils.BoolPtr(false), utils.BoolPtr(false)},
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

			deployJobComponent, err := NewJobComponentsBuilder(ra, environment, componentImages, make(radixv1.EnvVarsMap), nil).JobComponents()
			assert.NoError(t, err)

			assert.Equal(t, ts.expectedReadOnlyFile, deployJobComponent[0].ReadOnlyFileSystem)

		})
	}
}

func Test_GetRadixJobComponentAndEnv_Monitoring(t *testing.T) {
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

			deployComponent, _ := NewJobComponentsBuilder(ra, env, componentImages, envVarsMap, nil).JobComponents()
			assert.Equal(t, testCase.expectedMonitoring, deployComponent[0].Monitoring)
		})
	}
}

func Test_GetRadixJobComponents_VolumeMounts(t *testing.T) {
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	const (
		path1          = "/home/path1"
		path2          = "/home/path2"
		container1     = "container1"
		container2     = "container2"
		user1000       = "1000"
		user2000       = "2000"
		group1100      = "1100"
		group2200      = "2200"
		skuStandardLRS = "Standard_LRS"
		skuStandardGRS = "Standard_GRS"
	)
	var (
		accessModeReadWriteMany         = strings.ToLower(string(corev1.ReadWriteMany))
		accessModeReadOnlyMany          = strings.ToLower(string(corev1.ReadOnlyMany))
		bindingModeImmediate            = strings.ToLower(string(storagev1.VolumeBindingImmediate))
		bindingModeWaitForFirstConsumer = strings.ToLower(string(storagev1.VolumeBindingWaitForFirstConsumer))
	)
	testCases := []struct {
		description             string
		componentVolumeMounts   []radixv1.RadixVolumeMount
		environmentVolumeMounts []radixv1.RadixVolumeMount
		expectedVolumeMounts    []radixv1.RadixVolumeMount
	}{
		{description: "No configuration set"},
		{
			description: "Component sets VolumeMounts for azure-blob",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path1, Storage: container1, UID: user1000, GID: group1100, SkuName: skuStandardLRS, AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path1, Storage: container1, UID: user1000, GID: group1100, SkuName: skuStandardLRS, AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate},
			},
		},
		{
			description: "Component sets VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
		},
		{
			description: "Env sets VolumeMounts for azure-blob",
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path2, Storage: container2, UID: user2000, GID: group2200, SkuName: skuStandardGRS, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path2, Storage: container2, UID: user2000, GID: group2200, SkuName: skuStandardGRS, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer},
			},
		},
		{
			description: "Env sets VolumeMounts for blobFuse2",
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
		},
		{
			description: "Env overrides component VolumeMounts for azure-blob",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path1, Storage: container1, UID: user1000, GID: group1100, SkuName: skuStandardLRS, AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path2, Storage: container2, UID: user2000, GID: group2200, SkuName: skuStandardGRS, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Type: radixv1.MountTypeBlobFuse2FuseCsiAzure, Path: path2, Storage: container2, UID: user2000, GID: group2200, SkuName: skuStandardGRS, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer},
			},
		},
		{
			description: "Env overrides component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
		},
		{
			description: "Env overrides and adds some component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M",
					Streaming: &radixv1.RadixVolumeMountStreaming{BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
		},
		{
			description: "Env overrides and adds other component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2)},
				}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M",
					Streaming: &radixv1.RadixVolumeMountStreaming{BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
		},
		{
			description: "Env adds streaming to component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: container1,
					Streaming: nil,
				}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: container1,
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: container1,
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
				}},
			},
		},
		{
			description: "Env overrides and adds some component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1,
					Streaming: &radixv1.RadixVolumeMountStreaming{BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			environmentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: container1,
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false)},
				}},
			},
			expectedVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{Container: container1,
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
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

			deployComponent, _ := NewJobComponentsBuilder(ra, env, componentImages, envVarsMap, nil).JobComponents()
			assert.Equal(t, testCase.expectedVolumeMounts, deployComponent[0].VolumeMounts)
		})
	}
}

func Test_GetRadixJobComponents_VolumeMounts_MultipleEnvs(t *testing.T) {
	componentImages := make(pipeline.DeployComponentImages)
	componentImages["app"] = pipeline.DeployComponentImage{ImagePath: anyImagePath}
	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = "anycommit"
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = "anytag"

	const (
		path1          = "/home/path1"
		path2          = "/home/path2"
		container1     = "container1"
		container2     = "container2"
		user1000       = "1000"
		user2000       = "2000"
		group1100      = "1100"
		group2200      = "2200"
		skuStandardLRS = "Standard_LRS"
		skuStandardGRS = "Standard_GRS"
		env1           = "env1"
		env2           = "env2"
	)
	var (
		accessModeReadWriteMany         = strings.ToLower(string(corev1.ReadWriteMany))
		accessModeReadOnlyMany          = strings.ToLower(string(corev1.ReadOnlyMany))
		bindingModeImmediate            = strings.ToLower(string(storagev1.VolumeBindingImmediate))
		bindingModeWaitForFirstConsumer = strings.ToLower(string(storagev1.VolumeBindingWaitForFirstConsumer))
	)
	testCases := []struct {
		description             string
		componentVolumeMounts   []radixv1.RadixVolumeMount
		environmentVolumeMounts map[string][]radixv1.RadixVolumeMount
		expectedVolumeMounts    map[string][]radixv1.RadixVolumeMount
	}{
		{
			description: "Env overrides component VolumeMounts for blobFuse2",
			componentVolumeMounts: []radixv1.RadixVolumeMount{
				{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
					Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
					Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
				}},
			},
			environmentVolumeMounts: map[string][]radixv1.RadixVolumeMount{
				env1: {
					{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
						Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
					}}},
			},
			expectedVolumeMounts: map[string][]radixv1.RadixVolumeMount{
				env1: {
					{Name: "storage1", Path: path2, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: container2, GID: group2200, UID: user2000, SkuName: skuStandardGRS, RequestsStorage: "2M", AccessMode: accessModeReadOnlyMany, BindingMode: bindingModeWaitForFirstConsumer, UseAdls: pointers.Ptr(false),
						Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(false), BlockSize: pointers.Ptr[uint64](11), MaxBuffers: pointers.Ptr[uint64](22), BufferSize: pointers.Ptr[uint64](33), StreamCache: pointers.Ptr[uint64](44), MaxBlocksPerFile: pointers.Ptr[uint64](55)},
					}}},
				env2: {
					{Name: "storage1", Path: path1, BlobFuse2: &radixv1.RadixBlobFuse2VolumeMount{
						Container: container1, GID: group1100, UID: user1000, SkuName: skuStandardLRS, RequestsStorage: "1M", AccessMode: accessModeReadWriteMany, BindingMode: bindingModeImmediate, UseAdls: pointers.Ptr(true),
						Streaming: &radixv1.RadixVolumeMountStreaming{Enabled: pointers.Ptr(true), BlockSize: pointers.Ptr[uint64](1), MaxBuffers: pointers.Ptr[uint64](2), BufferSize: pointers.Ptr[uint64](3), StreamCache: pointers.Ptr[uint64](4), MaxBlocksPerFile: pointers.Ptr[uint64](5)},
					}}},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			componentBuilder := utils.NewApplicationJobComponentBuilder().WithName(componentName).WithVolumeMounts(testCase.componentVolumeMounts)
			for envName, volumeMounts := range testCase.environmentVolumeMounts {
				componentBuilder = componentBuilder.WithEnvironmentConfig(utils.AJobComponentEnvironmentConfig().WithEnvironment(envName).WithVolumeMounts(volumeMounts))
			}

			ra := utils.ARadixApplication().WithEnvironment(env1, "").WithEnvironment(env2, "").
				WithJobComponents(componentBuilder).BuildRA()

			for _, envName := range []string{env1, env2} {
				deployComponent, _ := NewJobComponentsBuilder(ra, envName, componentImages, envVarsMap, nil).JobComponents()
				assert.Equal(t, testCase.expectedVolumeMounts[envName], deployComponent[0].VolumeMounts)
			}
		})
	}
}
