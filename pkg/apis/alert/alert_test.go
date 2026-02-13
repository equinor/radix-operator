package alert

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	dynamicClient client.WithWatch
}

func TestAlertTestSuite(t *testing.T) {
	suite.Run(t, new(alertTestSuite))
}

func (s *alertTestSuite) SetupSuite() {
	test.SetRequiredEnvironmentVariables()
}

func (s *alertTestSuite) SetupTest() {
	s.dynamicClient = test.CreateClient()
}

func (s *alertTestSuite) createAlertSyncer(alert *radixv1.RadixAlert, options ...testAlertSyncerConfigOption) *alertSyncer {
	syncer := &alertSyncer{
		dynamicClient:        s.dynamicClient,
		radixAlert:           alert,
		slackMessageTemplate: slackMessageTemplate{},
		alertConfigs:         AlertConfigs{},
	}

	for _, f := range options {
		f(syncer)
	}

	return syncer
}

func (s *alertTestSuite) getRadixAlertAsOwnerReference(radixAlert *radixv1.RadixAlert) metav1.OwnerReference {
	return metav1.OwnerReference{Kind: radixv1.KindRadixAlert, Name: radixAlert.Name, UID: radixAlert.UID, APIVersion: radixv1.SchemeGroupVersion.Identifier(), Controller: utils.BoolPtr(true), BlockOwnerDeletion: utils.BoolPtr(true)}
}

func (s *alertTestSuite) Test_New() {
	ral := &radixv1.RadixAlert{}
	syncer := New(s.dynamicClient, ral)
	sut := syncer.(*alertSyncer)
	s.NotNil(sut)
	s.Equal(s.dynamicClient, sut.dynamicClient)
	s.Equal(ral, sut.radixAlert)
	s.Equal(defaultSlackMessageTemplate, sut.slackMessageTemplate)
	s.Equal(defaultAlertConfigs, sut.alertConfigs)
}

func (s *alertTestSuite) Test_ReconcileStatus() {
	ral := &radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "any-name", Namespace: "any-ns", Generation: 42}}
	err := s.dynamicClient.Create(context.Background(), ral)
	s.Require().NoError(err)

	// First sync sets status
	expectedGen := ral.Generation
	sut := s.createAlertSyncer(ral)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)
	err = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: ral.Name, Namespace: ral.Namespace}, ral)
	s.Require().NoError(err)
	s.Equal(radixv1.RadixAlertReconcileSucceeded, ral.Status.ReconcileStatus)
	s.Empty(ral.Status.Message)
	s.Equal(expectedGen, ral.Status.ObservedGeneration)
	s.False(ral.Status.Reconciled.IsZero())

	// Second sync with updated generation
	ral.Generation++
	expectedGen = ral.Generation
	sut = s.createAlertSyncer(ral)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)
	err = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: ral.Name, Namespace: ral.Namespace}, ral)
	s.Require().NoError(err)
	s.Equal(radixv1.RadixAlertReconcileSucceeded, ral.Status.ReconcileStatus)
	s.Empty(ral.Status.Message)
	s.Equal(expectedGen, ral.Status.ObservedGeneration)
	s.False(ral.Status.Reconciled.IsZero())
}

