package models

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	runtimeUtils "github.com/equinor/radix-operator/pkg/apis/runtime"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
)

// Component describe an component part of an deployment
// swagger:model Component
type Component struct {
	// Name the component
	//
	// required: true
	// example: server
	Name string `json:"name"`

	// Type of component
	//
	// required: true
	// enum: component,job
	// example: component
	Type string `json:"type"`

	// Status of the component
	// required: false
	// - Stopped = Component is stopped (no replica)
	// - Consistent = Component is consistent with config
	// - Restarting = User has triggered restart, but this is not reconciled
	//
	// enum: Stopped,Consistent,Reconciling,Restarting,Outdated
	// example: Consistent
	Status string `json:"status"`

	// Image name
	//
	// required: true
	// example: radixdev.azurecr.io/app-server:cdgkg
	Image string `json:"image"`

	// Ports defines the port number and protocol that a component is exposed for internally in environment
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/Port"
	Ports []Port `json:"ports"`

	// SchedulerPort defines the port number that a Job Scheduler is exposed internally in environment
	//
	// required: false
	// example: 8080
	SchedulerPort *int32 `json:"schedulerPort,omitempty"`

	// ScheduledJobPayloadPath defines the payload path, where payload for Job Scheduler will be mapped as a file. From radixconfig.yaml
	//
	// required: false
	// example: "/tmp/payload"
	ScheduledJobPayloadPath string `json:"scheduledJobPayloadPath,omitempty"`

	// Component secret names. From radixconfig.yaml
	//
	// required: false
	// example: ["DB_CON", "A_SECRET"]
	Secrets []string `json:"secrets"`

	// Variable names map to values. From radixconfig.yaml
	//
	// required: false
	Variables map[string]string `json:"variables"`

	// Deprecated: Array of pod names. Use ReplicaList instead
	//
	// required: false
	// example: ["server-78fc8857c4-hm76l", "server-78fc8857c4-asfa2"]
	Replicas []string `json:"replicas"`

	// Array of ReplicaSummary
	//
	// required: false
	ReplicaList []ReplicaSummary `json:"replicaList"`

	// Set if manual control of replicas is in place. Not set means automatic control, 0 means stopped and >= 1 is manually scaled.
	//
	// required: false
	// example: 5
	// Extensions:
	// x-nullable: true
	ReplicasOverride *int `json:"replicasOverride"`

	// HorizontalScaling defines horizontal scaling summary for this component
	//
	// required: false
	HorizontalScalingSummary *HorizontalScalingSummary `json:"horizontalScalingSummary"`

	// External identity information
	//
	// required: false
	Identity *Identity `json:"identity,omitempty"`

	// Notifications is the spec for notification about internal events or changes
	Notifications *Notifications `json:"notifications,omitempty"`

	// Array of external DNS configurations
	//
	// required: false
	ExternalDNS []ExternalDNS `json:"externalDNS,omitempty"`

	// Commit ID for the component. It can be different from the Commit ID, specified in deployment label
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID,omitempty"`

	// GitTags the git tags that the git commit hash points to
	//
	// required: false
	// example: "v1.22.1 v1.22.3"
	GitTags string `json:"gitTags,omitempty"`

	// SkipDeployment The component should not be deployed, but used existing
	//
	// required: false
	// example: true
	SkipDeployment bool `json:"skipDeployment,omitempty"`

	AuxiliaryResource `json:",inline"`

	// Resources Resource requirements for the pod
	//
	// required: false
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Runtime requirements for the component or job
	Runtime *Runtime `json:"runtime,omitempty"`

	// Network configuration for the component
	//
	// required: false
	Network *Network `json:"network,omitempty"`
}

// Network describes network configuration for a component
// swagger:model Network
type Network struct {
	// Ingress configuration
	//
	// required: false
	Ingress *Ingress `json:"ingress,omitempty"`
}

// Ingress describes ingress configuration for a component
// swagger:model Ingress
type Ingress struct {
	// Public ingress configuration
	//
	// required: false
	Public *IngressPublic `json:"public,omitempty"`
}

// IngressPublic describes public ingress configuration for a component
// swagger:model IngressPublic
type IngressPublic struct {
	// List of allowed IP addresses or CIDRs. All traffic is allowed if list is empty.
	//
	// required: true
	Allow []string `json:"allow"`
}

// ExternalDNS describes an external DNS entry for a component
// swagger:model ExternalDNS
type ExternalDNS struct {
	// Fully Qualified Domain Name
	//
	// required: true
	// example: site.example.com
	FQDN string `json:"fqdn"`

	// TLS configuration
	//
	// required: true
	TLS TLS `json:"tls"`
}

