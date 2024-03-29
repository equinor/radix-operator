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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	scheme "github.com/equinor/radix-operator/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// RadixEnvironmentsGetter has a method to return a RadixEnvironmentInterface.
// A group's client should implement this interface.
type RadixEnvironmentsGetter interface {
	RadixEnvironments() RadixEnvironmentInterface
}

// RadixEnvironmentInterface has methods to work with RadixEnvironment resources.
type RadixEnvironmentInterface interface {
	Create(ctx context.Context, radixEnvironment *v1.RadixEnvironment, opts metav1.CreateOptions) (*v1.RadixEnvironment, error)
	Update(ctx context.Context, radixEnvironment *v1.RadixEnvironment, opts metav1.UpdateOptions) (*v1.RadixEnvironment, error)
	UpdateStatus(ctx context.Context, radixEnvironment *v1.RadixEnvironment, opts metav1.UpdateOptions) (*v1.RadixEnvironment, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.RadixEnvironment, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.RadixEnvironmentList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.RadixEnvironment, err error)
	RadixEnvironmentExpansion
}

// radixEnvironments implements RadixEnvironmentInterface
type radixEnvironments struct {
	client rest.Interface
}

// newRadixEnvironments returns a RadixEnvironments
func newRadixEnvironments(c *RadixV1Client) *radixEnvironments {
	return &radixEnvironments{
		client: c.RESTClient(),
	}
}

// Get takes name of the radixEnvironment, and returns the corresponding radixEnvironment object, and an error if there is any.
func (c *radixEnvironments) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.RadixEnvironment, err error) {
	result = &v1.RadixEnvironment{}
	err = c.client.Get().
		Resource("radixenvironments").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of RadixEnvironments that match those selectors.
func (c *radixEnvironments) List(ctx context.Context, opts metav1.ListOptions) (result *v1.RadixEnvironmentList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.RadixEnvironmentList{}
	err = c.client.Get().
		Resource("radixenvironments").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested radixEnvironments.
func (c *radixEnvironments) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("radixenvironments").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a radixEnvironment and creates it.  Returns the server's representation of the radixEnvironment, and an error, if there is any.
func (c *radixEnvironments) Create(ctx context.Context, radixEnvironment *v1.RadixEnvironment, opts metav1.CreateOptions) (result *v1.RadixEnvironment, err error) {
	result = &v1.RadixEnvironment{}
	err = c.client.Post().
		Resource("radixenvironments").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(radixEnvironment).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a radixEnvironment and updates it. Returns the server's representation of the radixEnvironment, and an error, if there is any.
func (c *radixEnvironments) Update(ctx context.Context, radixEnvironment *v1.RadixEnvironment, opts metav1.UpdateOptions) (result *v1.RadixEnvironment, err error) {
	result = &v1.RadixEnvironment{}
	err = c.client.Put().
		Resource("radixenvironments").
		Name(radixEnvironment.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(radixEnvironment).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *radixEnvironments) UpdateStatus(ctx context.Context, radixEnvironment *v1.RadixEnvironment, opts metav1.UpdateOptions) (result *v1.RadixEnvironment, err error) {
	result = &v1.RadixEnvironment{}
	err = c.client.Put().
		Resource("radixenvironments").
		Name(radixEnvironment.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(radixEnvironment).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the radixEnvironment and deletes it. Returns an error if one occurs.
func (c *radixEnvironments) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("radixenvironments").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *radixEnvironments) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("radixenvironments").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched radixEnvironment.
func (c *radixEnvironments) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.RadixEnvironment, err error) {
	result = &v1.RadixEnvironment{}
	err = c.client.Patch(pt).
		Resource("radixenvironments").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
