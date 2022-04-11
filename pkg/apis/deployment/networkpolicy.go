package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"os"
	"strconv"
)

func (deploy *Deployment) setDefaultNetworkPolicies() []error {
	appName := deploy.registration.GetName()
	env := deploy.radixDeployment.Spec.Environment

	ns := utils.GetEnvironmentNamespace(appName, env)
	owner := []metav1.OwnerReference{getOwnerReferenceOfDeployment(deploy.radixDeployment)}

	networkPolicies := []*v1.NetworkPolicy{
		defaultIngressNetworkPolicy(appName, env, owner),
		allowJobSchedulerServerEgressNetworkPolicy(appName, env, owner),
		allowOauthAuxComponentEgressNetworkPolicy(appName, env, owner),
		allowBatchSchedulerServerEgressNetworkPolicy(appName, env, owner),
	}

	var errs []error
	for _, networkPolicy := range networkPolicies {
		err := deploy.kubeutil.ApplyNetworkPolicy(networkPolicy, ns)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// ref https://github.com/ahmetb/kubernetes-network-policy-recipes/blob/master/04-deny-traffic-from-other-namespaces.md
func defaultIngressNetworkPolicy(appName, env string, owner []metav1.OwnerReference) *v1.NetworkPolicy {
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
						createSelector(map[string]string{"app.kubernetes.io/name": "ingress-nginx"}, map[string]string{"purpose": "radix-base-ns"}),
						createSelector(map[string]string{"app.kubernetes.io/name": "prometheus"}, map[string]string{"purpose": "radix-base-ns"}),
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
	return allowAllHttpsAndDnsEgressNetworkPolicy("radix-allow-oauth-aux-egress", kube.RadixAuxiliaryComponentTypeLabel, defaults.OAuthProxyAuxiliaryComponentType, 443, appName, env, owner)
}

func allowJobSchedulerServerEgressNetworkPolicy(appName string, env string, owner []metav1.OwnerReference) *v1.NetworkPolicy {
	// We allow outbound to entire Internet from the job scheduler server pods.
	// This is because egress rule must allow traffic to public IP of k8s API server,
	// and the public IP is dynamic.
	var kubernetesApiPort, _ = strconv.ParseInt(os.Getenv(defaults.KubernetesApiPortEnvironmentVariable), 10, 32)
	return allowAllHttpsAndDnsEgressNetworkPolicy("radix-allow-job-scheduler-egress", kube.RadixPodIsJobSchedulerLabel, "true", int32(kubernetesApiPort), appName, env, owner)
}

func allowBatchSchedulerServerEgressNetworkPolicy(appName string, env string, owner []metav1.OwnerReference) *v1.NetworkPolicy {
	// We allow outbound to entire Internet from the batch scheduler server pods.
	// This is because egress rule must allow traffic to public IP of k8s API server,
	// and the public IP is dynamic.
	var kubernetesApiPort, _ = strconv.ParseInt(os.Getenv(defaults.KubernetesApiPortEnvironmentVariable), 10, 32)
	return allowAllHttpsAndDnsEgressNetworkPolicy("radix-allow-batch-scheduler-egress", kube.RadixJobTypeLabel, kube.RadixJobTypeBatchSchedule, int32(kubernetesApiPort), appName, env, owner)
}

func allowAllHttpsAndDnsEgressNetworkPolicy(policyName string, targetLabelKey string, targetLabelValue string, portNumber int32, appName string, env string, owner []metav1.OwnerReference) *v1.NetworkPolicy {
	var tcp = corev1.ProtocolTCP
	var udp = corev1.ProtocolUDP

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
					Ports: []v1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port: &intstr.IntOrString{
								IntVal: portNumber,
							},
						},
						{
							Protocol: &tcp,
							Port: &intstr.IntOrString{
								IntVal: 53,
							},
						},
						{
							Protocol: &udp,
							Port: &intstr.IntOrString{
								IntVal: 53,
							},
						},
					},
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
