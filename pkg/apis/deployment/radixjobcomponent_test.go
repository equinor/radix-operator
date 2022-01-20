package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
)

func Test_GetRadixJobComponents_BuildAllJobComponents(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job1").
				WithSchedulerPort(int32Ptr(8888)).
				WithPayloadPath(utils.StringPtr("/path/to/payload")),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{
		ra:              ra,
		env:             "any",
		componentImages: make(map[string]pipeline.ComponentImage),
	}
	jobs := cfg.JobComponents()

	assert.Len(t, jobs, 2)
	assert.Equal(t, "job1", jobs[0].Name)
	assert.Equal(t, int32Ptr(8888), jobs[0].SchedulerPort)
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
				WithSchedulerPort(int32Ptr(8888)).
				WithPayloadPath(utils.StringPtr("/path/to/payload")).
				WithNode(v1.RadixNode{Gpu: gpu, GpuCount: gpuCount}),
			utils.AnApplicationJobComponent().
				WithName("job2"),
		).BuildRA()

	cfg := jobComponentsBuilder{
		ra:              ra,
		env:             "any",
		componentImages: make(map[string]pipeline.ComponentImage),
	}
	jobs := cfg.JobComponents()

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

	cfg := jobComponentsBuilder{
		ra:              ra,
		env:             "env1",
		componentImages: make(map[string]pipeline.ComponentImage),
	}
	jobComponent := cfg.JobComponents()[0]

	assert.Len(t, jobComponent.EnvironmentVariables, 3)
	assert.Equal(t, "override1", jobComponent.EnvironmentVariables["COMMON1"])
	assert.Equal(t, "common2", jobComponent.EnvironmentVariables["COMMON2"])
	assert.Equal(t, "job1", jobComponent.EnvironmentVariables["JOB1"])
}

func Test_GetRadixJobComponents_Monitoring(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job").
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithMonitoring(true),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2"),
				),
		).BuildRA()

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(map[string]pipeline.ComponentImage)}
	job := cfg.JobComponents()[0]
	assert.True(t, job.Monitoring)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(map[string]pipeline.ComponentImage)}
	job = cfg.JobComponents()[0]
	assert.False(t, job.Monitoring)
}

func Test_GetRadixJobComponents_ImageTagName(t *testing.T) {
	componentImages := make(map[string]pipeline.ComponentImage)
	componentImages["job"] = pipeline.ComponentImage{Build: false, ImagePath: "img:{imageTagName}"}
	componentImages["job2"] = pipeline.ComponentImage{ImagePath: "job2:tag"}

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
	jobs := cfg.JobComponents()
	assert.Equal(t, "img:release", jobs[0].Image)
	assert.Equal(t, "job2:tag", jobs[1].Image)
}

func Test_GetRadixJobComponents_RunAsNonRoot(t *testing.T) {
	ra := utils.ARadixApplication().
		WithJobComponents(
			utils.AnApplicationJobComponent().
				WithName("job").
				WithEnvironmentConfigs(
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env1").
						WithRunAsNonRoot(true),
					utils.NewJobComponentEnvironmentBuilder().
						WithEnvironment("env2"),
				),
		).BuildRA()

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(map[string]pipeline.ComponentImage)}
	job := cfg.JobComponents()[0]
	assert.True(t, job.RunAsNonRoot)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(map[string]pipeline.ComponentImage)}
	job = cfg.JobComponents()[0]
	assert.False(t, job.RunAsNonRoot)
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
		jobs := cfg.JobComponents()
		assert.Equal(t, envGpu1, jobs[0].Node.Gpu)
		assert.Equal(t, envGpuCount1, jobs[0].Node.GpuCount)
	})
	t.Run("override job gpu-count with environment gpu-count", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env2"}
		jobs := cfg.JobComponents()
		assert.Equal(t, compGpu, jobs[0].Node.Gpu)
		assert.Equal(t, envGpuCount2, jobs[0].Node.GpuCount)
	})
	t.Run("override job gpu with environment gpu", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env3"}
		jobs := cfg.JobComponents()
		assert.Equal(t, envGpu3, jobs[0].Node.Gpu)
		assert.Equal(t, compGpuCount, jobs[0].Node.GpuCount)
	})
	t.Run("do not override job gpu or gpu-count with environment gpu or gpu-count", func(t *testing.T) {
		t.Parallel()
		cfg := jobComponentsBuilder{ra: ra, env: "env4"}
		jobs := cfg.JobComponents()
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

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(map[string]pipeline.ComponentImage)}
	jobs := cfg.JobComponents()
	assert.Equal(t, "1100Mi", jobs[0].Resources.Requests["memory"])
	assert.Equal(t, "1150m", jobs[0].Resources.Requests["cpu"])
	assert.Equal(t, "1200Mi", jobs[0].Resources.Limits["memory"])
	assert.Equal(t, "1250m", jobs[0].Resources.Limits["cpu"])
	assert.Empty(t, jobs[1].Resources)

	cfg = jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(map[string]pipeline.ComponentImage)}
	jobs = cfg.JobComponents()
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

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(map[string]pipeline.ComponentImage)}
	jobs := cfg.JobComponents()
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

	cfg := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(map[string]pipeline.ComponentImage)}
	jobs := cfg.JobComponents()
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

	cfgEnv1 := jobComponentsBuilder{ra: ra, env: "env1", componentImages: make(map[string]pipeline.ComponentImage)}
	cfgEnv2 := jobComponentsBuilder{ra: ra, env: "env2", componentImages: make(map[string]pipeline.ComponentImage)}
	env1Job := cfgEnv1.JobComponents()
	env2Job := cfgEnv2.JobComponents()
	assert.Equal(t, numbers.Int64Ptr(100), env1Job[0].TimeLimitSeconds)
	assert.Equal(t, numbers.Int64Ptr(200), env2Job[0].TimeLimitSeconds)
}
