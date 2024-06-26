// Code generated by MockGen. DO NOT EDIT.
// Source: ./pkg/apis/deployment/auxiliaryresourcemanager.go

// Package deployment is a generated GoMock package.
package deployment

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockAuxiliaryResourceManager is a mock of AuxiliaryResourceManager interface.
type MockAuxiliaryResourceManager struct {
	ctrl     *gomock.Controller
	recorder *MockAuxiliaryResourceManagerMockRecorder
}

// MockAuxiliaryResourceManagerMockRecorder is the mock recorder for MockAuxiliaryResourceManager.
type MockAuxiliaryResourceManagerMockRecorder struct {
	mock *MockAuxiliaryResourceManager
}

// NewMockAuxiliaryResourceManager creates a new mock instance.
func NewMockAuxiliaryResourceManager(ctrl *gomock.Controller) *MockAuxiliaryResourceManager {
	mock := &MockAuxiliaryResourceManager{ctrl: ctrl}
	mock.recorder = &MockAuxiliaryResourceManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAuxiliaryResourceManager) EXPECT() *MockAuxiliaryResourceManagerMockRecorder {
	return m.recorder
}

// GarbageCollect mocks base method.
func (m *MockAuxiliaryResourceManager) GarbageCollect(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GarbageCollect", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// GarbageCollect indicates an expected call of GarbageCollect.
func (mr *MockAuxiliaryResourceManagerMockRecorder) GarbageCollect(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GarbageCollect", reflect.TypeOf((*MockAuxiliaryResourceManager)(nil).GarbageCollect), ctx)
}

// Sync mocks base method.
func (m *MockAuxiliaryResourceManager) Sync(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sync", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Sync indicates an expected call of Sync.
func (mr *MockAuxiliaryResourceManagerMockRecorder) Sync(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sync", reflect.TypeOf((*MockAuxiliaryResourceManager)(nil).Sync), ctx)
}
