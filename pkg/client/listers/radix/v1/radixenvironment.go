/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// RadixEnvironmentLister helps list RadixEnvironments.
// All objects returned here must be treated as read-only.
type RadixEnvironmentLister interface {
	// List lists all RadixEnvironments in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.RadixEnvironment, err error)
	// Get retrieves the RadixEnvironment from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.RadixEnvironment, error)
	RadixEnvironmentListerExpansion
}

// radixEnvironmentLister implements the RadixEnvironmentLister interface.
type radixEnvironmentLister struct {
	indexer cache.Indexer
}

// NewRadixEnvironmentLister returns a new RadixEnvironmentLister.
func NewRadixEnvironmentLister(indexer cache.Indexer) RadixEnvironmentLister {
	return &radixEnvironmentLister{indexer: indexer}
}

// List lists all RadixEnvironments in the indexer.
func (s *radixEnvironmentLister) List(selector labels.Selector) (ret []*v1.RadixEnvironment, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.RadixEnvironment))
	})
	return ret, err
}

// Get retrieves the RadixEnvironment from the index for a given name.
func (s *radixEnvironmentLister) Get(name string) (*v1.RadixEnvironment, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("radixenvironment"), name)
	}
	return obj.(*v1.RadixEnvironment), nil
}
