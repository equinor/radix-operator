package application

import (
	"context"
	"github.com/equinor/radix-operator/radix-operator/common"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	informers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

const (
	clusterName       = "AnyClusterName"
	dnsZone           = "dev.radix.equinor.com"
	containerRegistry = "any.container.registry"
)

const (
	testControllerSyncTimeout = 5 * time.Second
)

type controllerTestSuite struct {
	suite.Suite
	kubeClient           *fake.Clientset
	radixClient          *fakeradix.Clientset
	promClient           *prometheusfake.Clientset
	kubeUtil             *kube.Kube
	eventRecorder        *record.FakeRecorder
	radixInformerFactory informers.SharedInformerFactory
	kubeInformerFactory  kubeinformers.SharedInformerFactory
	mockCtrl             *gomock.Controller
	handler              *common.MockHandler
	synced               chan bool
	stop                 chan struct{}
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (s *controllerTestSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient)
	s.promClient = prometheusfake.NewSimpleClientset()
	s.eventRecorder = &record.FakeRecorder{}
	s.radixInformerFactory = informers.NewSharedInformerFactory(s.radixClient, 0)
	s.kubeInformerFactory = kubeinformers.NewSharedInformerFactory(s.kubeClient, 0)
	s.mockCtrl = gomock.NewController(s.T())
	s.handler = common.NewMockHandler(s.mockCtrl)
	s.synced = make(chan bool)
	s.stop = make(chan struct{})
}

func (s *controllerTestSuite) TearDownTest() {
	close(s.synced)
	close(s.stop)
	s.mockCtrl.Finish()
}

func syncedChannelCallback(synced chan<- bool) func(namespace, name string, eventRecorder record.EventRecorder) error {
	return func(namespace, name string, eventRecorder record.EventRecorder) error {
		synced <- true
		return nil
	}
}

var synced chan bool

func setupTest() (*test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	client := fake.NewSimpleClientset()
	radixClient := fakeradix.NewSimpleClientset()
	kubeUtil, _ := kube.New(client, radixClient)

	handlerTestUtils := test.NewTestUtils(client, radixClient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry)
	return &handlerTestUtils, client, kubeUtil, radixClient
}

func (s *controllerTestSuite) Test_Controller_Calls_Handler() {
	anyAppName := "test-app"

	// Setup
	tu, client, kubeUtil, radixClient := setupTest()

	client.CoreV1().Namespaces().Create(
		context.TODO(),
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: utils.GetAppNamespace(anyAppName),
				Labels: map[string]string{
					kube.RadixAppLabel: anyAppName,
					kube.RadixEnvLabel: "app",
				},
			},
		},
		metav1.CreateOptions{})

	stop := make(chan struct{})
	synced := make(chan bool)

	defer close(stop)
	defer close(synced)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	radixInformerFactory := informers.NewSharedInformerFactory(radixClient, 0)

	applicationHandler := NewHandler(
		client,
		kubeUtil,
		radixClient,
		func(syncedOk bool) {
			synced <- syncedOk
		},
	)
	go startApplicationController(client, kubeUtil, radixClient, radixInformerFactory, kubeInformerFactory, applicationHandler, stop)

	// Test

	// Create registration should sync
	tu.ApplyApplication(
		utils.ARadixApplication().
			WithAppName(anyAppName).
			WithEnvironment("dev", "master"))

	op, ok := <-synced
	assert.True(s.T(), ok)
	assert.True(s.T(), op)
}

func startApplicationController(
	client kubernetes.Interface,
	kubeutil *kube.Kube,
	radixClient radixclient.Interface,
	radixInformerFactory informers.SharedInformerFactory,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	handler Handler, stop chan struct{}) {
	eventRecorder := &record.FakeRecorder{}

	waitForChildrenToSync := false
	controller := NewController(client, kubeutil, radixClient, &handler,
		kubeInformerFactory, radixInformerFactory, waitForChildrenToSync, eventRecorder)

	kubeInformerFactory.Start(stop)
	radixInformerFactory.Start(stop)
	controller.Run(1, stop)

}
