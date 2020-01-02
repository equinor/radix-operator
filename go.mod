module github.com/equinor/radix-operator

go 1.13

require (
	cloud.google.com/go v0.47.0 // indirect
	github.com/Azure/go-autorest v11.9.0+incompatible // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/coreos/prometheus-operator v0.33.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/emicklei/go-restful v2.11.0+incompatible // indirect
	github.com/go-openapi/jsonreference v0.19.3 // indirect
	github.com/go-openapi/spec v0.19.4 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/groupcache v0.0.0-20191002201903-404acd9df4cc // indirect
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/gophercloud/gophercloud v0.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.11.3 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/json-iterator/go v1.1.8-0.20191012130704-03217c3e9766 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mailru/easyjson v0.7.0 // indirect
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/common v0.7.0
	github.com/prometheus/procfs v0.0.5 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 // indirect
	golang.org/x/net v0.0.0-20191021144547-ec77196f6094 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20191022100944-742c48ecaeb7 // indirect
	golang.org/x/time v0.0.0-20191023065245-6d3f0bb11be5 // indirect
	google.golang.org/api v0.11.0 // indirect
	google.golang.org/appengine v1.6.5
	google.golang.org/grpc v1.24.0 // indirect
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.0.0-20191016225839-816a9b7df678
	k8s.io/apimachinery v0.0.0-20191020214737-6c8691705fc5
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d // indirect
	k8s.io/utils v0.0.0-20191010214722-8d271d903fe4 // indirect
)

replace (
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v0.0.0-20190818123050-43acd0e2e93f
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)
