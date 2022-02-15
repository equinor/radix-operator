package utils

import (
	rx "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"
)

func ConvertRadixRules(radixEgressRules []rx.EgressRule) []v1.NetworkPolicyEgressRule {
	var egressRules []v1.NetworkPolicyEgressRule

	for _, radixEgressRule := range radixEgressRules {
		egressRule := ConvertRadixRule(radixEgressRule)
		egressRules = append(egressRules, egressRule)

	}
	return egressRules
}

func ConvertRadixRule(radixEgressRule rx.EgressRule) (egressRule v1.NetworkPolicyEgressRule) {

	var ports []v1.NetworkPolicyPort
	var cidrs []v1.NetworkPolicyPeer

	for _, radixPort := range radixEgressRule.Ports {
		portProtocol := corev1.Protocol(strings.ToUpper(radixPort.Protocol))
		ports = append(ports, v1.NetworkPolicyPort{
			Protocol: &portProtocol,
			Port: &intstr.IntOrString{
				IntVal: radixPort.Number,
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
