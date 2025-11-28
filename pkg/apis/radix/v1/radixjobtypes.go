package v1

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Pipeline Type",type="string",JSONPath=".spec.pipeLineType"
// +kubebuilder:printcolumn:name="Target Environments",type="string",JSONPath=".status.targetEnvironments",priority=1
// +kubebuilder:printcolumn:name="Condition",type="string",JSONPath=".status.condition"
// +kubebuilder:printcolumn:name="Started",type="string",JSONPath=".status.started"
// +kubebuilder:printcolumn:name="Ended",type="string",JSONPath=".status.ended"
// +kubebuilder:printcolumn:name="Reconciled",type="string",JSONPath=".status.reconciled",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=radixjobs,shortName=rj
// +kubebuilder:subresource:status

// RadixJob describe a Radix job
type RadixJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec is the desired state of the RadixEnvironment
	Spec RadixJobSpec `json:"spec"`

	// Status is the observed state of the RadixJob
	// +kubebuilder:validation:Optional
	Status RadixJobStatus `json:"status,omitzero"`
}

type RadixJobReconcileStatus string

const (
	RadixJobReconcileSucceeded RadixJobReconcileStatus = "Succeeded"
	RadixJobReconcileFailed    RadixJobReconcileStatus = "Failed"
)

// RadixJobStatus is the observed state of the RadixJob
type RadixJobStatus struct {
	// Condition describes the current state of the job
	Condition RadixJobCondition `json:"condition"`

	// Created is the timestamp when the job was created
	// +kubebuilder:validation:Optional
	Created *metav1.Time `json:"created,omitempty"`

	// Started is the timestamp when the job started execution
	// +kubebuilder:validation:Optional
	Started *metav1.Time `json:"started,omitempty"`

	// Ended is the timestamp when the job completed or failed
	// +kubebuilder:validation:Optional
	Ended *metav1.Time `json:"ended,omitempty"`

	// TargetEnvs is the list of target environments for the job
	// +kubebuilder:validation:Optional
	TargetEnvs []string `json:"targetEnvironments,omitempty"`

	// Steps contains the status of each pipeline step
	// +kubebuilder:validation:Optional
	Steps []RadixJobStep `json:"steps,omitempty"`

	// Reconciled is the timestamp of the last successful reconciliation
	// +kubebuilder:validation:Optional
	Reconciled metav1.Time `json:"reconciled,omitzero"`

	// ObservedGeneration is the generation observed by the controller
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReconcileStatus indicates whether the last reconciliation succeeded or failed
	// +kubebuilder:validation:Optional
	ReconcileStatus RadixJobReconcileStatus `json:"reconcileStatus,omitempty"`

	// Message provides additional information about the reconciliation state, typically error details when reconciliation fails
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`
}

// RadixJobCondition Holds the condition of a job
type RadixJobCondition string

var doneConditions = []RadixJobCondition{JobSucceeded, JobFailed, JobStopped, JobStoppedNoChanges}

func (c RadixJobCondition) IsDoneCondition() bool {
	return slices.Contains(doneConditions, c)
}

// These are valid conditions of a deployment.
const (
	// JobQueued When another job is running with the same
	// condition app + branch, the job is queued
	JobQueued RadixJobCondition = "Queued"
	// JobWaiting When operator hasn't picked up the radix job
	// the API will show the job as waiting. Also when the
	// kubernetes jobs (steps) are in waiting the step will be
	// in JobWaiting
	JobWaiting          RadixJobCondition = "Waiting"
	JobRunning          RadixJobCondition = "Running"
	JobSucceeded        RadixJobCondition = "Succeeded"
	JobFailed           RadixJobCondition = "Failed"
	JobStopped          RadixJobCondition = "Stopped"
	JobStoppedNoChanges RadixJobCondition = "StoppedNoChanges"
)

// GitRefType Holds the type of git event refs
// Read more about Git refs https://git-scm.com/book/en/v2/Git-Internals-Git-References
type GitRefType string

const (
	// GitRefBranch event sent when a commit is made to a branch
	GitRefBranch GitRefType = "branch"
	// GitRefTag event sent when a tag is created
	GitRefTag GitRefType = "tag"
)

// RadixJobSpec is the spec for a job
type RadixJobSpec struct {
	// AppName Name of the Radix application
	AppName string `json:"appName"`

	// Deprecated: radix-api will be responsible for setting the CloneURL, it is taken from the RadixRegistration by the radix-operator
	// CloneURL GitHub repository URL
	// +kubebuilder:validation:Optional
	CloneURL string `json:"cloneURL,omitempty"`

	// PipeLineType Type of the pipeline
	PipeLineType RadixPipelineType `json:"pipeLineType"`

	// Build contains the configuration for build and build-deploy pipelines
	// +kubebuilder:validation:Optional
	Build RadixBuildSpec `json:"build,omitzero"`

	// Promote contains the configuration for promote pipelines
	// +kubebuilder:validation:Optional
	Promote RadixPromoteSpec `json:"promote,omitzero"`

	// Deploy contains the configuration for deploy pipelines
	// +kubebuilder:validation:Optional
	Deploy RadixDeploySpec `json:"deploy,omitzero"`

	// ApplyConfig contains the configuration for apply-config pipelines
	// +kubebuilder:validation:Optional
	ApplyConfig RadixApplyConfigSpec `json:"applyConfig,omitzero"`

	// Stop If true, the job will be stopped
	// +kubebuilder:validation:Optional
	Stop bool `json:"stop"`

	// TriggeredFromWebhook If true, the job was triggered from a webhook
	// +kubebuilder:validation:Optional
	TriggeredFromWebhook bool `json:"triggeredFromWebhook"`

	// TriggeredBy Name of the user or UID oa a system principal which triggered the job
	TriggeredBy string `json:"triggeredBy"`

	// Deprecated: radix-api will be responsible for setting the RadixConfigFullName, it is taken from the RadixRegistration by the radix-operator
	// RadixConfigFullName Full name of the radix config file within the cloned GitHUb repository
	// +kubebuilder:validation:Optional
	RadixConfigFullName string `json:"radixConfigFullName,omitempty"`
}

// RadixPipelineType Holds the different type of pipeline
type RadixPipelineType string

// These are valid conditions of a deployment.
const (
	Build       RadixPipelineType = "build"
	BuildDeploy RadixPipelineType = "build-deploy"
	Promote     RadixPipelineType = "promote"
	Deploy      RadixPipelineType = "deploy"
	ApplyConfig RadixPipelineType = "apply-config"
)

// RadixBuildSpec is the spec for a build job
type RadixBuildSpec struct {
	// Tag of the built image
	//
	// +required
	ImageTag string `json:"imageTag"`

	// Deprecated: use GitRef instead
	// Branch, from which the image to be built
	//
	// +required
	Branch string `json:"branch"`

	// ToEnvironment the environment to build or build-deploy to
	//
	// +optional
	ToEnvironment string `json:"toEnvironment,omitempty"`

	// CommitID, from which the image to be built
	//
	// +optional
	CommitID string `json:"commitID,omitempty"`

	// Is the built image need to be pushed to the container registry repository
	//
	// +optional
	PushImage bool `json:"pushImage,omitempty"`

	// Enables BuildKit when building Dockerfile.
	//
	// +optional
	UseBuildKit *bool `json:"useBuildKit,omitempty"`

	// Defaults to true and requires useBuildKit to have an effect.
	//
	// +optional
	UseBuildCache *bool `json:"useBuildCache,omitempty"`

	// OverrideUseBuildCache override default or configured build cache option
	//
	// +optional
	OverrideUseBuildCache *bool `json:"overrideUseBuildCache,omitempty"`

	// RefreshBuildCache forces to rebuild cache when UseBuildCache is true in the RadixApplication or OverrideUseBuildCache is true
	//
	// +optional
	RefreshBuildCache *bool `json:"refreshBuildCache,omitempty"`

	// GitRef Branch or tag to build from
	//
	// required: false
	// example: master
	GitRef string `json:"gitRef,omitempty"`

	// GitRefType When the pipeline job should be built from branch or tag specified in GitRef:
	// - branch
	// - tag
	// - <empty> - either branch or tag
	//
	// required false
	// enum: branch,tag,""
	// example: "branch"
	GitRefType GitRefType `json:"gitRefType,omitempty"`
}

// RadixPromoteSpec is the spec for a promote job
type RadixPromoteSpec struct {
	// Name of the Radix deployment to be promoted
	//
	// +optional
	DeploymentName string `json:"deploymentName,omitempty"`

	// Environment name, from which the Radix deployment is being promoted
	//
	// +required
	FromEnvironment string `json:"fromEnvironment"`

	// Environment name, to which the Radix deployment is being promoted
	//
	// +required
	ToEnvironment string `json:"toEnvironment"`

	// CommitID of the promoted deployment
	//
	// +optional
	CommitID string `json:"commitID,omitempty"`
}

// RadixDeploySpec is the spec for a deploy job
type RadixDeploySpec struct {
	// Target environment for deploy
	//
	// +required
	ToEnvironment string `json:"toEnvironment"`

	// Image tags names for components - if empty will use default logic
	//
	// +optional
	ImageTagNames map[string]string `json:"imageTagNames,omitempty"`

	// Commit ID connected to the deployment
	//
	// +optional
	CommitID string `json:"commitID,omitempty"`

	// ComponentsToDeploy List of components to deploy
	// OPTIONAL If specified, only these components are deployed
	//
	// +optional
	ComponentsToDeploy []string `json:"componentsToDeploy,omitempty"`
}

// RadixApplyConfigSpec is the spec for a apply-config job
type RadixApplyConfigSpec struct {
	// Deploy External DNS configuration
	//
	// +optional
	DeployExternalDNS bool `json:"deployExternalDNS,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixJobList is a list of Radix jobs
type RadixJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RadixJob `json:"items"`
}

