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
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	envConfigFileName           = "./testdata/re.yaml"
	egressRuleEnvConfigFileName = "./testdata/re_egress.yaml"
	regConfigFileName           = "./testdata/rr.yaml"
	namespaceName               = "testapp-testenv"

	limitDefaultReqestCPU    = "234m" // 0.234
	limitDefaultMemory       = "321M" // 321'000'000
	limitDefaultReqestMemory = "123M" // 123'000'000
)

func setupTest(t *testing.T) (test.Utils, kubernetes.Interface, *kube.Kube, radixclient.Interface) {
	fakekube := fake.NewSimpleClientset()
	fakeradix := radix.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(fakekube, fakeradix, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(fakekube, fakeradix, secretproviderclient)
	err := handlerTestUtils.CreateClusterPrerequisites("AnyClusterName", "0.0.0.0", "anysubid")
	require.NoError(t, err)

	_ = os.Setenv(defaults.OperatorEnvLimitDefaultRequestCPUEnvironmentVariable, limitDefaultReqestCPU)
	_ = os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, limitDefaultMemory)
	_ = os.Setenv(defaults.OperatorEnvLimitDefaultRequestMemoryEnvironmentVariable, limitDefaultReqestMemory)
	return handlerTestUtils, fakekube, kubeUtil, fakeradix
}

func newEnv(client kubernetes.Interface, kubeUtil *kube.Kube, radixclient radixclient.Interface, radixEnvFileName string) (*v1.RadixRegistration, *v1.RadixEnvironment, Environment, error) {
	rr, _ := utils.GetRadixRegistrationFromFile(regConfigFileName)
	re, _ := utils.GetRadixEnvironmentFromFile(radixEnvFileName)
	logger := logrus.WithFields(logrus.Fields{"environmentName": namespaceName})
	nw, _ := networkpolicy.NewNetworkPolicy(client, kubeUtil, logger, re.Spec.AppName)
	env, _ := NewEnvironment(client, kubeUtil, radixclient, re, rr, nil, logger, &nw)
	// register instance with radix-client so UpdateStatus() can find it
	if _, err := radixclient.RadixV1().RadixEnvironments().Create(context.TODO(), re, metav1.CreateOptions{}); err != nil {
		return nil, nil, env, err
	}
	return rr, re, env, nil
}

func Test_Create_Namespace(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest(t)
	defer os.Clearenv()
	rr, _, env, err := newEnv(client, kubeUtil, radixclient, envConfigFileName)
	require.NoError(t, err)

	sync(t, &env)

	namespaces, _ := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, rr.Name),
	})

	commonAsserts(t, env, namespacesAsMeta(namespaces.Items), namespaceName)

	expected := map[string]string{
		"sync":                         "cluster-wildcard-tls-cert",
		"cluster-wildcard-sync":        "cluster-wildcard-tls-cert",
		"radix-wildcard-sync":          "radix-wildcard-tls-cert",
		"app-wildcard-sync":            "app-wildcard-tls-cert",
		"active-cluster-wildcard-sync": "active-cluster-wildcard-tls-cert",
		fmt.Sprintf("%s-sync", defaults.PrivateImageHubSecretName): env.config.Spec.AppName,
		kube.RadixAppLabel: env.config.Spec.AppName,
		kube.RadixEnvLabel: env.config.Spec.EnvName,
	}
	assert.Equal(t, expected, namespaces.Items[0].GetLabels())
}

func createRadixDNSAliasForEnvironment(radixClient radixclient.Interface, aliasName string) error {
	return test.RegisterRadixDNSAliasBySpec(radixClient, aliasName, test.DNSAlias{
		AppName:     "testapp",
		Environment: "testenv",
		Component:   "testcomponent",
	})
}

func Test_Create_Namespace_PodSecurityStandardLabels(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest(t)
	os.Setenv(defaults.PodSecurityStandardEnforceLevelEnvironmentVariable, "enforceLvl")
	os.Setenv(defaults.PodSecurityStandardEnforceVersionEnvironmentVariable, "enforceVer")
	os.Setenv(defaults.PodSecurityStandardAuditLevelEnvironmentVariable, "auditLvl")
	os.Setenv(defaults.PodSecurityStandardAuditVersionEnvironmentVariable, "auditVer")
	os.Setenv(defaults.PodSecurityStandardWarnLevelEnvironmentVariable, "warnLvl")
	os.Setenv(defaults.PodSecurityStandardWarnVersionEnvironmentVariable, "warnVer")
	defer os.Clearenv()
	rr, _, env, err := newEnv(client, kubeUtil, radixclient, envConfigFileName)
	require.NoError(t, err)

	sync(t, &env)

	namespaces, _ := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, rr.Name),
	})

	commonAsserts(t, env, namespacesAsMeta(namespaces.Items), namespaceName)

	expected := map[string]string{
		"sync":                         "cluster-wildcard-tls-cert",
		"cluster-wildcard-sync":        "cluster-wildcard-tls-cert",
		"radix-wildcard-sync":          "radix-wildcard-tls-cert",
		"app-wildcard-sync":            "app-wildcard-tls-cert",
		"active-cluster-wildcard-sync": "active-cluster-wildcard-tls-cert",
		fmt.Sprintf("%s-sync", defaults.PrivateImageHubSecretName): env.config.Spec.AppName,
		kube.RadixAppLabel:                           env.config.Spec.AppName,
		kube.RadixEnvLabel:                           env.config.Spec.EnvName,
		"pod-security.kubernetes.io/enforce":         "enforceLvl",
		"pod-security.kubernetes.io/enforce-version": "enforceVer",
		"pod-security.kubernetes.io/audit":           "auditLvl",
		"pod-security.kubernetes.io/audit-version":   "auditVer",
		"pod-security.kubernetes.io/warn":            "warnLvl",
		"pod-security.kubernetes.io/warn-version":    "warnVer",
	}
	assert.Equal(t, expected, namespaces.Items[0].GetLabels())
}

