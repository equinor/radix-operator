package deployment

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controllerTestSuite struct {
	common.ControllerTestSuite
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) Test_Controller_Calls_Handler() {
	appName, rdName, namespace := "any-app", "any-rd", "any-ns"

	sut := NewController(context.Background(), s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory)
	s.RadixInformerFactory.Start(s.Ctx.Done())
	s.KubeInformerFactory.Start(s.Ctx.Done())
	go func() {
		err := sut.Run(s.Ctx, 5)
		s.Require().NoError(err)
	}()
	rr, err := s.RadixClient.RadixV1().RadixRegistrations().Create(context.Background(), &radixv1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Annotations: map[string]string{}}},
		metav1.CreateOptions{})
	s.Require().NoError(err)

	// Create RD should sync
	rd := &radixv1.RadixDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: rdName, Namespace: namespace, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec: radixv1.RadixDeploymentSpec{
			Components: []radixv1.RadixDeployComponent{
				{Name: "any", PublicPort: "http"},
			},
		},
	}
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rd, err = s.RadixClient.RadixV1().RadixDeployments(rd.Namespace).Create(s.Ctx, rd, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RD spec should sync.
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rd.Spec.Components[0].PublicPort = "metrics"
	rd, err = s.RadixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Update(s.Ctx, rd, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RD labels should sync.
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rd.Labels["key"] = "val"
	rd, err = s.RadixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Update(s.Ctx, rd, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RD annotations should sync.
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rd.Annotations = map[string]string{"key": "value"}
	rd, err = s.RadixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Update(s.Ctx, rd, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RD status condition should sync.
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rd.Status.Condition = radixv1.DeploymentActive
	rd, err = s.RadixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Update(s.Ctx, rd, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RD other status props should not sync.
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(0).DoAndReturn(s.SyncedChannelCallback())
	rd.Status.Message = "any"
	rd, err = s.RadixClient.RadixV1().RadixDeployments(rd.ObjectMeta.Namespace).Update(s.Ctx, rd, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called")

	// Create service should not trigger sync
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(0).DoAndReturn(s.SyncedChannelCallback())
	isController := true
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "any-svc",
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "radix.equinor.com/v1", Kind: "RadixDeployment", Name: rd.Name, Controller: &isController},
			},
		},
	}
	svc, err = s.KubeClient.CoreV1().Services(namespace).Create(s.Ctx, svc, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called")

	// Update service should sync
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	svc, err = s.KubeClient.CoreV1().Services(svc.Namespace).Update(s.Ctx, svc, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Delete service should sync
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	err = s.KubeClient.CoreV1().Services(svc.Namespace).Delete(s.Ctx, svc.Name, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Sync should trigger when annotation radix.equinor.com/preview-oauth2-proxy-mode changes on RR
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr.Annotations[annotations.PreviewOAuth2ProxyModeAnnotation] = "any"
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Sync should trigger when AdGroups changes on RR
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr.Spec.AdGroups = []string{"new-admin-group"}
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Sync should trigger when AdUsers changes on RR
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr.Spec.AdUsers = []string{"new-admin-user"}
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Sync should trigger when ReaderAdGroups changes on RR
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr.Spec.ReaderAdGroups = []string{"new-reader-group"}
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Sync should trigger when ReaderAdUsers changes on RR
	s.Handler.EXPECT().Sync(gomock.Any(), namespace, rdName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr.Spec.ReaderAdUsers = []string{"new-reader-user"}
	_, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")
}