// RadixJobStep holds status for a single step
type RadixJobStep struct {
	// Name is the name of the pipeline step
	Name string `json:"name"`

	// Condition describes the current state of the step
	// +kubebuilder:validation:Optional
	Condition RadixJobCondition `json:"condition,omitempty"`

	// Started is the timestamp when the step started execution
	// +kubebuilder:validation:Optional
	Started *metav1.Time `json:"started,omitempty"`

	// Ended is the timestamp when the step completed or failed
	// +kubebuilder:validation:Optional
	Ended *metav1.Time `json:"ended,omitempty"`

	// PodName is the name of the pod executing this step
	// +kubebuilder:validation:Optional
	PodName string `json:"podName,omitempty"`

	// Components is the list of components processed in this step
	// +kubebuilder:validation:Optional
	Components []string `json:"components,omitempty"`
}

// RadixJobResultType Type of the Radix pipeline job result
type RadixJobResultType string

const (
	// RadixJobResultStoppedNoChanges The Radix build-deploy pipeline job was stopped due to there were no changes in component source code
	RadixJobResultStoppedNoChanges RadixJobResultType = "stoppedNoChanges"
)

// RadixJobResult is returned by Radix pipeline jobs via ConfigMap
type RadixJobResult struct {
	Result  RadixJobResultType `json:"result"`
	Message string             `json:"message"`
}

// GetGitRefOrDefault Get git event ref or "branch" by default
func (buildSpec *RadixBuildSpec) GetGitRefOrDefault() string {
	if buildSpec.GitRef == "" {
		return buildSpec.Branch
	}
	return buildSpec.GitRef
}

// GetGitRefTypeOrDefault Get git event ref type or "branch" by default
func (buildSpec *RadixBuildSpec) GetGitRefTypeOrDefault() string {
	if buildSpec.GitRefType == "" {
		return string(GitRefBranch)
	}
	return string(buildSpec.GitRefType)
}
