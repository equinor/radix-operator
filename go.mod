module github.com/equinor/radix-operator

go 1.16

require (
	github.com/equinor/radix-common v1.1.6
	github.com/golang/mock v1.4.4
	github.com/prometheus-operator/prometheus-operator v0.44.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.44.0
	github.com/prometheus/client_golang v1.11.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.0
	k8s.io/apiextensions-apiserver v0.21.1
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/secrets-store-csi-driver v1.0.0 // indirect
)

replace k8s.io/client-go => k8s.io/client-go v0.19.9
