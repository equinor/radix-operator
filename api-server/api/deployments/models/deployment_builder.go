package models

import (
	"errors"
	"time"

	"github.com/equinor/radix-common/utils/pointers"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
)

// DeploymentBuilder Builds DTOs
type DeploymentBuilder interface {
	WithRadixDeployment(*v1.RadixDeployment) DeploymentBuilder
	WithPipelineJob(*v1.RadixJob) DeploymentBuilder
	WithComponents(components []*Component) DeploymentBuilder
	WithGitCommitHash(string) DeploymentBuilder
	WithGitTags(string) DeploymentBuilder
	WithRadixRegistration(*v1.RadixRegistration) DeploymentBuilder
	BuildDeploymentSummary() (*DeploymentSummary, error)
	BuildDeployment() (*Deployment, error)
}

type deploymentBuilder struct {
	name               string
	namespace          string
	environment        string
	status             DeploymentStatus
	statusReason       string
	activeFrom         time.Time
	activeTo           *time.Time
	jobName            string
	pipelineJob        *v1.RadixJob
	components         []*Component
	componentSummaries []*ComponentSummary
	errors             []error
	gitCommitHash      string
	gitTags            string
	repository         string
	useBuildKit        *bool
	useBuildCache      *bool
	refreshBuildCache  *bool
}

// NewDeploymentBuilder Constructor for application deploymentBuilder
func NewDeploymentBuilder() DeploymentBuilder {
	return &deploymentBuilder{}
}

func (b *deploymentBuilder) WithRadixDeployment(rd *v1.RadixDeployment) DeploymentBuilder {
	jobName := rd.Labels[kube.RadixJobNameLabel]
	status, statusReason := getDeploymentStatusFromRadixDeployment(rd)
	var activeTo *time.Time
	if !rd.Status.ActiveTo.IsZero() {
		activeTo = &rd.Status.ActiveTo.Time
	}

	b.withComponentSummariesFromRadixDeployment(rd).
		withEnvironment(rd.Spec.Environment).
		withNamespace(rd.GetNamespace()).
		withName(rd.GetName()).
		withStatus(status).
		withStatusReason(statusReason).
		withActiveFrom(rd.Status.ActiveFrom.Time).
		withJobName(jobName).
		withActiveTo(activeTo).
		withUseBuildKit(rd.Annotations[kube.RadixUseBuildKit]).
		withUseBuildCache(rd.Annotations[kube.RadixUseBuildCache]).
		withRefreshBuildCache(rd.Annotations[kube.RadixRefreshBuildCache]).
		WithGitCommitHash(rd.Annotations[kube.RadixCommitAnnotation]).
		WithGitTags(rd.Annotations[kube.RadixGitTagsAnnotation])

	return b
}

func (b *deploymentBuilder) WithPipelineJob(job *v1.RadixJob) DeploymentBuilder {
	if job != nil {
		b.withJobName(job.Name)
	}

	b.pipelineJob = job
	return b
}

func (b *deploymentBuilder) WithComponents(components []*Component) DeploymentBuilder {
	b.components = components
	return b
}

func (b *deploymentBuilder) WithGitCommitHash(gitCommitHash string) DeploymentBuilder {
	b.gitCommitHash = gitCommitHash
	return b
}

func (b *deploymentBuilder) WithGitTags(gitTags string) DeploymentBuilder {
	b.gitTags = gitTags
	return b
}

func (b *deploymentBuilder) WithRadixRegistration(rr *v1.RadixRegistration) DeploymentBuilder {
	gitCloneUrl := rr.Spec.CloneURL
	b.repository = crdUtils.GetGithubRepositoryURLFromCloneURL(gitCloneUrl)
	return b
}

func (b *deploymentBuilder) withName(name string) *deploymentBuilder {
	b.name = name
	return b
}

func (b *deploymentBuilder) withJobName(jobName string) *deploymentBuilder {
	b.jobName = jobName
	return b
}

func (b *deploymentBuilder) withActiveFrom(activeFrom time.Time) *deploymentBuilder {
	b.activeFrom = activeFrom
	return b
}

func (b *deploymentBuilder) withStatus(status DeploymentStatus) *deploymentBuilder {
	b.status = status
	return b
}

func (b *deploymentBuilder) withStatusReason(statusReason string) *deploymentBuilder {
	b.statusReason = statusReason
	return b
}

func (b *deploymentBuilder) withActiveTo(activeTo *time.Time) *deploymentBuilder {
	b.activeTo = activeTo
	return b
}

func (b *deploymentBuilder) withUseBuildKit(value string) *deploymentBuilder {
	if len(value) > 0 {
		b.useBuildKit = pointers.Ptr(value == "true")
	}
	return b
}

func (b *deploymentBuilder) withUseBuildCache(value string) *deploymentBuilder {
	if len(value) > 0 {
		b.useBuildCache = pointers.Ptr(value == "true")
	}
	return b
}

func (b *deploymentBuilder) withRefreshBuildCache(value string) *deploymentBuilder {
	if len(value) > 0 {
		b.refreshBuildCache = pointers.Ptr(value == "true")
	}
	return b
}

