// Code generated by MockGen. DO NOT EDIT.
// Source: ./pipeline-runner/internal/wait/job.go

// Package wait is a generated GoMock package.
package wait

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	v1 "k8s.io/api/batch/v1"
)

// MockJobCompletionWaiter is a mock of JobCompletionWaiter interface.
type MockJobCompletionWaiter struct {
	ctrl     *gomock.Controller
	recorder *MockJobCompletionWaiterMockRecorder
}

// MockJobCompletionWaiterMockRecorder is the mock recorder for MockJobCompletionWaiter.
type MockJobCompletionWaiterMockRecorder struct {
	mock *MockJobCompletionWaiter
}

// NewMockJobCompletionWaiter creates a new mock instance.
func NewMockJobCompletionWaiter(ctrl *gomock.Controller) *MockJobCompletionWaiter {
	mock := &MockJobCompletionWaiter{ctrl: ctrl}
	mock.recorder = &MockJobCompletionWaiterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockJobCompletionWaiter) EXPECT() *MockJobCompletionWaiterMockRecorder {
	return m.recorder
}

// Wait mocks base method.
func (m *MockJobCompletionWaiter) Wait(job *v1.Job) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", job)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockJobCompletionWaiterMockRecorder) Wait(job interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockJobCompletionWaiter)(nil).Wait), job)
}