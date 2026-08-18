package predicate_test

import (
	"testing"
	"time"

	jobmodels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/utils/predicate"
	"github.com/stretchr/testify/assert"
)

func TestIsBefore(t *testing.T) {
	job1 := jobmodels.JobSummary{}
	job2 := jobmodels.JobSummary{}

	job1.Created = createTime("")
	job2.Created = createTime("")
	assert.False(t, predicate.IsJobBefore(&job1, &job2))

	job1.Created = createTime("2019-08-26T12:56:48Z")
	job2.Created = createTime("")
	assert.True(t, predicate.IsJobBefore(&job1, &job2))

	job1.Created = createTime("2019-08-26T12:56:48Z")
	job2.Created = createTime("2019-08-26T12:56:49Z")
	assert.True(t, predicate.IsJobBefore(&job1, &job2))

	job1.Created = createTime("2019-08-26T12:56:48Z")
	job2.Created = createTime("2019-08-26T12:56:48Z")
	job1.Started = new(createTime("2019-08-26T12:56:51Z"))

	job2.Started = new(createTime("2019-08-26T12:56:52Z"))
	assert.True(t, predicate.IsJobBefore(&job1, &job2))

	job1.Created = createTime("2019-08-26T12:56:48Z")
	job2.Created = createTime("2019-08-26T12:56:48Z")
	job1.Started = nil
	job2.Started = new(createTime("2019-08-26T12:56:52Z"))
	assert.False(t, predicate.IsJobBefore(&job1, &job2))

}

func createTime(timestamp string) time.Time {
	if timestamp == "" {
		return time.Time{}
	}

	t, _ := time.Parse(time.RFC3339, timestamp)
	return t
}
