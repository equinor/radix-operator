package kube

import (
	"context"
	"fmt"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strconv"
)

func (kubeutil *Kube) ListUserDefinedNetworkPolicies(appName string, env string) (*v1.NetworkPolicyList, error) {
	ns := fmt.Sprintf("%s-%s", appName, env)
	labelsMap := map[string]string{
		RadixAppLabel:                      appName,
		RadixEnvLabel:                      env,
		RadixUserDefinedNetworkPolicyLabel: strconv.FormatBool(true),
	}
	return kubeutil.ListNetworkPoliciesByLabels(ns, labelsMap)
}

func (kubeutil *Kube) ListNetworkPoliciesByLabels(ns string, mapLabels map[string]string) (*v1.NetworkPolicyList, error) {
	networkPolicies, err := kubeutil.kubeClient.NetworkingV1().NetworkPolicies(ns).List(
		context.TODO(), metav1.ListOptions{
			LabelSelector: labels.Set(metav1.LabelSelector{
				MatchLabels: mapLabels,
			}.MatchLabels).String(),
		},
	)

	if err != nil {
		return networkPolicies, err
	}
	return networkPolicies, nil
}
