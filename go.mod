module github.com/equinor/radix-operator

go 1.13

require (
	github.com/prometheus-operator/prometheus-operator v0.44.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.44.0
	github.com/prometheus/client_golang v1.8.0
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.9
	k8s.io/apimachinery v0.19.9
	k8s.io/client-go v12.0.0+incompatible
)

replace (
	k8s.io/client-go => k8s.io/client-go v0.19.9
)
