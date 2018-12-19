package deployment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test(t *testing.T) {
	q, _ := resource.ParseQuantity("2")

	i, _ := q.AsInt64()
	assert.Equal(t, int64(2), i)

	q, _ = resource.ParseQuantity("2000m")

	d := q.AsDec()
	assert.Equal(t, "2", d.)
}
