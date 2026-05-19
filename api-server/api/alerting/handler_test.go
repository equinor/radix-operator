package alerting

import (
	"context"
	"testing"
	"time"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	alertModels "github.com/equinor/radix-operator/api-server/api/alerting/models"
	"github.com/equinor/radix-operator/api-server/models"
	operatoralert "github.com/equinor/radix-operator/pkg/apis/alert"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type HandlerTestSuite struct {
	suite.Suite
	accounts models.Accounts
}

func (s *HandlerTestSuite) SetupTest() {
	s.setupTest()
}

func (s *HandlerTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *HandlerTestSuite) setupTest() {
	kubeClient := kubefake.NewSimpleClientset()   //nolint:staticcheck
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	kedaClient := kedafake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()
	s.accounts = models.NewAccounts(kubeClient, radixClient, kedaClient, secretProviderClient, nil, certClient, kubeClient, radixClient, kedaClient, secretProviderClient, nil, certClient)
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) Test_GetAlertingConfig() {
	s.Run("no RadixAlert exist in namespace", func() {
		noAlertNs := "no-alert-ns"

		sut := handler{accounts: s.accounts, namespace: noAlertNs, appName: "any-app"}
		config, err := sut.GetAlertingConfig(context.Background())
		s.Nil(err)
		s.False(config.Enabled)
		s.False(config.Ready)
	})

	s.Run("RadixAlert exist in namespace, incorrect appname label", func() {
		incorrectLabelNs, incorrectLabelAppName := "incorrect-label-ns", "any-app"
		incorrectlLabelRal := radixv1.RadixAlert{
			ObjectMeta: metav1.ObjectMeta{Name: "any-alert", Labels: map[string]string{kube.RadixAppLabel: incorrectLabelAppName}, Generation: 1},
			Status:     radixv1.RadixAlertStatus{ObservedGeneration: 1, ReconcileStatus: radixv1.RadixAlertReconcileSucceeded},
		}
		_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(incorrectLabelNs).Create(context.Background(), &incorrectlLabelRal, metav1.CreateOptions{})
		require.NoError(s.T(), err)

		sut := handler{accounts: s.accounts, namespace: incorrectLabelNs, appName: "other-app"}
		config, err := sut.GetAlertingConfig(context.Background())
		s.Nil(err)
		s.False(config.Enabled)
		s.False(config.Ready)
	})

	s.Run("multiple RadixAlerts exists in namespace", func() {
		multipleAlertNs, multipleAlertApp := "multiple-alert-ns", "multiple-alert"
		multipleRal1 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "alert1", Labels: map[string]string{kube.RadixAppLabel: multipleAlertApp}}}
		multipleRal2 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "alert2", Labels: map[string]string{kube.RadixAppLabel: multipleAlertApp}}}
		_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(multipleAlertNs).Create(context.Background(), &multipleRal1, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(multipleAlertNs).Create(context.Background(), &multipleRal2, metav1.CreateOptions{})
		require.NoError(s.T(), err)

		sut := handler{accounts: s.accounts, namespace: multipleAlertNs, appName: multipleAlertApp}
		config, err := sut.GetAlertingConfig(context.Background())
		s.Error(err, MultipleAlertingConfigurationsError())
		s.Nil(config)
	})

	s.Run("RadixAlert not reconciled", func() {
		notReconciledNs, notReconciledApp := "not-reconciled-ns", "not-reconciled-app"
		notReconciledRal := radixv1.RadixAlert{
			ObjectMeta: metav1.ObjectMeta{Name: "alert1", Labels: map[string]string{kube.RadixAppLabel: notReconciledApp}},
			Status:     radixv1.RadixAlertStatus{},
		}
		_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(notReconciledNs).Create(context.Background(), &notReconciledRal, metav1.CreateOptions{})
		require.NoError(s.T(), err)

		sut := handler{accounts: s.accounts, namespace: notReconciledNs, appName: notReconciledApp}
		config, err := sut.GetAlertingConfig(context.Background())
		s.Nil(err)
		s.True(config.Enabled)
		s.False(config.Ready)
	})

	s.Run("RadixAlert incorrect generation reconciled", func() {
		notReconciledNs, notReconciledApp := "not-reconciled-ns", "not-reconciled-app"
		notReconciledRal := radixv1.RadixAlert{
			ObjectMeta: metav1.ObjectMeta{Name: "alert1", Labels: map[string]string{kube.RadixAppLabel: notReconciledApp}, Generation: 2},
			Status:     radixv1.RadixAlertStatus{ObservedGeneration: 1, ReconcileStatus: radixv1.RadixAlertReconcileSucceeded},
		}
		_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(notReconciledNs).Create(context.Background(), &notReconciledRal, metav1.CreateOptions{})
		require.NoError(s.T(), err)

		sut := handler{accounts: s.accounts, namespace: notReconciledNs, appName: notReconciledApp}
		config, err := sut.GetAlertingConfig(context.Background())
		s.Nil(err)
		s.True(config.Enabled)
		s.False(config.Ready)
	})

	s.Run("RadixAlert reconcile failed", func() {
		notReconciledNs, notReconciledApp := "not-reconciled-ns", "not-reconciled-app"
		notReconciledRal := radixv1.RadixAlert{
			ObjectMeta: metav1.ObjectMeta{Name: "alert1", Labels: map[string]string{kube.RadixAppLabel: notReconciledApp}, Generation: 2},
			Status:     radixv1.RadixAlertStatus{ObservedGeneration: 2, ReconcileStatus: radixv1.RadixAlertReconcileFailed},
		}
		_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(notReconciledNs).Create(context.Background(), &notReconciledRal, metav1.CreateOptions{})
		require.NoError(s.T(), err)

		sut := handler{accounts: s.accounts, namespace: notReconciledNs, appName: notReconciledApp}
		config, err := sut.GetAlertingConfig(context.Background())
		s.Nil(err)
		s.True(config.Enabled)
		s.False(config.Ready)
	})

	s.Run("RadixAlert configured and reconciled", func() {
		appNs, appName, alertName, alertNames := "myapp-app", "myapp", "alert", []string{"alert1", "alert2", "alert3"}
		ral := radixv1.RadixAlert{
			ObjectMeta: metav1.ObjectMeta{Name: alertName, Labels: map[string]string{kube.RadixAppLabel: appName}, Generation: 2},
			Spec: radixv1.RadixAlertSpec{
				Receivers: radixv1.ReceiverMap{
					"receiver1": radixv1.Receiver{
						SlackConfig: radixv1.SlackConfig{Enabled: true},
					},
					"receiver2": radixv1.Receiver{
						SlackConfig: radixv1.SlackConfig{Enabled: false},
					},
				},
				Alerts: []radixv1.Alert{
					{Receiver: "receiver1", Alert: "alert1"},
					{Receiver: "receiver2", Alert: "alert2"},
				},
			},
			Status: radixv1.RadixAlertStatus{ObservedGeneration: 2, ReconcileStatus: radixv1.RadixAlertReconcileSucceeded},
		}
		_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(appNs).Create(context.Background(), &ral, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: operatoralert.GetAlertSecretName(alertName)},
			Data: map[string][]byte{
				operatoralert.GetSlackConfigSecretKeyName("receiver2"): []byte("data"),
			},
		}
		_, err = s.accounts.UserAccount.Client.CoreV1().Secrets(appNs).Create(context.Background(), &secret, metav1.CreateOptions{})
		require.NoError(s.T(), err)

		sut := handler{accounts: s.accounts, namespace: appNs, appName: appName, validAlertNames: alertNames}
		config, err := sut.GetAlertingConfig(context.Background())
		s.Nil(err)
		s.True(config.Enabled)
		s.True(config.Ready)
		s.NotNil(config)
		s.ElementsMatch(alertNames, config.AlertNames)
		s.ElementsMatch(alertModels.AlertConfigList{{Alert: "alert1", Receiver: "receiver1"}, {Alert: "alert2", Receiver: "receiver2"}}, config.Alerts)
		s.Len(config.Receivers, 2)
		s.Equal(config.Receivers["receiver1"], alertModels.ReceiverConfig{SlackConfig: &alertModels.SlackConfig{Enabled: true}})
		s.Equal(config.Receivers["receiver2"], alertModels.ReceiverConfig{SlackConfig: &alertModels.SlackConfig{Enabled: false}})
		s.Len(config.ReceiverSecretStatus, 2)
		s.Equal(config.ReceiverSecretStatus["receiver1"], alertModels.ReceiverConfigSecretStatus{SlackConfig: &alertModels.SlackConfigSecretStatus{WebhookURLConfigured: false}})
		s.Equal(config.ReceiverSecretStatus["receiver2"], alertModels.ReceiverConfigSecretStatus{SlackConfig: &alertModels.SlackConfigSecretStatus{WebhookURLConfigured: true}})
	})
}

