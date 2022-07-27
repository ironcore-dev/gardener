// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver (interfaces: Interface)

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "k8s.io/apimachinery/pkg/types"

	config "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	vpnseedserver "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
)

// MockInterface is a mock of Interface interface.
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
}

// MockInterfaceMockRecorder is the mock recorder for MockInterface.
type MockInterfaceMockRecorder struct {
	mock *MockInterface
}

// NewMockInterface creates a new mock instance.
func NewMockInterface(ctrl *gomock.Controller) *MockInterface {
	mock := &MockInterface{ctrl: ctrl}
	mock.recorder = &MockInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInterface) EXPECT() *MockInterfaceMockRecorder {
	return m.recorder
}

// AlertingRules mocks base method.
func (m *MockInterface) AlertingRules() (map[string]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AlertingRules")
	ret0, _ := ret[0].(map[string]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AlertingRules indicates an expected call of AlertingRules.
func (mr *MockInterfaceMockRecorder) AlertingRules() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AlertingRules", reflect.TypeOf((*MockInterface)(nil).AlertingRules))
}

// Deploy mocks base method.
func (m *MockInterface) Deploy(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockInterfaceMockRecorder) Deploy(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deploy", reflect.TypeOf((*MockInterface)(nil).Deploy), arg0)
}

// Destroy mocks base method.
func (m *MockInterface) Destroy(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Destroy", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Destroy indicates an expected call of Destroy.
func (mr *MockInterfaceMockRecorder) Destroy(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destroy", reflect.TypeOf((*MockInterface)(nil).Destroy), arg0)
}

// ScrapeConfigs mocks base method.
func (m *MockInterface) ScrapeConfigs() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ScrapeConfigs")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ScrapeConfigs indicates an expected call of ScrapeConfigs.
func (mr *MockInterfaceMockRecorder) ScrapeConfigs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScrapeConfigs", reflect.TypeOf((*MockInterface)(nil).ScrapeConfigs))
}

// SetExposureClassHandlerName mocks base method.
func (m *MockInterface) SetExposureClassHandlerName(arg0 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetExposureClassHandlerName", arg0)
}

// SetExposureClassHandlerName indicates an expected call of SetExposureClassHandlerName.
func (mr *MockInterfaceMockRecorder) SetExposureClassHandlerName(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetExposureClassHandlerName", reflect.TypeOf((*MockInterface)(nil).SetExposureClassHandlerName), arg0)
}

// SetSNIConfig mocks base method.
func (m *MockInterface) SetSNIConfig(arg0 *config.SNI) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetSNIConfig", arg0)
}

// SetSNIConfig indicates an expected call of SetSNIConfig.
func (mr *MockInterfaceMockRecorder) SetSNIConfig(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSNIConfig", reflect.TypeOf((*MockInterface)(nil).SetSNIConfig), arg0)
}

// SetSecrets mocks base method.
func (m *MockInterface) SetSecrets(arg0 vpnseedserver.Secrets) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetSecrets", arg0)
}

// SetSecrets indicates an expected call of SetSecrets.
func (mr *MockInterfaceMockRecorder) SetSecrets(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSecrets", reflect.TypeOf((*MockInterface)(nil).SetSecrets), arg0)
}

// SetSeedNamespaceObjectUID mocks base method.
func (m *MockInterface) SetSeedNamespaceObjectUID(arg0 types.UID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetSeedNamespaceObjectUID", arg0)
}

// SetSeedNamespaceObjectUID indicates an expected call of SetSeedNamespaceObjectUID.
func (mr *MockInterfaceMockRecorder) SetSeedNamespaceObjectUID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSeedNamespaceObjectUID", reflect.TypeOf((*MockInterface)(nil).SetSeedNamespaceObjectUID), arg0)
}

// Wait mocks base method.
func (m *MockInterface) Wait(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Wait", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Wait indicates an expected call of Wait.
func (mr *MockInterfaceMockRecorder) Wait(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*MockInterface)(nil).Wait), arg0)
}

// WaitCleanup mocks base method.
func (m *MockInterface) WaitCleanup(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitCleanup", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitCleanup indicates an expected call of WaitCleanup.
func (mr *MockInterfaceMockRecorder) WaitCleanup(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitCleanup", reflect.TypeOf((*MockInterface)(nil).WaitCleanup), arg0)
}