func (s *alertTestSuite) Test_OnSync_ResourcesCreated() {
	appName, alertName, alertUID, namespace := "any-app", "any-alert", types.UID("alert-uid"), "any-ns"
	rr := &radixv1.RadixRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: appName},
		Spec:       radixv1.RadixRegistrationSpec{AdGroups: []string{"admin"}, ReaderAdGroups: []string{"reader"}},
	}
	err := s.dynamicClient.Create(context.Background(), rr)
	s.Require().NoError(err)
	radixAlert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, Labels: map[string]string{kube.RadixAppLabel: appName}, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixAlert))

	sut := s.createAlertSyncer(radixAlert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualSecret := &corev1.Secret{}
	err = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: GetAlertSecretName(alertName), Namespace: namespace}, actualSecret)
	s.Nil(err, "secret not found")
	actualRoles := &rbacv1.RoleList{}
	_ = s.dynamicClient.List(context.Background(), actualRoles, client.InNamespace(namespace))
	s.ElementsMatch([]string{"any-alert-alert-config-admin", "any-alert-alert-config-reader"}, s.getRoleNames(actualRoles))
	actualRoleBindings := &rbacv1.RoleBindingList{}
	_ = s.dynamicClient.List(context.Background(), actualRoleBindings, client.InNamespace(namespace))
	s.ElementsMatch([]string{"any-alert-alert-config-admin", "any-alert-alert-config-reader"}, s.getRoleBindingNames(actualRoleBindings))

	target := client.ObjectKey{Name: getAlertmanagerConfigName(radixAlert.Name), Namespace: namespace}
	err = s.dynamicClient.Get(s.T().Context(), target, &v1alpha1.AlertmanagerConfig{})
	s.Nil(err, "alertmanagerConfig not found")
}

func (s *alertTestSuite) Test_OnSync_Rbac_SkipCreateOnMissingRR() {
	appName, alertName, alertUID, namespace := "any-app", "any-alert", types.UID("alert-uid"), "any-ns"
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, Labels: map[string]string{kube.RadixAppLabel: appName}, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualRoles := &rbacv1.RoleList{}
	_ = s.dynamicClient.List(context.Background(), actualRoles, client.InNamespace(namespace))
	s.Len(actualRoles.Items, 0)
	actualRoleBindings := &rbacv1.RoleBindingList{}
	_ = s.dynamicClient.List(context.Background(), actualRoleBindings, client.InNamespace(namespace))
	s.Len(actualRoleBindings.Items, 0)
}

func (s *alertTestSuite) Test_OnSync_Rbac_DeleteExistingOnMissingRR() {
	appName, alertName, alertUID, namespace := "any-app", "any-alert", types.UID("alert-uid"), "any-ns"
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, Labels: map[string]string{kube.RadixAppLabel: appName}, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	err := s.dynamicClient.Create(context.Background(), &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretAdminRoleName(alertName), Namespace: namespace}})
	s.Nil(err)
	err = s.dynamicClient.Create(context.Background(), &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretAdminRoleName(alertName), Namespace: namespace}})
	s.Nil(err)
	err = s.dynamicClient.Create(context.Background(), &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretReaderRoleName(alertName), Namespace: namespace}})
	s.Nil(err)
	err = s.dynamicClient.Create(context.Background(), &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretReaderRoleName(alertName), Namespace: namespace}})
	s.Nil(err)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualRoles := &rbacv1.RoleList{}
	_ = s.dynamicClient.List(context.Background(), actualRoles, client.InNamespace(namespace))
	s.Len(actualRoles.Items, 0)
	actualRoleBindings := &rbacv1.RoleBindingList{}
	_ = s.dynamicClient.List(context.Background(), actualRoleBindings, client.InNamespace(namespace))
	s.Len(actualRoleBindings.Items, 0)
}

func (s *alertTestSuite) Test_OnSync_Rbac_CreateWithOwnerReference() {
	namespace, appName := "any-ns", "any-app"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	rr := &radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}
	err := s.dynamicClient.Create(context.Background(), rr)
	s.Require().NoError(err)

	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualRoles := &rbacv1.RoleList{}
	_ = s.dynamicClient.List(context.Background(), actualRoles, client.InNamespace(namespace))
	s.Len(actualRoles.Items, 2)
	for _, actualRole := range actualRoles.Items {
		s.Len(actualRole.OwnerReferences, 1, "role ownerReference length not as expected")
		s.Equal(expectedAlertOwnerRef, actualRole.OwnerReferences[0], "role ownerReference not as expected")
	}
	actualRoleBindings := &rbacv1.RoleBindingList{}
	_ = s.dynamicClient.List(context.Background(), actualRoleBindings, client.InNamespace(namespace))
	s.Len(actualRoleBindings.Items, 2)
	for _, actualRoleBinding := range actualRoleBindings.Items {
		s.Len(actualRoleBinding.OwnerReferences, 1, "rolebinding ownerReference length not as expected")
		s.Equal(expectedAlertOwnerRef, actualRoleBinding.OwnerReferences[0], "rolebinding ownerReference not as expected")
	}
}

