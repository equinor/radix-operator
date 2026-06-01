package networkpolicy

import (
	"context"
	"strconv"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	rx "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
)

const (
	appName     = "myapp"
	envName     = "myenv"
	gatewayName = "test-gateway"
)

var testConfig = config.Config{
	Gateway: config.GatewayConfig{
		Name: gatewayName,
	},
}

func setupTest(t *testing.T) (*fake.Clientset, *kube.Kube) {
	t.Helper()
	kubeClient := fake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
	kedaClient := kedafake.NewSimpleClientset()
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	kubeUtil, err := kube.New(kubeClient, radixClient, kedaClient, secretProviderClient)
	require.NoError(t, err)
	return kubeClient, kubeUtil
}

func namespace() string {
	return utils.GetEnvironmentNamespace(appName, envName)
}

func boolPtr(v bool) *bool {
	return &v
}

func getNetworkPolicies(t *testing.T, kubeClient *fake.Clientset) []networkingv1.NetworkPolicy {
	t.Helper()
	policies, err := kubeClient.NetworkingV1().NetworkPolicies(namespace()).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	return policies.Items
}

func getUserDefinedNetworkPolicies(t *testing.T, kubeClient *fake.Clientset) []networkingv1.NetworkPolicy {
	t.Helper()
	policies, err := kubeClient.NetworkingV1().NetworkPolicies(namespace()).List(context.Background(), metav1.ListOptions{
		LabelSelector: kube.RadixUserDefinedNetworkPolicyLabel + "=true",
	})
	require.NoError(t, err)
	return policies.Items
}

func TestUpdateEnvEgressRules_NoRulesAndNilAllowRadix_DeletesExistingPolicies(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)
	ns := namespace()

	// Pre-create a user-defined network policy
	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userDefinedEgressPolicyName,
			Namespace: ns,
			Labels: map[string]string{
				kube.RadixAppLabel:                      appName,
				kube.RadixEnvLabel:                      envName,
				kube.RadixUserDefinedNetworkPolicyLabel: strconv.FormatBool(true),
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}
	_, err := kubeClient.NetworkingV1().NetworkPolicies(ns).Create(context.Background(), existing, metav1.CreateOptions{})
	require.NoError(t, err)

	// Verify it exists before the call
	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	// Call with no rules and nil allowRadix
	err = nw.UpdateEnvEgressRules(context.Background(), nil, nil, appName, envName)
	require.NoError(t, err)

	// Policy should be deleted
	policies = getUserDefinedNetworkPolicies(t, kubeClient)
	assert.Empty(t, policies)
}

func TestUpdateEnvEgressRules_NoRulesAndNilAllowRadix_NoExistingPolicies_NoError(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	err := nw.UpdateEnvEgressRules(context.Background(), nil, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	assert.Empty(t, policies)
}

func TestUpdateEnvEgressRules_EmptyRulesAndNilAllowRadix_DeletesExistingPolicies(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)
	ns := namespace()

	// Pre-create a user-defined network policy
	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userDefinedEgressPolicyName,
			Namespace: ns,
			Labels: map[string]string{
				kube.RadixAppLabel:                      appName,
				kube.RadixEnvLabel:                      envName,
				kube.RadixUserDefinedNetworkPolicyLabel: strconv.FormatBool(true),
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}
	_, err := kubeClient.NetworkingV1().NetworkPolicies(ns).Create(context.Background(), existing, metav1.CreateOptions{})
	require.NoError(t, err)

	err = nw.UpdateEnvEgressRules(context.Background(), []rx.EgressRule{}, nil, appName, envName)
	require.NoError(t, err)

	policies := getUserDefinedNetworkPolicies(t, kubeClient)
	assert.Empty(t, policies)
}

func TestUpdateEnvEgressRules_WithRules_CreatesPolicy(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports: []rx.EgressPort{
				{Port: 443, Protocol: "TCP"},
			},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	policy := policies[0]
	assert.Equal(t, userDefinedEgressPolicyName, policy.Name)
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, policy.Spec.PolicyTypes)
}

func TestUpdateEnvEgressRules_PolicyLabels(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	labels := policies[0].Labels
	assert.Equal(t, appName, labels[kube.RadixAppLabel])
	assert.Equal(t, envName, labels[kube.RadixEnvLabel])
	assert.Equal(t, "true", labels[kube.RadixUserDefinedNetworkPolicyLabel])
}

