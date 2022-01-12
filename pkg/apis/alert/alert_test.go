package alert

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type testAlertSyncerConfigOption func(sut *alertSyncer)

func testAlertSyncerWithSlackMessageTemplate(template slackMessageTemplate) testAlertSyncerConfigOption {
	return func(sut *alertSyncer) {
		sut.slackMessageTemplate = template
	}
}

func testAlertSyncerWithAlertConfigs(configs AlertConfigs) testAlertSyncerConfigOption {
	return func(sut *alertSyncer) {
		sut.alertConfigs = configs
	}
}

type alertTestSuite struct {
	suite.Suite
	kubeClient  *fake.Clientset
	radixClient *fakeradix.Clientset
	promClient  *prometheusfake.Clientset
	kubeUtil    *kube.Kube
}

func TestAlertTestSuite(t *testing.T) {
	suite.Run(t, new(alertTestSuite))
}

func (s *alertTestSuite) SetupSuite() {
	test.SetRequiredEnvironmentVariables()
}

func (s *alertTestSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, secretproviderfake.NewSimpleClientset())
	s.promClient = prometheusfake.NewSimpleClientset()
}

func (s *alertTestSuite) createAlertSyncer(alert *radixv1.RadixAlert, options ...testAlertSyncerConfigOption) *alertSyncer {
	syncer := &alertSyncer{
		kubeClient:           s.kubeClient,
		radixClient:          s.radixClient,
		kubeUtil:             s.kubeUtil,
		prometheusClient:     s.promClient,
		radixAlert:           alert,
		slackMessageTemplate: slackMessageTemplate{},
		alertConfigs:         AlertConfigs{},
		logger:               log.NewEntry(log.StandardLogger()),
	}

	for _, f := range options {
		f(syncer)
	}

	return syncer
}

func (s *alertTestSuite) getRadixAlertAsOwnerReference(radixAlert *radixv1.RadixAlert) metav1.OwnerReference {
	return metav1.OwnerReference{Kind: "RadixAlert", Name: radixAlert.Name, UID: radixAlert.UID, APIVersion: "radix.equinor.com/v1", Controller: utils.BoolPtr(true)}
}

func (s *alertTestSuite) Test_New() {
	ral := &radixv1.RadixAlert{}
	syncer := New(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, ral)
	sut := syncer.(*alertSyncer)
	s.NotNil(sut)
	s.Equal(s.kubeClient, sut.kubeClient)
	s.Equal(s.radixClient, sut.radixClient)
	s.Equal(s.kubeUtil, sut.kubeUtil)
	s.Equal(s.promClient, sut.prometheusClient)
	s.Equal(ral, sut.radixAlert)
	s.Equal(defaultSlackMessageTemplate, sut.slackMessageTemplate)
	s.Equal(defaultAlertConfigs, sut.alertConfigs)
	s.NotNil(sut.logger)
}

