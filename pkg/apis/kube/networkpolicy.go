package kube

import (
	"context"
	"fmt"
	v1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strconv"
)

// ListUserDefinedNetworkPolicies Returns list of user defined network policies
func (kubeutil *Kube) ListUserDefinedNetworkPolicies(appName string, env string) (*v1.NetworkPolicyList, error) {
	ns := fmt.Sprintf("%s-%s", appName, env)
	labelsMap := map[string]string{
		RadixAppLabel:                      appName,
		RadixEnvLabel:                      env,
		RadixUserDefinedNetworkPolicyLabel: strconv.FormatBool(true),
	}
	return kubeutil.listNetworkPoliciesByLabels(ns, labelsMap)
}

// ApplyNetworkPolicy Applies a k8s network policy to specified namespace
func (kubeutil *Kube) ApplyNetworkPolicy(networkPolicy *v1.NetworkPolicy, ns string) error {
	_, err := kubeutil.kubeClient.NetworkingV1().NetworkPolicies(ns).Create(context.TODO(), networkPolicy, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		_, err = kubeutil.kubeClient.NetworkingV1().NetworkPolicies(ns).Update(context.TODO(), networkPolicy, metav1.UpdateOptions{})
	}
	return err
}

func (kubeutil *Kube) listNetworkPoliciesByLabels(ns string, mapLabels map[string]string) (*v1.NetworkPolicyList, error) {
	return kubeutil.kubeClient.NetworkingV1().NetworkPolicies(ns).List(
		context.TODO(), metav1.ListOptions{
			LabelSelector: labels.Set(metav1.LabelSelector{
				MatchLabels: mapLabels,
			}.MatchLabels).String(),
		},
	)
}
