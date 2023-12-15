package alert

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type controllerTestSuite struct {
	common.ControllerTestSuite
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) Test_RadixAlertEvents() {
	alertName, namespace := "any-alert", "any-ns"
	alert := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alertName}}

	sut := NewController(s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, false, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Stop)
	s.KubeInformerFactory.Start(s.Stop)
	go func() {
		_ = sut.Run(5, s.Stop)
	}()

	// Adding a RadixAlert should trigger sync
	s.Handler.EXPECT().Sync(namespace, alertName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alert, _ = s.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert, metav1.CreateOptions{})
	s.WaitForSynced("first call")

	// Updating the RadixAlert with changes should trigger a sync
	s.Handler.EXPECT().Sync(namespace, alertName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	alert.Labels = map[string]string{"foo": "bar"}
	_, err := s.RadixClient.RadixV1().RadixAlerts(namespace).Update(context.TODO(), alert, metav1.UpdateOptions{})
	require.NoError(s.T(), err)
	s.WaitForSynced("second call")

	// Updating the RadixAlert with no changes should not trigger a sync
	s.Handler.EXPECT().Sync(namespace, alertName, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	_, err = s.RadixClient.RadixV1().RadixAlerts(namespace).Update(context.TODO(), alert, metav1.UpdateOptions{})
	require.NoError(s.T(), err)
	s.WaitForNotSynced("Sync should not be called when updating RadixAlert with no changes")
}

func (s *controllerTestSuite) Test_RadixRegistrationEvents() {
	alert1Name, alert2Name, namespace, appName := "alert1", "alert2", "any-ns", "any-app"
	alert1 := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alert1Name, Labels: map[string]string{kube.RadixAppLabel: appName}}}
	alert2 := &v1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: alert2Name}}
	rr := &v1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}, Spec: v1.RadixRegistrationSpec{Owner: "first-owner", AdGroups: []string{"first-admin-group"}, ReaderAdGroups: []string{"first-reader-group"}}}
	rr, err := s.RadixClient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})
	require.NoError(s.T(), err)

	sut := NewController(s.KubeClient, s.RadixClient, s.Handler, s.KubeInformerFactory, s.RadixInformerFactory, false, s.EventRecorder)
	s.RadixInformerFactory.Start(s.Stop)
	s.KubeInformerFactory.Start(s.Stop)
	go func() {
		_ = sut.Run(5, s.Stop)
	}()

	hasSynced := cache.WaitForCacheSync(s.Stop, s.RadixInformerFactory.Radix().V1().RadixRegistrations().Informer().HasSynced)
	s.True(hasSynced)

	// Initial Sync for the two alerts
	_, err = s.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert1, metav1.CreateOptions{})
	require.NoError(s.T(), err)
	s.Handler.EXPECT().Sync(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	s.WaitForSynced("sync of alert1")

	_, err = s.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), alert2, metav1.CreateOptions{})
	require.NoError(s.T(), err)
	s.Handler.EXPECT().Sync(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	s.WaitForSynced("initial sync of alert2")

	// Update adGroups should trigger sync of alert1
	rr.Spec.AdGroups = []string{"another-admin-group"}
	rr.ResourceVersion = "2"
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	require.NoError(s.T(), err)
	s.Handler.EXPECT().Sync(namespace, alert1Name, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	s.WaitForSynced("sync on adGroups update")

	// Update adGroups should trigger sync of alert1
	rr.Spec.ReaderAdGroups = []string{"another-reader-group"}
	rr.ResourceVersion = "3"
	rr, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	require.NoError(s.T(), err)
	s.Handler.EXPECT().Sync(namespace, alert1Name, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(1)
	s.WaitForSynced("sync on ReaderAdGroups update")

	// Update other props on RR should not trigger sync of alert1
	rr.Spec.Owner = "owner"
	rr.ResourceVersion = "4"
	_, err = s.RadixClient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	require.NoError(s.T(), err)
	s.Handler.EXPECT().Sync(namespace, alert1Name, s.EventRecorder).DoAndReturn(s.SyncedChannelCallback()).Times(0)
	s.WaitForNotSynced("Sync should not be called when updating other RR props")
}
