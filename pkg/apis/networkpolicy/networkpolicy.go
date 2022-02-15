package networkpolicy

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	rx "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"strconv"
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

func (nw *NetworkPolicy) UpdateEnvEgressRules(radixEgressRules []rx.EgressRule, env string) error {

	ns := utils.GetEnvironmentNamespace(nw.appName, env)

	var err error
	// if there are _no_ egress rules defined in radixconfig, we delete existing policy
	if len(radixEgressRules) == 0 {
		err = nw.deleteUserDefinedEgressPolicies(ns, env)
		if err != nil {
			return err
		}
		return nil
	}

	// converting rules from radixconfig to k8s egress rules
	userDefinedEgressRules := utils.ConvertRadixRules(radixEgressRules)

	// adding kube-dns to user-defined egress rules
	egressRules := append(userDefinedEgressRules, nw.createAllowKubeDnsEgressRule())

	// combining the egress rules created from previous steps in a network policy and applying it
	egressPolicy := nw.createEgressPolicy(env, egressRules, true)
	err = nw.applyEgressPolicy(egressPolicy, ns)
	if err != nil {
		return err
	}

	return nil
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

func (nw *NetworkPolicy) applyEgressPolicy(networkPolicy *v1.NetworkPolicy, ns string) error {
	_, err := nw.kubeClient.NetworkingV1().NetworkPolicies(ns).Create(context.TODO(), networkPolicy, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		_, err = nw.kubeClient.NetworkingV1().NetworkPolicies(ns).Update(context.TODO(), networkPolicy, metav1.UpdateOptions{})
	}
	if err != nil {
		return err
	}
	nw.logger.Debugf("Successfully inserted %s to ns %s", networkPolicy.Name, ns)
	return nil
}

func (nw *NetworkPolicy) createAllowKubeDnsEgressRule() v1.NetworkPolicyEgressRule {
	var tcp = corev1.Protocol("TCP")
	var udp = corev1.Protocol("UDP")

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
			NamespaceSelector: &metav1.LabelSelector{},
		}},
	}

	return dnsEgressRule
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
			PolicyTypes: []v1.PolicyType{"Egress"},
			Egress:      egressRules,
		},
	}
	return &np
}