func (s *alertTestSuite) Test_OnSync_Rbac_UpdateWithOwnerReference() {
	namespace, appName := "any-ns", "any-app"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	err := s.dynamicClient.Create(context.Background(), radixalert)
	s.Require().NoError(err)
	rr := &radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}}
	err = s.dynamicClient.Create(context.Background(), rr)
	s.Require().NoError(err)

	err = s.dynamicClient.Create(context.Background(), &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretAdminRoleName(alertName), Namespace: namespace}})
	s.Nil(err)
	err = s.dynamicClient.Create(context.Background(), &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretAdminRoleName(alertName), Namespace: namespace}})
	s.Nil(err)
	err = s.dynamicClient.Create(context.Background(), &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretReaderRoleName(alertName), Namespace: namespace}})
	s.Nil(err)
	err = s.dynamicClient.Create(context.Background(), &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: getAlertConfigSecretReaderRoleName(alertName), Namespace: namespace}})
	s.Nil(err)
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualRoles := &rbacv1.RoleList{}
	_ = s.dynamicClient.List(context.Background(), actualRoles, client.InNamespace(namespace))
	s.Len(actualRoles.Items, 2)
	for _, actualRole := range actualRoles.Items {
		s.Len(actualRole.OwnerReferences, 1, "role ownerReference length not as expected")
		s.Equal(expectedAlertOwnerRef, actualRole.OwnerReferences[0], "role ownerReference not as expected")
	}
	actualRoleBindings := &rbacv1.RoleBindingList{}
	_ = s.dynamicClient.List(context.Background(), actualRoleBindings, client.InNamespace(namespace))
	s.Len(actualRoleBindings.Items, 2)
	for _, actualRoleBinding := range actualRoleBindings.Items {
		s.Len(actualRoleBinding.OwnerReferences, 1, "rolebinding ownerReference length not as expected")
		s.Equal(expectedAlertOwnerRef, actualRoleBinding.OwnerReferences[0], "rolebinding ownerReference not as expected")
	}
}

