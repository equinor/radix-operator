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
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// RadixBatchLister helps list RadixBatches.
// All objects returned here must be treated as read-only.
type RadixBatchLister interface {
	// List lists all RadixBatches in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*radixv1.RadixBatch, err error)
	// RadixBatches returns an object that can list and get RadixBatches.
	RadixBatches(namespace string) RadixBatchNamespaceLister
	RadixBatchListerExpansion
}

// radixBatchLister implements the RadixBatchLister interface.
type radixBatchLister struct {
	listers.ResourceIndexer[*radixv1.RadixBatch]
}

// NewRadixBatchLister returns a new RadixBatchLister.
func NewRadixBatchLister(indexer cache.Indexer) RadixBatchLister {
	return &radixBatchLister{listers.New[*radixv1.RadixBatch](indexer, radixv1.Resource("radixbatch"))}
}

// RadixBatches returns an object that can list and get RadixBatches.
func (s *radixBatchLister) RadixBatches(namespace string) RadixBatchNamespaceLister {
	return radixBatchNamespaceLister{listers.NewNamespaced[*radixv1.RadixBatch](s.ResourceIndexer, namespace)}
}

// RadixBatchNamespaceLister helps list and get RadixBatches.
// All objects returned here must be treated as read-only.
type RadixBatchNamespaceLister interface {
	// List lists all RadixBatches in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*radixv1.RadixBatch, err error)
	// Get retrieves the RadixBatch from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*radixv1.RadixBatch, error)
	RadixBatchNamespaceListerExpansion
}

// radixBatchNamespaceLister implements the RadixBatchNamespaceLister
// interface.
type radixBatchNamespaceLister struct {
	listers.ResourceIndexer[*radixv1.RadixBatch]
}
