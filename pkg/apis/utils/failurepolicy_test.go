package utils_test

import (
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
)

func Test_GetPodFailurePolicy_Action(t *testing.T) {
	radixPolicy := radixv1.RadixJobComponentFailurePolicy{
		Rules: []radixv1.RadixJobComponentFailurePolicyRule{
			{Action: radixv1.RadixJobComponentFailurePolicyActionCount},
			{Action: radixv1.RadixJobComponentFailurePolicyActionFailJob},
			{Action: radixv1.RadixJobComponentFailurePolicyActionIgnore},
		},
	}
	actualPolilcy := utils.GetPodFailurePolicy(&radixPolicy)
	expectedPolicy := &batchv1.PodFailurePolicy{
		Rules: []batchv1.PodFailurePolicyRule{
			{Action: batchv1.PodFailurePolicyActionCount, OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{}},
			{Action: batchv1.PodFailurePolicyActionFailJob, OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{}},
			{Action: batchv1.PodFailurePolicyActionIgnore, OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{}},
		},
	}
	assert.Equal(t, expectedPolicy, actualPolilcy)
}

func Test_GetPodFailurePolicy_OnExitCodes_Operator(t *testing.T) {
	radixPolicy := radixv1.RadixJobComponentFailurePolicy{
		Rules: []radixv1.RadixJobComponentFailurePolicyRule{
			{Action: radixv1.RadixJobComponentFailurePolicyActionCount, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn}},
			{Action: radixv1.RadixJobComponentFailurePolicyActionCount, OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpNotIn}},
		},
	}
	actualPolilcy := utils.GetPodFailurePolicy(&radixPolicy)
	expectedPolicy := &batchv1.PodFailurePolicy{
		Rules: []batchv1.PodFailurePolicyRule{
			{Action: batchv1.PodFailurePolicyActionCount, OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{Operator: batchv1.PodFailurePolicyOnExitCodesOpIn}},
			{Action: batchv1.PodFailurePolicyActionCount, OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{Operator: batchv1.PodFailurePolicyOnExitCodesOpNotIn}},
		},
	}
	assert.Equal(t, expectedPolicy, actualPolilcy)
}

func Test_GetPodFailurePolicy_OnExitCodes_ValuesSorted(t *testing.T) {
	radixPolicy := radixv1.RadixJobComponentFailurePolicy{
		Rules: []radixv1.RadixJobComponentFailurePolicyRule{
			{
				Action:      radixv1.RadixJobComponentFailurePolicyActionCount,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn, Values: []int32{4, 2, 3, 1}},
			},
		},
	}
	actualPolilcy := utils.GetPodFailurePolicy(&radixPolicy)
	expectedPolicy := &batchv1.PodFailurePolicy{
		Rules: []batchv1.PodFailurePolicyRule{
			{
				Action:      batchv1.PodFailurePolicyActionCount,
				OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{Operator: batchv1.PodFailurePolicyOnExitCodesOpIn, Values: []int32{1, 2, 3, 4}},
			},
		},
	}
	assert.Equal(t, expectedPolicy, actualPolilcy)
}

func Test_GetPodFailurePolicy_Nil(t *testing.T) {
	assert.Nil(t, utils.GetPodFailurePolicy(nil))
}