func (s *alertTestSuite) Test_OnSync_Rbac_ConfiguredCorrectly() {
	namespace, appName := "any-ns", "any-app"
	adminGroups, adminUsers := []string{"admin1", "admin2"}, []string{"adminUser1", "adminUser2"}
	readerGroups, readerUsers := []string{"reader1", "reader2"}, []string{"readerUser1", "readerUser2"}
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	rr := &radixv1.RadixRegistration{ObjectMeta: metav1.ObjectMeta{Name: appName}, Spec: radixv1.RadixRegistrationSpec{AdGroups: adminGroups, AdUsers: adminUsers, ReaderAdGroups: readerGroups, ReaderAdUsers: readerUsers}}
	err := s.dynamicClient.Create(context.Background(), rr)
	s.Require().NoError(err)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)
	actualAdminRole := &rbacv1.Role{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: getAlertConfigSecretAdminRoleName(alertName), Namespace: namespace}, actualAdminRole)
	s.Len(actualAdminRole.Rules, 1, "role rules not as expected")
	s.ElementsMatch([]string{GetAlertSecretName(alertName)}, actualAdminRole.Rules[0].ResourceNames, "role rule resource names not as expected")
	s.ElementsMatch([]string{"secrets"}, actualAdminRole.Rules[0].Resources, "role rule resources not as expected")
	s.ElementsMatch([]string{""}, actualAdminRole.Rules[0].APIGroups, "role rule API groups not as expected")
	s.ElementsMatch([]string{"get", "list", "watch", "update", "patch", "delete"}, actualAdminRole.Rules[0].Verbs, "role rule verbs not as expected")
	actualAdminRoleBinding := &rbacv1.RoleBinding{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: getAlertConfigSecretAdminRoleName(alertName), Namespace: namespace}, actualAdminRoleBinding)
	s.Equal(actualAdminRole.Name, actualAdminRoleBinding.RoleRef.Name, "rolebinding role reference not as expected")
	s.Equal("Role", actualAdminRoleBinding.RoleRef.Kind, "rolebinding role kind not as expected")
	s.ElementsMatch(
		[]rbacv1.Subject{
			{APIGroup: "rbac.authorization.k8s.io", Kind: rbacv1.GroupKind, Name: "admin1"},
			{APIGroup: "rbac.authorization.k8s.io", Kind: rbacv1.GroupKind, Name: "admin2"},
			{APIGroup: "rbac.authorization.k8s.io", Kind: rbacv1.UserKind, Name: "adminUser1"},
			{APIGroup: "rbac.authorization.k8s.io", Kind: rbacv1.UserKind, Name: "adminUser2"},
		},
		actualAdminRoleBinding.Subjects)

	actualReaderRole := &rbacv1.Role{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: getAlertConfigSecretReaderRoleName(alertName), Namespace: namespace}, actualReaderRole)
	s.Len(actualReaderRole.Rules, 1, "role rules not as expected")
	s.ElementsMatch([]string{GetAlertSecretName(alertName)}, actualReaderRole.Rules[0].ResourceNames, "role rule resource names not as expected")
	s.ElementsMatch([]string{"secrets"}, actualReaderRole.Rules[0].Resources, "role rule resources not as expected")
	s.ElementsMatch([]string{""}, actualReaderRole.Rules[0].APIGroups, "role rule API groups not as expected")
	s.ElementsMatch([]string{"get", "list", "watch"}, actualReaderRole.Rules[0].Verbs, "role rule verbs not as expected")
	actualReaderRoleBinding := &rbacv1.RoleBinding{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: getAlertConfigSecretReaderRoleName(alertName), Namespace: namespace}, actualReaderRoleBinding)
	s.Equal(actualReaderRole.Name, actualReaderRoleBinding.RoleRef.Name, "rolebinding role reference not as expected")
	s.Equal("Role", actualReaderRoleBinding.RoleRef.Kind, "rolebinding role kind not as expected")
	s.ElementsMatch(
		[]rbacv1.Subject{
			{APIGroup: "rbac.authorization.k8s.io", Kind: rbacv1.GroupKind, Name: "reader1"},
			{APIGroup: "rbac.authorization.k8s.io", Kind: rbacv1.GroupKind, Name: "reader2"},
			{APIGroup: "rbac.authorization.k8s.io", Kind: rbacv1.UserKind, Name: "readerUser1"},
			{APIGroup: "rbac.authorization.k8s.io", Kind: rbacv1.UserKind, Name: "readerUser2"},
		},
		actualReaderRoleBinding.Subjects)
}

func (s *alertTestSuite) Test_OnSync_Secret_RemoveOrphanedKeys() {
	alertName, namespace, receiver1, receiver2 := "any-alert", "any-ns", "receiver1", "receiver2"
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace},
		Spec: radixv1.RadixAlertSpec{
			Receivers: radixv1.ReceiverMap{
				receiver1: radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: true}},
				receiver2: radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: false}},
			},
		},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: GetAlertSecretName(alertName), Namespace: namespace},
		Data: map[string][]byte{
			GetSlackConfigSecretKeyName("orphaned1"): []byte("foo"),
			GetSlackConfigSecretKeyName(receiver1):   []byte(receiver1),
			GetSlackConfigSecretKeyName(receiver2):   []byte(receiver2),
			GetSlackConfigSecretKeyName("orphaned2"): []byte("bar"),
		},
	}
	err := s.dynamicClient.Create(context.Background(), secret)
	s.Require().NoError(err)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualSecret := &corev1.Secret{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: GetAlertSecretName(alertName), Namespace: namespace}, actualSecret)
	s.Len(actualSecret.Data, 2)
	s.Equal(receiver1, string(actualSecret.Data[GetSlackConfigSecretKeyName(receiver1)]))
	s.Equal(receiver2, string(actualSecret.Data[GetSlackConfigSecretKeyName(receiver2)]))
}

