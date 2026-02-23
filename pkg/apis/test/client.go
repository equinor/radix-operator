package test

import (
	"fmt"
	"os"
	"reflect"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

var testscheme = scheme.NewScheme()

func CreateClient(filenameOrObject ...any) client.Client {
	var initObjs []client.Object
	for _, filename := range filenameOrObject {
		switch v := filename.(type) {
		case string:
			initObjs = append(initObjs, Load[client.Object](v))
		case client.Object:
			initObjs = append(initObjs, v)
		default:
			log.Fatal().Str("filename", fmt.Sprintf("%v", filename)).Msg("unsupported type for init object")
		}
	}
	return fake.NewClientBuilder().WithScheme(testscheme).WithStatusSubresource(&radixv1.RadixAlert{}).WithObjects(initObjs...).Build()
}

func Load[T client.Object](filename string) T {
	raw := struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata"`
	}{}

	file, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal().Err(err).Str("filename", filename).Msg("failed to read file")
	}

	// Important: Must use sigs.k8s.io/yaml decoder to correctly unmarshal Kubernetes objects.
	// This package supports encoding and decoding of yaml for CRD struct types using the json tag.
	err = yaml.Unmarshal(file, &raw)
	if err != nil {
		panic(err)
	}

	gvk := raw.GetObjectKind().GroupVersionKind()
	t, ok := testscheme.AllKnownTypes()[gvk]
	if !ok {
		panic(fmt.Sprintf("scheme does not know GroupVersionKind %s", gvk.String()))
	}

	obj := reflect.New(t)
	objP := obj.Interface()
	err = yaml.Unmarshal(file, objP)
	if err != nil {
		panic(err)
	}

	return objP.(T)
}
