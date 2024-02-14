package deployment

import (
	"fmt"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"

	"github.com/equinor/radix-operator/pkg/apis/defaults"

	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				WithNode(v1.RadixNode{Gpu: gpu, GpuCount: gpuCount}),
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

	envVarsMap := make(v1.EnvVarsMap)
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
	monitoringConfig := v1.MonitoringConfig{
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
						WithMonitoring(true),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2"),
				),
			utils.AnApplicationJobComponent().
				WithName("job_2").
				WithMonitoringConfig(monitoringConfig).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithMonitoring(true),
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
		commonConfig         *v1.Identity
		configureEnvironment bool
		environmentConfig    *v1.Identity
		expected             *v1.Identity
	}

	scenarios := []scenarioSpec{
		{name: "nil when commonConfig and environmentConfig is empty", commonConfig: &v1.Identity{}, configureEnvironment: true, environmentConfig: &v1.Identity{}, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: &v1.Identity{}, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is nil", commonConfig: &v1.Identity{}, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is not set", commonConfig: &v1.Identity{}, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &v1.Identity{}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "use commonConfig when environmentConfig.Azure is empty", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &v1.Identity{Azure: &v1.AzureIdentity{}}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "override non-empty commonConfig with environmentConfig.Azure", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}, configureEnvironment: true, environmentConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig with environmentConfig", commonConfig: &v1.Identity{}, configureEnvironment: true, environmentConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "override empty commonConfig.Azure with environmentConfig", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{}}, configureEnvironment: true, environmentConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "66666666-7777-8888-9999-aaaaaaaaaaaa"}}},
		{name: "transform clientId with curly to standard format", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "{11111111-2222-3333-4444-555555555555}"}}, configureEnvironment: false, environmentConfig: nil, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId with urn:uuid to standard format", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "urn:uuid:11111111-2222-3333-4444-555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
		{name: "transform clientId without dashes to standard format", commonConfig: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111222233334444555555555555"}}, configureEnvironment: false, environmentConfig: nil, expected: &v1.Identity{Azure: &v1.AzureIdentity{ClientId: "11111111-2222-3333-4444-555555555555"}}},
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
				WithNode(v1.RadixNode{Gpu: compGpu, GpuCount: compGpuCount}).
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithNode(v1.RadixNode{Gpu: envGpu1, GpuCount: envGpuCount1}),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2").
						WithNode(v1.RadixNode{GpuCount: envGpuCount2}),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env3").
						WithNode(v1.RadixNode{Gpu: envGpu3}),
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
	portMap := make(map[string]v1.ComponentPort)
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
		commonConfig         *v1.Notifications
		configureEnvironment bool
		environmentConfig    *v1.Notifications
		expected             *v1.Notifications
	}

	scenarios := []scenarioSpec{
		{name: "nil when commonConfig and environmentConfig is empty", commonConfig: &v1.Notifications{}, configureEnvironment: true, environmentConfig: &v1.Notifications{}, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is empty", commonConfig: nil, configureEnvironment: true, environmentConfig: &v1.Notifications{}, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is nil", commonConfig: &v1.Notifications{}, configureEnvironment: true, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is nil and environmentConfig is not set", commonConfig: nil, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "nil when commonConfig is empty and environmentConfig is not set", commonConfig: &v1.Notifications{}, configureEnvironment: false, environmentConfig: nil, expected: nil},
		{name: "use commonConfig when environmentConfig is empty", commonConfig: &v1.Notifications{Webhook: pointers.Ptr("http://api:8080")}, configureEnvironment: true, environmentConfig: &v1.Notifications{}, expected: &v1.Notifications{Webhook: pointers.Ptr("http://api:8080")}},
		{name: "use commonConfig when environmentConfig.Webhook is empty", commonConfig: &v1.Notifications{Webhook: pointers.Ptr("http://api:8080")}, configureEnvironment: true, environmentConfig: &v1.Notifications{Webhook: nil}, expected: &v1.Notifications{Webhook: pointers.Ptr("http://api:8080")}},
		{name: "override non-empty commonConfig with environmentConfig.Webhook", commonConfig: &v1.Notifications{Webhook: pointers.Ptr("http://api:8080")}, configureEnvironment: true, environmentConfig: &v1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}, expected: &v1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}},
		{name: "override empty commonConfig with environmentConfig", commonConfig: &v1.Notifications{}, configureEnvironment: true, environmentConfig: &v1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}, expected: &v1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}},
		{name: "override empty commonConfig.Webhook with environmentConfig", commonConfig: &v1.Notifications{Webhook: nil}, configureEnvironment: true, environmentConfig: &v1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}, expected: &v1.Notifications{Webhook: pointers.Ptr("http://comp1:8099")}},
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
			name: "with image-tags",
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
				componentBuilder.WithName(jobComponentName).WithImage(ts.componentImages[jobComponentName]).
					WithEnvironmentConfig(utils.NewJobComponentEnvironmentBuilder().WithEnvironment(environment).WithImageTagName(ts.environmentConfigImageTagNames[jobComponentName]))
				componentBuilders = append(componentBuilders, componentBuilder)
			}

			ra := utils.ARadixApplication().WithEnvironment(environment, "master").WithJobComponents(componentBuilders...).BuildRA()

			deployJobComponents, err := NewJobComponentsBuilder(ra, environment, componentImages, make(v1.EnvVarsMap), nil).JobComponents()
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
		{"ReadOnlyFileSystem is nil", nil, nil, nil},
		{"ReadOnlyFileSystem is nil", nil, utils.BoolPtr(true), utils.BoolPtr(true)},
		{"ReadOnlyFileSystem is nil", nil, utils.BoolPtr(false), utils.BoolPtr(false)},
		{"ReadOnlyFileSystem is true", utils.BoolPtr(true), nil, utils.BoolPtr(true)},
		{"ReadOnlyFileSystem is true", utils.BoolPtr(true), utils.BoolPtr(true), utils.BoolPtr(true)},
		{"ReadOnlyFileSystem is true", utils.BoolPtr(true), utils.BoolPtr(false), utils.BoolPtr(false)},
		{"ReadOnlyFileSystem is false", utils.BoolPtr(false), nil, utils.BoolPtr(false)},
		{"ReadOnlyFileSystem is false", utils.BoolPtr(false), utils.BoolPtr(true), utils.BoolPtr(true)},
		{"ReadOnlyFileSystem is false", utils.BoolPtr(false), utils.BoolPtr(false), utils.BoolPtr(false)},
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

			deployJobComponent, err := NewJobComponentsBuilder(ra, environment, componentImages, make(v1.EnvVarsMap), nil).JobComponents()
			assert.NoError(t, err)

			assert.Equal(t, ts.expectedReadOnlyFile, deployJobComponent[0].ReadOnlyFileSystem)

		})
	}
}