// Identity describes external identities
type Identity struct {
	// Azure identity
	//
	// required: false
	Azure *AzureIdentity `json:"azure,omitempty"`
}

// Notifications is the spec for notification about internal events or changes
type Notifications struct {
	// Webhook is a URL for notification about internal events or changes. The URL should be of a Radix component or job-component, with not public port.
	//
	// required: false
	Webhook *string `json:"webhook,omitempty"`
}

// AzureIdentity describes an identity in Azure
type AzureIdentity struct {
	// ClientId is the client ID of an Azure User Assigned Managed Identity
	// or the application ID of an Azure AD Application Registration
	//
	// required: true
	ClientId string `json:"clientId,omitempty"`

	// The Service Account name to use when configuring Kubernetes Federation Credentials for the identity
	//
	// required: true
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// The Azure Key Vaults names, which use Azure Identity
	//
	// required: false
	AzureKeyVaults []string `json:"azureKeyVaults,omitempty"`
}

// AuxiliaryResource describes an auxiliary resources for a component
// swagger:ignore
type AuxiliaryResource struct {
	// OAuth2 describes the oauth2 resource
	//
	// required: false
	// - oauth: OAuth2 auxiliary resource
	OAuth2 *OAuth2AuxiliaryResource `json:"oauth2,omitempty"`
}

type OAuth2AuxiliaryResource struct {
	// Deprecated. Use Deployments instead
	// Deployment describes the underlying Kubernetes deployment for the resource
	//
	// required: true
	Deployment AuxiliaryResourceDeployment `json:"deployment,omitempty"`

	// Deployments describes the underlying Kubernetes deployments for the resource
	//
	// required: false
	Deployments []AuxiliaryResourceDeployment `json:"deployments,omitempty"`

	// External identity information
	//
	// required: false
	Identity *Identity `json:"identity,omitempty"`

	// SessionStoreType type of session store
	//
	// enum: cookie,redis,systemManaged,""
	// required: false
	SessionStoreType string `json:"sessionStoreType,omitempty"`
}

// AuxiliaryResourceDeployment describes the state of the auxiliary resource's deployment
// swagger:model AuxiliaryResourceDeployment
type AuxiliaryResourceDeployment struct {
	// Name of the auxiliary resource's deployment
	//
	// enum: oauth,oauth-redis,""
	// required: false
	Type string `json:"type,omitempty"`

	// Status of the auxiliary resource's deployment
	// required: true
	// - Consistent: All replicas are running with the desired state
	// - Reconciling: Waiting for new replicas to enter desired state
	// - Stopped: Replica count is set to 0
	//
	// enum: Stopped,Consistent,Reconciling
	// example: Consistent
	Status string `json:"status"`

	// Running replicas of the auxiliary resource's deployment
	//
	// required: false
	ReplicaList []ReplicaSummary `json:"replicaList"`
}

// Port describe a port of a component
// swagger:model Port
type Port struct {
	// Component port name. From radixconfig.yaml
	//
	// required: true
	// example: http
	Name string `json:"name"`

	// Component port number. From radixconfig.yaml
	//
	// required: true
	// example: 8080
	Port int32 `json:"port"`

	// IsPublic indicates that the port is accessible from the Internet by proxying traffic from 443
	//
	// required: true
	// example: true
	IsPublic bool `json:"isPublic"`
}

// ComponentSummary describe a component part of a deployment
// swagger:model ComponentSummary
type ComponentSummary struct {
	// Name the component
	//
	// required: true
	// example: server
	Name string `json:"name"`

	// Type of component
	//
	// required: true
	// enum: component,job
	// example: component
	Type string `json:"type"`

	// Image name
	//
	// required: true
	// example: radixdev.azurecr.io/app-server:cdgkg
	Image string `json:"image"`

	// CommitID the commit ID of the branch to build
	// REQUIRED for "build" and "build-deploy" pipelines
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID,omitempty"`

	// GitTags the git tags that the git commit hash points to
	//
	// required: false
	// example: "v1.22.1 v1.22.3"
	GitTags string `json:"gitTags,omitempty"`

	// SkipDeployment The component should not be deployed, but used existing
	//
	// required: false
	// example: true
	SkipDeployment bool `json:"skipDeployment,omitempty"`

	// Resources Resource requirements for the component
	//
	// required: false
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Runtime requirements for the component or job
	Runtime *Runtime `json:"runtime,omitempty"`
}

