module github.com/equinor/radix-operator

go 1.13

require (
	cloud.google.com/go v0.47.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	git.apache.org/thrift.git v0.12.0 // indirect
	github.com/Azure/go-autorest v11.9.0+incompatible
	github.com/PuerkitoBio/purell v1.1.1
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d
	github.com/beorn7/perks v1.0.1
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/coreos/prometheus-operator v0.33.0
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/emicklei/go-restful v2.11.0+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/go-openapi/jsonpointer v0.19.3
	github.com/go-openapi/jsonreference v0.19.3
	github.com/go-openapi/spec v0.19.4
	github.com/go-openapi/swag v0.19.5
	github.com/gogo/protobuf v1.3.1
	github.com/golang/groupcache v0.0.0-20191002201903-404acd9df4cc
	github.com/golang/lint v0.0.0-20180702182130-06c8688daad7 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.1
	github.com/google/gofuzz v1.0.0
	github.com/googleapis/gnostic v0.3.1
	github.com/gophercloud/gophercloud v0.6.0
	github.com/grpc-ecosystem/grpc-gateway v1.11.3
	github.com/hashicorp/golang-lru v0.5.3
	github.com/imdario/mergo v0.3.8
	github.com/json-iterator/go v1.1.8-0.20191012130704-03217c3e9766
	github.com/konsorten/go-windows-terminal-sequences v1.0.2
	github.com/mailru/easyjson v0.7.0
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.1
	github.com/pkg/errors v0.8.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/prometheus/common v0.7.0
	github.com/prometheus/procfs v0.0.5
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/net v0.0.0-20191021144547-ec77196f6094
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20191022100944-742c48ecaeb7
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20191023065245-6d3f0bb11be5
	golang.org/x/tools v0.0.0-20191022213345-0bbdf54effa2
	google.golang.org/api v0.11.0
	google.golang.org/appengine v1.6.5
	google.golang.org/genproto v0.0.0-20191009194640-548a555dbc03
	google.golang.org/grpc v1.24.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.0.0-20191016225839-816a9b7df678
	k8s.io/apimachinery v0.0.0-20191020214737-6c8691705fc5
	k8s.io/client-go v2.0.0-alpha.0.0.20181121191925-a47917edff34+incompatible
	k8s.io/code-generator v0.0.0-20191017183038-0b22993d207c
	k8s.io/gengo v0.0.0-20191010091904-7fa3014cb28f
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d
	k8s.io/utils v0.0.0-20191010214722-8d271d903fe4
	sigs.k8s.io/yaml v1.1.0
)

replace k8s.io/client-go => k8s.io/client-go v12.0.0+incompatible
