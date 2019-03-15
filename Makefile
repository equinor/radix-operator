DOCKER_FILES	= operator pipeline
ENVIRONMENT ?= dev
VERSION 	?= latest

DNS_ZONE = dev.radix.equinor.com
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
VAULT_NAME ?= radix-vault-$(ENVIRONMENT)

# If you want to escape branch-environment contraint, pass in OVERIDE_BRANCH=true

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
endif
endif

ifdef IS_PROD
	DNS_ZONE = radix.equinor.com
endif

VAULT_NAME = radix-vault-$(ENVIRONMENT)
CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io
APP_ALIAS_BASE_URL = app.$(DNS_ZONE)

DATE = $(shell date +%F_%T)
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

.PHONY: test
test:	
	go test -cover `go list ./... | grep -v 'pkg/client'`

define make-docker-build
  	build-$1:
		docker build -t $(DOCKER_REGISTRY)/radix-$1:$(VERSION) -t $(DOCKER_REGISTRY)/radix-$1:$(BRANCH)-$(VERSION) -t $(DOCKER_REGISTRY)/radix-$1:$(TAG) --build-arg date="$(DATE)" --build-arg branch="$(BRANCH)" --build-arg commitid="$(HASH)" -f $1.Dockerfile .
  	build:: build-$1
endef

define make-docker-push
  	push-$1:
		az acr login --name $(CONTAINER_REPO)
		docker push $(DOCKER_REGISTRY)/radix-$1:$(BRANCH)-$(VERSION)
		docker push $(DOCKER_REGISTRY)/radix-$1:$(TAG)
  	push:: push-$1
endef

define make-docker-deploy
  	deploy-$1:
		make build-$1
		make push-$1
endef

$(foreach element,$(DOCKER_FILES),$(eval $(call make-docker-build,$(element))))
$(foreach element,$(DOCKER_FILES),$(eval $(call make-docker-push,$(element))))
$(foreach element,$(DOCKER_FILES),$(eval $(call make-docker-deploy,$(element))))

# deploys radix operator using helm chart in radixdev/radixprod acr
deploy-via-helm:
ifndef OVERIDE_BRANCH
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
	    ./charts/radix-operator/. \
		--namespace default \
	    --set dnsZone=$(DNS_ZONE) \
		--set appAliasBaseURL=$(APP_ALIAS_BASE_URL) \
		--set prometheusName=radix-stage1 \
		--set imageRegistry=$(DOCKER_REGISTRY) \
		--set clusterName=$(CLUSTER_NAME) \
		--set image.tag=$(BRANCH)-$(VERSION) \
    	--set isPlaygroundCluster="false" \
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

deploy-acr-builder:
	az acr login --name $(CONTAINER_REPO)
	docker build -t $(DOCKER_REGISTRY)/radix-image-builder:$(BRANCH)-$(VERSION) ./pipeline-runner/builder/
	docker push $(DOCKER_REGISTRY)/radix-image-builder:$(BRANCH)-$(VERSION)

ROOT_PACKAGE=github.com/equinor/radix-operator
CUSTOM_RESOURCE_NAME=radix
CUSTOM_RESOURCE_VERSION=v1

.PHONY: code-gen
code-gen: 
	vendor/k8s.io/code-generator/generate-groups.sh all $(ROOT_PACKAGE)/pkg/client $(ROOT_PACKAGE)/pkg/apis $(CUSTOM_RESOURCE_NAME):$(CUSTOM_RESOURCE_VERSION)