func (s *alertTestSuite) Test_OnSync_ResourcesCreated() {
	appName, alertName, alertUID, namespace, adGroup := "any-app", "any-alert", types.UID("alert-uid"), "any-ns", "any-group"
	rr := &radixv1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: appName},
		Spec:       radixv1.RadixRegistrationSpec{AdGroups: []string{adGroup}, MachineUser: true},
	}
	s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Labels: map[string]string{kube.RadixAppLabel: appName}, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	_, err = s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), metav1.GetOptions{})
	s.Nil(err, "secret not found")
	_, err = s.kubeClient.RbacV1().Roles(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Nil(err, "role not found")
	_, err = s.kubeClient.RbacV1().RoleBindings(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Nil(err, "roleBinding not found")
	_, err = s.promClient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Get(context.Background(), getAlertmanagerConfigName(radixalert.Name), metav1.GetOptions{})
	s.Nil(err, "alertmanagerConfig not found")
}

func (s *alertTestSuite) Test_OnSync_Rbac_SkipCreateOnMissingRR() {
	appName, alertName, alertUID, namespace := "any-app", "any-alert", types.UID("alert-uid"), "any-ns"
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Labels: map[string]string{kube.RadixAppLabel: appName}, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	_, err = s.kubeClient.RbacV1().Roles(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.True(errors.IsNotFound(err), "expected role to not exist")
	_, err = s.kubeClient.RbacV1().RoleBindings(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.True(errors.IsNotFound(err), "expected rolebinding to not exist")
}

func (s *alertTestSuite) Test_OnSync_Rbac_DeleteExistingOnMissingRR() {
	appName, alertName, alertUID, namespace := "any-app", "any-alert", types.UID("alert-uid"), "any-ns"
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Labels: map[string]string{kube.RadixAppLabel: appName}, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	_, err := s.kubeClient.RbacV1().Roles(namespace).Create(context.Background(), &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretRoleName(alertName)}}, metav1.CreateOptions{})
	s.Nil(err)
	_, err = s.kubeClient.RbacV1().RoleBindings(namespace).Create(context.Background(), &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretRoleName(alertName)}}, metav1.CreateOptions{})
	s.Nil(err)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync()
	s.Nil(err)
	_, err = s.kubeClient.RbacV1().Roles(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.True(errors.IsNotFound(err), "expected role to be deleted")
	_, err = s.kubeClient.RbacV1().RoleBindings(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.True(errors.IsNotFound(err), "expected rolebinding to be deleted")
}

func (s *alertTestSuite) Test_OnSync_Rbac_CreateWithOwnerReference() {
	namespace, appName := "any-ns", "any-app"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	rr := &radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}
	s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	actualRole, _ := s.kubeClient.RbacV1().Roles(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Len(actualRole.OwnerReferences, 1, "role ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualRole.OwnerReferences[0], "role ownerReference not as expected")
	actualRoleBinding, _ := s.kubeClient.RbacV1().RoleBindings(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Len(actualRoleBinding.OwnerReferences, 1, "rolebinding ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualRoleBinding.OwnerReferences[0], "rolebinding ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_Rbac_UpdateWithOwnerReference() {
	namespace, appName := "any-ns", "any-app"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	rr := &radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}
	s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	_, err := s.kubeClient.RbacV1().Roles(namespace).Create(context.Background(), &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretRoleName(alertName)}}, metav1.CreateOptions{})
	s.Nil(err)
	_, err = s.kubeClient.RbacV1().RoleBindings(namespace).Create(context.Background(), &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretRoleName(alertName)}}, metav1.CreateOptions{})
	s.Nil(err)
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync()
	s.Nil(err)
	actualRole, _ := s.kubeClient.RbacV1().Roles(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Len(actualRole.OwnerReferences, 1, "role ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualRole.OwnerReferences[0], "role ownerReference not as expected")
	actualRoleBinding, _ := s.kubeClient.RbacV1().RoleBindings(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Len(actualRoleBinding.OwnerReferences, 1, "rolebinding ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualRoleBinding.OwnerReferences[0], "rolebinding ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_Rbac_ConfiguredCorrectly() {
	namespace, appName := "any-ns", "any-app"
	adGroup1, adGroup2 := "group1", "group2"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	rr := &radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}, Spec: radixv1.RadixRegistrationSpec{AdGroups: []string{adGroup1, adGroup2}, MachineUser: true}}
	rr, _ = s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	actualRole, _ := s.kubeClient.RbacV1().Roles(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Len(actualRole.Rules, 1, "role rules not as expected")
	s.ElementsMatch([]string{GetAlertSecretName(alertName)}, actualRole.Rules[0].ResourceNames, "role rule resource names not as expected")
	actualRoleBinding, _ := s.kubeClient.RbacV1().RoleBindings(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Equal(actualRole.Name, actualRoleBinding.RoleRef.Name, "rolebinding role reference not as expected")
	s.Equal("Role", actualRoleBinding.RoleRef.Kind, "rolebinding role kind not as expected")
	s.Len(actualRoleBinding.Subjects, 3, "rolebinding subject count not as expected")
	actualSubject, found := s.getSubjectByName(actualRoleBinding.Subjects, adGroup1)
	s.True(found, "group1 not found in rolebinding")
	s.Equal(rbacv1.GroupKind, actualSubject.Kind, "incorrect kind for group1")
	actualSubject, found = s.getSubjectByName(actualRoleBinding.Subjects, adGroup2)
	s.True(found, "group2 not found in rolebinding")
	s.Equal(rbacv1.GroupKind, actualSubject.Kind, "incorrect kind for group2")
	actualSubject, found = s.getSubjectByName(actualRoleBinding.Subjects, defaults.GetMachineUserRoleName(appName))
	s.True(found, "machineuser not found in rolebinding")
	s.Equal(rbacv1.ServiceAccountKind, actualSubject.Kind, "incorrect kind for machineuser")

	// Disable machine user and remove group2 from RR
	rr.Spec.MachineUser = false
	rr.Spec.AdGroups = []string{adGroup1}
	s.radixClient.RadixV1().RadixRegistrations().Update(context.Background(), rr, metav1.UpdateOptions{})
	err = sut.OnSync()
	s.Nil(err)
	actualRoleBinding, _ = s.kubeClient.RbacV1().RoleBindings(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), metav1.GetOptions{})
	s.Len(actualRoleBinding.Subjects, 1, "rolebinding subject count not as expected")
	actualSubject, found = s.getSubjectByName(actualRoleBinding.Subjects, adGroup1)
	s.True(found, "group1 not found in rolebinding after updating RR")
}

func (s *alertTestSuite) Test_OnSync_Secret_RemoveOrphanedKeys() {
	alertName, namespace, receiver1, receiver2 := "any-alert", "any-ns", "receiver1", "receiver2"
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName},
		Spec: radixv1.RadixAlertSpec{
			Receivers: radixv1.ReceiverMap{
				receiver1: radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: true}},
				receiver2: radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: false}},
			},
		},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: GetAlertSecretName(alertName)},
		Data: map[string][]byte{
			GetSlackConfigSecretKeyName("orphaned1"): []byte("foo"),
			GetSlackConfigSecretKeyName(receiver1):   []byte(receiver1),
			GetSlackConfigSecretKeyName(receiver2):   []byte(receiver2),
			GetSlackConfigSecretKeyName("orphaned2"): []byte("bar"),
		},
	}
	s.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), metav1.GetOptions{})
	s.Len(actualSecret.Data, 2)
	s.Equal(receiver1, string(actualSecret.Data[GetSlackConfigSecretKeyName(receiver1)]))
	s.Equal(receiver2, string(actualSecret.Data[GetSlackConfigSecretKeyName(receiver2)]))
}

