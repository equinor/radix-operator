package job

import (
	"context"
	"github.com/equinor/radix-operator/radix-operator/config"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobs "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName       = "AnyClusterName"
	containerRegistry = "any.container.registry"
	egressIps         = "0.0.0.0"
)

type jobTestSuite struct {
	suite.Suite
	promClient *prometheusfake.Clientset
	kubeUtil   *kube.Kube
	tu         test.Utils
	mockCtrl   *gomock.Controller
	config     *config.MockConfig
}

func TestJobTestSuite(t *testing.T) {
	suite.Run(t, new(jobTestSuite))
}

func (s *jobTestSuite) SetupSuite() {
	test.SetRequiredEnvironmentVariables()
}

func (s *jobTestSuite) SetupTest() {
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(fake.NewSimpleClientset(), fakeradix.NewSimpleClientset(), secretProviderClient)
	s.promClient = prometheusfake.NewSimpleClientset()
	s.tu = test.NewTestUtils(s.kubeUtil.KubeClient(), s.kubeUtil.RadixClient(), secretProviderClient)
	s.tu.CreateClusterPrerequisites(clusterName, containerRegistry, egressIps)
	s.mockCtrl = gomock.NewController(s.T())
	s.config = config.NewMockConfig(s.mockCtrl)
}

func (s *jobTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)

}

func (s *jobTestSuite) Test_Controller_Calls_Handler() {
	anyAppName := "test-app"

	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(s.kubeUtil.KubeClient(), 0)
	radixInformerFactory := informers.NewSharedInformerFactory(s.kubeUtil.RadixClient(), 0)

	jobHandler := NewHandler(
		s.kubeUtil.KubeClient(),
		s.kubeUtil,
		s.kubeUtil.RadixClient(),
		s.config,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go startJobController(s.kubeUtil.KubeClient(), s.kubeUtil.RadixClient(), radixInformerFactory, kubeInformerFactory, jobHandler, stop, s.config)

	// Test

	// Create job should sync
	rj, _ := s.tu.ApplyJob(
		utils.ARadixBuildDeployJob().
			WithAppName(anyAppName))

	op, ok := <-synced
	s.True(ok)
	s.True(op)

	// Update  radix job should sync. Controller will skip if an update
	// changes nothing, except for spec or metadata, labels or annotations
	rj.Spec.Stop = true
	s.kubeUtil.RadixClient().RadixV1().RadixJobs(rj.ObjectMeta.Namespace).Update(context.TODO(), rj, metav1.UpdateOptions{})

	op, ok = <-synced
	s.True(ok)
	s.True(op)

	// Child job should sync
	childJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: jobs.GetOwnerReference(rj),
		},
	}

	// Only update of Kubernetes Job is something that the job-controller handles
	s.kubeUtil.KubeClient().BatchV1().Jobs(rj.ObjectMeta.Namespace).Create(context.TODO(), &childJob, metav1.CreateOptions{})
	childJob.ObjectMeta.ResourceVersion = "1234"
	s.kubeUtil.KubeClient().BatchV1().Jobs(rj.ObjectMeta.Namespace).Update(context.TODO(), &childJob, metav1.UpdateOptions{})

	op, ok = <-synced
	s.True(ok)
	s.True(op)
}

func startJobController(client kubernetes.Interface, radixClient radixclient.Interface, radixInformerFactory informers.SharedInformerFactory, kubeInformerFactory kubeinformers.SharedInformerFactory, handler Handler, stop chan struct{}, config config.Config) {

	eventRecorder := &record.FakeRecorder{}

	waitForChildrenToSync := false
	controller := NewController(client, radixClient, &handler, kubeInformerFactory, radixInformerFactory, waitForChildrenToSync, eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(4, stop)

}
