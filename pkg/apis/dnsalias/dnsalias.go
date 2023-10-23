package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// DNSAlias is the aggregate-root for manipulating RadixDNSAliases
type DNSAlias struct {
	kubeclient  kubernetes.Interface
	radixclient radixclient.Interface
	kubeutil    *kube.Kube
	config      *radixv1.RadixDNSAlias
	appConfig   *radixv1.RadixApplication
	logger      *logrus.Entry
}

// NewDNSAlias is the constructor for DNSAlias
func NewDNSAlias(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	config *radixv1.RadixDNSAlias,
	appConfig *radixv1.RadixApplication,
	logger *logrus.Entry) (DNSAlias, error) {

	return DNSAlias{
		kubeclient,
		radixclient,
		kubeutil,
		config,
		appConfig,
		logger}, nil
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (dnsAlias *DNSAlias) OnSync(time metav1.Time) error {

	// TODO
	err := dnsAlias.updateRadixDNSAliasStatus(dnsAlias.config, func(currStatus *radixv1.RadixDNSAliasStatus) {
		// time is parameterized for testability
		currStatus.Reconciled = time
	})
	if err != nil {
		return fmt.Errorf("failed to update status on DNS alias %s: %v", dnsAlias.config.GetName(), err)
	}
	dnsAlias.logger.Debugf("DNSAlias %s reconciled", dnsAlias.config.GetName())
	return nil
}

func (dnsAlias *DNSAlias) updateRadixDNSAliasStatus(rEnv *radixv1.RadixDNSAlias, changeStatusFunc func(currStatus *radixv1.RadixDNSAliasStatus)) error {
	radixDNSAliasInterface := dnsAlias.radixclient.RadixV1().RadixDNSAliases()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentEnv, err := radixDNSAliasInterface.Get(context.TODO(), rEnv.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentEnv.Status)
		_, err = radixDNSAliasInterface.UpdateStatus(context.TODO(), currentEnv, metav1.UpdateOptions{})
		if err == nil && dnsAlias.config.GetName() == rEnv.GetName() {
			currentEnv, err = radixDNSAliasInterface.Get(context.TODO(), rEnv.GetName(), metav1.GetOptions{})
			if err == nil {
				dnsAlias.config = currentEnv
			}
		}
		return err
	})
}

func (dnsAlias *DNSAlias) GetConfig() *radixv1.RadixDNSAlias {
	return dnsAlias.config
}
