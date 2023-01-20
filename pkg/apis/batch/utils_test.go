package batch

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_isResourceLabeledWithBatchJobName(t *testing.T) {

	obj := metav1.ObjectMeta{
		Labels: labels.Merge(
			labels.ForBatchJobName("job1"),
			map[string]string{"other-label1": "any-value1", "other-label2": "any-value2"},
		),
	}

	assert.True(t, isResourceLabeledWithBatchJobName("job1", &obj))
	assert.False(t, isResourceLabeledWithBatchJobName("job2", &obj))

}
