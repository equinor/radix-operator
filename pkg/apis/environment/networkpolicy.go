package environment

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	rx "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
	"strings"
)

const (
	allowEgressToKubeDns            = "allow-egress-to-kube-dns"
	userDefinedEgressRuleNamePrefix = "user-defined-egress-rule-"
)

func (envObject *Environment) updateEgressRules() error {
	appName := envObject.GetConfig().Spec.AppName
	env := envObject.GetConfig().Spec.EnvName

	ns := utils.GetEnvironmentNamespace(appName, env)

	// cleaning up all user defined egress policies before applying new ones
	n, err := deleteUserDefinedEgressPolicies(envObject, appName, ns, env)
	if err != nil {
		return err
	}
	envObject.logger.Debugf("Deleted %d existing user defined egress policies in ns %s before applying new ones", n, ns)

	// if there are _no_ egress rules defined in radixconfig, we also clean up the default egress policy which allows
	// DNS. This is because if there are one or more egress policies applied to the namespace, the default behaviour
	// is to block all other traffic.
	if len(envObject.config.Spec.EgressRules) == 0 {
		err = deleteDefaultDnsEgressPolicy(envObject, ns)
		if err != nil {
			return err
		}
		return nil
	}

	// creating user defined egress objects as specified in radixconfig
	userDefinedEgressPolicies := createEgressPolicies(envObject, appName, env)

	// applying the policies from previous step
	err = applyEgressPolicies(envObject, userDefinedEgressPolicies, ns)
	if err != nil {
		return err
	}

	// creating default policy object allowing DNS egress
	dnsEgressPolicy := []*v1.NetworkPolicy{createAllowKubeDnsEgressPolicy(appName, env)}

	// applying DNS egress policy object
	err = applyEgressPolicies(envObject, dnsEgressPolicy, ns)
	if err != nil {
		return err
	}
	envObject.logger.Debugf("Applied default DNS allow egress policy to ns %s", ns)

	return nil
}

func deleteDefaultDnsEgressPolicy(envObject *Environment, ns string) error {
	existingDnsEgressPolicy, err := envObject.kubeclient.NetworkingV1().NetworkPolicies(ns).Get(
		context.TODO(), allowEgressToKubeDns, metav1.GetOptions{},
	)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			envObject.logger.Debugf("No egress rules defined in radixconfig.yaml. No existing default DNS egress policy in ns %s. Nothing to delete.", ns)
			return nil
		}
		return err
	}

	err = envObject.kubeclient.NetworkingV1().NetworkPolicies(ns).Delete(context.TODO(), existingDnsEgressPolicy.GetName(), metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	envObject.logger.Debugf(
		"Deleted existing default DNS egress policy in ns %s because egress rules in radiconfig are empty", ns,
	)
	return nil
}

func deleteUserDefinedEgressPolicies(envObject *Environment, appName string, ns string, env string) (int, error) {
	nrOfDeletedPolicies := 0
	existingPolicies, err := envObject.kubeclient.NetworkingV1().NetworkPolicies(ns).List(
		context.TODO(), metav1.ListOptions{
			LabelSelector: labels.Set(metav1.LabelSelector{
				MatchLabels: map[string]string{
					kube.RadixAppLabel:                      appName,
					kube.RadixEnvLabel:                      env,
					kube.RadixUserDefinedNetworkPolicyLabel: strconv.FormatBool(true),
				},
			}.MatchLabels).String(),
		},
	)

	if err != nil {
		return nrOfDeletedPolicies, err
	}

	for _, policy := range existingPolicies.Items {
		err = envObject.kubeclient.NetworkingV1().NetworkPolicies(ns).Delete(context.TODO(), policy.GetName(), metav1.DeleteOptions{})
		nrOfDeletedPolicies++
		if err != nil {
			return nrOfDeletedPolicies, err
		}
	}

	return nrOfDeletedPolicies, nil
}

func applyEgressPolicies(envObject *Environment, networkPolicies []*v1.NetworkPolicy, ns string) error {
	for _, networkPolicy := range networkPolicies {
		_, err := envObject.kubeclient.NetworkingV1().NetworkPolicies(ns).Create(context.TODO(), networkPolicy, metav1.CreateOptions{})
		if k8serrors.IsAlreadyExists(err) {
			_, err = envObject.kubeclient.NetworkingV1().NetworkPolicies(ns).Update(context.TODO(), networkPolicy, metav1.UpdateOptions{})
		}
		if err != nil {
			return err
		}
		envObject.logger.Debugf("Successfully inserted %s to ns %s", networkPolicy.Name, ns)
	}
	return nil
}

func createEgressPolicies(envObject *Environment, appName string, env string) []*v1.NetworkPolicy {
	var networkPolicies []*v1.NetworkPolicy

	// Iterating over egress rules in radixconfig, making one k8s policy for each
	for i, egressRule := range envObject.config.Spec.EgressRules {
		networkPolicy := createEgressPolicy(appName, env, i, egressRule, true)
		networkPolicies = append(networkPolicies, networkPolicy)
	}
	return networkPolicies
}

func createAllowKubeDnsEgressPolicy(appName string, env string) *v1.NetworkPolicy {
	var tcp = corev1.Protocol("TCP")
	var udp = corev1.Protocol("UDP")
	np := v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf(allowEgressToKubeDns),
			Labels: map[string]string{
				kube.RadixAppLabel:                      appName,
				kube.RadixEnvLabel:                      env,
				kube.RadixUserDefinedNetworkPolicyLabel: strconv.FormatBool(false),
			},
		},
		Spec: v1.NetworkPolicySpec{
			PolicyTypes: []v1.PolicyType{"Egress"},
			Egress: []v1.NetworkPolicyEgressRule{
				{
					To: []v1.NetworkPolicyPeer{{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"name": "kube-system"},
						},
					},
					},
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
				},
			},
		},
	}
	return &np
}

func createEgressPolicy(appName string, env string, i int, egressRule rx.EgressRule, isUserDefined bool) *v1.NetworkPolicy {
	var networkPolicyPeers []v1.NetworkPolicyPeer
	var networkPolicyPorts []v1.NetworkPolicyPort

	for _, destination := range egressRule.Destinations {
		networkPolicyPeers = append(networkPolicyPeers, v1.NetworkPolicyPeer{
			PodSelector:       nil,
			NamespaceSelector: nil,
			IPBlock: &v1.IPBlock{
				CIDR: destination,
			},
		})
	}

	for _, port := range egressRule.Ports {
		var protocol = corev1.Protocol(strings.ToUpper(port.Protocol))
		var portNr = port.Number
		networkPolicyPorts = append(networkPolicyPorts, v1.NetworkPolicyPort{
			Protocol: &protocol,
			Port: &intstr.IntOrString{
				IntVal: portNr,
			},
		})
	}

	np := v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s%02d", userDefinedEgressRuleNamePrefix, i),
			Labels: map[string]string{
				kube.RadixAppLabel:                      appName,
				kube.RadixEnvLabel:                      env,
				kube.RadixUserDefinedNetworkPolicyLabel: strconv.FormatBool(isUserDefined),
			},
		},
		Spec: v1.NetworkPolicySpec{
			PolicyTypes: []v1.PolicyType{"Egress"},
			Egress: []v1.NetworkPolicyEgressRule{
				{
					To:    networkPolicyPeers,
					Ports: networkPolicyPorts,
				},
			},
		},
	}
	return &np
}
