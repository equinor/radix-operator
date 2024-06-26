// Code generated by MockGen. DO NOT EDIT.
// Source: ./pkg/apis/deployment/deployment.go

// Package deployment is a generated GoMock package.
package deployment

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockDeploymentSyncer is a mock of DeploymentSyncer interface.
type MockDeploymentSyncer struct {
	ctrl     *gomock.Controller
	recorder *MockDeploymentSyncerMockRecorder
}

// MockDeploymentSyncerMockRecorder is the mock recorder for MockDeploymentSyncer.
type MockDeploymentSyncerMockRecorder struct {
	mock *MockDeploymentSyncer
}

// NewMockDeploymentSyncer creates a new mock instance.
func NewMockDeploymentSyncer(ctrl *gomock.Controller) *MockDeploymentSyncer {
	mock := &MockDeploymentSyncer{ctrl: ctrl}
	mock.recorder = &MockDeploymentSyncerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDeploymentSyncer) EXPECT() *MockDeploymentSyncerMockRecorder {
	return m.recorder
}

// OnSync mocks base method.
func (m *MockDeploymentSyncer) OnSync(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OnSync", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// OnSync indicates an expected call of OnSync.
func (mr *MockDeploymentSyncerMockRecorder) OnSync(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OnSync", reflect.TypeOf((*MockDeploymentSyncer)(nil).OnSync), ctx)
}
