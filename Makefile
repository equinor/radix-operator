ENVIRONMENT ?= dev
VERSION 	?= latest

DNS_ZONE = dev.radix.equinor.com
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

CRD_TEMP_DIR := ./.temp-resources/
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
	LOG_LEVEL=warn go test -cover `go list ./... | grep -v 'github.com/equinor/radix-operator/pkg/client' | grep -v 'github.com/equinor/radix-operator/e2e'`

.PHONY: test-e2e
test-e2e: generate
	cd e2e && go test -v -p 1 -timeout 30m ./...
	# Note: -p 1 is used to run tests sequentially to allow printing logs sequentially

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
	mockgen -source ./operator/batch/internal/syncerfactory.go -destination ./operator/batch/internal/syncerfactory_mock.go -package internal
	mockgen -source ./pkg/apis/dnsalias/syncer.go -destination ./pkg/apis/dnsalias/syncer_mock.go -package dnsalias
	mockgen -source ./operator/dnsalias/internal/syncerfactory.go -destination ./operator/dnsalias/internal/syncerfactory_mock.go -package internal
	mockgen -source ./operator/common/handler.go -destination ./operator/common/handler_mock.go -package common
	mockgen -source ./operator/job/handler.go -destination ./operator/job/handler_mock.go -package job
	mockgen -source ./pipeline-runner/internal/wait/job.go -destination ./pipeline-runner/internal/wait/job_mock.go -package wait
	mockgen -source ./pipeline-runner/internal/watcher/radix_deployment_watcher.go -destination ./pipeline-runner/internal/watcher/radix_deployment_watcher_mock.go -package watcher
	mockgen -source ./pipeline-runner/internal/watcher/namespace.go -destination ./pipeline-runner/internal/watcher/namespace_mock.go -package watcher
	mockgen -source ./pipeline-runner/internal/jobs/build/interface.go -destination ./pipeline-runner/internal/jobs/build/mock/job.go -package mock
	mockgen -source ./pipeline-runner/steps/internal/wait/pipelinerun.go -destination ./pipeline-runner/steps/internal/wait/pipelinerun_mock.go -package wait
	mockgen -source ./pipeline-runner/steps/preparepipeline/internal/context_builder.go -destination ./pipeline-runner/steps/preparepipeline/internal/context_builder_mock.go -package internal
	mockgen -source ./pipeline-runner/steps/preparepipeline/internal/subpipeline_reader.go -destination ./pipeline-runner/steps/preparepipeline/internal/subpipeline_reader_mock.go -package internal
	mockgen -source ./pipeline-runner/steps/preparepipeline/internal/radix_config_reader.go -destination ./pipeline-runner/steps/preparepipeline/internal/radix_config_reader_mock.go -package internal
	mockgen -source ./pipeline-runner/steps/internal/ownerreferences/owner_references.go -destination ./pipeline-runner/steps/internal/ownerreferences/owner_references_mock.go -package ownerreferences
	mockgen -source ./pipeline-runner/utils/git/git.go -destination ./pipeline-runner/utils/git/git_mock.go -package git
	mockgen -source ./job-scheduler/api/v1/handlers/jobs/job_handler.go -destination ./job-scheduler/api/v1/handlers/jobs/mock/job_mock.go -package mock
	mockgen -source ./job-scheduler/api/v1/handlers/batches/batch_handler.go -destination ./job-scheduler/api/v1/handlers/batches/mock/batch_mock.go -package mock
	mockgen -source ./job-scheduler/pkg/notifications/notifier.go -destination ./job-scheduler/pkg/notifications/notifier_mock.go -package notifications
	mockgen -source ./job-scheduler/pkg/batch/history.go -destination ./job-scheduler/pkg/batch/history_mock.go -package batch


.PHONY: tidy
tidy:
	go mod tidy

.PHONY: build-pipeline
build-pipeline:
	docker buildx build -t $(DOCKER_REGISTRY)/radix-pipeline:$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(TAG) --platform linux/arm64,linux/amd64 -f pipeline.Dockerfile .

.PHONY: deploy-pipeline
deploy-pipeline:
	az acr login --name $(CONTAINER_REPO)
	docker buildx build -t $(DOCKER_REGISTRY)/radix-pipeline:$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(TAG) --platform linux/arm64,linux/amd64 -f pipeline.Dockerfile --push .

.PHONY: deploy-pipeline-arm64
deploy-pipeline-arm64:
	az acr login --name $(CONTAINER_REPO)
	docker buildx build -t $(DOCKER_REGISTRY)/radix-pipeline:$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-pipeline:$(TAG) --platform linux/arm64 -f pipeline.Dockerfile --push .

.PHONY: build-operator
build-operator:
	docker buildx build -t $(DOCKER_REGISTRY)/radix-operator:$(VERSION) -t $(DOCKER_REGISTRY)/radix-operator:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-operator:$(TAG) --platform linux/arm64,linux/amd64 -f operator.Dockerfile . --no-cache

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

