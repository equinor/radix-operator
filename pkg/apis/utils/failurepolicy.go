package utils

import (
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	batchv1 "k8s.io/api/batch/v1"
)

func GetPodFailurePolicy(failurePolicy *radixv1.RadixJobComponentFailurePolicy) *batchv1.PodFailurePolicy {
	if failurePolicy == nil {
		return nil
	}

	onExitCodesMapper := func(onExitCodes radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes) *batchv1.PodFailurePolicyOnExitCodesRequirement {
		slices.Sort(onExitCodes.Values)
		return &batchv1.PodFailurePolicyOnExitCodesRequirement{
			Operator: batchv1.PodFailurePolicyOnExitCodesOperator(onExitCodes.Operator),
			Values:   onExitCodes.Values,
		}
	}

	ruleMapper := func(rule radixv1.RadixJobComponentFailurePolicyRule) batchv1.PodFailurePolicyRule {
		return batchv1.PodFailurePolicyRule{
			Action:      batchv1.PodFailurePolicyAction(rule.Action),
			OnExitCodes: onExitCodesMapper(rule.OnExitCodes),
		}
	}

	return &batchv1.PodFailurePolicy{
		Rules: slice.Map(failurePolicy.Rules, ruleMapper),
	}
}
