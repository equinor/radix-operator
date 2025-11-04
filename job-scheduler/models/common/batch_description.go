package common

// +kubebuilder:object:generate=true

// BatchScheduleDescription holds description about batch scheduling job
// swagger:model BatchScheduleDescription
type BatchScheduleDescription struct {
	// Defines a user defined ID of the batch.
	//
	// required: false
	// example: 'batch-id-1'
	BatchId string `json:"batchId,omitempty"`

	// JobScheduleDescriptions descriptions of jobs to schedule within the batch
	//
	// required: true
	JobScheduleDescriptions []JobScheduleDescription `json:"jobScheduleDescriptions" yaml:"jobScheduleDescriptions"`

	// DefaultRadixJobComponentConfig default resources configuration
	//
	// required: false
	DefaultRadixJobComponentConfig *RadixJobComponentConfig `json:"defaultRadixJobComponentConfig,omitempty" yaml:"defaultRadixJobComponentConfig,omitempty"`
}
