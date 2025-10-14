package common

// +kubebuilder:object:generate=true

// RadixJobComponentConfig holds description of RadixJobComponent
type RadixJobComponentConfig struct {
	// Resource describes the compute resource requirements.
	//
	// required: false
	Resources *Resources `json:"resources,omitempty"`

	// Deprecated: use Runtime.NodeType instead.
	// Node defines node attributes, where container should be scheduled
	//
	// required: false
	Node *Node `json:"node,omitempty"`

	// TimeLimitSeconds defines maximum job run time. Corresponds to ActiveDeadlineSeconds in K8s.
	//
	// required: false
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`

	// BackoffLimit defines attempts to restart job if it fails. Corresponds to BackoffLimit in K8s.
	//
	// required: false
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// FailurePolicy defines how failed job replicas influence the backoffLimit.
	//
	// required: false
	FailurePolicy *FailurePolicy `json:"failurePolicy,omitempty"`

	// Name of an existing container image to use when running the job. Overrides an image in the RadixDeployment
	// More info: https://www.radix.equinor.com/radix-config#image-2
	// +optional
	Image string `json:"image,omitempty"`

	// ImageTagName defines the image tag name to use for the job image
	//
	// required: false
	ImageTagName string `json:"imageTagName,omitempty"`

	// Runtime defines the target runtime requirements for the component
	// +optional
	Runtime *Runtime `json:"runtime,omitempty"`

	// List of environment variables and values. Combines with RadixDeployment Variables.
	// More info: https://www.radix.equinor.com/radix-config#variables-common-2
	// +optional
	Variables EnvVars `json:"variables,omitempty"`

	// Entrypoint array. Not executed within a shell.
	// The container image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Command *[]string `json:"command,omitempty"`

	// Arguments to the entrypoint.
	// The container image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	// +listType=atomic
	Args *[]string `json:"args,omitempty"`
}

// JobScheduleDescription holds description about scheduling job
// swagger:model JobScheduleDescription
type JobScheduleDescription struct {
	// JobId Optional ID of a job
	//
	// required: false
	// example: 'job1'
	JobId string `json:"jobId,omitempty"`

	// Payload holding json data to be mapped to component
	//
	// required: false
	// example: {'data':'value'}
	Payload string `json:"payload"`

	// RadixJobComponentConfig holding data relating to resource configuration
	//
	// required: false
	RadixJobComponentConfig `json:",inline"`
}
