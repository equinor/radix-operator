package deployment

import (
	"context"
	"errors"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (deploy *Deployment) setDefaultNetworkPolicies(ctx context.Context) error {
	appName := deploy.registration.GetName()
	env := deploy.radixDeployment.Spec.Environment

	ns := utils.GetEnvironmentNamespace(appName, env)
	owner := []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}

	networkPolicies := []*v1.NetworkPolicy{
		defaultIngressNetworkPolicy(appName, env, owner, deploy.config.Gateway.Name),
		allowJobSchedulerServerEgressNetworkPolicy(appName, env, owner, deploy.config.DeploymentSyncer.KubernetesAPIPort),
		allowOauthAuxComponentEgressNetworkPolicy(appName, env, owner),
	}

	var errs []error
	for _, networkPolicy := range networkPolicies {
		err := deploy.kubeutil.ApplyNetworkPolicy(ctx, networkPolicy, ns)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// ref https://github.com/ahmetb/kubernetes-network-policy-recipes/blob/master/04-deny-traffic-from-other-namespaces.md
func defaultIngressNetworkPolicy(appName, env string, owner []metav1.OwnerReference, gatewayName string) *v1.NetworkPolicy {
	np := v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "radix-deny-traffic-from-other-ns",
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
				kube.RadixEnvLabel: env,
			},
			OwnerReferences: owner,
		},
		Spec: v1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress: []v1.NetworkPolicyIngressRule{
				{
					From: []v1.NetworkPolicyPeer{
						// active all pod in the namespace this nsp is created in
						{
							PodSelector: &metav1.LabelSelector{},
						},
						// namespace hosting prometheus and ingress-nginx need label "purpose:radix-base-ns"
						// TODO: Make this configurable in helm values
						createSelector(map[string]string{"app.kubernetes.io/name": "ingress-nginx"}, map[string]string{"purpose": "radix-base-ns"}),
						createSelector(map[string]string{"app.kubernetes.io/name": "prometheus"}, map[string]string{"purpose": "radix-base-ns"}),
						createSelector(map[string]string{"gateway.networking.k8s.io/gateway-name": gatewayName}, map[string]string{"purpose": "radix-base-ns"}),
					},
				},
			},
		},
	}
	return &np
}

func allowOauthAuxComponentEgressNetworkPolicy(appName string, env string, owner []metav1.OwnerReference) *v1.NetworkPolicy {
	// We allow outbound to entire Internet from the Oauth aux component pods.
	// This is because egress rule must allow traffic to the login.microsoftonline.com FQDN.
	// This FQDN has IP ranges 20.190.128.0/18 and 40.126.0.0/18 as of April 2022,
	// but may change at some point in the future.
	return allowEgressNetworkByPortPolicy("radix-allow-oauth-aux-egress", kube.RadixAuxiliaryComponentTypeLabel, radixv1.OAuthProxyAuxiliaryComponentType, appName, env, owner, []egreessPortPolicy{
		{port: 53, protocol: corev1.ProtocolTCP},
		{port: 53, protocol: corev1.ProtocolUDP},
		{port: 443, protocol: corev1.ProtocolTCP},
		{port: 6379, protocol: corev1.ProtocolTCP}, // Redis Plain
		{port: 6380, protocol: corev1.ProtocolTCP}, // Redis TLS
	})
}

func allowJobSchedulerServerEgressNetworkPolicy(appName string, env string, owner []metav1.OwnerReference, kubernetesApiPort int32) *v1.NetworkPolicy {
	// We allow outbound to entire Internet from the job scheduler server pods.
	// This is because egress rule must allow traffic to public IP of k8s API server,
	// and the public IP is dynamic.
	return allowEgressNetworkByPortPolicy("radix-allow-job-scheduler-egress", kube.RadixPodIsJobSchedulerLabel, "true", appName, env, owner, []egreessPortPolicy{
		{port: 53, protocol: corev1.ProtocolTCP},
		{port: 53, protocol: corev1.ProtocolUDP},
		{port: kubernetesApiPort, protocol: corev1.ProtocolTCP},
	})
}

type egreessPortPolicy struct {
	port     int32
	protocol corev1.Protocol
}

func allowEgressNetworkByPortPolicy(policyName string, targetLabelKey string, targetLabelValue string, appName string, env string, owner []metav1.OwnerReference, egressPorts []egreessPortPolicy) *v1.NetworkPolicy {
	var egressPortsV1 []v1.NetworkPolicyPort
	for _, port := range egressPorts {
		egressPortsV1 = append(egressPortsV1, v1.NetworkPolicyPort{Port: &intstr.IntOrString{IntVal: port.port}, Protocol: &port.protocol})
	}

	np := v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
				kube.RadixEnvLabel: env,
			},
			OwnerReferences: owner,
		},
		Spec: v1.NetworkPolicySpec{
			PolicyTypes: []v1.PolicyType{v1.PolicyTypeEgress},
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					targetLabelKey: targetLabelValue,
				},
			},
			Egress: []v1.NetworkPolicyEgressRule{
				{
					Ports: egressPortsV1,
				},
				{
					To: []v1.NetworkPolicyPeer{{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								kube.RadixAppLabel: appName,
								kube.RadixEnvLabel: env,
							},
						},
					}},
				},
			},
		},
	}
	return &np
}

func createSelector(podSelector, namespaceSelector map[string]string) v1.NetworkPolicyPeer {
	return v1.NetworkPolicyPeer{
		PodSelector: &metav1.LabelSelector{
			MatchLabels: podSelector,
		},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: namespaceSelector,
		},
	}
}
