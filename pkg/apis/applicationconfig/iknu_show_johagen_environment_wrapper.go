package applicationconfig

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	v1Interface "github.com/equinor/radix-operator/pkg/client/clientset/versioned/typed/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO : IKNU : a way to wrap RadixV1().RadixEnvironments()

// EnvironmentWrapper wrapps v1.RadixEnvironmentInterface
type EnvironmentWrapper interface {
	Get(name string) (*v1.RadixEnvironment, error)
}

// EnvironmentWrapperImpl Implementation of EnvironmentWrapper
type EnvironmentWrapperImpl struct {
	environmentInterface v1Interface.RadixEnvironmentInterface
}

// NewEnvironmentWrapperImpl Constructor
func NewEnvironmentWrapperImpl(radixclient radixclient.Interface) EnvironmentWrapper {
	return &EnvironmentWrapperImpl{
		environmentInterface: radixclient.RadixV1().RadixEnvironments(),
	}
}

// Get Implementation
func (impl *EnvironmentWrapperImpl) Get(name string) (*v1.RadixEnvironment, error) {
	return impl.environmentInterface.Get(name, metav1.GetOptions{})
}
