package v1

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RadixDeployment describe a deployment
type RadixDeployment struct {
	meta_v1.TypeMeta   `json:",inline" yaml:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec               RadixDeploymentSpec `json:"spec" yaml:"spec"`
	Status             RadixDeployStatus   `json:"status" yaml:"status"`
}

//RadixDeployStatus is the status for a rd
type RadixDeployStatus struct {
	ActiveFrom meta_v1.Time         `json:"activeFrom" yaml:"activeFrom"`
	ActiveTo   meta_v1.Time         `json:"activeTo" yaml:"activeTo"`
	Condition  RadixDeployCondition `json:"condition" yaml:"condition"`
	Reconciled meta_v1.Time         `json:"reconciled" yaml:"reconciled"`
}

// RadixDeployCondition Holds the condition of a component
type RadixDeployCondition string

// These are valid conditions of a deployment.
const (
	// Active means the radix deployment is active and should be consolidated
	DeploymentActive RadixDeployCondition = "Active"
	// Inactive means radix deployment is inactive and should not be consolidated
	DeploymentInactive RadixDeployCondition = "Inactive"
)

//RadixDeploymentSpec is the spec for a deployment
type RadixDeploymentSpec struct {
	AppName          string                         `json:"appname" yaml:"appname"`
	Components       []RadixDeployComponent         `json:"components"`
	Jobs             []RadixDeployJobComponent      `json:"jobs"`
	Environment      string                         `json:"environment" yaml:"environment"`
	ImagePullSecrets []core_v1.LocalObjectReference `json:"imagePullSecrets" yaml:"imagePullSecrets"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//RadixDeploymentList is a list of Radix deployments
type RadixDeploymentList struct {
	meta_v1.TypeMeta `json:",inline" yaml:",inline"`
	meta_v1.ListMeta `json:"metadata" yaml:"metadata"`
	Items            []RadixDeployment `json:"items" yaml:"items"`
}

//RadixDeployComponent defines a single component within a RadixDeployment - maps to single deployment/service/ingress etc
type RadixDeployComponent struct {
	Name     string          `json:"name" yaml:"name"`
	Image    string          `json:"image" yaml:"image"`
	Ports    []ComponentPort `json:"ports" yaml:"ports"`
	Replicas *int            `json:"replicas" yaml:"replicas"`
	// Deprecated: For backwards compatibility Public is still supported, new code should use PublicPort instead
	Public                  bool                    `json:"public" yaml:"public"`
	PublicPort              string                  `json:"publicPort,omitempty" yaml:"publicPort,omitempty"`
	EnvironmentVariables    EnvVarsMap              `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	Secrets                 []string                `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	IngressConfiguration    []string                `json:"ingressConfiguration,omitempty" yaml:"ingressConfiguration,omitempty"`
	DNSAppAlias             bool                    `json:"dnsAppAlias,omitempty" yaml:"dnsAppAlias,omitempty"`
	DNSExternalAlias        []string                `json:"dnsExternalAlias,omitempty" yaml:"dnsExternalAlias,omitempty"`
	Monitoring              bool                    `json:"monitoring" yaml:"monitoring"`
	Resources               ResourceRequirements    `json:"resources,omitempty" yaml:"resources,omitempty"`
	HorizontalScaling       *RadixHorizontalScaling `json:"horizontalScaling,omitempty" yaml:"horizontalScaling,omitempty"`
	AlwaysPullImageOnDeploy bool                    `json:"alwaysPullImageOnDeploy" yaml:"alwaysPullImageOnDeploy"`
	VolumeMounts            []RadixVolumeMount      `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
}

// GetNrOfReplicas gets number of replicas component will run
func (deployComponent RadixDeployComponent) GetNrOfReplicas() int32 {
	replicas := int32(1)
	if deployComponent.HorizontalScaling != nil && deployComponent.HorizontalScaling.MinReplicas != nil {
		replicas = *deployComponent.HorizontalScaling.MinReplicas
	} else if deployComponent.Replicas != nil {
		replicas = int32(*deployComponent.Replicas)
	}
	return replicas
}

// GetResourceRequirements maps to core_v1.ResourceRequirements
func (deployComponent RadixDeployComponent) GetResourceRequirements() *core_v1.ResourceRequirements {
	return buildResourceRequirement(deployComponent.Resources)
}

//RadixDeployJobComponent defines a single job component within a RadixDeployment
// The job component is used by the radix-job-scheduler to create Kubernetes Job objects
type RadixDeployJobComponent struct {
	Name                 string                    `json:"name" yaml:"name"`
	Image                string                    `json:"image" yaml:"image"`
	Ports                []ComponentPort           `json:"ports" yaml:"ports"`
	EnvironmentVariables EnvVarsMap                `json:"environmentVariables,omitempty" yaml:"environmentVariables,omitempty"`
	Secrets              []string                  `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Monitoring           bool                      `json:"monitoring" yaml:"monitoring"`
	Resources            ResourceRequirements      `json:"resources,omitempty" yaml:"resources,omitempty"`
	VolumeMounts         []RadixVolumeMount        `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	SchedulerPort        *int32                    `json:"schedulerPort,omitempty" yaml:"schedulerPort,omitempty"`
	Payload              *RadixJobComponentPayload `json:"payload,omitempty" yaml:"payload,omitempty"`
}

// GetResourceRequirements maps to core_v1.ResourceRequirements
func (jobComponent RadixDeployJobComponent) GetResourceRequirements() *core_v1.ResourceRequirements {
	return buildResourceRequirement(jobComponent.Resources)
}

func buildResourceRequirement(source ResourceRequirements) *core_v1.ResourceRequirements {
	defaultLimits := map[core_v1.ResourceName]resource.Quantity{
		core_v1.ResourceName("cpu"):    *defaults.GetDefaultCPULimit(),
		core_v1.ResourceName("memory"): *defaults.GetDefaultMemoryLimit(),
	}

	// if you only set limit, it will use the same values for request
	limits := core_v1.ResourceList{}
	requests := core_v1.ResourceList{}

	for name, limit := range source.Limits {
		resName := core_v1.ResourceName(name)

		if limit != "" {
			limits[resName], _ = resource.ParseQuantity(limit)
		}

		// TODO: We probably should check some hard limit that cannot by exceeded here
	}

	for name, req := range source.Requests {
		resName := core_v1.ResourceName(name)

		if req != "" {
			requests[resName], _ = resource.ParseQuantity(req)

			if _, hasLimit := limits[resName]; !hasLimit {
				// There is no defined limit, but there is a request
				reqQuantity := requests[resName]
				if reqQuantity.Cmp(defaultLimits[resName]) == 1 {
					// Requested quantity is larger than the default limit
					// We use the requested value as the limit
					limits[resName] = requests[resName].DeepCopy()

					// TODO: If we introduce a hard limit, that should not be exceeded here
				}
			}
		}
	}

	if len(limits) <= 0 && len(requests) <= 0 {
		return nil
	}

	req := core_v1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}

	return &req
}
