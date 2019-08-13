package utils

import (
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
	WithEmptyStatus() JobBuilder
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
	emptyStatus        bool
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

// WithEmptyStatus Indicates that the RJ has no reconciled status
func (jb *JobBuilderStruct) WithEmptyStatus() JobBuilder {
	jb.emptyStatus = true
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

	var status v1.RadixJobStatus
	if !jb.emptyStatus {
		status = v1.RadixJobStatus{}
	}

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
				kube.RadixBranchAnnotation: jb.branch,
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
		Status: status,
	}
	return radixJob
}

// NewJobBuilder Constructor for radixjob builder
func NewJobBuilder() JobBuilder {
	return &JobBuilderStruct{
		created: time.Now().UTC(),
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