func (b *deploymentBuilder) withComponentSummariesFromRadixDeployment(rd *v1.RadixDeployment) *deploymentBuilder {
	components := make([]*ComponentSummary, 0, len(rd.Spec.Components)+len(rd.Spec.Jobs))
	for _, component := range rd.Spec.Components {
		componentDto, err := NewComponentBuilder().WithComponent(&component).BuildComponentSummary()
		if err != nil {
			b.errors = append(b.errors, err)
			continue
		}
		components = append(components, componentDto)
	}
	for _, component := range rd.Spec.Jobs {
		componentDto, err := NewComponentBuilder().WithComponent(&component).BuildComponentSummary()
		if err != nil {
			b.errors = append(b.errors, err)
			continue
		}
		components = append(components, componentDto)
	}
	b.componentSummaries = components
	return b
}

func (b *deploymentBuilder) withEnvironment(environment string) *deploymentBuilder {
	b.environment = environment
	return b
}

func (b *deploymentBuilder) withNamespace(namespace string) *deploymentBuilder {
	b.namespace = namespace
	return b
}

func (b *deploymentBuilder) buildError() error {
	if len(b.errors) == 0 {
		return nil
	}

	return errors.Join(b.errors...)
}

func (b *deploymentBuilder) BuildDeploymentSummary() (*DeploymentSummary, error) {
	b.setSkipDeploymentForComponentSummaries()
	return &DeploymentSummary{
		Name:                             b.name,
		Components:                       b.componentSummaries,
		Environment:                      b.environment,
		Status:                           b.status,
		StatusReason:                     b.statusReason,
		ActiveFrom:                       b.activeFrom,
		ActiveTo:                         b.activeTo,
		DeploymentSummaryPipelineJobInfo: b.buildDeploySummaryPipelineJobInfo(),
		GitCommitHash:                    b.gitCommitHash,
		GitTags:                          b.gitTags,
		UseBuildKit:                      b.useBuildKit,
		UseBuildCache:                    b.useBuildCache,
		RefreshBuildCache:                b.refreshBuildCache,
	}, b.buildError()
}

func (b *deploymentBuilder) setSkipDeploymentForComponentSummaries() {
	if b.pipelineJob == nil || len(b.pipelineJob.Spec.Deploy.ComponentsToDeploy) == 0 {
		return
	}
	for i := 0; i < len(b.componentSummaries); i++ {
		b.componentSummaries[i].SkipDeployment = !slice.Any(b.pipelineJob.Spec.Deploy.ComponentsToDeploy,
			func(componentName string) bool { return b.componentSummaries[i].Name == componentName })
	}
}

func (b *deploymentBuilder) setSkipDeploymentForComponents() {
	if b.pipelineJob == nil || len(b.pipelineJob.Spec.Deploy.ComponentsToDeploy) == 0 {
		return
	}
	for i := 0; i < len(b.components); i++ {
		b.components[i].SkipDeployment = !slice.Any(b.pipelineJob.Spec.Deploy.ComponentsToDeploy,
			func(componentName string) bool { return b.components[i].Name == componentName })
	}
}

func (b *deploymentBuilder) buildDeploySummaryPipelineJobInfo() DeploymentSummaryPipelineJobInfo {
	jobInfo := DeploymentSummaryPipelineJobInfo{
		CreatedByJob: b.jobName,
	}

	if b.pipelineJob != nil {
		jobInfo.CommitID = b.pipelineJob.Spec.Build.CommitID
		jobInfo.PipelineJobType = string(b.pipelineJob.Spec.PipeLineType)
		jobInfo.BuiltFromBranch = b.pipelineJob.Spec.Build.Branch //nolint:staticcheck
		jobInfo.GitRef = b.pipelineJob.Spec.Build.GitRef
		jobInfo.GitRefType = string(b.pipelineJob.Spec.Build.GitRefType)
		jobInfo.PromotedFromEnvironment = b.pipelineJob.Spec.Promote.FromEnvironment
	}

	return jobInfo
}

func (b *deploymentBuilder) BuildDeployment() (*Deployment, error) {
	b.setSkipDeploymentForComponents()
	deployment := Deployment{
		Name:              b.name,
		Namespace:         b.namespace,
		Environment:       b.environment,
		Status:            b.status,
		StatusReason:      b.statusReason,
		ActiveFrom:        b.activeFrom,
		ActiveTo:          b.activeTo,
		Components:        b.components,
		CreatedByJob:      b.jobName,
		GitCommitHash:     b.gitCommitHash,
		GitTags:           b.gitTags,
		Repository:        b.repository,
		UseBuildKit:       b.useBuildKit,
		UseBuildCache:     b.useBuildCache,
		RefreshBuildCache: b.refreshBuildCache,
	}
	if b.pipelineJob != nil {
		deployment.BuiltFromBranch = b.pipelineJob.Spec.Build.Branch //nolint:staticcheck
		deployment.GitRef = b.pipelineJob.Spec.Build.GitRef
		deployment.GitRefType = string(b.pipelineJob.Spec.Build.GitRefType)
	}
	return &deployment, b.buildError()
}

func getDeploymentStatusFromRadixDeployment(rd *v1.RadixDeployment) (DeploymentStatus, string) {
	if rd.Status.Condition != v1.DeploymentActive {
		return DeploymentStatusInactive, ""
	}

	if rd.Status.ObservedGeneration < rd.Generation {
		return DeploymentStatusReconciling, ""
	}

	switch rd.Status.ReconcileStatus {
	case v1.RadixDeploymentReconcileSucceeded:
		return DeploymentStatusReady, ""
	case v1.RadixDeploymentReconcileFailed:
		return DeploymentStatusFailed, rd.Status.Message
	}

	return DeploymentStatusReconciling, ""
}
