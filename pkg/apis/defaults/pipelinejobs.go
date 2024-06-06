package defaults

// RadixPipelineJobPipelineContainerName The container name of the Radix pipeline orchestration job
const RadixPipelineJobPipelineContainerName = "radix-pipeline"

// RadixPipelineJobPreparePipelinesContainerName The container name of the pipeline job, reading the RadixApplication from the Radix configuration file and preparing the Sub-pipleine, if it is configured
const RadixPipelineJobPreparePipelinesContainerName = "prepare-pipelines"

// RadixPipelineJobRunPipelinesContainerName The container name of the pipeline job, running the Sub-pipleine, if it is configured
const RadixPipelineJobRunPipelinesContainerName = "run-pipelines"

// RadixCacheLayerNamePrefix The name of the cache artifact
const RadixCacheLayerNamePrefix = "radix-cache"

// DefaultRadixConfigFileName Default name for the radix configuration file
const DefaultRadixConfigFileName = "radixconfig.yaml"

// SecurityContextRunAsUser The user ID to run the container as
const SecurityContextRunAsUser = 65534 // Required by container image alpine/git used in pipeline for cloning repos

// SecurityContextRunAsGroup A group ID which the user running the container is member of
const SecurityContextRunAsGroup = 1000

// SecurityContextFsGroup A group ID which the user running the container is member of. This is also the group ID of
// files in any mounted volume
const SecurityContextFsGroup = 1000