func (s *HandlerTestSuite) Test_EnableAlerting() {
	namespace, appName, alertNames := "anyapp-app", "anyapp", []string{"alert1", "alert2"}
	sut := handler{accounts: s.accounts, namespace: namespace, appName: appName, validAlertNames: alertNames, reconcilePollInterval: time.Millisecond, reconcilePollTimeout: time.Millisecond}
	config, err := sut.EnableAlerting(context.Background())
	s.Nil(err)
	s.True(config.Enabled)
	s.False(config.Ready) // Unable to test radix-operator reconciliation, so Ready will be false
	s.Len(config.Receivers, 1)
	s.Equal(config.Receivers[defaultReceiverName], alertModels.ReceiverConfig{SlackConfig: &alertModels.SlackConfig{Enabled: true}})
	s.Len(config.ReceiverSecretStatus, 1)
	s.Equal(config.ReceiverSecretStatus[defaultReceiverName], alertModels.ReceiverConfigSecretStatus{SlackConfig: &alertModels.SlackConfigSecretStatus{WebhookURLConfigured: false}})
	s.ElementsMatch([]alertModels.AlertConfig{{Receiver: defaultReceiverName, Alert: alertNames[0]}, {Receiver: defaultReceiverName, Alert: alertNames[1]}}, config.Alerts)

	_, err = sut.EnableAlerting(context.Background())
	s.Error(err, AlertingAlreadyEnabledError())
}

