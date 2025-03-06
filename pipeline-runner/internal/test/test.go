package test

import (
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-tekton/pkg/models/env"
	"github.com/golang/mock/gomock"
)

// MockEnv Mocks the env.Env interface
func MockEnv(mockCtrl *gomock.Controller, appName string) *env.MockEnv {
	mockEnv := env.NewMockEnv(mockCtrl)
	mockEnv.EXPECT().GetAppName().Return(appName).AnyTimes()
	mockEnv.EXPECT().GetRadixImageTag().Return(RadixImageTag).AnyTimes()
	mockEnv.EXPECT().GetRadixPipelineJobName().Return(RadixPipelineJobName).AnyTimes()
	mockEnv.EXPECT().GetBranch().Return(BranchMain).AnyTimes()
	mockEnv.EXPECT().GetAppNamespace().Return(utils.GetAppNamespace(AppName)).AnyTimes()
	mockEnv.EXPECT().GetRadixPipelineType().Return(v1.Deploy).AnyTimes()
	mockEnv.EXPECT().GetRadixConfigMapName().Return(RadixConfigMapName).AnyTimes()
	mockEnv.EXPECT().GetRadixDeployToEnvironment().Return(Env1).AnyTimes()
	mockEnv.EXPECT().GetDNSConfig().Return(&dnsalias.DNSConfig{}).AnyTimes()
	mockEnv.EXPECT().GetRadixConfigBranch().Return(Env1).AnyTimes()
	return mockEnv
}
