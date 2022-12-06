package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
)

const defaultServiceAccountName = "default"

// ServiceAccountSpec defines methods for getting service account spec for a deployment pod
type ServiceAccountSpec interface {
	ServiceAccountName() string
	AutomountServiceAccountToken() *bool
}

// Service account spec for Radix API deployment
type radixAPIServiceAccountSpec struct{}

func (spec *radixAPIServiceAccountSpec) ServiceAccountName() string {
	return defaults.RadixAPIServiceAccountName
}

func (spec *radixAPIServiceAccountSpec) AutomountServiceAccountToken() *bool {
	return utils.BoolPtr(true)
}

// Service account spec for Radix GitHub Webhook deployment
type radixWebhookServiceAccountSpec struct{}

func (spec *radixWebhookServiceAccountSpec) ServiceAccountName() string {
	return defaults.RadixGithubWebhookServiceAccountName
}

func (spec *radixWebhookServiceAccountSpec) AutomountServiceAccountToken() *bool {
	return utils.BoolPtr(true)
}

// Service account spec for Radix job scheduler deployment
type jobSchedulerServiceAccountSpec struct{}

func (spec *jobSchedulerServiceAccountSpec) ServiceAccountName() string {
	return defaults.RadixJobSchedulerServerServiceName
}

func (spec *jobSchedulerServiceAccountSpec) AutomountServiceAccountToken() *bool {
	return utils.BoolPtr(true)
}

// Service account spec for Radix component deployments
type radixComponentServiceAccountSpec struct {
	component v1.RadixCommonDeployComponent
}

func (spec *radixComponentServiceAccountSpec) ServiceAccountName() string {
	if componentRequiresServiceAccount(spec.component) {
		return utils.GetComponentServiceAccountName(spec.component.GetName())
	}
	return defaultServiceAccountName
}

func (spec *radixComponentServiceAccountSpec) AutomountServiceAccountToken() *bool {
	return utils.BoolPtr(false)
}

// NewServiceAccountSpec Create ServiceAccountSpec based on RadixDeployment and RadixCommonDeployComponent
func NewServiceAccountSpec(radixDeploy *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) ServiceAccountSpec {
	isComponent := deployComponent.GetType() == v1.RadixComponentTypeComponent
	isJobScheduler := isDeployComponentJobSchedulerDeployment(deployComponent)

	if isComponent && isRadixAPI(radixDeploy) {
		return &radixAPIServiceAccountSpec{}
	}

	if isComponent && isRadixWebHook(radixDeploy) {
		return &radixWebhookServiceAccountSpec{}
	}

	if isJobScheduler {
		return &jobSchedulerServiceAccountSpec{}
	}

	return &radixComponentServiceAccountSpec{component: deployComponent}
}