// ReplicaType The replica type
type ReplicaType int

const (
	// JobManager Replica of a Radix job-component scheduler
	JobManager ReplicaType = iota
	// JobManagerAux Replica of a Radix job-component scheduler auxiliary
	JobManagerAux
	// OAuth2 Replica of a Radix OAuth2 Proxy component
	OAuth2
	// OAuth2Redis Replica of a Radix OAuth2 Redis component
	OAuth2Redis
	// Undefined Replica without defined type - to be extended
	Undefined
	numReplicaType
)

// Convert ReplicaType to a string
func (p ReplicaType) String() string {
	if p >= numReplicaType {
		return "Unsupported"
	}
	return [...]string{"JobManager", "JobManagerAux", "OAuth2", "OAuth2Redis", "Undefined"}[p]
}

// ReplicaSummary describes condition of a pod
// swagger:model ReplicaSummary
type ReplicaSummary struct {
	// Pod name
	//
	// required: true
	// example: server-78fc8857c4-hm76l
	Name string `json:"name"`

	// Pod type
	// - ComponentReplica = Replica of a Radix component
	// - ScheduledJobReplica = Replica of a Radix job-component
	// - JobManager = Replica of a Radix job-component scheduler
	// - JobManagerAux = Replica of a Radix job-component scheduler auxiliary
	// - OAuth2 = Replica of a Radix OAuth2 component
	// - Undefined = Replica without defined type - to be extended
	//
	// required: false
	// enum: ComponentReplica,ScheduledJobReplica,JobManager,JobManagerAux,OAuth2,OAuth2Redis,Undefined
	// example: ComponentReplica
	Type string `json:"type"`

	// Created timestamp
	//
	// required: true
	// swagger:strfmt date-time
	Created time.Time `json:"created"`

	// The time at which the batch job's pod finishedAt.
	//
	// required: false
	// swagger:strfmt date-time
	EndTime *time.Time `json:"endTime"`

	// Container started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	ContainerStarted *time.Time `json:"containerStarted"`

	// Status describes the component container status
	//
	// required: false
	Status ReplicaStatus `json:"replicaStatus,omitempty"`

	// StatusMessage provides message describing the status of a component container inside a pod
	//
	// required: false
	StatusMessage string `json:"statusMessage,omitempty"`

	// RestartCount count of restarts of a component container inside a pod
	//
	// required: false
	RestartCount int32 `json:"restartCount,omitempty"`

	// The image the container is running.
	//
	// required: false
	// example: radixdev.azurecr.io/app-server:cdgkg
	Image string `json:"image,omitempty"`

	// ImageID of the container's image.
	//
	// required: false
	// example: radixdev.azurecr.io/app-server@sha256:d40cda01916ef63da3607c03785efabc56eb2fc2e0dab0726b1a843e9ded093f
	ImageId string `json:"imageId,omitempty"`

	// The index of the pod in the re-starts
	PodIndex int `json:"podIndex,omitempty"`

	// Exit status from the last termination of the container
	ExitCode int32 `json:"exitCode"`

	// A brief CamelCase message indicating details about why the job is in this phase
	Reason string `json:"reason,omitempty"`

	// Resources Resource requirements for the pod
	//
	// required: false
	Resources *ResourceRequirements `json:"resources,omitempty"`
}

// ReplicaStatus describes the status of a component container inside a pod
// swagger:model ReplicaStatus
type ReplicaStatus struct {
	// Status of the container
	// - Pending = Container in Waiting state and the reason is ContainerCreating
	// - Starting = Container is starting
	// - Failed = Container is failed
	// - Failing = Container is failed
	// - Running = Container in Running state
	// - Succeeded = Container in Succeeded state
	// - Stopped = Replica was deleted du to job stopped
	// - Terminated = Container in Terminated state
	//
	// required: true
	// example: Running
	Status ContainerStatus `json:"status"`
}