func TestUpdateEnvEgressRules_UserDefinedRulesAreConverted(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	tcp := corev1.ProtocolTCP
	udp := corev1.ProtocolUDP
	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8", "192.168.1.0/24"},
			Ports: []rx.EgressPort{
				{Port: 443, Protocol: "TCP"},
				{Port: 53, Protocol: "UDP"},
			},
		},
		{
			Destinations: []rx.EgressDestination{"172.16.0.0/12"},
			Ports: []rx.EgressPort{
				{Port: 8080, Protocol: "TCP"},
			},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// user rules + kube-dns + own-namespace = 2 + 2 = 4
	require.Len(t, egressRules, 4)

	// First user-defined rule
	rule0 := egressRules[0]
	require.Len(t, rule0.Ports, 2)
	assert.Equal(t, &tcp, rule0.Ports[0].Protocol)
	assert.Equal(t, int32(443), rule0.Ports[0].Port.IntVal)
	assert.Equal(t, &udp, rule0.Ports[1].Protocol)
	assert.Equal(t, int32(53), rule0.Ports[1].Port.IntVal)
	require.Len(t, rule0.To, 2)
	assert.Equal(t, "10.0.0.0/8", rule0.To[0].IPBlock.CIDR)
	assert.Equal(t, "192.168.1.0/24", rule0.To[1].IPBlock.CIDR)

	// Second user-defined rule
	rule1 := egressRules[1]
	require.Len(t, rule1.Ports, 1)
	assert.Equal(t, &tcp, rule1.Ports[0].Protocol)
	assert.Equal(t, int32(8080), rule1.Ports[0].Port.IntVal)
	require.Len(t, rule1.To, 1)
	assert.Equal(t, "172.16.0.0/12", rule1.To[0].IPBlock.CIDR)
}

func TestUpdateEnvEgressRules_AlwaysIncludesKubeDnsRule(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	tcp := corev1.ProtocolTCP
	udp := corev1.ProtocolUDP

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// user rule + kube-dns + own-namespace = 3
	require.Len(t, egressRules, 3)

	// kube-dns rule is the second-to-last
	dnsRule := egressRules[len(egressRules)-2]
	require.Len(t, dnsRule.Ports, 2)
	assert.Equal(t, &tcp, dnsRule.Ports[0].Protocol)
	assert.Equal(t, intstr.IntOrString{IntVal: 53}, *dnsRule.Ports[0].Port)
	assert.Equal(t, &udp, dnsRule.Ports[1].Protocol)
	assert.Equal(t, intstr.IntOrString{IntVal: 53}, *dnsRule.Ports[1].Port)
	require.Len(t, dnsRule.To, 1)
	assert.Equal(t, map[string]string{kube.K8sAppLabel: "kube-dns"}, dnsRule.To[0].PodSelector.MatchLabels)
	assert.NotNil(t, dnsRule.To[0].NamespaceSelector)
}

func TestUpdateEnvEgressRules_AlwaysIncludesOwnNamespaceRule(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// user rule + kube-dns + own-namespace = 3
	require.Len(t, egressRules, 3)

	// own namespace rule is the last
	ownNsRule := egressRules[len(egressRules)-1]
	assert.Nil(t, ownNsRule.Ports, "own namespace rule should allow all ports")
	require.Len(t, ownNsRule.To, 1)
	assert.Equal(t, map[string]string{
		kube.RadixAppLabel: appName,
		kube.RadixEnvLabel: envName,
	}, ownNsRule.To[0].NamespaceSelector.MatchLabels)
}

func TestUpdateEnvEgressRules_AllowRadixTrue_IncludesRadixEgressRule(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	tcp := corev1.ProtocolTCP
	udp := corev1.ProtocolUDP

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, boolPtr(true), appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// user rule + kube-dns + own-namespace + allowRadix = 4
	require.Len(t, egressRules, 4)

	radixRule := egressRules[3]
	require.Len(t, radixRule.Ports, 4)

	// TCP 80, TCP 443, UDP 80, UDP 443
	assert.Equal(t, &tcp, radixRule.Ports[0].Protocol)
	assert.Equal(t, int32(80), radixRule.Ports[0].Port.IntVal)
	assert.Equal(t, &tcp, radixRule.Ports[1].Protocol)
	assert.Equal(t, int32(443), radixRule.Ports[1].Port.IntVal)
	assert.Equal(t, &udp, radixRule.Ports[2].Protocol)
	assert.Equal(t, int32(80), radixRule.Ports[2].Port.IntVal)
	assert.Equal(t, &udp, radixRule.Ports[3].Protocol)
	assert.Equal(t, int32(443), radixRule.Ports[3].Port.IntVal)

	require.Len(t, radixRule.To, 2)

	// ingress-nginx peer
	assert.Equal(t, map[string]string{"app.kubernetes.io/name": "ingress-nginx"}, radixRule.To[0].PodSelector.MatchLabels)
	assert.Equal(t, map[string]string{"purpose": "radix-base-ns"}, radixRule.To[0].NamespaceSelector.MatchLabels)

	// gateway peer
	assert.Equal(t, map[string]string{"gateway.networking.k8s.io/gateway-name": gatewayName}, radixRule.To[1].PodSelector.MatchLabels)
	assert.Equal(t, map[string]string{"purpose": "radix-base-ns"}, radixRule.To[1].NamespaceSelector.MatchLabels)
}