func (s *alertTestSuite) Test_OnSync_Secret_CreateWithOwnerReference() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), metav1.GetOptions{})
	s.Len(actualSecret.OwnerReferences, 1, "secret ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualSecret.OwnerReferences[0], "secret ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_UpdateWithOwnerReference() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	_, err := s.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: GetAlertSecretName(alertName)}}, metav1.CreateOptions{})
	s.Nil(err)
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync()
	s.Nil(err)
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), metav1.GetOptions{})
	s.Len(actualSecret.OwnerReferences, 1, "secret ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualSecret.OwnerReferences[0], "secret ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_CreateWithAppLabel() {
	namespace, appName := "any-ns", "any-app"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), metav1.GetOptions{})
	s.Equal(map[string]string{kube.RadixAppLabel: appName}, actualSecret.Labels, "secret labels not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_CreateWithoutAppLabel() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), metav1.GetOptions{})
	s.Empty(actualSecret.Labels, "secret labels not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_UpdateSetAppLabel() {
	namespace, appName := "any-ns", "any-app"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	_, err := s.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: GetAlertSecretName(alertName)}}, metav1.CreateOptions{})
	s.Nil(err)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync()
	s.Nil(err)
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), metav1.GetOptions{})
	s.Equal(map[string]string{kube.RadixAppLabel: appName}, actualSecret.Labels, "secret labels not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_UpdateRemoveAppLabel() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	_, err := s.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: GetAlertSecretName(alertName), Labels: map[string]string{kube.RadixAppLabel: "any-app"}}}, metav1.CreateOptions{})
	s.Nil(err)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync()
	s.Nil(err)
	actualSecret, _ := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), metav1.GetOptions{})
	s.Empty(actualSecret.Labels, "secret labels not as expected")
}

