package registration

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/operator/common"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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
	appName := "any-app"

	sut := NewController(context.Background(), s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory)
	s.RadixInformerFactory.Start(s.Ctx.Done())
	s.KubeInformerFactory.Start(s.Ctx.Done())
	go func() {
		err := sut.Run(s.Ctx, 5)
		s.Require().NoError(err)
	}()

	// Create RR should sync
	rr := &radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr, err := s.RadixClient.RadixV1().RadixRegistrations().Create(s.Ctx, rr, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RR spec should sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr.Spec.ConfigBranch = "any-branch"
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(s.Ctx, rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RR labels should sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr.Labels = map[string]string{"key": "value"}
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(s.Ctx, rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RR annotations should sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	rr.Annotations = map[string]string{"key": "value"}
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(s.Ctx, rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Update RR other props should not sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(0).DoAndReturn(s.SyncedChannelCallback())
	rr.Status.Message = "any-msg"
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(s.Ctx, rr, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called")

	// Create namespace should not sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(0).DoAndReturn(s.SyncedChannelCallback())
	isController := true
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "any-ns",
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "radix.equinor.com/v1", Kind: "RadixRegistration", Name: rr.Name, Controller: &isController},
			},
		},
	}
	ns, err = s.KubeClient.CoreV1().Namespaces().Create(s.Ctx, ns, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called")

	// Add Git deploy secret should not sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(0).DoAndReturn(s.SyncedChannelCallback())
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: defaults.GitPrivateKeySecretName, Namespace: ns.Name}}
	secret, err = s.KubeClient.CoreV1().Secrets(secret.Namespace).Create(s.Ctx, secret, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called")

	// Update Git deploy secret should sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	secret, err = s.KubeClient.CoreV1().Secrets(secret.Namespace).Update(s.Ctx, secret, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Delete Git deploy secret should sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	err = s.KubeClient.CoreV1().Secrets(secret.Namespace).Delete(s.Ctx, secret.Name, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")

	// Test arbitrary secret should not sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(0).DoAndReturn(s.SyncedChannelCallback())
	arbitrarySecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "any-secret", Namespace: ns.Name}}
	arbitrarySecret, err = s.KubeClient.CoreV1().Secrets(arbitrarySecret.Namespace).Create(s.Ctx, arbitrarySecret, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called")

	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(0).DoAndReturn(s.SyncedChannelCallback())
	arbitrarySecret, err = s.KubeClient.CoreV1().Secrets(arbitrarySecret.Namespace).Update(s.Ctx, arbitrarySecret, metav1.UpdateOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called")

	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(0).DoAndReturn(s.SyncedChannelCallback())
	err = s.KubeClient.CoreV1().Secrets(arbitrarySecret.Namespace).Delete(s.Ctx, arbitrarySecret.Name, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForNotSynced("Sync should not be called")

	// Delete namespace should sync
	s.Handler.EXPECT().Sync(gomock.Any(), "", appName).Times(1).DoAndReturn(s.SyncedChannelCallback())
	err = s.KubeClient.CoreV1().Namespaces().Delete(s.Ctx, ns.Name, metav1.DeleteOptions{})
	s.Require().NoError(err)
	s.WaitForSynced("Sync should be called")
}
