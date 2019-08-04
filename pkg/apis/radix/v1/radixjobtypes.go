package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixJob describe a Radix job
type RadixJob struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixJobSpec   `json:"spec" yaml:"spec"`
	Status             RadixJobStatus `json:"status" yaml:"status"`
}

//RadixJobStatus is the status for a Radix job
type RadixJobStatus struct {
	Condition  RadixJobCondition `json:"condition" yaml:"condition"`
	Started    *meta_v1.Time     `json:"started" yaml:"started"`
	Ended      *meta_v1.Time     `json:"ended" yaml:"ended"`
	TargetEnvs []string          `json:"targetEnvironments" yaml:"targetEnvironments"`
	Steps      []RadixJobStep    `json:"steps" yaml:"steps"`
}

// RadixJobCondition Holds the condition of a job
type RadixJobCondition string

// These are valid conditions of a deployment.
const (
	JobQueued    RadixJobCondition = "Queued"
	JobWaiting   RadixJobCondition = "Waiting"
	JobRunning   RadixJobCondition = "Running"
	JobSucceeded RadixJobCondition = "Succeeded"
	JobFailed    RadixJobCondition = "Failed"
)

//RadixJobSpec is the spec for a job
type RadixJobSpec struct {
	AppName        string            `json:"appName" yaml:"appName"`
	PipeLineType   RadixPipelineType `json:"pipeLineType" yaml:"pipeLineType"`
	DockerRegistry string            `json:"dockerRegistry" yaml:"dockerRegistry"`
	PipelineImage  string            `json:"pipelineImage" yaml:"pipelineImage"`
	Build          RadixBuildSpec    `json:"build" yaml:"build"`
	Promote        RadixPromoteSpec  `json:"promote" yaml:"promote"`
}

// RadixPipelineType Holds the different type of pipeline
type RadixPipelineType string

// These are valid conditions of a deployment.
const (
	Build       RadixPipelineType = "build"
	BuildDeploy RadixPipelineType = "build-deploy"
	Promote     RadixPipelineType = "promote"
)

//RadixBuildSpec is the spec for a build job
type RadixBuildSpec struct {
	ImageTag      string `json:"imageTag" yaml:"imageTag"`
	Branch        string `json:"branch" yaml:"branch"`
	CommitID      string `json:"commitID" yaml:"commitID"`
	PushImage     bool   `json:"pushImage" yaml:"pushImage"`
	RadixFileName string `json:"radixFileName" yaml:"radixFileName"`
}

//RadixPromoteSpec is the spec for a promote job
type RadixPromoteSpec struct {
	DeploymentName  string `json:"deploymentName" yaml:"deploymentName"`
	FromEnvironment string `json:"fromEnvironment" yaml:"fromEnvironment"`
	ToEnvironment   string `json:"toEnvironment" yaml:"toEnvironment"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixJobList is a list of Radix jobs
type RadixJobList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixJob `json:"items" yaml:"items"`
}

//RadixJobStep holds status for a single step
type RadixJobStep struct {
	Name      string            `json:"name" yaml:"name"`
	Condition RadixJobCondition `json:"condition" yaml:"condition"`
	Started   *meta_v1.Time     `json:"started" yaml:"started"`
	Ended     *meta_v1.Time     `json:"ended" yaml:"ended"`
	PodName   string            `json:"podName" yaml:"podName"`
}