func (s *alertTestSuite) Test_OnSync_AlertmanagerConfig_CreateWithOwnerReference() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync()
	s.Nil(err)
	actualAmr, _ := s.promClient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Get(context.Background(), getAlertmanagerConfigName(alertName), metav1.GetOptions{})
	s.Len(actualAmr.OwnerReferences, 1, "alertmanagerconfig ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualAmr.OwnerReferences[0], "alertmanagerconfig ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_AlertmanagerConfig_UpdateWithOwnerReference() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})
	_, err := s.promClient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Create(context.Background(), &v1alpha1.AlertmanagerConfig{ObjectMeta: metav1.ObjectMeta{Name: getAlertmanagerConfigName(alertName)}}, metav1.CreateOptions{})
	s.Nil(err)
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync()
	s.Nil(err)
	actualAmr, _ := s.promClient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Get(context.Background(), getAlertmanagerConfigName(alertName), metav1.GetOptions{})
	s.Len(actualAmr.OwnerReferences, 1, "alertmanagerconfig ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualAmr.OwnerReferences[0], "alertmanagerconfig ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_AlertmanagerConfig_ConfiguredCorrectly() {

	namespace := "any-ns"
	alertName := "alert"
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName},
		Spec: radixv1.RadixAlertSpec{
			Receivers: radixv1.ReceiverMap{
				"rec1": radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: true}},
				"rec2": radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: false}},
				"rec3": radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: true}},
				"rec4": radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: true}},
				"rec5": radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: true}},
			},
			Alerts: []radixv1.Alert{
				{Alert: "deploy", Receiver: "rec1"},
				{Alert: "job", Receiver: "rec1"},
				{Alert: "deploy", Receiver: "rec2"},
				{Alert: "deploy", Receiver: "rec3"},
				{Alert: "undefined", Receiver: "rec3"},
				{Alert: "job", Receiver: "rec4"},
				{Alert: "undefined", Receiver: "rec5"},
			},
		},
	}
	radixalert, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), radixalert, metav1.CreateOptions{})

	alertConfigs := AlertConfigs{
		"deploy": AlertConfig{GroupBy: []string{"g1", "g2"}, Resolvable: true},
		"job":    AlertConfig{GroupBy: []string{"g3"}, Resolvable: false},
	}
	slackTemplate := slackMessageTemplate{title: "atitle", titleLink: "alink", text: "atext"}
	expectedSlackConfigFactory := func(receiverName string, resolvable bool) v1alpha1.SlackConfig {
		return v1alpha1.SlackConfig{
			SendResolved: utils.BoolPtr(resolvable),
			APIURL: &corev1.SecretKeySelector{
				Key:                  GetSlackConfigSecretKeyName(receiverName),
				LocalObjectReference: corev1.LocalObjectReference{Name: GetAlertSecretName(alertName)}},
			Title:     &slackTemplate.title,
			TitleLink: &slackTemplate.titleLink,
			Text:      &slackTemplate.text,
		}
	}
	getRouteByAlertandReceiver := func(routes []v1alpha1.Route, alert, receiver string) (route v1alpha1.Route, found bool) {
		resolvable := alertConfigs[alert].Resolvable
		receiverName := getRouteReceiverNameForAlert(receiver, resolvable)
		for _, r := range routes {
			if r.Receiver == receiverName && len(r.Matchers) == 1 && r.Matchers[0].Name == "alertname" && r.Matchers[0].Value == alert {
				route = r
				found = true
				return
			}
		}
		return
	}
	expectedRouteFactory := func(alert, receiver string) v1alpha1.Route {
		resolvable := alertConfigs[alert].Resolvable
		receiverName := getRouteReceiverNameForAlert(receiver, resolvable)
		repeateInterval := nonResolvableRepeatInterval
		if resolvable {
			repeateInterval = resolvableRepeatInterval
		}

		return v1alpha1.Route{
			Receiver:       receiverName,
			Matchers:       []v1alpha1.Matcher{{Name: "alertname", Value: alert}},
			GroupBy:        alertConfigs[alert].GroupBy,
			GroupWait:      defaultGroupWait,
			GroupInterval:  defaultGroupInterval,
			RepeatInterval: repeateInterval,
		}
	}

	sut := s.createAlertSyncer(radixalert, testAlertSyncerWithAlertConfigs(alertConfigs), testAlertSyncerWithSlackMessageTemplate(slackTemplate))
	err := sut.OnSync()
	s.Nil(err)
	// Receivers
	actualAmr, _ := s.promClient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Get(context.Background(), getAlertmanagerConfigName(alertName), metav1.GetOptions{})
	s.Len(actualAmr.Spec.Receivers, 5)
	_, found := s.getAlertManagerReceiverByName(actualAmr.Spec.Receivers, noopRecevierName)
	s.True(found)
	actualReceiver, found := s.getAlertManagerReceiverByName(actualAmr.Spec.Receivers, getRouteReceiverNameForAlert("rec1", true))
	s.True(found)
	s.Len(actualReceiver.SlackConfigs, 1)
	s.Equal(expectedSlackConfigFactory("rec1", true), actualReceiver.SlackConfigs[0])
	actualReceiver, found = s.getAlertManagerReceiverByName(actualAmr.Spec.Receivers, getRouteReceiverNameForAlert("rec1", false))
	s.True(found)
	s.Len(actualReceiver.SlackConfigs, 1)
	s.Equal(expectedSlackConfigFactory("rec1", false), actualReceiver.SlackConfigs[0])
	actualReceiver, found = s.getAlertManagerReceiverByName(actualAmr.Spec.Receivers, getRouteReceiverNameForAlert("rec3", true))
	s.True(found)
	s.Len(actualReceiver.SlackConfigs, 1)
	s.Equal(expectedSlackConfigFactory("rec3", true), actualReceiver.SlackConfigs[0])
	actualReceiver, found = s.getAlertManagerReceiverByName(actualAmr.Spec.Receivers, getRouteReceiverNameForAlert("rec4", false))
	s.True(found)
	s.Len(actualReceiver.SlackConfigs, 1)
	s.Equal(expectedSlackConfigFactory("rec4", false), actualReceiver.SlackConfigs[0])
	// Routes
	s.Equal(noopRecevierName, actualAmr.Spec.Route.Receiver)
	s.Len(actualAmr.Spec.Route.Routes, 4)
	var childRoutes []v1alpha1.Route
	for _, routeJson := range actualAmr.Spec.Route.Routes {
		childRoute := v1alpha1.Route{}
		err = json.Unmarshal(routeJson.Raw, &childRoute)
		s.Nil(err)
		childRoutes = append(childRoutes, childRoute)
	}
	actualRoute, found := getRouteByAlertandReceiver(childRoutes, "deploy", "rec1")
	s.True(found)
	expectedRoute := expectedRouteFactory("deploy", "rec1")
	s.Equal(expectedRoute, actualRoute)
	actualRoute, found = getRouteByAlertandReceiver(childRoutes, "job", "rec1")
	s.True(found)
	expectedRoute = expectedRouteFactory("job", "rec1")
	s.Equal(expectedRoute, actualRoute)
	actualRoute, found = getRouteByAlertandReceiver(childRoutes, "deploy", "rec3")
	s.True(found)
	expectedRoute = expectedRouteFactory("deploy", "rec3")
	s.Equal(expectedRoute, actualRoute)
	actualRoute, found = getRouteByAlertandReceiver(childRoutes, "job", "rec4")
	s.True(found)
	expectedRoute = expectedRouteFactory("job", "rec4")
	s.Equal(expectedRoute, actualRoute)

	// Update radixAlert
	radixalert.Spec.Alerts = []radixv1.Alert{
		{Alert: "deploy", Receiver: "rec1"},
		{Alert: "deploy", Receiver: "rec2"},
		{Alert: "deploy", Receiver: "rec3"},
		{Alert: "undefined", Receiver: "rec3"},
		{Alert: "undefined", Receiver: "rec5"},
	}
	radixalert, err = s.radixClient.RadixV1().RadixAlerts(namespace).Update(context.Background(), radixalert, metav1.UpdateOptions{})
	s.Nil(err)
	sut = s.createAlertSyncer(radixalert, testAlertSyncerWithAlertConfigs(alertConfigs), testAlertSyncerWithSlackMessageTemplate(slackTemplate))
	sut.OnSync()
	actualAmr, _ = s.promClient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Get(context.Background(), getAlertmanagerConfigName(alertName), metav1.GetOptions{})
	s.Len(actualAmr.Spec.Receivers, 3)
	s.Len(actualAmr.Spec.Route.Routes, 2)
}

func (s *alertTestSuite) getSubjectByName(subjects []rbacv1.Subject, name string) (subject rbacv1.Subject, found bool) {
	for _, s := range subjects {
		if s.Name == name {
			subject = s
			found = true
			return
		}
	}

	return
}

func (s *alertTestSuite) getAlertManagerReceiverByName(subjects []v1alpha1.Receiver, name string) (receiver v1alpha1.Receiver, found bool) {
	for _, s := range subjects {
		if s.Name == name {
			receiver = s
			found = true
			return
		}
	}

	return
}