func Test_Create_EgressRules(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest(t)
	defer os.Clearenv()
	rr, _, env, err := newEnv(client, kubeUtil, radixclient, egressRuleEnvConfigFileName)
	require.NoError(t, err)

	sync(t, &env)

	namespaces, _ := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixAppLabel, rr.Name),
	})

	t.Run("Egress rules are correct", func(t *testing.T) {
		egressRules := env.config.Spec.Egress.Rules
		assert.Len(t, egressRules, 1)
		assert.Equal(t, string(egressRules[0].Destinations[0]), "195.88.55.16/32")
		assert.Len(t, egressRules[0].Ports, 2)
	})

	commonAsserts(t, env, namespacesAsMeta(namespaces.Items), namespaceName)
}

func Test_Create_RoleBinding(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest(t)
	defer os.Clearenv()
	rr, _, env, err := newEnv(client, kubeUtil, radixclient, envConfigFileName)
	require.NoError(t, err)

	sync(t, &env)

	rolebindings, _ := client.RbacV1().RoleBindings(namespaceName).List(context.TODO(), metav1.ListOptions{})

	commonAsserts(t, env, roleBindingsAsMeta(rolebindings.Items), "radix-tekton-env", "radix-app-admin-envs", "radix-pipeline-env", "radix-app-reader-envs")
	adGroupName := rr.Spec.AdGroups[0]
	t.Run("It contains the correct AD groups", func(t *testing.T) {
		subjects := rolebindings.Items[0].Subjects
		assert.Len(t, subjects, 1)
		assert.Equal(t, adGroupName, subjects[0].Name)
	})

	readerAdGroupName := rr.Spec.ReaderAdGroups[0]
	t.Run("It contains the correct reader AD groups", func(t *testing.T) {
		subjects := rolebindings.Items[1].Subjects
		assert.Len(t, subjects, 1)
		assert.Equal(t, readerAdGroupName, subjects[0].Name)
	})
}

func Test_Create_LimitRange(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest(t)
	defer os.Clearenv()
	_, _, env, err := newEnv(client, kubeUtil, radixclient, envConfigFileName)
	require.NoError(t, err)

	sync(t, &env)

	limitranges, _ := client.CoreV1().LimitRanges(namespaceName).List(context.TODO(), metav1.ListOptions{})

	commonAsserts(t, env, limitRangesAsMeta(limitranges.Items), "mem-cpu-limit-range-env")

	t.Run("Received correct limitrange values", func(t *testing.T) {
		limits := limitranges.Items[0].Spec.Limits[0]
		assert.True(t, limits.Default.Cpu().IsZero())
		assert.Equal(t, limitDefaultReqestCPU, limits.DefaultRequest.Cpu().String())
		assert.Equal(t, limitDefaultMemory, limits.Default.Memory().String())
		assert.Equal(t, limitDefaultReqestMemory, limits.DefaultRequest.Memory().String())
	})
}

func Test_Orphaned_Status(t *testing.T) {
	_, client, kubeUtil, radixclient := setupTest(t)
	defer os.Clearenv()
	_, _, env, err := newEnv(client, kubeUtil, radixclient, envConfigFileName)
	require.NoError(t, err)

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
	time := metav1.NewTime(time.Now().UTC())
	err := env.OnSync(time)

	t.Run("Method succeeds", func(t *testing.T) {
		assert.NoError(t, err)
	})

	t.Run("Reconciled time is set", func(t *testing.T) {
		assert.Equal(t, time, env.config.Status.Reconciled)
	})
}

// commonAsserts runs a generic set of assertions about resource creation
func commonAsserts(t *testing.T, env Environment, resources []metav1.Object, names ...string) {
	t.Run("It creates a single resource", func(t *testing.T) {
		assert.Len(t, resources, len(names))
	})

	t.Run("Resource has a correct name", func(t *testing.T) {
		for _, resource := range resources {
			assert.Contains(t, names, resource.GetName())
		}
	})

	t.Run("Resource has a correct owner", func(t *testing.T) {
		for _, resource := range resources {
			assert.Equal(t, env.AsOwnerReference(), resource.GetOwnerReferences())
		}
	})

	t.Run("Creation is idempotent", func(t *testing.T) {
		err := env.OnSync(metav1.NewTime(time.Now().UTC()))
		assert.NoError(t, err)
		assert.Len(t, resources, len(names))
	})
}

// following code is necessary noise to account for the lack of covariance and overloading
// it simply exposes a resource slice as a generic interface

func namespacesAsMeta(items []core.Namespace) []metav1.Object {
	var slice []metav1.Object
	for _, w := range items {
		w := w
		slice = append(slice, w.GetObjectMeta())
	}
	return slice
}
func roleBindingsAsMeta(items []rbac.RoleBinding) []metav1.Object {
	var slice []metav1.Object
	for _, w := range items {
		w := w
		slice = append(slice, w.GetObjectMeta())
	}
	return slice
}
func limitRangesAsMeta(items []core.LimitRange) []metav1.Object {
	var slice []metav1.Object
	for _, w := range items {
		w := w
		slice = append(slice, w.GetObjectMeta())
	}
	return slice
}
