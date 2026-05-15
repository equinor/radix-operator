package models

import (
	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/api-server/api/utils/owner"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
)

// ComponentStatus Enumeration of the statuses of component
type ComponentStatus int

const (
	// StoppedComponent stopped component
	StoppedComponent ComponentStatus = iota

	// ConsistentComponent consistent component
	ConsistentComponent

	// ComponentReconciling Component reconciling
	ComponentReconciling

	// ComponentRestarting restarting component
	ComponentRestarting

	// ComponentOutdated has outdated image
	ComponentOutdated

	numComponentStatuses
)

func (p ComponentStatus) String() string {
	if p >= numComponentStatuses {
		return "Unsupported"
	}
	return [...]string{"Stopped", "Consistent", "Reconciling", "Restarting", "Outdated"}[p]
}

type ComponentStatuserFunc func(component radixv1.RadixCommonDeployComponent, kd *appsv1.Deployment, rd *radixv1.RadixDeployment) ComponentStatus

func ComponentStatusFromDeployment(component radixv1.RadixCommonDeployComponent, kd *appsv1.Deployment, rd *radixv1.RadixDeployment) ComponentStatus {
	if kd == nil || kd.GetName() == "" {
		return ComponentReconciling
	}
	replicasUnavailable := kd.Status.UnavailableReplicas
	replicasReady := kd.Status.ReadyReplicas
	replicas := pointers.Val(kd.Spec.Replicas)

	if isComponentRestarting(component, rd) {
		return ComponentRestarting
	}

	if !owner.VerifyCorrectObjectGeneration(rd, kd, kube.RadixDeploymentObservedGeneration) {
		return ComponentOutdated
	}

	if replicas == 0 {
		return StoppedComponent
	}

	// Check if component is scaling up or down
	if replicasUnavailable > 0 || replicas < replicasReady {
		return ComponentReconciling
	}

	return ConsistentComponent
}

func isComponentRestarting(component radixv1.RadixCommonDeployComponent, rd *radixv1.RadixDeployment) bool {
	restarted := component.GetEnvironmentVariables()[operatordefaults.RadixRestartEnvironmentVariable]
	if restarted == "" {
		return false
	}
	restartedTime, err := commonutils.ParseTimestamp(restarted)
	if err != nil {
		log.Logger.Warn().Err(err).Msgf("unable to parse restarted time %v, component: %s", restarted, component.GetName())
		return false
	}
	reconciledTime := rd.Status.Reconciled
	return reconciledTime.IsZero() || restartedTime.After(reconciledTime.Time)
}
