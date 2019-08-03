package job

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Job Instance variables
type Job struct {
	kubeclient   kubernetes.Interface
	radixclient  radixclient.Interface
	kubeutil     *kube.Kube
	registration *v1.RadixRegistration
	radixJob     *v1.RadixJob
}

// NewJob Constructor
func NewJob(kubeclient kubernetes.Interface, radixclient radixclient.Interface, registration *v1.RadixRegistration, radixJob *v1.RadixJob) (Job, error) {
	kubeutil, err := kube.New(kubeclient)
	if err != nil {
		return Job{}, err
	}

	return Job{
		kubeclient,
		radixclient,
		kubeutil, registration, radixJob}, nil
}

// OnSync compares the actual state with the desired, and attempts to
// converge the two
func (job *Job) OnSync() error {
	_, err := job.kubeclient.BatchV1().Jobs(job.radixJob.Namespace).Get(job.radixJob.Name, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		return job.createJob()
	} else if err != nil {
		return err
	}

	return nil
}
