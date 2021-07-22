package utils

import (
	"encoding/json"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobBuilder Handles construction of RJ
type JobBuilder interface {
	WithRadixApplication(ApplicationBuilder) JobBuilder
	WithJobName(string) JobBuilder
	WithAppName(string) JobBuilder
	WithPipeline(v1.RadixPipelineType) JobBuilder
	WithBranch(string) JobBuilder
	WithDeploymentName(string) JobBuilder
	WithStatusOnAnnotation(JobStatusBuilder) JobBuilder
	WithEmptyStatus() JobBuilder
	WithStatus(JobStatusBuilder) JobBuilder
	WithCreated(time.Time) JobBuilder
	GetApplicationBuilder() ApplicationBuilder
	BuildRJ() *v1.RadixJob
}

// JobBuilderStruct Holds instance variables
type JobBuilderStruct struct {
	applicationBuilder ApplicationBuilder
	appName            string
	jobName            string
	pipeline           v1.RadixPipelineType
	restoredStatus     string
	emptyStatus        bool
	status             v1.RadixJobStatus
	branch             string
	deploymentName     string
	buildSpec          v1.RadixBuildSpec
	promoteSpec        v1.RadixPromoteSpec
	created            time.Time
}

// WithRadixApplication Links to RA builder
func (jb *JobBuilderStruct) WithRadixApplication(applicationBuilder ApplicationBuilder) JobBuilder {
	jb.applicationBuilder = applicationBuilder
	return jb
}

// WithJobName Sets name of the radix job
func (jb *JobBuilderStruct) WithJobName(name string) JobBuilder {
	jb.jobName = name
	return jb
}

// WithAppName Sets name of the application
func (jb *JobBuilderStruct) WithAppName(name string) JobBuilder {
	jb.appName = name

	if jb.applicationBuilder != nil {
		jb.applicationBuilder = jb.applicationBuilder.WithAppName(name)
	}

	return jb
}

// WithPipeline Sets pipeline
func (jb *JobBuilderStruct) WithPipeline(pipeline v1.RadixPipelineType) JobBuilder {
	jb.pipeline = pipeline
	return jb
}

// WithBranch Sets branch
func (jb *JobBuilderStruct) WithBranch(branch string) JobBuilder {
	jb.branch = branch

	jb.buildSpec = v1.RadixBuildSpec{
		Branch: branch,
	}

	return jb
}

// WithDeploymentName Sets deployment name
func (jb *JobBuilderStruct) WithDeploymentName(deploymentName string) JobBuilder {
	jb.deploymentName = deploymentName

	jb.promoteSpec = v1.RadixPromoteSpec{
		DeploymentName: deploymentName,
	}

	return jb
}

// WithStatusOnAnnotation Emulates velero plugin
func (jb *JobBuilderStruct) WithStatusOnAnnotation(jobStatus JobStatusBuilder) JobBuilder {
	restoredStatus, _ := json.Marshal(jobStatus.Build())
	jb.restoredStatus = string(restoredStatus)
	return jb
}

// WithEmptyStatus Indicates that the RJ has no reconciled status
func (jb *JobBuilderStruct) WithEmptyStatus() JobBuilder {
	jb.emptyStatus = true
	return jb
}

// WithStatus Sets status on job
func (jb *JobBuilderStruct) WithStatus(jobStatus JobStatusBuilder) JobBuilder {
	jb.status = jobStatus.Build()
	return jb
}

// WithCreated Sets timestamp
func (jb *JobBuilderStruct) WithCreated(created time.Time) JobBuilder {
	jb.created = created
	return jb
}

// GetApplicationBuilder Obtains the builder for the corresponding RA, if exists (used for testing)
func (jb *JobBuilderStruct) GetApplicationBuilder() ApplicationBuilder {
	if jb.applicationBuilder != nil {
		return jb.applicationBuilder
	}

	return nil
}

// BuildRJ Builds RJ structure based on set variables
func (jb *JobBuilderStruct) BuildRJ() *v1.RadixJob {
	anyPipelineImageVersion := "any-latest"
	anyDockerRegistry := "any.azurecr.io"

	radixJob := &v1.RadixJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      jb.jobName,
			Namespace: GetAppNamespace(jb.appName),
			Labels: map[string]string{
				kube.RadixAppLabel: jb.appName,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation:    jb.branch,
				kube.RestoredStatusAnnotation: jb.restoredStatus,
			},
			CreationTimestamp: metav1.Time{Time: jb.created},
		},
		Spec: v1.RadixJobSpec{
			AppName:        jb.appName,
			PipeLineType:   jb.pipeline,
			PipelineImage:  anyPipelineImageVersion,
			DockerRegistry: anyDockerRegistry,
			Build:          jb.buildSpec,
			Promote:        jb.promoteSpec,
		},
	}

	if !jb.emptyStatus {
		radixJob.Status = jb.status
	}

	return radixJob
}