func (s *alertTestSuite) Test_OnSync_Secret_CreateWithOwnerReference() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualSecret := &corev1.Secret{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: GetAlertSecretName(alertName), Namespace: namespace}, actualSecret)
	s.Len(actualSecret.OwnerReferences, 1, "secret ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualSecret.OwnerReferences[0], "secret ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_UpdateWithOwnerReference() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	err := s.dynamicClient.Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: GetAlertSecretName(alertName), Namespace: namespace}})
	s.Nil(err)
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualSecret := &corev1.Secret{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: GetAlertSecretName(alertName), Namespace: namespace}, actualSecret)
	s.Len(actualSecret.OwnerReferences, 1, "secret ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualSecret.OwnerReferences[0], "secret ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_CreateWithAppLabel() {
	namespace, appName := "any-ns", "any-app"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualSecret := &corev1.Secret{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: GetAlertSecretName(alertName), Namespace: namespace}, actualSecret)
	s.Equal(map[string]string{kube.RadixAppLabel: appName}, actualSecret.Labels, "secret labels not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_CreateWithoutAppLabel() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualSecret := &corev1.Secret{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: GetAlertSecretName(alertName), Namespace: namespace}, actualSecret)
	s.Empty(actualSecret.Labels, "secret labels not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_UpdateSetAppLabel() {
	namespace, appName := "any-ns", "any-app"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID, Labels: map[string]string{kube.RadixAppLabel: appName}},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	err := s.dynamicClient.Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: GetAlertSecretName(alertName), Namespace: namespace}})
	s.Nil(err)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualSecret := &corev1.Secret{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: GetAlertSecretName(alertName), Namespace: namespace}, actualSecret)
	s.Equal(map[string]string{kube.RadixAppLabel: appName}, actualSecret.Labels, "secret labels not as expected")
}

func (s *alertTestSuite) Test_OnSync_Secret_UpdateRemoveAppLabel() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	err := s.dynamicClient.Create(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: GetAlertSecretName(alertName), Namespace: namespace, Labels: map[string]string{kube.RadixAppLabel: "any-app"}}})
	s.Nil(err)

	sut := s.createAlertSyncer(radixalert)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualSecret := &corev1.Secret{}
	_ = s.dynamicClient.Get(context.Background(), client.ObjectKey{Name: GetAlertSecretName(alertName), Namespace: namespace}, actualSecret)
	s.Empty(actualSecret.Labels, "secret labels not as expected")
}

func (s *alertTestSuite) Test_OnSync_AlertmanagerConfig_CreateWithOwnerReference() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))
	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualAmr := &v1alpha1.AlertmanagerConfig{ObjectMeta: metav1.ObjectMeta{Name: getAlertmanagerConfigName(alertName), Namespace: namespace}}
	s.Require().NoError(s.dynamicClient.Get(s.T().Context(), client.ObjectKeyFromObject(actualAmr), actualAmr))
	s.Len(actualAmr.OwnerReferences, 1, "alertmanagerconfig ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualAmr.OwnerReferences[0], "alertmanagerconfig ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_AlertmanagerConfig_UpdateWithOwnerReference() {
	namespace := "any-ns"
	alertName, alertUID := "alert", types.UID("alertuid")
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	s.Require().NoError(s.dynamicClient.Create(s.T().Context(), radixalert))

	// Pre-create AlertmanagerConfig
	s.Require().NoError(s.dynamicClient.Create(s.T().Context(), &v1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: getAlertmanagerConfigName(alertName), Namespace: namespace},
	}))

	expectedAlertOwnerRef := s.getRadixAlertAsOwnerReference(radixalert)

	sut := s.createAlertSyncer(radixalert)
	err := sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualAmr := &v1alpha1.AlertmanagerConfig{ObjectMeta: metav1.ObjectMeta{Name: getAlertmanagerConfigName(alertName), Namespace: namespace}}
	err = s.dynamicClient.Get(s.T().Context(), client.ObjectKeyFromObject(actualAmr), actualAmr)
	s.Require().NoError(err)
	s.Len(actualAmr.OwnerReferences, 1, "alertmanagerconfig ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualAmr.OwnerReferences[0], "alertmanagerconfig ownerReference not as expected")
}

