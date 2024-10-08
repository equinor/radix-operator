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
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeRadixApplications implements RadixApplicationInterface
type FakeRadixApplications struct {
	Fake *FakeRadixV1
	ns   string
}

var radixapplicationsResource = v1.SchemeGroupVersion.WithResource("radixapplications")

var radixapplicationsKind = v1.SchemeGroupVersion.WithKind("RadixApplication")

// Get takes name of the radixApplication, and returns the corresponding radixApplication object, and an error if there is any.
func (c *FakeRadixApplications) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.RadixApplication, err error) {
	emptyResult := &v1.RadixApplication{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(radixapplicationsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.RadixApplication), err
}

// List takes label and field selectors, and returns the list of RadixApplications that match those selectors.
func (c *FakeRadixApplications) List(ctx context.Context, opts metav1.ListOptions) (result *v1.RadixApplicationList, err error) {
	emptyResult := &v1.RadixApplicationList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(radixapplicationsResource, radixapplicationsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.RadixApplicationList{ListMeta: obj.(*v1.RadixApplicationList).ListMeta}
	for _, item := range obj.(*v1.RadixApplicationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested radixApplications.
func (c *FakeRadixApplications) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(radixapplicationsResource, c.ns, opts))

}

// Create takes the representation of a radixApplication and creates it.  Returns the server's representation of the radixApplication, and an error, if there is any.
func (c *FakeRadixApplications) Create(ctx context.Context, radixApplication *v1.RadixApplication, opts metav1.CreateOptions) (result *v1.RadixApplication, err error) {
	emptyResult := &v1.RadixApplication{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(radixapplicationsResource, c.ns, radixApplication, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.RadixApplication), err
}

// Update takes the representation of a radixApplication and updates it. Returns the server's representation of the radixApplication, and an error, if there is any.
func (c *FakeRadixApplications) Update(ctx context.Context, radixApplication *v1.RadixApplication, opts metav1.UpdateOptions) (result *v1.RadixApplication, err error) {
	emptyResult := &v1.RadixApplication{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(radixapplicationsResource, c.ns, radixApplication, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.RadixApplication), err
}

// Delete takes name of the radixApplication and deletes it. Returns an error if one occurs.
func (c *FakeRadixApplications) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(radixapplicationsResource, c.ns, name, opts), &v1.RadixApplication{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeRadixApplications) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(radixapplicationsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1.RadixApplicationList{})
	return err
}

// Patch applies the patch and returns the patched radixApplication.
func (c *FakeRadixApplications) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.RadixApplication, err error) {
	emptyResult := &v1.RadixApplication{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(radixapplicationsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.RadixApplication), err
}