// NewJobBuilder Constructor for radixjob builder
func NewJobBuilder() JobBuilder {
	return &JobBuilderStruct{
		created: time.Now(),
	}
}

// ARadixBuildDeployJob Constructor for radix job builder containing test data
func ARadixBuildDeployJob() JobBuilder {
	builder := NewJobBuilder().
		WithRadixApplication(
			ARadixApplication().
				WithAppName("someapp")).
		WithAppName("someapp").
		WithJobName("somejob").
		WithPipeline(v1.BuildDeploy).
		WithBranch("master")

	return builder
}

// AStartedBuildDeployJob Constructor for radix job builder containing test data
func AStartedBuildDeployJob() JobBuilder {
	builder := ARadixBuildDeployJob().
		WithCreated(time.Now()).
		WithStatus(AStartedJobStatus())

	return builder
}

// JobStatusBuilder Handles construction of job status
type JobStatusBuilder interface {
	WithCondition(v1.RadixJobCondition) JobStatusBuilder
	WithStarted(time.Time) JobStatusBuilder
	WithEnded(time.Time) JobStatusBuilder
	WithSteps(...JobStepBuilder) JobStatusBuilder
	Build() v1.RadixJobStatus
}

type jobStatusBuilder struct {
	condition v1.RadixJobCondition
	started   time.Time
	ended     time.Time
	steps     []JobStepBuilder
}

func (jsb *jobStatusBuilder) WithCondition(condition v1.RadixJobCondition) JobStatusBuilder {
	jsb.condition = condition
	return jsb
}

func (jsb *jobStatusBuilder) WithStarted(started time.Time) JobStatusBuilder {
	jsb.started = started
	return jsb
}

func (jsb *jobStatusBuilder) WithEnded(ended time.Time) JobStatusBuilder {
	jsb.ended = ended
	return jsb
}

func (jsb *jobStatusBuilder) WithSteps(steps ...JobStepBuilder) JobStatusBuilder {
	jsb.steps = steps
	return jsb
}

func (jsb *jobStatusBuilder) Build() v1.RadixJobStatus {
	jobSteps := make([]v1.RadixJobStep, 0)
	for _, step := range jsb.steps {
		jobSteps = append(jobSteps, step.Build())
	}

	// Need to trim away milliseconds, as reading job status from annotation wont hold them
	started := jsb.started.Truncate(1 * time.Second)
	ended := jsb.ended.Truncate(1 * time.Second)
	targetEnvs := []string{"test"}

	return v1.RadixJobStatus{
		Condition:  jsb.condition,
		Started:    &metav1.Time{Time: started},
		Ended:      &metav1.Time{Time: ended},
		Steps:      jobSteps,
		TargetEnvs: targetEnvs,
	}
}

// NewJobStatusBuilder Constructor for job status builder
func NewJobStatusBuilder() JobStatusBuilder {
	return &jobStatusBuilder{}
}

// AStartedJobStatus Constructor build-app
func AStartedJobStatus() JobStatusBuilder {
	builder := NewJobStatusBuilder().
		WithCondition(v1.JobRunning).
		WithStarted(time.Now()).
		WithSteps(
			ACloneConfigStep().
				WithCondition(v1.JobSucceeded).
				WithStarted(time.Now()).
				WithEnded(time.Now()),
			ARadixPipelineStep().
				WithCondition(v1.JobRunning).
				WithStarted(time.Now()),
			ACloneStep().
				WithCondition(v1.JobSucceeded).
				WithStarted(time.Now()).
				WithEnded(time.Now()),
			ABuildAppStep().
				WithCondition(v1.JobRunning).
				WithStarted(time.Now()))

	return builder
}

