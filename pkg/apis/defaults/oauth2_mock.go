// Code generated by MockGen. DO NOT EDIT.
// Source: ./pkg/apis/defaults/oauth2.go

// Package defaults is a generated GoMock package.
package defaults

import (
	reflect "reflect"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	gomock "github.com/golang/mock/gomock"
)

// MockOAuth2Config is a mock of OAuth2Config interface.
type MockOAuth2Config struct {
	ctrl     *gomock.Controller
	recorder *MockOAuth2ConfigMockRecorder
}

// MockOAuth2ConfigMockRecorder is the mock recorder for MockOAuth2Config.
type MockOAuth2ConfigMockRecorder struct {
	mock *MockOAuth2Config
}

// NewMockOAuth2Config creates a new mock instance.
func NewMockOAuth2Config(ctrl *gomock.Controller) *MockOAuth2Config {
	mock := &MockOAuth2Config{ctrl: ctrl}
	mock.recorder = &MockOAuth2ConfigMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOAuth2Config) EXPECT() *MockOAuth2ConfigMockRecorder {
	return m.recorder
}

// MergeWith mocks base method.
func (m *MockOAuth2Config) MergeWith(source *v1.OAuth2) (*v1.OAuth2, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MergeWith", source)
	ret0, _ := ret[0].(*v1.OAuth2)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MergeWith indicates an expected call of MergeWith.
func (mr *MockOAuth2ConfigMockRecorder) MergeWith(source interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MergeWith", reflect.TypeOf((*MockOAuth2Config)(nil).MergeWith), source)
}