// HorizontalScalingSummary describe the summary of horizontal scaling of a component
// swagger:model HorizontalScalingSummary
type HorizontalScalingSummary struct {
	// Component minimum replicas. From radixconfig.yaml
	//
	// example: 2
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// Component maximum replicas. From radixconfig.yaml
	//
	// example: 5
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`

	// CooldownPeriod in seconds. From radixconfig.yaml
	//
	// example: 300
	CooldownPeriod *int32 `json:"cooldownPeriod,omitempty"`

	// PollingInterval in seconds. From radixconfig.yaml
	//
	// example: 30
	PollingInterval *int32 `json:"pollingInterval,omitempty"`

	// Triggers lists status of all triggers found in radixconfig.yaml
	//
	// required: true
	Triggers []HorizontalScalingSummaryTriggerStatus `json:"triggers"`

	// Deprecated: Component current average CPU utilization over all pods, represented as a percentage of requested CPU. Use Triggers instead. Will be removed from Radix API 2025-01-01.
	//
	// example: 70
	CurrentCPUUtilizationPercentage *int32 `json:"currentCPUUtilizationPercentage"`

	// Deprecated: Component target average CPU utilization over all pods. Use Triggers instead. Will be removed from Radix API 2025-01-01.
	//
	// example: 80
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage"`

	// Deprecated: Component current average memory utilization over all pods, represented as a percentage of requested memory. Use Triggers instead. Will be removed from Radix API 2025-01-01.
	//
	// example: 80
	CurrentMemoryUtilizationPercentage *int32 `json:"currentMemoryUtilizationPercentage"`

	// Deprecated: Component target average memory utilization over all pods. use Triggers instead. Will be removed from Radix API 2025-01-01.
	//
	// example: 80
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage"`

	// CurrentReplicas returns the current number of replicas
	// example: 1
	// required: true
	CurrentReplicas int32 `json:"currentReplicas"`

	// DesiredReplicas returns the target number of replicas across all triggers
	// example: 2
	// required: true
	DesiredReplicas int32 `json:"desiredReplicas"`
}

type HorizontalScalingSummaryTriggerStatus struct {
	// Name of trigger
	Name string `json:"name"`

	// CurrentUtilization is the last measured utilization
	CurrentUtilization string `json:"currentUtilization"`

	// TargetUtilization  is the average target across replicas
	TargetUtilization string `json:"targetUtilization"`

	// Type of trigger
	Type string `json:"type"`

	// Error contains short description if trigger have problems
	Error string `json:"error,omitempty"`
}

// Node Defines node attributes, where pod should be scheduled
type Node struct {
	// Gpu Holds lists of node GPU types, with dashed types to exclude
	Gpu string `json:"gpu,omitempty"`
	// GpuCount Holds minimum count of GPU on node
	GpuCount string `json:"gpuCount,omitempty"`
}

// Resources Required for pods
type Resources struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// ResourceRequirements Requirements of resources for pods
type ResourceRequirements struct {
	Limits   Resources `json:"limits,omitempty"`
	Requests Resources `json:"requests,omitempty"`
}

// Runtime requirements for the component or job
type Runtime struct {
	// CPU architecture
	//
	// example: amd64
	Architecture string `json:"architecture,omitempty"`

	// NodeType is the node type of the component
	//
	// example:memory-optimized-2-v1
	NodeType string `json:"nodeType,omitempty"`
}

func GetReplicaSummary(pod corev1.Pod, lastEventWarning string) ReplicaSummary {
	replicaSummary := ReplicaSummary{
		Type: getReplicaType(pod).String(),
	}
	replicaSummary.Name = pod.GetName()
	creationTimestamp := pod.GetCreationTimestamp()
	replicaSummary.Created = creationTimestamp.Time

	// Set default Pending status
	replicaSummary.Status = ReplicaStatus{Status: Pending}

	if len(pod.Status.ContainerStatuses) == 0 {
		condition := getLastReadyCondition(pod.Status.Conditions)
		if condition != nil {
			replicaSummary.Status = ReplicaStatus{Status: getReplicaStatusByPodStatus(pod.Status.Phase)}
			replicaSummary.StatusMessage = fmt.Sprintf("%s: %s", condition.Reason, condition.Message)
		}
		return replicaSummary
	}
	// We assume one component container per component pod
	containerStatus := pod.Status.ContainerStatuses[0]
	containerState := containerStatus.State

	if containerState.Waiting != nil {
		replicaSummary.StatusMessage = containerState.Waiting.Message
		if !strings.EqualFold(containerState.Waiting.Reason, "ContainerCreating") {
			replicaSummary.Status = ReplicaStatus{Status: Failing}
		}
	}
	if containerState.Running != nil {
		replicaSummary.ContainerStarted = &containerState.Running.StartedAt.Time
		if containerStatus.Ready {
			replicaSummary.Status = ReplicaStatus{Status: Running}
		} else {
			replicaSummary.Status = ReplicaStatus{Status: Starting}
		}
	}
	if containerState.Terminated != nil {
		replicaSummary.Status = ReplicaStatus{Status: Terminated}
		replicaSummary.StatusMessage = containerState.Terminated.Message
	}
	terminated := containerStatus.LastTerminationState.Terminated
	if terminated != nil {
		compositeMessage := []string{fmt.Sprintf("Last time container was terminated at: %s, with code: %d", radixutils.FormatTime(&terminated.FinishedAt), terminated.ExitCode)}
		if terminated.Reason != "" {
			compositeMessage = append(compositeMessage, fmt.Sprintf("reason: '%s'", terminated.Reason))
		}
		if terminated.Message != "" {
			compositeMessage = append(compositeMessage, fmt.Sprintf("message: '%s'", terminated.Message))
		}
		replicaSummary.StatusMessage = strings.Join(compositeMessage, ", ")
		replicaSummary.ExitCode = terminated.ExitCode
	}
	replicaSummary.RestartCount = containerStatus.RestartCount
	replicaSummary.Image = pod.Spec.Containers[0].Image
	replicaSummary.ImageId = containerStatus.ImageID
	if len(pod.Spec.Containers) > 0 {
		replicaSummary.Resources = pointers.Ptr(ConvertResourceRequirements(pod.Spec.Containers[0].Resources))
	}
	if len(replicaSummary.StatusMessage) == 0 && (replicaSummary.Status.Status == Failing || replicaSummary.Status.Status == Pending) {
		replicaSummary.StatusMessage = lastEventWarning
	}
	return replicaSummary
}

