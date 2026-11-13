// Package dextest provides an in-memory fake of the Dex gRPC API for tests
package dextest

import (
	"context"
	"errors"
	"slices"
	"sync"

	api "github.com/dexidp/dex/api/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// FakeDex stores clients in memory. Methods that are not implemented panic.
type FakeDex struct {
	api.DexClient

	mu      sync.Mutex
	clients map[string]*api.Client
	err     error
	calls   map[string]int
}

func NewFakeDex(clients ...*api.Client) *FakeDex {
	f := &FakeDex{clients: map[string]*api.Client{}, calls: map[string]int{}}
	for _, c := range clients {
		f.clients[c.Id] = proto.CloneOf(c)
	}
	return f
}

// SetError makes all methods return err. Set to nil to return to normal behaviour.
func (f *FakeDex) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Calls returns the number of calls to method
func (f *FakeDex) Calls(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method]
}

// Client returns a copy of the stored client, or nil if it doesn't exist
func (f *FakeDex) Client(id string) *api.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.clients[id]; ok {
		return proto.CloneOf(c)
	}
	return nil
}

// call records the call and returns the configured error. Must be called with f.mu held.
func (f *FakeDex) call(method string) error {
	f.calls[method]++
	return f.err
}

func (f *FakeDex) GetClient(_ context.Context, in *api.GetClientReq, _ ...grpc.CallOption) (*api.GetClientResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.call("GetClient"); err != nil {
		return nil, err
	}
	c, ok := f.clients[in.Id]
	if !ok {
		// Same as Dex, which returns the storage error as is
		return nil, status.Error(codes.Unknown, "not found")
	}
	return &api.GetClientResp{Client: proto.CloneOf(c)}, nil
}

func (f *FakeDex) CreateClient(_ context.Context, in *api.CreateClientReq, _ ...grpc.CallOption) (*api.CreateClientResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.call("CreateClient"); err != nil {
		return nil, err
	}
	if in.Client == nil || in.Client.Id == "" {
		return nil, errors.New("invalid client")
	}
	if _, ok := f.clients[in.Client.Id]; ok {
		return &api.CreateClientResp{AlreadyExists: true}, nil
	}
	f.clients[in.Client.Id] = proto.CloneOf(in.Client)
	return &api.CreateClientResp{Client: proto.CloneOf(in.Client)}, nil
}

func (f *FakeDex) UpdateClient(_ context.Context, in *api.UpdateClientReq, _ ...grpc.CallOption) (*api.UpdateClientResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.call("UpdateClient"); err != nil {
		return nil, err
	}
	c, ok := f.clients[in.Id]
	if !ok {
		return &api.UpdateClientResp{NotFound: true}, nil
	}
	if in.RedirectUris != nil {
		c.RedirectUris = slices.Clone(in.RedirectUris)
	}
	if in.Name != "" {
		c.Name = in.Name
	}
	return &api.UpdateClientResp{}, nil
}

func (f *FakeDex) DeleteClient(_ context.Context, in *api.DeleteClientReq, _ ...grpc.CallOption) (*api.DeleteClientResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.call("DeleteClient"); err != nil {
		return nil, err
	}
	if _, ok := f.clients[in.Id]; !ok {
		return &api.DeleteClientResp{NotFound: true}, nil
	}
	delete(f.clients, in.Id)
	return &api.DeleteClientResp{}, nil
}

func (f *FakeDex) ListClients(_ context.Context, _ *api.ListClientReq, _ ...grpc.CallOption) (*api.ListClientResp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.call("ListClients"); err != nil {
		return nil, err
	}
	resp := &api.ListClientResp{}
	for _, c := range f.clients {
		resp.Clients = append(resp.Clients, &api.ClientInfo{Id: c.Id, Name: c.Name, RedirectUris: slices.Clone(c.RedirectUris)})
	}
	return resp, nil
}

var _ api.DexClient = &FakeDex{}