.PHONY: swagger
swagger: swagger-job-scheduler

.PHONY: swagger-job-scheduler
swagger-job-scheduler: SHELL:=/bin/bash
swagger-job-scheduler: bootstrap
	swagger generate spec -w ./job-scheduler/ -o ./job-scheduler/swaggerui/html/swagger.json --scan-models --exclude-deps
	swagger validate ./job-scheduler/swaggerui/html/swagger.json

.PHONY: code-gen
code-gen: bootstrap
	./hack/update-codegen.sh

.PHONY: helmresources
helmresources: temp-resources radixregistration-crd radixapplication-crd radixbatch-crd radixdnsalias-crd radixdeployment-crd radixalert-crd radixenvironment-crd radixjob-crd radixwebhook delete-temp-resources

.PHONY: radixregistration-crd
radixregistration-crd: temp-resources
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixregistrations.yaml $(CRD_CHART_DIR)radixregistration.yaml

.PHONY: radixapplication-crd
radixapplication-crd: temp-resources
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixapplications.yaml $(CRD_CHART_DIR)radixapplication.yaml
	yq eval '.spec.versions[0].schema.openAPIV3Schema' -ojson $(CRD_CHART_DIR)radixapplication.yaml | jq 'del(.properties.status)' > $(JSON_SCHEMA_DIR)radixapplication.json

.PHONY: radixbatch-crd
radixbatch-crd: temp-resources
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixbatches.yaml $(CRD_CHART_DIR)radixbatch.yaml

.PHONY: radixdeployment-crd
radixdeployment-crd: temp-resources
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixdeployments.yaml $(CRD_CHART_DIR)radixdeployment.yaml

.PHONY: radixdnsalias-crd
radixdnsalias-crd: temp-resources
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixdnsaliases.yaml $(CRD_CHART_DIR)radixdnsalias.yaml

.PHONY: radixalert-crd
radixalert-crd: temp-resources
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixalerts.yaml $(CRD_CHART_DIR)radixalert.yaml

.PHONY: radixenvironment-crd
radixenvironment-crd: temp-resources
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixenvironments.yaml $(CRD_CHART_DIR)radixenvironment.yaml

.PHONY: radixjob-crd
radixjob-crd: temp-resources
	cp $(CRD_TEMP_DIR)radix.equinor.com_radixjobs.yaml $(CRD_CHART_DIR)radixjob.yaml

.PHONY: radixwebhook
radixwebhook: temp-resources
	cp $(CRD_TEMP_DIR)radix-webhook-configuration.yaml $(CRD_CHART_DIR)radix-webhook-configuration.yaml

.PHONY: temp-resources
temp-resources: bootstrap
	controller-gen +crd:crdVersions=v1 paths=./pkg/apis/radix/v1/ output:dir:=$(CRD_TEMP_DIR)
	controller-gen +webhook paths=./webhook/validation/ output:stdout > $(CRD_TEMP_DIR)radix-webhook-configuration.yaml
	./hack/helmify-admission-webhook.sh $(CRD_TEMP_DIR)radix-webhook-configuration.yaml

.PHONY: delete-temp-resources
delete-temp-resources:
	rm -rf $(CRD_TEMP_DIR)

.PHONY: lint
lint: bootstrap
	golangci-lint run

.PHONY: generate
generate: bootstrap code-gen helmresources mocks swagger

.PHONY: verify-generate
verify-generate: bootstrap tidy generate
	git diff --exit-code

.PHONY: apply-ra
apply-ra: bootstrap
	kubectl apply -f ./charts/radix-operator/templates/radixapplication.yaml

.PHONY: apply-rb
apply-rb: bootstrap
	kubectl apply -f ./charts/radix-operator/templates/radixbatch.yaml

.PHONY: apply-rd
apply-rd: bootstrap
	kubectl apply -f ./charts/radix-operator/templates/radixdeployment.yaml

HAS_GOLANGCI_LINT  := $(shell command -v golangci-lint;)
HAS_MOCKGEN        := $(shell command -v mockgen;)
HAS_CONTROLLER_GEN := $(shell command -v controller-gen;)
HAS_YQ             := $(shell command -v yq;)
HAS_KUBECTL        := $(shell command -v kubectl;)
HAS_SWAGGER        := $(shell command -v swagger;)

.PHONY: bootstrap
bootstrap: vendor
ifndef HAS_GOLANGCI_LINT
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2
endif
ifndef HAS_MOCKGEN
	go install github.com/golang/mock/mockgen@v1.6.0
endif
ifndef HAS_CONTROLLER_GEN
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.2
endif
ifndef HAS_YQ
	go install github.com/mikefarah/yq/v4@latest
endif
ifndef HAS_KUBECTL
	go install k8s.io/kubernetes/cmd/kubectl@latest
endif
ifndef HAS_SWAGGER
	go install github.com/go-swagger/go-swagger/cmd/swagger@v0.33.1
endif