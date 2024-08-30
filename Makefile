ENVIRONMENT ?= dev
VERSION 	?= latest

DNS_ZONE = dev.radix.equinor.com
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

CRD_TEMP_DIR := ./.temp-crds/
CRD_CHART_DIR := ./charts/radix-operator/templates/
JSON_SCHEMA_DIR := ./json-schema/

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

CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io
APP_ALIAS_BASE_URL = app.$(DNS_ZONE)

HASH := $(shell git rev-parse HEAD)

CLUSTER_NAME = $(shell kubectl config get-contexts | grep '*' | tr -s ' ' | cut -f 3 -d ' ')

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
	LOG_LEVEL=warn go test -cover `go list ./... | grep -v 'pkg/client'`

.PHONY: mocks
mocks: bootstrap
	mockgen -source ./pkg/apis/defaults/oauth2.go -destination ./pkg/apis/defaults/oauth2_mock.go -package defaults
	mockgen -source ./pkg/apis/deployment/deploymentfactory.go -destination ./pkg/apis/deployment/deploymentfactory_mock.go -package deployment
	mockgen -source ./pkg/apis/deployment/deployment.go -destination ./pkg/apis/deployment/deployment_mock.go -package deployment
	mockgen -source ./pkg/apis/deployment/auxiliaryresourcemanager.go -destination ./pkg/apis/deployment/auxiliaryresourcemanager_mock.go -package deployment
	mockgen -source ./pkg/apis/ingress/ingressannotationprovider.go -destination ./pkg/apis/ingress/ingressannotationprovider_mock.go -package ingress
	mockgen -source ./pkg/apis/alert/alert.go -destination ./pkg/apis/alert/alert_mock.go -package alert
	mockgen -source ./pkg/apis/alert/alertfactory.go -destination ./pkg/apis/alert/alertfactory_mock.go -package alert
	mockgen -source ./pkg/apis/batch/syncer.go -destination ./pkg/apis/batch/syncer_mock.go -package batch
	mockgen -source ./radix-operator/batch/internal/syncerfactory.go -destination ./radix-operator/batch/internal/syncerfactory_mock.go -package internal
	mockgen -source ./pkg/apis/dnsalias/syncer.go -destination ./pkg/apis/dnsalias/syncer_mock.go -package dnsalias
	mockgen -source ./radix-operator/dnsalias/internal/syncerfactory.go -destination ./radix-operator/dnsalias/internal/syncerfactory_mock.go -package internal
	mockgen -source ./radix-operator/common/handler.go -destination ./radix-operator/common/handler_mock.go -package common
	mockgen -source ./pkg/apis/job/job_history.go -destination ./radix-operator/job/job_history_mock.go -package job
	mockgen -source ./pipeline-runner/internal/wait/job.go -destination ./pipeline-runner/internal/wait/job_mock.go -package wait
	mockgen -source ./pipeline-runner/internal/watcher/radix_deployment_watcher.go -destination ./pipeline-runner/internal/watcher/radix_deployment_watcher_mock.go -package watcher

.PHONY: build-pipeline
build-pipeline:
	docker build -t $(DOCKER_REGISTRY)/radix-pipeline:$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(TAG) -f pipeline.Dockerfile .

.PHONY: deploy-pipeline
deploy-pipeline: build-pipeline
	az acr login --name $(CONTAINER_REPO)
	docker push $(DOCKER_REGISTRY)/radix-pipeline:$(BRANCH)-$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-pipeline:$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-pipeline:$(TAG)

.PHONY: build-operator
build-operator:
	docker build -t $(DOCKER_REGISTRY)/radix-operator:$(VERSION) -t $(DOCKER_REGISTRY)/radix-operator:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-operator:$(TAG) -f operator.Dockerfile .

.PHONY: deploy-operator
deploy-operator: build-operator
	az acr login --name $(CONTAINER_REPO)
	docker push $(DOCKER_REGISTRY)/radix-operator:$(BRANCH)-$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-operator:$(VERSION)
	docker push $(DOCKER_REGISTRY)/radix-operator:$(TAG)

ROOT_PACKAGE=github.com/equinor/radix-operator
CUSTOM_RESOURCE_NAME=radix
CUSTOM_RESOURCE_VERSION=v1

.PHONY: vendor
vendor:
	go mod vendor

.PHONY: code-gen
code-gen: bootstrap
	./hack/update-codegen.sh

.PHONY: crds
crds: temp-crds radixapplication-crd radixbatch-crd radixdnsalias-crd delete-temp-crds

.PHONY: radixapplication-crd
radixapplication-crd: temp-crds
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixapplications.yaml $(CRD_CHART_DIR)radixapplication.yaml
	yq eval '.spec.versions[0].schema.openAPIV3Schema' -ojson $(CRD_CHART_DIR)radixapplication.yaml > $(JSON_SCHEMA_DIR)radixapplication.json

.PHONY: radixbatch-crd
radixbatch-crd: temp-crds
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixbatches.yaml $(CRD_CHART_DIR)radixbatch.yaml

.PHONY: radixdnsalias-crd
radixdnsalias-crd: temp-crds
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixdnsaliases.yaml $(CRD_CHART_DIR)radixdnsalias.yaml

.PHONY: temp-crds
temp-crds: bootstrap
	controller-gen crd:crdVersions=v1 paths=./pkg/apis/radix/v1/ output:dir:=$(CRD_TEMP_DIR)

.PHONY: delete-temp-crds
delete-temp-crds:
	rm -rf $(CRD_TEMP_DIR)

.PHONY: lint
lint: bootstrap
	golangci-lint run

.PHONY: generate
generate: bootstrap code-gen crds mocks

.PHONY: verify-generate
verify-generate: bootstrap generate
	git diff --exit-code

HAS_GOLANGCI_LINT := $(shell command -v golangci-lint;)
HAS_MOCKGEN       := $(shell command -v mockgen;)
HAS_CONTROLLER_GEN := $(shell command -v controller-gen;)

.PHONY: bootstrap
bootstrap: vendor
ifndef HAS_GOLANGCI_LINT
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.60.3
endif
ifndef HAS_MOCKGEN
	go install github.com/golang/mock/mockgen@v1.6.0
endif
ifndef HAS_CONTROLLER_GEN
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.2
endif
