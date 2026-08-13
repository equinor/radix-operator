package deployment

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	rv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_garbageCollectServicesNoLongerInSpec(t *testing.T) {
	tests := []struct {
		name                 string
		rd                   *rv1.RadixDeployment
		services             []*corev1.Service
		expectedServiceNames []string
	}{
		{
			name: "skip batch services",
			rd: utils.ARadixDeployment().
				WithAppName("app").
				WithEnvironment("dev").
				WithJobComponents().
				WithComponents().
				BuildRD(),
			services: []*corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "batch-service",
					Labels: map[string]string{
						kube.RadixComponentLabel:    "job",
						kube.RadixJobTypeLabel:      kube.RadixJobTypeJobSchedule,
						kube.RadixBatchNameLabel:    "batch-1",
						kube.RadixBatchJobNameLabel: "job-1",
					},
				},
			}},
			expectedServiceNames: []string{"batch-service"},
		},
		{
			name: "collect non-batch job-scheduler service when job is missing",
			rd: utils.ARadixDeployment().
				WithAppName("app").
				WithEnvironment("dev").
				WithJobComponents().
				WithComponents().
				BuildRD(),
			services: []*corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-scheduler-service",
					Labels: map[string]string{
						kube.RadixComponentLabel: "job",
						kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
					},
				},
			}},
			expectedServiceNames: []string{},
		},
		{
			name: "collect component service without ports",
			rd: utils.ARadixDeployment().
				WithAppName("app").
				WithEnvironment("dev").
				WithComponents(utils.NewDeployComponentBuilder().WithName("component")).
				BuildRD(),
			services: []*corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "component",
					Labels: map[string]string{
						kube.RadixComponentLabel: "component",
					},
				},
			}},
			expectedServiceNames: []string{},
		},
		{
			name: "keep component service with ports",
			rd: utils.ARadixDeployment().
				WithAppName("app").
				WithEnvironment("dev").
				WithComponents(utils.NewDeployComponentBuilder().
					WithName("component").
					WithPorts([]rv1.ComponentPort{{Name: "http", Port: 8080}})).
				BuildRD(),
			services: []*corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "component",
					Labels: map[string]string{
						kube.RadixComponentLabel: "component",
					},
				},
			}},
			expectedServiceNames: []string{"component"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tu, kubeclient, kubeUtil, _, _, _, _, _ := SetupTest(t)
			defer TeardownTest()
			_ = tu

			namespace := tt.rd.GetNamespace()
			for _, svc := range tt.services {
				svc.SetNamespace(namespace)
				_, err := kubeclient.CoreV1().Services(namespace).Create(context.Background(), svc, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			deploy := &Deployment{radixDeployment: tt.rd, kubeutil: kubeUtil, kubeclient: kubeclient}

			err := deploy.garbageCollectServicesNoLongerInSpec(context.Background())
			require.NoError(t, err)

			remainingServices, err := kubeclient.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)

			remainingServiceNames := make([]string, 0, len(remainingServices.Items))
			for _, svc := range remainingServices.Items {
				remainingServiceNames = append(remainingServiceNames, svc.Name)
			}

			assert.ElementsMatch(t, tt.expectedServiceNames, remainingServiceNames)
		})
	}
}
