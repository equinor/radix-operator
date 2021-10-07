package alert

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	log "github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

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
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient)
	s.promClient = prometheusfake.NewSimpleClientset()
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
		ObjectMeta: v1.ObjectMeta{Name: appName},
		Spec:       radixv1.RadixRegistrationSpec{AdGroups: []string{adGroup}, MachineUser: true},
	}
	s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, v1.CreateOptions{})
	ral := &radixv1.RadixAlert{
		ObjectMeta: v1.ObjectMeta{Name: alertName, Labels: map[string]string{kube.RadixAppLabel: appName}, UID: alertUID},
		Spec:       radixv1.RadixAlertSpec{},
	}
	ral, _ = s.radixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), ral, v1.CreateOptions{})

	sut := alertSyncer{
		kubeClient:           s.kubeClient,
		radixClient:          s.radixClient,
		kubeUtil:             s.kubeUtil,
		prometheusClient:     s.promClient,
		radixAlert:           ral,
		slackMessageTemplate: slackMessageTemplate{},
		alertConfigs:         alertConfigs{},
		logger:               log.NewEntry(log.StandardLogger()),
	}
	err := sut.OnSync()
	expectedAlertOwnerRef := v1.OwnerReference{Kind: "RadixAlert", Name: alertName, UID: alertUID, APIVersion: "radix.equinor.com/v1"}
	s.Nil(err)
	actualSecret, err := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), GetAlertSecretName(alertName), v1.GetOptions{})
	s.Nil(err, "secret not found")
	s.Len(actualSecret.OwnerReferences, 1, "secret ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualSecret.OwnerReferences[0], "secret ownerReference not as expected")
	actualRole, _ := s.kubeClient.RbacV1().Roles(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), v1.GetOptions{})
	s.NotNil(actualRole, "role not found")
	s.Len(actualRole.OwnerReferences, 1, "role ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualRole.OwnerReferences[0], "role ownerReference not as expected")
	s.ElementsMatch([]string{actualSecret.Name}, actualRole.Rules[0].ResourceNames, "role resourceNames not as expected")
	actualRoleBinding, _ := s.kubeClient.RbacV1().RoleBindings(namespace).Get(context.Background(), getAlertConfigSecretRoleName(alertName), v1.GetOptions{})
	s.NotNil(actualRoleBinding, "roleBinding not found")
	s.Len(actualRoleBinding.OwnerReferences, 1, "rolebinding ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualRoleBinding.OwnerReferences[0], "rolebinding ownerReference not as expected")
	actualAlertmanagerConfig, _ := s.promClient.MonitoringV1alpha1().AlertmanagerConfigs(namespace).Get(context.Background(), getAlertmanagerConfigName(ral.Name), v1.GetOptions{})
	s.NotNil(actualAlertmanagerConfig, "alertmanagerConfig not found")
	s.Len(actualAlertmanagerConfig.OwnerReferences, 1, "alertmanagerConfig ownerReference length not as expected")
	s.Equal(expectedAlertOwnerRef, actualAlertmanagerConfig.OwnerReferences[0], "alertmanagerConfig ownerReference not as expected")
	// s.Equal(actualRole.Name, actualRoleBinding.RoleRef.Name)
	// _, found := s.getSubjectByName(actualRoleBinding.Subjects, adGroup)
	// s.True(found, "missing ad group in rolebinding subject")
	// _, found = s.getSubjectByName(actualRoleBinding.Subjects, defaults.GetMachineUserRoleName(rr.Name))
	// s.True(found, "missing machine user in rolebinding subject")
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
