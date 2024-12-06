package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixJob describe a Radix job
type RadixJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RadixJobSpec   `json:"spec"`
	Status            RadixJobStatus `json:"status"`
}

// RadixJobStatus is the status for a Radix job
type RadixJobStatus struct {
	Condition  RadixJobCondition `json:"condition"`
	Created    *metav1.Time      `json:"created"`
	Started    *metav1.Time      `json:"started"`
	Ended      *metav1.Time      `json:"ended"`
	TargetEnvs []string          `json:"targetEnvironments"`
	Steps      []RadixJobStep    `json:"steps"`
}

// RadixJobCondition Holds the condition of a job
type RadixJobCondition string

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

// RadixJobSpec is the spec for a job
type RadixJobSpec struct {
	AppName             string               `json:"appName"`
	CloneURL            string               `json:"cloneURL"`
	TektonImage         string               `json:"tektonImage"`
	PipeLineType        RadixPipelineType    `json:"pipeLineType"`
	PipelineImage       string               `json:"pipelineImage"`
	Build               RadixBuildSpec       `json:"build"`
	Promote             RadixPromoteSpec     `json:"promote"`
	Deploy              RadixDeploySpec      `json:"deploy"`
	ApplyConfig         RadixApplyConfigSpec `json:"applyConfig"`
	Stop                bool                 `json:"stop"`
	TriggeredBy         string               `json:"triggeredBy"`
	RadixConfigFullName string               `json:"radixConfigFullName"`
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

	// OverrideUseBuildCache override default or configured build cache option
	//
	// +optional
	OverrideUseBuildCache *bool `json:"overrideUseBuildCache,omitempty"`
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
	Name       string            `json:"name"`
	Condition  RadixJobCondition `json:"condition"`
	Started    *metav1.Time      `json:"started"`
	Ended      *metav1.Time      `json:"ended"`
	PodName    string            `json:"podName"`
	Components []string          `json:"components,omitempty"`
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
