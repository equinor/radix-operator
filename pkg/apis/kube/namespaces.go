package kube

import (
	"context"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func (kubeutil *Kube) CreateNamespace(ctx context.Context, namespace *corev1.Namespace) (*corev1.Namespace, error) {
	created, err := kubeutil.kubeClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	log.Ctx(ctx).Info().Msgf("Created namespace %s", created.Name)
	return created, err
}

// UpdateNamespace updates the `modified` namespace.
// If `original` is set, the two namespaces are compared, and the namespace is only updated if they are not equal.
func (kubeutil *Kube) UpdateNamespace(ctx context.Context, original, modified *corev1.Namespace) error {
	if original != nil && equality.Semantic.DeepEqual(original, modified) {
		log.Ctx(ctx).Debug().Msgf("No need to update namespace %s", modified.Name)
		return nil
	}

	updated, err := kubeutil.kubeClient.CoreV1().Namespaces().Update(ctx, modified, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Updated namespace %s", updated.Name)
	return err
}

// ListNamespacesWithSelector List namespaces with selector
func (kubeutil *Kube) ListNamespacesWithSelector(ctx context.Context, labelSelectorString string) ([]*corev1.Namespace, error) {
	if kubeutil.NamespaceLister != nil {
		selector, err := kubelabels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}
		return kubeutil.NamespaceLister.List(selector)
	}

	list, err := kubeutil.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: labelSelectorString})
	if err != nil {
		return nil, err
	}

	return slice.PointersOf(list.Items).([]*corev1.Namespace), nil
}

// GetEnvNamespacesForApp Get all env namespaces for an application
func (kubeutil *Kube) GetEnvNamespacesForApp(ctx context.Context, appName string) ([]*corev1.Namespace, error) {
	return kubeutil.ListNamespacesWithSelector(ctx, envNamespacesLabelForApp(appName).String())
}

func envNamespacesLabelForApp(appName string) kubelabels.Selector {
	return kubelabels.NewSelector().
		Add(*requirementRadixAppNameLabel(appName)).
		Add(*requirementNotRadixAppNamespaceLabel())
}

func requirementRadixAppNameLabel(appName string) *kubelabels.Requirement {
	requirement, err := kubelabels.NewRequirement(RadixAppLabel, selection.Equals, []string{appName})
	if err != nil {
		panic(err)
	}
	return requirement
}

func requirementNotRadixAppNamespaceLabel() *kubelabels.Requirement {
	requirement, err := kubelabels.NewRequirement(RadixEnvLabel, selection.NotEquals, []string{"app"})
	if err != nil {
		panic(err)
	}
	return requirement
}
