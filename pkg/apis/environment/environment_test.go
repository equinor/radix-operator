package environment

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/networkpolicy"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	tektonclientfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName                 = "AnyClusterName"
	containerRegistry           = "any.container.registry"
	envConfigFileName           = "./testdata/re.yaml"
	egressRuleEnvConfigFileName = "./testdata/re_egress.yaml"
	regConfigFileName           = "./testdata/rr.yaml"
	namespaceName               = "testapp-testenv"
	egressIps                   = "0.0.0.0"

	limitDefaultCPU          = "432m" // 0.432
	limitDefaultReqestCPU    = "234m" // 0.234
	limitDefaultMemory       = "321M" // 321'000'000
	limitDefaultReqestMemory = "123M" // 123'000'000
)

func setupTest() (test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	fakekube := fake.NewSimpleClientset()
	fakeradix := radix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	tektonclient := tektonclientfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(fakekube, fakeradix, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(fakekube, fakeradix, secretproviderclient, tektonclient)
	handlerTestUtils.CreateClusterPrerequisites(clusterName, containerRegistry, egressIps)

	os.Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, limitDefaultCPU)
	os.Setenv(defaults.OperatorEnvLimitDefaultReqestCPUEnvironmentVariable, limitDefaultReqestCPU)
	os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, limitDefaultMemory)
	os.Setenv(defaults.OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable, limitDefaultReqestMemory)

	return handlerTestUtils, fakekube, kubeUtil, fakeradix
}

func newEnv(client kubernetes.Interface, kubeUtil *kube.Kube, radixclient radixclient.Interface, radixEnvFileName string) (*v1.RadixRegistration, *v1.RadixEnvironment, Environment) {
	rr, _ := utils.GetRadixRegistrationFromFile(regConfigFileName)
	re, _ := utils.GetRadixEnvironmentFromFile(radixEnvFileName)
	logger := logrus.WithFields(logrus.Fields{"environmentName": namespaceName})
	nw, _ := networkpolicy.NewNetworkPolicy(client, kubeUtil, logger, re.Spec.AppName)
	env, _ := NewEnvironment(client, kubeUtil, radixclient, re, rr, nil, logger, &nw)
	// register instance with radix-client so UpdateStatus() can find it
	radixclient.RadixV1().RadixEnvironments().Create(context.TODO(), re, meta.CreateOptions{})
	return rr, re, env
}

func Test_Create_Namespace(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest()
	rr, _, env := newEnv(client, kubeUtil, radixclient, envConfigFileName)

	sync(t, &env)

	namespaces, _ := client.CoreV1().Namespaces().List(context.TODO(), meta.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, rr.Name),
	})

	commonAsserts(t, env, namespacesAsMeta(namespaces.Items), namespaceName)
}

func Test_Create_EgressRules(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest()
	rr, _, env := newEnv(client, kubeUtil, radixclient, egressRuleEnvConfigFileName)

	sync(t, &env)

	namespaces, _ := client.CoreV1().Namespaces().List(context.TODO(), meta.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, rr.Name),
	})

	t.Run("Egress rules are correct", func(t *testing.T) {
		egressRules := env.config.Spec.EgressRules
		assert.Len(t, egressRules, 1)
		assert.Equal(t, egressRules[0].Destinations[0], "195.88.55.16/32")
		assert.Len(t, egressRules[0].Ports, 2)
	})

	commonAsserts(t, env, namespacesAsMeta(namespaces.Items), namespaceName)
}

func Test_Create_RoleBinding(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest()
	rr, _, env := newEnv(client, kubeUtil, radixclient, envConfigFileName)

	sync(t, &env)

	rolebindings, _ := client.RbacV1().RoleBindings(namespaceName).List(context.TODO(), meta.ListOptions{})

	commonAsserts(t, env, roleBindingsAsMeta(rolebindings.Items), "radix-app-admin-envs")

	adGroupName := rr.Spec.AdGroups[0]
	t.Run("It contains the correct AD groups", func(t *testing.T) {
		subjects := rolebindings.Items[0].Subjects
		assert.Len(t, subjects, 1)
		assert.Equal(t, adGroupName, subjects[0].Name)
	})

	t.Run("It contains machine-user", func(t *testing.T) {
		rr.Spec.MachineUser = true
		sync(t, &env)
		rolebindings, _ = client.RbacV1().RoleBindings(namespaceName).List(context.TODO(), meta.ListOptions{})
		subjects := rolebindings.Items[0].Subjects
		assert.Len(t, subjects, 2)
		assert.Equal(t, "testapp-machine-user", subjects[1].Name)
	})
}