// ACompletedJobStatus Constructor for a completed job
func ACompletedJobStatus() JobStatusBuilder {
	started := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	ended := started.Add(5 * time.Minute)
	builder := NewJobStatusBuilder().
		WithCondition(v1.JobSucceeded).
		WithStarted(started).
		WithEnded(ended).
		WithSteps(
			ACloneConfigStep().
				WithCondition(v1.JobSucceeded).
				WithStarted(started).
				WithEnded(ended),
			ARadixPipelineStep().
				WithCondition(v1.JobRunning).
				WithStarted(started).
				WithEnded(ended),
			ACloneStep().
				WithCondition(v1.JobSucceeded).
				WithStarted(started).
				WithEnded(ended),
			ABuildAppStep().
				WithCondition(v1.JobRunning).
				WithStarted(started).
				WithEnded(ended),
			AScanAppStep().
				WithCondition(v1.JobRunning).
				WithStarted(started).
				WithEnded(ended).
				WithOutput(
					&v1.RadixJobStepOutput{
						Scan: &v1.RadixJobStepScanOutput{
							Status:                     v1.ScanSuccess,
							Vulnerabilities:            v1.VulnerabilityMap{"critical": 1, "high": 2},
							VulnerabilityListConfigMap: "scan-configmap",
							VulnerabilityListKey:       "list-of-vulnerabilities",
						},
					},
				))

	return builder
}

// JobStepBuilder Handles construction of job status step
type JobStepBuilder interface {
	WithCondition(v1.RadixJobCondition) JobStepBuilder
	WithName(string) JobStepBuilder
	WithStarted(time.Time) JobStepBuilder
	WithEnded(time.Time) JobStepBuilder
	WithComponents(...string) JobStepBuilder
	WithOutput(*v1.RadixJobStepOutput) JobStepBuilder
	Build() v1.RadixJobStep
}

type jobStepBuilder struct {
	condition  v1.RadixJobCondition
	name       string
	started    time.Time
	ended      time.Time
	components []string
	output     *v1.RadixJobStepOutput
}

func (sb *jobStepBuilder) WithCondition(condition v1.RadixJobCondition) JobStepBuilder {
	sb.condition = condition
	return sb
}

func (sb *jobStepBuilder) WithName(name string) JobStepBuilder {
	sb.name = name
	return sb
}

func (sb *jobStepBuilder) WithStarted(started time.Time) JobStepBuilder {
	sb.started = started
	return sb
}

func (sb *jobStepBuilder) WithEnded(ended time.Time) JobStepBuilder {
	sb.ended = ended
	return sb
}

func (sb *jobStepBuilder) WithComponents(components ...string) JobStepBuilder {
	sb.components = components
	return sb
}

func (sb *jobStepBuilder) WithOutput(output *v1.RadixJobStepOutput) JobStepBuilder {
	sb.output = output
	return sb
}

func (sb *jobStepBuilder) Build() v1.RadixJobStep {
	// Need to trim away milliseconds, as reading job status from annotation wont hold them
	started := sb.started.Truncate(1 * time.Second)
	ended := sb.ended.Truncate(1 * time.Second)

	return v1.RadixJobStep{
		Condition:  sb.condition,
		Started:    &metav1.Time{Time: started},
		Ended:      &metav1.Time{Time: ended},
		Name:       sb.name,
		Components: sb.components,
		Output:     sb.output,
	}
}

// NewJobStepBuilder Constructor for job step builder
func NewJobStepBuilder() JobStepBuilder {
	return &jobStepBuilder{}
}

// ACloneConfigStep Constructor clone-config
func ACloneConfigStep() JobStepBuilder {
	builder := NewJobStepBuilder().
		WithName("clone-config")

	return builder
}

// ARadixPipelineStep Constructor radix-pipeline
func ARadixPipelineStep() JobStepBuilder {
	builder := NewJobStepBuilder().
		WithName("radix-pipeline")

	return builder
}

// ACloneStep Constructor radix-pipeline
func ACloneStep() JobStepBuilder {
	builder := NewJobStepBuilder().
		WithName("clone")

	return builder
}

// ABuildAppStep Constructor build-app
func ABuildAppStep() JobStepBuilder {
	builder := NewJobStepBuilder().
		WithName("build-app")

	return builder
}

// ABuildAppStep Constructor build-app
func AScanAppStep() JobStepBuilder {
	builder := NewJobStepBuilder().
		WithName("scan-app")

	return builder
}
