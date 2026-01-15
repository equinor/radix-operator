package applicationconfig

import (
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(radixv1.AddToScheme(scheme))
}
