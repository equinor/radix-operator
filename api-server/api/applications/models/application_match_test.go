package models

import (
	"testing"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_MatchByNamesFunc(t *testing.T) {
	rr := radixv1.RadixRegistration{ObjectMeta: v1.ObjectMeta{Name: "app1"}}

	assert.True(t, MatchByNamesFunc([]string{"app1"})(&rr))
	assert.True(t, MatchByNamesFunc([]string{"app1", "app2"})(&rr))
	assert.False(t, MatchByNamesFunc([]string{"app2"})(&rr))
	assert.False(t, MatchByNamesFunc([]string{})(&rr))
}

func Test_MatchAll(t *testing.T) {
	rr := radixv1.RadixRegistration{ObjectMeta: v1.ObjectMeta{Name: "any-app"}}
	assert.True(t, MatchAll(&rr))
}
