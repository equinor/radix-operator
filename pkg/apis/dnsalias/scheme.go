package dnsalias

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

var scheme = runtime.NewScheme()

func init() {
	radixv1.AddToScheme(scheme)
}