func Test_Create_LimitRange(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest()
	_, _, env := newEnv(client, kubeUtil, radixclient, envConfigFileName)

	sync(t, &env)

	limitranges, _ := client.CoreV1().LimitRanges(namespaceName).List(context.TODO(), meta.ListOptions{})

	commonAsserts(t, env, limitRangesAsMeta(limitranges.Items), "mem-cpu-limit-range-env")

	t.Run("Received correct limitrange values", func(t *testing.T) {
		limits := limitranges.Items[0].Spec.Limits[0]
		assert.Equal(t, limitDefaultCPU, limits.Default.Cpu().String())
		assert.Equal(t, limitDefaultReqestCPU, limits.DefaultRequest.Cpu().String())
		assert.Equal(t, limitDefaultMemory, limits.Default.Memory().String())
		assert.Equal(t, limitDefaultReqestMemory, limits.DefaultRequest.Memory().String())
	})
}

func Test_Orphaned_Status(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest()
	_, _, env := newEnv(client, kubeUtil, radixclient, envConfigFileName)

	env.appConfig = nil
	sync(t, &env)

	t.Run("Orphaned is true when app config nil", func(t *testing.T) {
		assert.True(t, env.config.Status.Orphaned)
	})

	env.appConfig = utils.NewRadixApplicationBuilder().
		WithAppName("testapp").
		WithEnvironment("testenv", "master").
		BuildRA()
	sync(t, &env)

	t.Run("Orphaned is false when app config contains environment name", func(t *testing.T) {
		assert.False(t, env.config.Status.Orphaned)
	})

	env.appConfig = utils.NewRadixApplicationBuilder().
		WithAppName("testapp").
		BuildRA()
	sync(t, &env)

	t.Run("Orphaned is true when app config is cleared", func(t *testing.T) {
		assert.True(t, env.config.Status.Orphaned)
	})
}

// sync calls OnSync on the Environment resource and asserts success
func sync(t *testing.T, env *Environment) {
	time := meta.NewTime(time.Now().UTC())
	err := env.OnSync(time)

	t.Run("Method succeeds", func(t *testing.T) {
		assert.NoError(t, err)
	})

	t.Run("Reconciled time is set", func(t *testing.T) {
		assert.Equal(t, time, env.config.Status.Reconciled)
	})
}

// commonAsserts runs a generic set of assertions about resource creation
func commonAsserts(t *testing.T, env Environment, resources []meta.Object, name string) {
	t.Run("It creates a single resource", func(t *testing.T) {
		assert.Len(t, resources, 1)
	})

	t.Run("Resource has a correct name", func(t *testing.T) {
		assert.Equal(t, name, resources[0].GetName())
	})

	t.Run("Resource has a correct owner", func(t *testing.T) {
		assert.Equal(t, env.AsOwnerReference(), resources[0].GetOwnerReferences())
	})

	t.Run("Creation is idempotent", func(t *testing.T) {
		err := env.OnSync(meta.NewTime(time.Now().UTC()))
		assert.NoError(t, err)
		assert.Len(t, resources, 1)
	})
}

// following code is necessary noise to account for the lack of covariance and overloading
// it simply exposes a resource slice as a generic interface

func namespacesAsMeta(items []core.Namespace) []meta.Object {
	var slice []meta.Object
	for _, w := range items {
		slice = append(slice, w.GetObjectMeta())
	}
	return slice
}
func roleBindingsAsMeta(items []rbac.RoleBinding) []meta.Object {
	var slice []meta.Object
	for _, w := range items {
		slice = append(slice, w.GetObjectMeta())
	}
	return slice
}
func limitRangesAsMeta(items []core.LimitRange) []meta.Object {
	var slice []meta.Object
	for _, w := range items {
		slice = append(slice, w.GetObjectMeta())
	}
	return slice
}
