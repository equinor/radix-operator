package models

// AzureKeyVaultSecretVersion holds a version of a Azure Key vault secret
// swagger:model AzureKeyVaultSecretVersion
type AzureKeyVaultSecretVersion struct {
	// ReplicaName which uses the secret
	//
	// required: true
	// example: abcdf
	ReplicaName string `json:"replicaName"`

	// ReplicaCreated which uses the secret
	//
	// required: true
	// example: 2006-01-02T15:04:05Z
	ReplicaCreated string `json:"replicaCreated"`

	// JobName which uses the secret
	//
	// required: false
	// example: job-abc
	JobName string `json:"jobName"`

	// JobCreated which uses the secret
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	JobCreated string `json:"jobCreated"`

	// BatchName which uses the secret
	//
	// required: false
	// example: batch-abc
	BatchName string `json:"batchName"`

	// BatchCreated which uses the secret
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	BatchCreated string `json:"batchCreated"`

	// Version of the secret
	//
	// required: true
	// example: 0123456789
	Version string `json:"version"`
}
