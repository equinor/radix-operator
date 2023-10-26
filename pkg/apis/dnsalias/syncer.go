package dnsalias

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DNSAlias is the aggregate-root for manipulating RadixDNSAliases
type syncer struct {
	kubeClient    kubernetes.Interface
	radixClient   radixclient.Interface
	kubeUtil      *kube.Kube
	radixDNSAlias *radixv1.RadixDNSAlias
	appConfig     *radixv1.RadixApplication
	logger        *logrus.Entry
}

// NewDNSAliasSyncer is the constructor for RadixDNSAlias syncer
func NewDNSAliasSyncer(
	kubeClient kubernetes.Interface,
	kubeUtil *kube.Kube,
	radixClient radixclient.Interface,
	radixDNSAlias *radixv1.RadixDNSAlias,
	appConfig *radixv1.RadixApplication,
	logger *logrus.Entry) (syncer, error) {

	return syncer{
		kubeClient:    kubeClient,
		radixClient:   radixClient,
		kubeUtil:      kubeUtil,
		radixDNSAlias: radixDNSAlias,
		appConfig:     appConfig,
		logger:        logger}, nil
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (s *syncer) OnSync(time metav1.Time) error {

	// TODO
	err := s.updateStatus(func(currStatus *radixv1.RadixDNSAliasStatus) {
		currStatus.Reconciled = time // time is parameterized for testability
	})
	if err != nil {
		return fmt.Errorf("failed to update status on DNS alias %s: %v", s.radixDNSAlias.GetName(), err)
	}
	s.logger.Debugf("RadixDNSAlias %s reconciled", s.radixDNSAlias.GetName())
	return nil
}

// GetRadixDNSAlias Gets RadixDNSAlias under sync
func (s *syncer) GetRadixDNSAlias() *radixv1.RadixDNSAlias {
	return s.radixDNSAlias
}
