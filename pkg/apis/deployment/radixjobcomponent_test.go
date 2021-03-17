package deployment

import (
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