// NewRuntime creates an API runtime model by the Radix runtime
func NewRuntime(radixRuntime *radixv1.Runtime) *Runtime {
	runtimeModel := &Runtime{
		Architecture: string(defaults.DefaultNodeSelectorArchitecture),
	}
	if radixRuntime == nil {
		return runtimeModel
	}
	runtimeModel.Architecture = runtimeUtils.GetArchitectureFromRuntimeOrDefault(radixRuntime)
	if nodeType := radixRuntime.GetNodeType(); nodeType != nil && len(*nodeType) > 0 {
		runtimeModel.NodeType = *nodeType
	}
	return runtimeModel
}

func getReplicaType(pod corev1.Pod) ReplicaType {
	switch {
	case pod.GetLabels()[kube.RadixPodIsJobSchedulerLabel] == "true":
		return JobManager
	case pod.GetLabels()[kube.RadixAuxiliaryComponentTypeLabel] == kube.RadixJobTypeManagerAux:
		return JobManagerAux
	case pod.GetLabels()[kube.RadixAuxiliaryComponentTypeLabel] == "oauth":
		return OAuth2
	case pod.GetLabels()[kube.RadixAuxiliaryComponentTypeLabel] == "oauth-redis":
		return OAuth2Redis
	default:
		return Undefined
	}
}

func getLastReadyCondition(conditions []corev1.PodCondition) *corev1.PodCondition {
	if len(conditions) == 1 {
		return &conditions[0]
	}
	conditions = sortStatusConditionsDesc(conditions)
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionTrue {
			return &condition
		}
	}
	if len(conditions) > 0 {
		return &conditions[0]
	}
	return nil
}

func sortStatusConditionsDesc(conditions []corev1.PodCondition) []corev1.PodCondition {
	sort.Slice(conditions, func(i, j int) bool {
		if conditions[i].LastTransitionTime.Time.IsZero() || conditions[j].LastTransitionTime.Time.IsZero() {
			return false
		}
		return conditions[j].LastTransitionTime.Time.Before(conditions[i].LastTransitionTime.Time)
	})
	return conditions
}

func (job *ScheduledJobSummary) GetCreated() *time.Time {
	return job.Created
}

func (job *ScheduledJobSummary) GetStarted() *time.Time {
	return job.Started
}

func (job *ScheduledJobSummary) GetEnded() *time.Time {
	return job.Ended
}

func (job *ScheduledJobSummary) GetStatus() string {
	return string(job.Status)
}

func (job *ScheduledBatchSummary) GetCreated() *time.Time {
	return job.Created
}

func (job *ScheduledBatchSummary) GetStarted() *time.Time {
	return job.Started
}

func (job *ScheduledBatchSummary) GetEnded() *time.Time {
	return job.Ended
}

func (job *ScheduledBatchSummary) GetStatus() string {
	return string(job.Status)
}

func getReplicaStatusByPodStatus(podPhase corev1.PodPhase) ContainerStatus {
	switch podPhase {
	case corev1.PodPending:
		return Pending
	case corev1.PodRunning:
		return Running
	case corev1.PodFailed:
		return Failing
	case corev1.PodSucceeded:
		return Terminated
	default:
		return ContainerStatus("")
	}
}
