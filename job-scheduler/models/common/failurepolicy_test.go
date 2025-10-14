package common_test

import (
	"testing"

	models "github.com/equinor/radix-operator/job-scheduler/models/common"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_MapToRadixFailurePolicy(t *testing.T) {
	sut := models.FailurePolicy{
		Rules: []models.FailurePolicyRule{
			{
				Action: models.FailurePolicyRuleActionCount,
				OnExitCodes: models.FailurePolicyRuleOnExitCodes{
					Operator: models.FailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{1, 2, 3},
				},
			},
			{
				Action: models.FailurePolicyRuleActionFailJob,
				OnExitCodes: models.FailurePolicyRuleOnExitCodes{
					Operator: models.FailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{4},
				},
			},
			{
				Action: models.FailurePolicyRuleActionIgnore,
				OnExitCodes: models.FailurePolicyRuleOnExitCodes{
					Operator: models.FailurePolicyRuleOnExitCodesOpNotIn,
					Values:   []int32{5},
				},
			},
		},
	}
	expected := &radixv1.RadixJobComponentFailurePolicy{
		Rules: []radixv1.RadixJobComponentFailurePolicyRule{
			{
				Action: radixv1.RadixJobComponentFailurePolicyActionCount,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
					Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{1, 2, 3},
				},
			},
			{
				Action: radixv1.RadixJobComponentFailurePolicyActionFailJob,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
					Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{4},
				},
			},
			{
				Action: radixv1.RadixJobComponentFailurePolicyActionIgnore,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
					Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpNotIn,
					Values:   []int32{5},
				},
			},
		},
	}
	actual := sut.MapToRadixFailurePolicy()
	assert.Equal(t, expected, actual)
}
