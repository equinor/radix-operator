package restore

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func radixRegistrationExists(ctx context.Context, kube *kube.Kube, name string) (bool, error) {
	_, err := kube.GetRegistration(ctx, name)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getLoggerForObject(sourceLog logrus.FieldLogger, object runtime.Object) *logrus.Entry {
	l := sourceLog.WithField("groupResource", object.GetObjectKind().GroupVersionKind().GroupKind().String())

	if objWithMeta, ok := object.(metav1.Object); ok {
		l = l.WithFields(logrus.Fields{
			"namespace": objWithMeta.GetNamespace(),
			"name":      objWithMeta.GetName(),
		})
	}

	return l
}
