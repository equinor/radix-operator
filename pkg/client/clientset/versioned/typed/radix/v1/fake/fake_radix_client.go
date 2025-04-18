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
	v1 "github.com/equinor/radix-operator/pkg/client/clientset/versioned/typed/radix/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeRadixV1 struct {
	*testing.Fake
}

func (c *FakeRadixV1) RadixAlerts(namespace string) v1.RadixAlertInterface {
	return newFakeRadixAlerts(c, namespace)
}

func (c *FakeRadixV1) RadixApplications(namespace string) v1.RadixApplicationInterface {
	return newFakeRadixApplications(c, namespace)
}

func (c *FakeRadixV1) RadixBatches(namespace string) v1.RadixBatchInterface {
	return newFakeRadixBatches(c, namespace)
}

func (c *FakeRadixV1) RadixDNSAliases() v1.RadixDNSAliasInterface {
	return newFakeRadixDNSAliases(c)
}

func (c *FakeRadixV1) RadixDeployments(namespace string) v1.RadixDeploymentInterface {
	return newFakeRadixDeployments(c, namespace)
}

func (c *FakeRadixV1) RadixEnvironments() v1.RadixEnvironmentInterface {
	return newFakeRadixEnvironments(c)
}

func (c *FakeRadixV1) RadixJobs(namespace string) v1.RadixJobInterface {
	return newFakeRadixJobs(c, namespace)
}

func (c *FakeRadixV1) RadixRegistrations() v1.RadixRegistrationInterface {
	return newFakeRadixRegistrations(c)
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeRadixV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
