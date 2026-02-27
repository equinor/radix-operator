package deployment

import (
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func Test_defaultIngressNetworkPolicy(t *testing.T) {
	appName := "myapp"
	env := "test"
	gatewayName := "my-gateway"
	owner := []metav1.OwnerReference{{Name: "owner1"}}

	np := defaultIngressNetworkPolicy(appName, env, owner, gatewayName)

	t.Run("metadata", func(t *testing.T) {
		assert.Equal(t, "radix-deny-traffic-from-other-ns", np.Name)
		assert.Equal(t, appName, np.Labels[kube.RadixAppLabel])
		assert.Equal(t, env, np.Labels[kube.RadixEnvLabel])
		assert.Equal(t, owner, np.OwnerReferences)
	})

	t.Run("pod selector selects all pods", func(t *testing.T) {
		assert.Empty(t, np.Spec.PodSelector.MatchLabels)
		assert.Empty(t, np.Spec.PodSelector.MatchExpressions)
	})

	t.Run("has single ingress rule", func(t *testing.T) {
		require.Len(t, np.Spec.Ingress, 1)
	})

	t.Run("ingress allows same namespace pods", func(t *testing.T) {
		rule := np.Spec.Ingress[0]
		require.GreaterOrEqual(t, len(rule.From), 4)

		// First peer: all pods in same namespace
		peer := rule.From[0]
		require.NotNil(t, peer.PodSelector)
		assert.Empty(t, peer.PodSelector.MatchLabels)
		assert.Nil(t, peer.NamespaceSelector)
	})

	t.Run("ingress allows ingress-nginx from radix-base-ns", func(t *testing.T) {
		peer := np.Spec.Ingress[0].From[1]
		require.NotNil(t, peer.PodSelector)
		assert.Equal(t, "ingress-nginx", peer.PodSelector.MatchLabels["app.kubernetes.io/name"])
		require.NotNil(t, peer.NamespaceSelector)
		assert.Equal(t, "radix-base-ns", peer.NamespaceSelector.MatchLabels["purpose"])
	})

	t.Run("ingress allows prometheus from radix-base-ns", func(t *testing.T) {
		peer := np.Spec.Ingress[0].From[2]
		require.NotNil(t, peer.PodSelector)
		assert.Equal(t, "prometheus", peer.PodSelector.MatchLabels["app.kubernetes.io/name"])
		require.NotNil(t, peer.NamespaceSelector)
		assert.Equal(t, "radix-base-ns", peer.NamespaceSelector.MatchLabels["purpose"])
	})

	t.Run("ingress allows gateway pods from radix-base-ns", func(t *testing.T) {
		peer := np.Spec.Ingress[0].From[3]
		require.NotNil(t, peer.PodSelector)
		assert.Equal(t, gatewayName, peer.PodSelector.MatchLabels["gateway.networking.k8s.io/gateway-name"])
		require.NotNil(t, peer.NamespaceSelector)
		assert.Equal(t, "radix-base-ns", peer.NamespaceSelector.MatchLabels["purpose"])
	})

	t.Run("no policy types specified means default ingress", func(t *testing.T) {
		assert.Empty(t, np.Spec.PolicyTypes, "default ingress policy should not explicitly set PolicyTypes")
	})
}

func Test_allowOauthAuxComponentEgressNetworkPolicy(t *testing.T) {
	appName := "myapp"
	env := "prod"
	owner := []metav1.OwnerReference{{Name: "owner1"}}

	np := allowOauthAuxComponentEgressNetworkPolicy(appName, env, owner)

	t.Run("metadata", func(t *testing.T) {
		assert.Equal(t, "radix-allow-oauth-aux-egress", np.Name)
		assert.Equal(t, appName, np.Labels[kube.RadixAppLabel])
		assert.Equal(t, env, np.Labels[kube.RadixEnvLabel])
		assert.Equal(t, owner, np.OwnerReferences)
	})

	t.Run("policy type is egress only", func(t *testing.T) {
		require.Len(t, np.Spec.PolicyTypes, 1)
		assert.Equal(t, v1.PolicyTypeEgress, np.Spec.PolicyTypes[0])
	})

	t.Run("pod selector targets oauth auxiliary component", func(t *testing.T) {
		expected := map[string]string{
			kube.RadixAuxiliaryComponentTypeLabel: radixv1.OAuthProxyAuxiliaryComponentType,
		}
		assert.Equal(t, expected, np.Spec.PodSelector.MatchLabels)
	})

	t.Run("egress has two rules", func(t *testing.T) {
		require.Len(t, np.Spec.Egress, 2)
	})

	t.Run("egress ports include DNS TCP, DNS UDP, HTTPS, Redis plain and Redis TLS", func(t *testing.T) {
		portRule := np.Spec.Egress[0]
		require.Len(t, portRule.Ports, 5)

		expectedPorts := []struct {
			port     int32
			protocol corev1.Protocol
		}{
			{53, corev1.ProtocolTCP},
			{53, corev1.ProtocolUDP},
			{443, corev1.ProtocolTCP},
			{6379, corev1.ProtocolTCP},
			{6380, corev1.ProtocolTCP},
		}

		for i, expected := range expectedPorts {
			assert.Equal(t, intstr.IntOrString{IntVal: expected.port}, *portRule.Ports[i].Port, "port %d", i)
			assert.Equal(t, expected.protocol, *portRule.Ports[i].Protocol, "protocol for port %d", expected.port)
		}
	})

	t.Run("egress allows traffic to same app namespace", func(t *testing.T) {
		nsRule := np.Spec.Egress[1]
		require.Len(t, nsRule.To, 1)
		require.NotNil(t, nsRule.To[0].NamespaceSelector)
		assert.Equal(t, appName, nsRule.To[0].NamespaceSelector.MatchLabels[kube.RadixAppLabel])
		assert.Equal(t, env, nsRule.To[0].NamespaceSelector.MatchLabels[kube.RadixEnvLabel])
	})
}

func Test_allowJobSchedulerServerEgressNetworkPolicy(t *testing.T) {
	appName := "myapp"
	env := "dev"
	owner := []metav1.OwnerReference{{Name: "owner1"}}
	var kubernetesAPIPort int32 = 6443

	np := allowJobSchedulerServerEgressNetworkPolicy(appName, env, owner, kubernetesAPIPort)

	t.Run("metadata", func(t *testing.T) {
		assert.Equal(t, "radix-allow-job-scheduler-egress", np.Name)
		assert.Equal(t, appName, np.Labels[kube.RadixAppLabel])
		assert.Equal(t, env, np.Labels[kube.RadixEnvLabel])
		assert.Equal(t, owner, np.OwnerReferences)
	})

	t.Run("policy type is egress only", func(t *testing.T) {
		require.Len(t, np.Spec.PolicyTypes, 1)
		assert.Equal(t, v1.PolicyTypeEgress, np.Spec.PolicyTypes[0])
	})

	t.Run("pod selector targets job scheduler pods", func(t *testing.T) {
		expected := map[string]string{
			kube.RadixPodIsJobSchedulerLabel: "true",
		}
		assert.Equal(t, expected, np.Spec.PodSelector.MatchLabels)
	})

	t.Run("egress has two rules", func(t *testing.T) {
		require.Len(t, np.Spec.Egress, 2)
	})

	t.Run("egress ports include DNS TCP, DNS UDP, and Kubernetes API port", func(t *testing.T) {
		portRule := np.Spec.Egress[0]
		require.Len(t, portRule.Ports, 3)

		expectedPorts := []struct {
			port     int32
			protocol corev1.Protocol
		}{
			{53, corev1.ProtocolTCP},
			{53, corev1.ProtocolUDP},
			{kubernetesAPIPort, corev1.ProtocolTCP},
		}

		for i, expected := range expectedPorts {
			assert.Equal(t, intstr.IntOrString{IntVal: expected.port}, *portRule.Ports[i].Port, "port %d", i)
			assert.Equal(t, expected.protocol, *portRule.Ports[i].Protocol, "protocol for port %d", expected.port)
		}
	})

	t.Run("egress allows traffic to same app namespace", func(t *testing.T) {
		nsRule := np.Spec.Egress[1]
		require.Len(t, nsRule.To, 1)
		require.NotNil(t, nsRule.To[0].NamespaceSelector)
		assert.Equal(t, appName, nsRule.To[0].NamespaceSelector.MatchLabels[kube.RadixAppLabel])
		assert.Equal(t, env, nsRule.To[0].NamespaceSelector.MatchLabels[kube.RadixEnvLabel])
	})

	t.Run("kubernetes API port is configurable", func(t *testing.T) {
		var customPort int32 = 8443
		np2 := allowJobSchedulerServerEgressNetworkPolicy(appName, env, owner, customPort)
		portRule := np2.Spec.Egress[0]
		// Last port should be the kubernetes API port
		lastPort := portRule.Ports[len(portRule.Ports)-1]
		assert.Equal(t, intstr.IntOrString{IntVal: customPort}, *lastPort.Port)
	})
}

func Test_allowEgressNetworkByPortPolicy(t *testing.T) {
	appName := "testapp"
	env := "staging"
	owner := []metav1.OwnerReference{{Name: "owner1"}}
	policyName := "test-egress-policy"
	targetLabelKey := "my-label"
	targetLabelValue := "my-value"

	ports := []egreessPortPolicy{
		{port: 80, protocol: corev1.ProtocolTCP},
		{port: 443, protocol: corev1.ProtocolTCP},
	}

	np := allowEgressNetworkByPortPolicy(policyName, targetLabelKey, targetLabelValue, appName, env, owner, ports)

	t.Run("metadata", func(t *testing.T) {
		assert.Equal(t, policyName, np.Name)
		assert.Equal(t, appName, np.Labels[kube.RadixAppLabel])
		assert.Equal(t, env, np.Labels[kube.RadixEnvLabel])
		assert.Equal(t, owner, np.OwnerReferences)
	})

	t.Run("policy type is egress", func(t *testing.T) {
		require.Len(t, np.Spec.PolicyTypes, 1)
		assert.Equal(t, v1.PolicyTypeEgress, np.Spec.PolicyTypes[0])
	})

	t.Run("pod selector uses target label", func(t *testing.T) {
		assert.Equal(t, map[string]string{targetLabelKey: targetLabelValue}, np.Spec.PodSelector.MatchLabels)
	})

	t.Run("egress port rule matches input ports", func(t *testing.T) {
		require.Len(t, np.Spec.Egress, 2)
		portRule := np.Spec.Egress[0]
		require.Len(t, portRule.Ports, len(ports))
		for i, p := range ports {
			assert.Equal(t, intstr.IntOrString{IntVal: p.port}, *portRule.Ports[i].Port)
			assert.Equal(t, p.protocol, *portRule.Ports[i].Protocol)
		}
	})

	t.Run("egress namespace rule allows same app and env", func(t *testing.T) {
		nsRule := np.Spec.Egress[1]
		require.Len(t, nsRule.To, 1)
		require.NotNil(t, nsRule.To[0].NamespaceSelector)
		assert.Equal(t, appName, nsRule.To[0].NamespaceSelector.MatchLabels[kube.RadixAppLabel])
		assert.Equal(t, env, nsRule.To[0].NamespaceSelector.MatchLabels[kube.RadixEnvLabel])
	})

	t.Run("no ingress rules", func(t *testing.T) {
		assert.Empty(t, np.Spec.Ingress)
	})

	t.Run("empty ports produces empty egress port list", func(t *testing.T) {
		np2 := allowEgressNetworkByPortPolicy(policyName, targetLabelKey, targetLabelValue, appName, env, owner, nil)
		require.Len(t, np2.Spec.Egress, 2)
		assert.Empty(t, np2.Spec.Egress[0].Ports)
	})
}

func Test_createSelector(t *testing.T) {
	podLabels := map[string]string{"app": "web"}
	nsLabels := map[string]string{"env": "prod"}

	peer := createSelector(podLabels, nsLabels)

	t.Run("pod selector", func(t *testing.T) {
		require.NotNil(t, peer.PodSelector)
		assert.Equal(t, podLabels, peer.PodSelector.MatchLabels)
	})

	t.Run("namespace selector", func(t *testing.T) {
		require.NotNil(t, peer.NamespaceSelector)
		assert.Equal(t, nsLabels, peer.NamespaceSelector.MatchLabels)
	})
}
