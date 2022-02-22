package networkpolicy

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	rx "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"strings"
)

const (
	userDefinedEgressPolicyName = "user-defined-egress-policy"
)

type NetworkPolicy struct {
	kubeClient kubernetes.Interface
	kubeUtil   *kube.Kube
	logger     *logrus.Entry
	appName    string
}

func NewNetworkPolicy(
	kubeClient kubernetes.Interface,
	kubeUtil *kube.Kube,
	logger *logrus.Entry,
	appName string,
) (NetworkPolicy, error) {
	return NetworkPolicy{
		kubeClient: kubeClient,
		kubeUtil:   kubeUtil,
		logger:     logger,
		appName:    appName,
	}, nil
}

func convertToK8sEgressRules(radixEgressRules []rx.EgressRule) []v1.NetworkPolicyEgressRule {
	var egressRules []v1.NetworkPolicyEgressRule

	for _, radixEgressRule := range radixEgressRules {
		egressRule := convertToK8sEgressRule(radixEgressRule)
		egressRules = append(egressRules, egressRule)

	}
	return egressRules
}

func convertToK8sEgressRule(radixEgressRule rx.EgressRule) (egressRule v1.NetworkPolicyEgressRule) {

	var ports []v1.NetworkPolicyPort
	var cidrs []v1.NetworkPolicyPeer

	for _, radixPort := range radixEgressRule.Ports {
		portProtocol := corev1.Protocol(strings.ToUpper(radixPort.Protocol))
		ports = append(ports, v1.NetworkPolicyPort{
			Protocol: &portProtocol,
			Port: &intstr.IntOrString{
				IntVal: radixPort.Port,
			},
		})
	}

	for _, radixCidr := range radixEgressRule.Destinations {
		cidrs = append(cidrs, v1.NetworkPolicyPeer{
			IPBlock: &v1.IPBlock{
				CIDR: radixCidr,
			},
		})
	}

	return v1.NetworkPolicyEgressRule{
		Ports: ports,
		To:    cidrs,
	}
}

// UpdateEnvEgressRules Applies a list of egress rules to the specified radix app environment
func (nw *NetworkPolicy) UpdateEnvEgressRules(radixEgressRules []rx.EgressRule, env string) error {

	ns := utils.GetEnvironmentNamespace(nw.appName, env)

	// if there are _no_ egress rules defined in radixconfig, we delete existing policy
	if len(radixEgressRules) == 0 {
		return nw.deleteUserDefinedEgressPolicies(ns, env)
	}

	userDefinedEgressRules := convertToK8sEgressRules(radixEgressRules)

	// adding kube-dns and own namespace to user-defined egress rules
	egressRules := append(userDefinedEgressRules,
		nw.createAllowKubeDnsEgressRule(),
		nw.createAllowOwnNamespaceEgressRule(env),
	)

	egressPolicy := nw.createEgressPolicy(env, egressRules, true)

	err := nw.kubeUtil.ApplyEgressPolicy(egressPolicy, ns)
	if err == nil {
		nw.logger.Debugf("Successfully inserted %s to ns %s", egressPolicy.Name, ns)
	}
	return err
}

func (nw *NetworkPolicy) deleteUserDefinedEgressPolicies(ns string, env string) error {
	existingPolicies, err := nw.kubeUtil.ListUserDefinedNetworkPolicies(nw.appName, env)

	for _, policy := range existingPolicies.Items {
		err = nw.kubeClient.NetworkingV1().NetworkPolicies(ns).Delete(context.TODO(), policy.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (nw *NetworkPolicy) createAllowKubeDnsEgressRule() v1.NetworkPolicyEgressRule {
	var tcp = corev1.ProtocolTCP
	var udp = corev1.ProtocolUDP

	dnsEgressRule := v1.NetworkPolicyEgressRule{
		Ports: []v1.NetworkPolicyPort{
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
		To: []v1.NetworkPolicyPeer{{
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{kube.K8sAppLabel: "kube-dns"},
			},
			// empty namespaceSelector is necessary for podSelector to work
			NamespaceSelector: &metav1.LabelSelector{},
		}},
	}

	return dnsEgressRule
}

func (nw *NetworkPolicy) createAllowOwnNamespaceEgressRule(env string) v1.NetworkPolicyEgressRule {
	ownNamespaceEgressRule := v1.NetworkPolicyEgressRule{
		Ports: nil, // allowing all ports
		To: []v1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					kube.RadixAppLabel: nw.appName,
					kube.RadixEnvLabel: env,
				},
			},
		}},
	}

	return ownNamespaceEgressRule
}

func (nw *NetworkPolicy) createEgressPolicy(env string, egressRules []v1.NetworkPolicyEgressRule, isUserDefined bool) *v1.NetworkPolicy {
	np := v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: userDefinedEgressPolicyName,
			Labels: map[string]string{
				kube.RadixAppLabel:                      nw.appName,
				kube.RadixEnvLabel:                      env,
				kube.RadixUserDefinedNetworkPolicyLabel: strconv.FormatBool(isUserDefined),
			},
		},
		Spec: v1.NetworkPolicySpec{
			PolicyTypes: []v1.PolicyType{v1.PolicyTypeEgress},
			Egress:      egressRules,
		},
	}
	return &np
}