func (s *HandlerTestSuite) Test_DisableAlerting() {
	namespace, appName := "myapp-app", "myapp"

	fixture := func() {
		ral1 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "alert1", Labels: map[string]string{kube.RadixAppLabel: appName}}}
		ral2 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "alert2", Labels: map[string]string{kube.RadixAppLabel: appName}}}
		ral3 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "alert3", Labels: map[string]string{kube.RadixAppLabel: "another-app"}}}
		ral4 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "alert4"}}
		ral5 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "alert5", Labels: map[string]string{kube.RadixAppLabel: appName}}}
		_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral1, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral2, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral3, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &ral4, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts("another-ns").Create(context.Background(), &ral5, metav1.CreateOptions{})
		require.NoError(s.T(), err)
	}

	s.Run("disable alerting should delete expected RadixAlert resources", func() {
		fixture()
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName}
		config, err := sut.DisableAlerting(context.Background())
		s.Nil(err)
		s.False(config.Enabled)
		s.False(config.Ready)
		actualAlerts, _ := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
		s.Len(actualAlerts.Items, 3)
		s.True(s.alertExists("alert3", actualAlerts.Items))
		s.True(s.alertExists("alert4", actualAlerts.Items))
		s.True(s.alertExists("alert5", actualAlerts.Items))
	})

	s.Run("disable alerting when not enabled should not return an error", func() {
		fixture()
		sut := handler{accounts: s.accounts, namespace: "any-ns", appName: "any-app"}
		_, err := sut.DisableAlerting(context.Background())
		s.Nil(err)
	})
}

