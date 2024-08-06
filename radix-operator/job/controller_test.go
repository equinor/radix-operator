package job

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	jobs "github.com/equinor/radix-operator/pkg/apis/job"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/golang/mock/gomock"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type jobTestSuite struct {
	suite.Suite
	promClient *prometheusfake.Clientset
	kubeUtil   *kube.Kube
	tu         test.Utils
}

func TestJobTestSuite(t *testing.T) {
	suite.Run(t, new(jobTestSuite))
}

func (s *jobTestSuite) SetupSuite() {
	test.SetRequiredEnvironmentVariables()
}

func (s *jobTestSuite) SetupTest() {
	secretProviderClient := secretproviderfake.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(fake.NewSimpleClientset(), fakeradix.NewSimpleClientset(), kedaClient, secretProviderClient)
	s.promClient = prometheusfake.NewSimpleClientset()
	s.tu = test.NewTestUtils(s.kubeUtil.KubeClient(), s.kubeUtil.RadixClient(), kedaClient, secretProviderClient)
	err := s.tu.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	s.Require().NoError(err)
}

func (s *jobTestSuite) TearDownTest() {
	os.Unsetenv(defaults.OperatorRollingUpdateMaxUnavailable)
	os.Unsetenv(defaults.OperatorRollingUpdateMaxSurge)
	os.Unsetenv(defaults.OperatorReadinessProbeInitialDelaySeconds)
	os.Unsetenv(defaults.OperatorReadinessProbePeriodSeconds)
}

func (s *jobTestSuite) Test_Controller_Calls_Handler() {
	anyAppName := "test-app"

	ctx, stop := context.WithCancel(context.Background())
	synced := make(chan bool)

	defer stop()
	defer close(synced)

	hasSynced := func(syncedOk bool) { synced <- syncedOk }
	ctrl := gomock.NewController(s.T())
	mockHistory := NewMockHistory(ctrl)
	withHistoryOption := func(h *handler) { h.jobHistory = mockHistory }
	jobHandler := s.createHandler(hasSynced, withHistoryOption)

	go func() {
		err := s.startJobController(ctx, jobHandler)
		s.Require().NoError(err)
	}()

	mockHistory.EXPECT().Cleanup(gomock.Any(), anyAppName).Times(1)

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
	_, err := s.kubeUtil.RadixClient().RadixV1().RadixJobs(rj.ObjectMeta.Namespace).Update(context.Background(), rj, metav1.UpdateOptions{})
	s.Require().NoError(err)

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
	_, err = s.kubeUtil.KubeClient().BatchV1().Jobs(rj.ObjectMeta.Namespace).Create(context.Background(), &childJob, metav1.CreateOptions{})
	s.Require().NoError(err)

	childJob.ObjectMeta.ResourceVersion = "1234"
	_, err = s.kubeUtil.KubeClient().BatchV1().Jobs(rj.ObjectMeta.Namespace).Update(context.Background(), &childJob, metav1.UpdateOptions{})
	s.Require().NoError(err)

	op, ok = <-synced
	s.True(ok)
	s.True(op)
}

func (s *jobTestSuite) createHandler(hasSynced func(syncedOk bool), opts ...handlerOpts) Handler {
	return NewHandler(
		s.kubeUtil.KubeClient(),
		s.kubeUtil,
		s.kubeUtil.RadixClient(),
		createConfig(),
		hasSynced,
		opts...,
	)
}

func createConfig() *config.Config {
	return &config.Config{
		DNSConfig: &dnsalias.DNSConfig{
			DNSZone:               "dev.radix.equinor.com",
			ReservedAppDNSAliases: map[string]string{"api": "radix-api"},
			ReservedDNSAliases:    []string{"grafana"},
		},
		PipelineJobConfig: &pipelinejob.Config{
			PipelineJobsHistoryLimit:          3,
			AppBuilderResourcesRequestsCPU:    pointers.Ptr(resource.MustParse("100m")),
			AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1000Mi")),
			AppBuilderResourcesLimitsMemory:   pointers.Ptr(resource.MustParse("2000Mi")),
		},
	}
}

func (s *jobTestSuite) startJobController(ctx context.Context, handler Handler) error {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(s.kubeUtil.KubeClient(), 0)
	radixInformerFactory := informers.NewSharedInformerFactory(s.kubeUtil.RadixClient(), 0)
	eventRecorder := &record.FakeRecorder{}
	const waitForChildrenToSync = false
	controller := NewController(ctx, s.kubeUtil.KubeClient(), s.kubeUtil.RadixClient(), handler, kubeInformerFactory, radixInformerFactory, waitForChildrenToSync, eventRecorder)
	kubeInformerFactory.Start(ctx.Done())
	radixInformerFactory.Start(ctx.Done())
	return controller.Run(ctx, 4)
}
