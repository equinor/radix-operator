package application

import (
	"context"
	"testing"

	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controllerTestSuite struct {
	common.ControllerTestSuite
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) Test_Controller_Calls_Handler() {
	appName := "any-app"
	namespace := utils.GetAppNamespace(appName)
	appNamespace := test.CreateAppNamespace(s.KubeClient, appName)

	sut := NewController(s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, false, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Stop)
	s.KubeInformerFactory.Start(s.Stop)

	go func() {
		err := sut.Run(1, s.Stop)
		s.Require().NoError(err)
	}()

	ra := utils.ARadixApplication().WithAppName(appName).WithEnvironment("dev", "master").BuildRA()
	_, err := s.RadixClient.RadixV1().RadixApplications(appNamespace).Create(context.TODO(), ra, metav1.CreateOptions{})
	s.Require().NoError(err)

	s.Handler.EXPECT().Sync(namespace, appName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	s.WaitForSynced("added app")
}

func (s *controllerTestSuite) Test_Controller_Calls_Handler_On_Admin_Or_Reader_Change() {
	appName := "any-app"
	namespace := utils.GetAppNamespace(appName)
	appNamespace := test.CreateAppNamespace(s.KubeClient, appName)
	rr := &v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}, Spec: v1.RadixRegistrationSpec{AdGroups: []string{"first-admin"}, ReaderAdGroups: []string{"first-reader-group"}}}
	rr, err := s.RadixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})
	if err != nil {
		s.Require().NoError(err)
	}

	sut := NewController(s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, false, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Stop)
	s.KubeInformerFactory.Start(s.Stop)

	go func() {
		err := sut.Run(1, s.Stop)
		s.Require().NoError(err)
	}()

	ra := utils.ARadixApplication().WithAppName(appName).WithEnvironment("dev", "master").BuildRA()
	_, err = s.RadixClient.RadixV1().RadixApplications(appNamespace).Create(context.TODO(), ra, metav1.CreateOptions{})
	s.Require().NoError(err)

	s.Handler.EXPECT().Sync(namespace, appName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	s.WaitForSynced("added app")

	rr.Spec.AdGroups = []string{"another-admin-group"}
	_, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)

	s.Handler.EXPECT().Sync(namespace, appName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	s.WaitForSynced("AdGroups changed")

	rr.Spec.ReaderAdGroups = []string{"another-reader-group"}
	_, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)

	s.Handler.EXPECT().Sync(namespace, appName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	s.WaitForSynced("ReaderAdGroups changed")
}
