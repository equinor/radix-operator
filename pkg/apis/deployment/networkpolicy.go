package deployment

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) denyTrafficFromOtherNamespaces() error {
	appName := deploy.registration.GetName()
	env := deploy.radixDeployment.Spec.Environment

	ns := utils.GetEnvironmentNamespace(appName, env)
	owner := getOwnerReferenceOfDeployment(deploy.radixDeployment)

	networkPolicy := defaultNetworkPolicy(appName, env, owner)

	_, err := deploy.kubeclient.NetworkingV1().NetworkPolicies(ns).Create(networkPolicy)
	if errors.IsAlreadyExists(err) {
		_, err = deploy.kubeclient.NetworkingV1().NetworkPolicies(ns).Update(networkPolicy)
	}

	if err != nil {
		return err
	}
	return nil
}

// ref https://github.com/ahmetb/kubernetes-network-policy-recipes/blob/master/04-deny-traffic-from-other-namespaces.md
func defaultNetworkPolicy(appName, env string, owner []metav1.OwnerReference) *v1.NetworkPolicy {
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
				v1.NetworkPolicyIngressRule{
					From: []v1.NetworkPolicyPeer{
						// active all pod in the namespace this nsp is created in
						v1.NetworkPolicyPeer{
							PodSelector: &metav1.LabelSelector{},
						},
						// default namespace need label "purpose:common"
						createSelector(map[string]string{"app": "nginx-ingress"}, map[string]string{"purpose": "common"}),
						createSelector(map[string]string{"app": "prometheus"}, map[string]string{"purpose": "common"}),
					},
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