func TestUpdateEnvEgressRules_AllowRadixFalse_ExcludesRadixEgressRule(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, boolPtr(false), appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// user rule + kube-dns + own-namespace = 3 (no radix rule)
	assert.Len(t, egressRules, 3)
}

func TestUpdateEnvEgressRules_AllowRadixNil_WithRules_ExcludesRadixEgressRule(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// user rule + kube-dns + own-namespace = 3 (no radix rule)
	assert.Len(t, egressRules, 3)
}

func TestUpdateEnvEgressRules_NoRulesButAllowRadixTrue_CreatesPolicyWithRadixRule(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	err := nw.UpdateEnvEgressRules(context.Background(), nil, boolPtr(true), appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// kube-dns + own-namespace + allowRadix = 3
	require.Len(t, egressRules, 3)

	// Verify the radix rule is present (last one)
	radixRule := egressRules[2]
	require.Len(t, radixRule.To, 2)
	assert.Equal(t, map[string]string{"app.kubernetes.io/name": "ingress-nginx"}, radixRule.To[0].PodSelector.MatchLabels)
}

func TestUpdateEnvEgressRules_NoRulesButAllowRadixFalse_CreatesPolicyWithoutRadixRule(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	err := nw.UpdateEnvEgressRules(context.Background(), nil, boolPtr(false), appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// kube-dns + own-namespace = 2
	assert.Len(t, egressRules, 2)
}

func TestUpdateEnvEgressRules_PolicyCreatedInCorrectNamespace(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	ns := namespace()
	policies, err := kubeClient.NetworkingV1().NetworkPolicies(ns).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, policies.Items, 1)

	// Verify no policies in other namespaces
	allPolicies, err := kubeClient.NetworkingV1().NetworkPolicies("").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, allPolicies.Items, 1)
}

func TestUpdateEnvEgressRules_UpdateExistingPolicy(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	// Create initial policy
	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}
	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	// Update with new rules
	newRules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"172.16.0.0/12"},
			Ports:        []rx.EgressPort{{Port: 443, Protocol: "TCP"}},
		},
	}
	err = nw.UpdateEnvEgressRules(context.Background(), newRules, boolPtr(true), appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1, "should update the existing policy, not create a new one")

	egressRules := policies[0].Spec.Egress
	// new user rule + kube-dns + own-namespace + allowRadix = 4
	require.Len(t, egressRules, 4)

	// Verify the new user-defined rule replaces old one
	assert.Equal(t, "172.16.0.0/12", egressRules[0].To[0].IPBlock.CIDR)
	tcp := corev1.ProtocolTCP
	assert.Equal(t, &tcp, egressRules[0].Ports[0].Protocol)
	assert.Equal(t, int32(443), egressRules[0].Ports[0].Port.IntVal)
}

func TestUpdateEnvEgressRules_MultipleDestinationsAndPorts(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{
				"195.88.55.16/32",
				"195.88.54.16/31",
				"10.0.0.0/8",
			},
			Ports: []rx.EgressPort{
				{Port: 80, Protocol: "TCP"},
				{Port: 443, Protocol: "TCP"},
				{Port: 8080, Protocol: "UDP"},
			},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	userRule := policies[0].Spec.Egress[0]
	assert.Len(t, userRule.To, 3)
	assert.Equal(t, "195.88.55.16/32", userRule.To[0].IPBlock.CIDR)
	assert.Equal(t, "195.88.54.16/31", userRule.To[1].IPBlock.CIDR)
	assert.Equal(t, "10.0.0.0/8", userRule.To[2].IPBlock.CIDR)

	assert.Len(t, userRule.Ports, 3)
	assert.Equal(t, int32(80), userRule.Ports[0].Port.IntVal)
	assert.Equal(t, int32(443), userRule.Ports[1].Port.IntVal)
	assert.Equal(t, int32(8080), userRule.Ports[2].Port.IntVal)
}

