// Package uncachedclient provides a controller-runtime client that reads directly from the API server
package uncachedclient

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New returns a client that uses reader for Get and List, and c for everything else.
// Use it with a manager's APIReader for objects that are not in the manager cache,
// for example with controllerutil.CreateOrUpdate, which reads the object before writing it.
func New(c client.Client, reader client.Reader) client.Client {
	return &uncachedClient{Client: c, reader: reader}
}

type uncachedClient struct {
	client.Client
	reader client.Reader
}

func (c *uncachedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.reader.Get(ctx, key, obj, opts...)
}

func (c *uncachedClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.reader.List(ctx, list, opts...)
}
