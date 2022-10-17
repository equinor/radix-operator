ENVIRONMENT ?= dev
VERSION 	?= latest

DNS_ZONE = dev.radix.equinor.com
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
VAULT_NAME ?= radix-vault-$(ENVIRONMENT)

# If you want to escape branch-environment constraint, pass in OVERRIDE_BRANCH=true

ifeq ($(ENVIRONMENT),prod)
	IS_PROD = yes
else
	IS_DEV = yes
endif

ifeq ($(BRANCH),release)
	IS_PROD_BRANCH = yes
endif

ifeq ($(BRANCH),master)
	IS_DEV_BRANCH = yes
endif

ifdef IS_PROD
ifdef IS_PROD_BRANCH
	CAN_DEPLOY_OPERATOR = yes
endif
endif

ifdef IS_DEV
ifdef IS_DEV_BRANCH
	CAN_DEPLOY_OPERATOR = yes
else
	VERSION = dev
endif
endif

ifdef IS_PROD
	DNS_ZONE = radix.equinor.com
endif

VAULT_NAME = radix-vault-$(ENVIRONMENT)
CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io
APP_ALIAS_BASE_URL = app.$(DNS_ZONE)

HASH := $(shell git rev-parse HEAD)

CLUSTER_NAME = $(shell kubectl config get-contexts | grep '*' | tr -s ' ' | cut -f 3 -d ' ')
CHART_VERSION = $(shell cat charts/radix-operator/Chart.yaml | yq --raw-output .version)

TAG := $(BRANCH)-$(HASH)

echo:
	@echo "ENVIRONMENT : " $(ENVIRONMENT)
	@echo "DNS_ZONE : " $(DNS_ZONE)
	@echo "CONTAINER_REPO : " $(CONTAINER_REPO)
	@echo "DOCKER_REGISTRY : " $(DOCKER_REGISTRY)
	@echo "BRANCH : " $(BRANCH)
	@echo "CLUSTER_NAME : " $(CLUSTER_NAME)
	@echo "APP_ALIAS_BASE_URL : " $(APP_ALIAS_BASE_URL)
	@echo "IS_PROD : " $(IS_PROD)
	@echo "IS_DEV : " $(IS_DEV)
	@echo "IS_PROD_BRANCH : " $(IS_PROD_BRANCH)
	@echo "IS_DEV_BRANCH : " $(IS_DEV_BRANCH)
	@echo "CAN_DEPLOY_OPERATOR : " $(CAN_DEPLOY_OPERATOR)
	@echo "VERSION : " $(VERSION)
	@echo "TAG : " $(TAG)

.PHONY: test
test:	
	go test -cover `go list ./... | grep -v 'pkg/client'`

mocks:
	mockgen -source ./pkg/apis/defaults/oauth2.go -destination ./pkg/apis/defaults/oauth2_mock.go -package defaults
	mockgen -source ./pkg/apis/deployment/deploymentfactory.go -destination ./pkg/apis/deployment/deploymentfactory_mock.go -package deployment
	mockgen -source ./pkg/apis/deployment/deployment.go -destination ./pkg/apis/deployment/deployment_mock.go -package deployment
	mockgen -source ./pkg/apis/deployment/securitycontext.go -destination ./pkg/apis/deployment/securitycontext_mock.go -package deployment
	mockgen -source ./pkg/apis/deployment/auxiliaryresourcemanager.go -destination ./pkg/apis/deployment/auxiliaryresourcemanager_mock.go -package deployment
	mockgen -source ./pkg/apis/deployment/ingressannotationprovider.go -destination ./pkg/apis/deployment/ingressannotationprovider_mock.go -package deployment
	mockgen -source ./pkg/apis/alert/alert.go -destination ./pkg/apis/alert/alert_mock.go -package alert
	mockgen -source ./pkg/apis/alert/alertfactory.go -destination ./pkg/apis/alert/alertfactory_mock.go -package alert
	mockgen -source ./radix-operator/common/handler.go -destination ./radix-operator/common/handler_mock.go -package common
	mockgen -source ./pipeline-runner/model/env/env.go -destination ./pipeline-runner/model/mock/env_mock.go -package mock

build-pipeline:
	docker build -t $(DOCKER_REGISTRY)/radix-pipeline:$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(TAG) -f pipeline.Dockerfile .

deploy-pipeline:
	az acr login --name $(CONTAINER_REPO)
	make build-pipeline
	docker push $(DOCKER_REGISTRY)/radix-pipeline:$(BRANCH)-$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-pipeline:$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-pipeline:$(TAG)

build-operator:
	docker build -t $(DOCKER_REGISTRY)/radix-operator:$(VERSION) -t $(DOCKER_REGISTRY)/radix-operator:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-operator:$(TAG) -f operator.Dockerfile .

deploy-operator:
	az acr login --name $(CONTAINER_REPO)
	make build-operator
	docker push $(DOCKER_REGISTRY)/radix-operator:$(BRANCH)-$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-operator:$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-operator:$(TAG)

# deploys radix operator using helm chart in radixdev/radixprod acr
deploy-via-helm:
ifndef OVERRIDE_BRANCH
ifndef CAN_DEPLOY_OPERATOR
		@echo "Cannot release Operator to this cluster";\
		exit 1
endif
endif

	az acr helm repo add --name $(CONTAINER_REPO)
	helm repo update

	az keyvault secret download \
		--vault-name $(VAULT_NAME) \
		--name radix-operator-values \
		--file radix-operator-values.yaml

	helm upgrade --install radix-operator \
	    ./charts/radix-operator/ \
		--namespace default \
	    --set dnsZone=$(DNS_ZONE) \
		--set appAliasBaseURL=$(APP_ALIAS_BASE_URL) \
		--set prometheusName=radix-stage1 \
		--set image.repository=$(DOCKER_REGISTRY) \
		--set clusterName=$(CLUSTER_NAME) \
		--set image.tag=$(BRANCH)-$(VERSION) \
    	--set clusterType="development" \
    	-f radix-operator-values.yaml

	rm -f radix-operator-values.yaml

# build and deploy radix operator
helm-up:
	make deploy-operator
	make deploy-via-helm

# upgrades helm chart in radixdev/radixprod acr (does not deploy radix-operator)
helm-upgrade-operator-chart:
	az acr helm repo add --name $(CONTAINER_REPO)
	tar -zcvf radix-operator-$(CHART_VERSION).tgz charts/radix-operator
	az acr helm push --name $(CONTAINER_REPO) charts/radix-operator-$(CHART_VERSION).tgz
	rm charts/radix-operator-$(CHART_VERSION).tgz

ROOT_PACKAGE=github.com/equinor/radix-operator
CUSTOM_RESOURCE_NAME=radix
CUSTOM_RESOURCE_VERSION=v1

.PHONY: code-gen
code-gen: 
	$(GOPATH)/pkg/mod/k8s.io/code-generator@v0.25.3/generate-groups.sh all $(ROOT_PACKAGE)/pkg/client $(ROOT_PACKAGE)/pkg/apis $(CUSTOM_RESOURCE_NAME):$(CUSTOM_RESOURCE_VERSION) --go-header-file $(GOPATH)/pkg/mod/k8s.io/code-generator@v0.23.9/hack/boilerplate.go.txt


.HONY: staticcheck
staticcheck:
	staticcheck `go list ./... | grep -v "pkg/client"` &&     go vet `go list ./... | grep -v "pkg/client"`