func (s *alertTestSuite) Test_OnSync_AlertmanagerConfig_ConfiguredCorrectly() {

	namespace := "any-ns"
	alertName := "alert"
	radixalert := &radixv1.RadixAlert{
		ObjectMeta: metav1.ObjectMeta{Name: alertName, Namespace: namespace},
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
	s.Require().NoError(s.dynamicClient.Create(context.Background(), radixalert))

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
			Title:     slackTemplate.title,
			TitleLink: slackTemplate.titleLink,
			Text:      slackTemplate.text,
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
			Matchers:       []v1alpha1.Matcher{{Name: "alertname", Value: alert, MatchType: v1alpha1.MatchEqual}},
			GroupBy:        alertConfigs[alert].GroupBy,
			GroupWait:      defaultGroupWait,
			GroupInterval:  defaultGroupInterval,
			RepeatInterval: repeateInterval,
		}
	}

	sut := s.createAlertSyncer(radixalert, testAlertSyncerWithAlertConfigs(alertConfigs), testAlertSyncerWithSlackMessageTemplate(slackTemplate))
	err := sut.OnSync(context.Background())
	s.Require().NoError(err)

	// Receivers
	actualAmr := &v1alpha1.AlertmanagerConfig{ObjectMeta: metav1.ObjectMeta{Name: getAlertmanagerConfigName(alertName), Namespace: namespace}}
	err = s.dynamicClient.Get(s.T().Context(), client.ObjectKeyFromObject(actualAmr), actualAmr)
	s.Require().NoError(err)
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
	err = s.dynamicClient.Get(context.Background(), client.ObjectKeyFromObject(radixalert), radixalert)
	s.Require().NoError(err)
	radixalert.Spec.Alerts = []radixv1.Alert{
		{Alert: "deploy", Receiver: "rec1"},
		{Alert: "deploy", Receiver: "rec2"},
		{Alert: "deploy", Receiver: "rec3"},
		{Alert: "undefined", Receiver: "rec3"},
		{Alert: "undefined", Receiver: "rec5"},
	}
	err = s.dynamicClient.Update(context.Background(), radixalert)
	s.Nil(err)
	sut = s.createAlertSyncer(radixalert, testAlertSyncerWithAlertConfigs(alertConfigs), testAlertSyncerWithSlackMessageTemplate(slackTemplate))
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)

	actualAmr = &v1alpha1.AlertmanagerConfig{ObjectMeta: metav1.ObjectMeta{Name: getAlertmanagerConfigName(alertName), Namespace: namespace}}
	err = s.dynamicClient.Get(s.T().Context(), client.ObjectKeyFromObject(actualAmr), actualAmr)
	s.Require().NoError(err)
	s.Len(actualAmr.Spec.Receivers, 3)
	s.Len(actualAmr.Spec.Route.Routes, 2)
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

func (s *alertTestSuite) getRoleNames(roles *rbacv1.RoleList) []string {
	if roles == nil {
		return nil
	}
	return slice.Map(roles.Items, func(r rbacv1.Role) string { return r.GetName() })
}

func (s *alertTestSuite) getRoleBindingNames(roleBindings *rbacv1.RoleBindingList) []string {
	if roleBindings == nil {
		return nil
	}
	return slice.Map(roleBindings.Items, func(r rbacv1.RoleBinding) string { return r.GetName() })
}
