/*
MIT License

Copyright (c) 2024 Equinor

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// RadixBatchLister helps list RadixBatches.
// All objects returned here must be treated as read-only.
type RadixBatchLister interface {
	// List lists all RadixBatches in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.RadixBatch, err error)
	// RadixBatches returns an object that can list and get RadixBatches.
	RadixBatches(namespace string) RadixBatchNamespaceLister
	RadixBatchListerExpansion
}

// radixBatchLister implements the RadixBatchLister interface.
type radixBatchLister struct {
	indexer cache.Indexer
}

// NewRadixBatchLister returns a new RadixBatchLister.
func NewRadixBatchLister(indexer cache.Indexer) RadixBatchLister {
	return &radixBatchLister{indexer: indexer}
}

// List lists all RadixBatches in the indexer.
func (s *radixBatchLister) List(selector labels.Selector) (ret []*v1.RadixBatch, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.RadixBatch))
	})
	return ret, err
}

// RadixBatches returns an object that can list and get RadixBatches.
func (s *radixBatchLister) RadixBatches(namespace string) RadixBatchNamespaceLister {
	return radixBatchNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// RadixBatchNamespaceLister helps list and get RadixBatches.
// All objects returned here must be treated as read-only.
type RadixBatchNamespaceLister interface {
	// List lists all RadixBatches in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.RadixBatch, err error)
	// Get retrieves the RadixBatch from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.RadixBatch, error)
	RadixBatchNamespaceListerExpansion
}

// radixBatchNamespaceLister implements the RadixBatchNamespaceLister
// interface.
type radixBatchNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all RadixBatches in the indexer for a given namespace.
func (s radixBatchNamespaceLister) List(selector labels.Selector) (ret []*v1.RadixBatch, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.RadixBatch))
	})
	return ret, err
}

// Get retrieves the RadixBatch from the indexer for a given namespace and name.
func (s radixBatchNamespaceLister) Get(name string) (*v1.RadixBatch, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("radixbatch"), name)
	}
	return obj.(*v1.RadixBatch), nil
}