func TestUpdateEnvEgressRules_MultipleRules(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
		{
			Destinations: []rx.EgressDestination{"172.16.0.0/12"},
			Ports:        []rx.EgressPort{{Port: 443, Protocol: "TCP"}},
		},
		{
			Destinations: []rx.EgressDestination{"192.168.0.0/16"},
			Ports:        []rx.EgressPort{{Port: 8443, Protocol: "UDP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	// 3 user rules + kube-dns + own-namespace = 5
	require.Len(t, egressRules, 5)

	assert.Equal(t, "10.0.0.0/8", egressRules[0].To[0].IPBlock.CIDR)
	assert.Equal(t, "172.16.0.0/12", egressRules[1].To[0].IPBlock.CIDR)
	assert.Equal(t, "192.168.0.0/16", egressRules[2].To[0].IPBlock.CIDR)
}

func TestUpdateEnvEgressRules_ProtocolCaseConversion(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports: []rx.EgressPort{
				{Port: 80, Protocol: "tcp"},
				{Port: 443, Protocol: "TCP"},
				{Port: 53, Protocol: "udp"},
				{Port: 8080, Protocol: "UDP"},
			},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	userRule := policies[0].Spec.Egress[0]
	tcp := corev1.ProtocolTCP
	udp := corev1.ProtocolUDP
	assert.Equal(t, &tcp, userRule.Ports[0].Protocol, "lowercase tcp should be converted to TCP")
	assert.Equal(t, &tcp, userRule.Ports[1].Protocol)
	assert.Equal(t, &udp, userRule.Ports[2].Protocol, "lowercase udp should be converted to UDP")
	assert.Equal(t, &udp, userRule.Ports[3].Protocol)
}

func TestUpdateEnvEgressRules_GatewayNameFromConfig(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	customGateway := "custom-gw"
	cfg := config.Config{
		Gateway: config.GatewayConfig{
			Name: customGateway,
		},
	}
	nw := NewNetworkPolicy(kubeClient, kubeUtil, cfg)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, boolPtr(true), appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	// radix rule is the last one
	egressRules := policies[0].Spec.Egress
	radixRule := egressRules[len(egressRules)-1]
	require.Len(t, radixRule.To, 2)
	assert.Equal(t, map[string]string{"gateway.networking.k8s.io/gateway-name": customGateway}, radixRule.To[1].PodSelector.MatchLabels)
}

func TestUpdateEnvEgressRules_PolicyTypeIsEgress(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, nil, appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, policies[0].Spec.PolicyTypes)
}

func TestUpdateEnvEgressRules_EgressRuleOrdering(t *testing.T) {
	kubeClient, kubeUtil := setupTest(t)
	nw := NewNetworkPolicy(kubeClient, kubeUtil, testConfig)

	rules := []rx.EgressRule{
		{
			Destinations: []rx.EgressDestination{"10.0.0.0/8"},
			Ports:        []rx.EgressPort{{Port: 80, Protocol: "TCP"}},
		},
	}

	err := nw.UpdateEnvEgressRules(context.Background(), rules, boolPtr(true), appName, envName)
	require.NoError(t, err)

	policies := getNetworkPolicies(t, kubeClient)
	require.Len(t, policies, 1)

	egressRules := policies[0].Spec.Egress
	require.Len(t, egressRules, 4)

	// Order: user rules, kube-dns, own-namespace, allowRadix
	// 0: user rule
	assert.NotNil(t, egressRules[0].To[0].IPBlock, "first rule should be user-defined with IPBlock")
	// 1: kube-dns
	assert.Equal(t, map[string]string{kube.K8sAppLabel: "kube-dns"}, egressRules[1].To[0].PodSelector.MatchLabels, "second rule should be kube-dns")
	// 2: own namespace
	assert.Equal(t, map[string]string{kube.RadixAppLabel: appName, kube.RadixEnvLabel: envName}, egressRules[2].To[0].NamespaceSelector.MatchLabels, "third rule should be own-namespace")
	// 3: allowRadix
	assert.Len(t, egressRules[3].To, 2, "fourth rule should be the radix rule with ingress-nginx and gateway peers")
}
