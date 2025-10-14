package common

// +kubebuilder:object:generate=true

import (
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// swagger:enum FailurePolicyRuleOnExitCodesOperator
type FailurePolicyRuleOnExitCodesOperator string

const (
	// The requirement is satisfied if the job replica's exit code is in the set of specified values.
	FailurePolicyRuleOnExitCodesOpIn FailurePolicyRuleOnExitCodesOperator = "In"

	// The requirement is satisfied if the job replica's exit code is not in the set of specified values.
	FailurePolicyRuleOnExitCodesOpNotIn FailurePolicyRuleOnExitCodesOperator = "NotIn"
)

// FailurePolicyRuleOnExitCodes describes the requirement for handling
// a failed job replica based on its exit code.
type FailurePolicyRuleOnExitCodes struct {
	// Represents the relationship between the job replica's exit code and the
	// specified values. Replicas completed with success (exit code 0) are
	// excluded from the requirement check.
	//
	// required: true
	Operator FailurePolicyRuleOnExitCodesOperator `json:"operator"`

	// Specifies the set of values. The job replica's exit code is checked against this set of
	// values with respect to the operator. The list must not contain duplicates.
	// Value '0' cannot be used for the In operator.
	//
	// minItems: 1
	// maxItems: 255
	// minimum: 0
	// required: true
	Values []int32 `json:"values"`
}

func (o FailurePolicyRuleOnExitCodes) mapToRadixFailurePolicyRuleOnExitCodes() radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes {
	return radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
		Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOperator(o.Operator),
		Values:   o.Values,
	}
}

// swagger:enum FailurePolicyRuleAction
type FailurePolicyRuleAction string

const (
	// This is an action which might be taken on a job replica failure - mark the
	// job as Failed and terminate all running pods.
	FailurePolicyRuleActionFailJob FailurePolicyRuleAction = "FailJob"

	// This is an action which might be taken on a job replica failure - the counter towards
	// .backoffLimit is not incremented and a replacement replica is created.
	FailurePolicyRuleActionIgnore FailurePolicyRuleAction = "Ignore"

	// This is an action which might be taken on a job replica failure - the replica failure
	// is handled in the default way - the counter towards .backoffLimit is incremented.
	FailurePolicyRuleActionCount FailurePolicyRuleAction = "Count"
)

// FailurePolicyRule describes how a job replica failure is handled when the onExitCodes rules are met.
type FailurePolicyRule struct {
	// Specifies the action taken on a job replica failure when the onExitCodes requirements are satisfied.
	//
	// required: true
	Action FailurePolicyRuleAction `json:"action"`

	// Represents the requirement on the job replica exit codes.
	//
	// required: true
	OnExitCodes FailurePolicyRuleOnExitCodes `json:"onExitCodes"`
}

func (r FailurePolicyRule) mapToRadixFailurePolicyRule() radixv1.RadixJobComponentFailurePolicyRule {
	return radixv1.RadixJobComponentFailurePolicyRule{
		Action:      radixv1.RadixJobComponentFailurePolicyAction(r.Action),
		OnExitCodes: r.OnExitCodes.mapToRadixFailurePolicyRuleOnExitCodes(),
	}
}

// FailurePolicy describes how failed job replicas influence the backoffLimit.
type FailurePolicy struct {
	// A list of failure policy rules. The rules are evaluated in order.
	// Once a rule matches a job replica failure, the remaining of the rules are ignored.
	// When no rule matches the failure, the default handling applies - the
	// counter of failures is incremented and it is checked against
	// the backoffLimit.
	//
	// maxItems: 20
	// required: true
	Rules []FailurePolicyRule `json:"rules"`
}

// MapToRadixFailurePolicy maps the object to a RadixJobComponentFailurePolicy object
func (f *FailurePolicy) MapToRadixFailurePolicy() *radixv1.RadixJobComponentFailurePolicy {
	if f == nil {
		return nil
	}

	ruleMapper := func(r FailurePolicyRule) radixv1.RadixJobComponentFailurePolicyRule {
		return r.mapToRadixFailurePolicyRule()
	}

	return &radixv1.RadixJobComponentFailurePolicy{
		Rules: slice.Map(f.Rules, ruleMapper),
	}
}