func (s *HandlerTestSuite) Test_UpdateAlertingConfig() {
	appName1, appName2, appName3, appName4, namespace := "app1", "app2", "app3", "app4", "app-ns"
	aSecretValue := "anysecretvalue"

	fixture := func() {
		app1Alert1 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "app1-alert1", Labels: map[string]string{kube.RadixAppLabel: appName1}}}
		app1Alert2 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "app1-alert2", Labels: map[string]string{kube.RadixAppLabel: appName1}}}
		app2Alert1 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "app2-alert1", Labels: map[string]string{kube.RadixAppLabel: appName2}}}
		app3Alert1 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "app3-alert1", Labels: map[string]string{kube.RadixAppLabel: appName3}}}
		app4Alert1 := radixv1.RadixAlert{ObjectMeta: metav1.ObjectMeta{Name: "app4-alert1", Labels: map[string]string{kube.RadixAppLabel: appName4}}}

		_, err := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &app1Alert1, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &app1Alert2, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts("another-ns").Create(context.Background(), &app2Alert1, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &app3Alert1, metav1.CreateOptions{})
		require.NoError(s.T(), err)
		_, err = s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Create(context.Background(), &app4Alert1, metav1.CreateOptions{})
		require.NoError(s.T(), err)

		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: operatoralert.GetAlertSecretName("app4-alert1")},
			Data: map[string][]byte{
				operatoralert.GetSlackConfigSecretKeyName("receiver1"): []byte(aSecretValue),
				operatoralert.GetSlackConfigSecretKeyName("receiver2"): []byte(aSecretValue),
				operatoralert.GetSlackConfigSecretKeyName("receiver3"): []byte(aSecretValue),
			},
		}
		_, err = s.accounts.UserAccount.Client.CoreV1().Secrets(namespace).Create(context.Background(), &secret, metav1.CreateOptions{})
		require.NoError(s.T(), err)
	}

	s.Run("multiple RadixAlert resources returns error", func() {
		fixture()
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName1}
		_, err := sut.UpdateAlertingConfig(context.Background(), alertModels.UpdateAlertingConfig{})
		s.Error(err, MultipleAlertingConfigurationsError())
	})
	s.Run("no RadixAlert in namespace returns error", func() {
		fixture()
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName2}
		_, err := sut.UpdateAlertingConfig(context.Background(), alertModels.UpdateAlertingConfig{})
		s.Error(err, AlertingNotEnabledError())
	})

	s.Run("missing Secret for RadixAlert returns error", func() {
		fixture()
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName3}
		update := alertModels.UpdateAlertingConfig{
			Receivers: alertModels.ReceiverConfigMap{"receiver1": alertModels.ReceiverConfig{}},
			ReceiverSecrets: alertModels.UpdateReceiverConfigSecretsMap{
				"receiver1": alertModels.UpdateReceiverConfigSecrets{},
			},
		}
		_, err := sut.UpdateAlertingConfig(context.Background(), update)
		s.NotNil(err)
		s.True(errors.IsNotFound(err))
		s.IsType(&errors.StatusError{}, err)
		apiErr := err.(*errors.StatusError)
		s.Equal(apiErr.ErrStatus.Details.Name, operatoralert.GetAlertSecretName("app3-alert1"))
	})

	s.Run("undefined receiver reference in receiverSecrets", func() {
		fixture()
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName4}
		update := alertModels.UpdateAlertingConfig{
			Receivers: alertModels.ReceiverConfigMap{"receiver1": alertModels.ReceiverConfig{}},
			ReceiverSecrets: alertModels.UpdateReceiverConfigSecretsMap{
				"receiver2": alertModels.UpdateReceiverConfigSecrets{},
			},
		}
		_, err := sut.UpdateAlertingConfig(context.Background(), update)
		s.Error(err, UpdateReceiverSecretNotDefinedError("receiver2"))
	})

	s.Run("undefined receiver reference in list of alerts", func() {
		fixture()
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName4}
		update := alertModels.UpdateAlertingConfig{
			Receivers: alertModels.ReceiverConfigMap{"receiver1": alertModels.ReceiverConfig{}},
			Alerts:    alertModels.AlertConfigList{{Alert: "alert1", Receiver: "receiver2"}},
		}
		_, err := sut.UpdateAlertingConfig(context.Background(), update)
		s.Error(err, InvalidAlertReceiverError("alert1", "receiver2"))
	})

	s.Run("alert name not in list of valid alerts", func() {
		fixture()
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName4, validAlertNames: []string{"alert1"}}
		update := alertModels.UpdateAlertingConfig{
			Receivers: alertModels.ReceiverConfigMap{"receiver1": alertModels.ReceiverConfig{}},
			Alerts:    alertModels.AlertConfigList{{Alert: "alert2", Receiver: "receiver1"}},
		}
		_, err := sut.UpdateAlertingConfig(context.Background(), update)
		s.Error(err, InvalidAlertError("alert2"))
	})

	s.Run("invalid slack URL scheme", func() {
		fixture()
		invalidSlackUrl := "http://any.com"
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName4, validAlertNames: []string{"alert1"}}
		update := alertModels.UpdateAlertingConfig{
			Receivers: alertModels.ReceiverConfigMap{
				"receiver1": alertModels.ReceiverConfig{},
			},
			ReceiverSecrets: alertModels.UpdateReceiverConfigSecretsMap{
				"receiver1": alertModels.UpdateReceiverConfigSecrets{SlackConfig: &alertModels.UpdateSlackConfigSecrets{WebhookURL: &invalidSlackUrl}},
			},
			Alerts: alertModels.AlertConfigList{{Alert: "alert1", Receiver: "receiver1"}},
		}
		_, err := sut.UpdateAlertingConfig(context.Background(), update)
		s.Error(err, InvalidSlackURLSchemeError())
	})

	s.Run("verify all resources updated correctly", func() {
		fixture()
		slackUrl, emptySlackUrl := "https://any.com", ""
		sut := handler{accounts: s.accounts, namespace: namespace, appName: appName4, validAlertNames: []string{"alert1", "alert2"}}
		update := alertModels.UpdateAlertingConfig{
			Receivers: alertModels.ReceiverConfigMap{
				"receiver1": alertModels.ReceiverConfig{SlackConfig: &alertModels.SlackConfig{Enabled: true}},
				"receiver2": alertModels.ReceiverConfig{SlackConfig: &alertModels.SlackConfig{Enabled: false}},
			},
			ReceiverSecrets: alertModels.UpdateReceiverConfigSecretsMap{
				"receiver1": alertModels.UpdateReceiverConfigSecrets{SlackConfig: &alertModels.UpdateSlackConfigSecrets{WebhookURL: &slackUrl}},
				"receiver2": alertModels.UpdateReceiverConfigSecrets{SlackConfig: &alertModels.UpdateSlackConfigSecrets{WebhookURL: &emptySlackUrl}},
			},
			Alerts: alertModels.AlertConfigList{
				{Alert: "alert1", Receiver: "receiver1"},
				{Alert: "alert2", Receiver: "receiver1"},
				{Alert: "alert1", Receiver: "receiver2"},
			},
		}
		_, err := sut.UpdateAlertingConfig(context.Background(), update)
		s.Nil(err)

		// Check that receiver1 secret is updated and receiver2 secret is deleted
		actualSecret, _ := s.accounts.UserAccount.Client.CoreV1().Secrets(namespace).Get(context.Background(), operatoralert.GetAlertSecretName("app4-alert1"), metav1.GetOptions{})
		s.Len(actualSecret.Data, 2)
		s.Equal(slackUrl, string(actualSecret.Data[operatoralert.GetSlackConfigSecretKeyName("receiver1")]))
		s.Equal(aSecretValue, string(actualSecret.Data[operatoralert.GetSlackConfigSecretKeyName("receiver3")]))
		actualRAL, _ := s.accounts.UserAccount.RadixClient.RadixV1().RadixAlerts(namespace).Get(context.Background(), "app4-alert1", metav1.GetOptions{})
		s.Len(actualRAL.Spec.Receivers, 2)
		s.Equal(radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: true}}, actualRAL.Spec.Receivers["receiver1"])
		s.Equal(radixv1.Receiver{SlackConfig: radixv1.SlackConfig{Enabled: false}}, actualRAL.Spec.Receivers["receiver2"])
		s.Len(actualRAL.Spec.Alerts, 3)
		s.ElementsMatch([]radixv1.Alert{
			{Receiver: "receiver1", Alert: "alert1"},
			{Receiver: "receiver1", Alert: "alert2"},
			{Receiver: "receiver2", Alert: "alert1"},
		}, actualRAL.Spec.Alerts)
	})
}

func (s *HandlerTestSuite) alertExists(name string, alerts []radixv1.RadixAlert) bool {
	for _, a := range alerts {
		if a.Name == name {
			return true
		}
	}
	return false
}
