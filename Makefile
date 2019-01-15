DOCKER_FILES	= operator pipeline

VERSION 	?= latest

ENVIRONMENT ?= dev

CONTAINER_REPO ?= radix$(ENVIRONMENT)
DOCKER_REGISTRY	?= $(CONTAINER_REPO).azurecr.io

DATE = $(shell date +%F_%T)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
HASH := $(shell git rev-parse HEAD)

CLUSTER_NAME = $(shell kubectl config get-contexts | grep '*' | tr -s ' ' | cut -f 3 -d ' ')
CHART_VERSION = $(shell cat charts/radix-operator/Chart.yaml | yq --raw-output .version)

TAG := $(BRANCH)-$(HASH)

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
	az acr helm repo add --name $(CONTAINER_REPO)
	helm repo update
	helm upgrade --install radix-operator $(CONTAINER_REPO)/radix-operator --set prometheusName=radix-stage1 --set clusterName=$(CLUSTER_NAME) --set imageRegistry=$(DOCKER_REGISTRY) --set image.tag=$(TAG)

# build and deploy radix operator
helm-up:
	make deploy-operator
	make deploy-via-helm

# upgrades helm chart in radixdev/radixprod acr (does not deploy radix-operator)
helm-upgrade-operator-chart:
	tar -zcvf radix-operator-$(CHART_VERSION).tgz charts/radix-operator
	az acr helm push --name $(CONTAINER_REPO) charts/radix-operator-$(CHART_VERSION).tgz
	rm charts/radix-operator-$(CHART_VERSION).tgz

deploy-acr-builder:
	docker build -t $(DOCKER_REGISTRY)/radix-image-builder:$(BRANCH)-$(VERSION) ./pipeline-runner/builder/
	docker push $(DOCKER_REGISTRY)/radix-image-builder:$(BRANCH)-$(VERSION)

ROOT_PACKAGE=github.com/statoil/radix-operator
CUSTOM_RESOURCE_NAME=radix
CUSTOM_RESOURCE_VERSION=v1

.PHONY: code-gen
code-gen: 
	vendor/k8s.io/code-generator/generate-groups.sh all $(ROOT_PACKAGE)/pkg/client $(ROOT_PACKAGE)/pkg/apis $(CUSTOM_RESOURCE_NAME):$(CUSTOM_RESOURCE_VERSION)
	
HAS_GOMETALINTER := $(shell command -v gometalinter;)
HAS_DEP          := $(shell command -v dep;)
HAS_GIT          := $(shell command -v git;)

vendor:
ifndef HAS_GIT
	$(error You must install git)
endif
ifndef HAS_DEP
	go get -u github.com/golang/dep/cmd/dep
endif
ifndef HAS_GOMETALINTER
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install
endif
	dep ensure

.PHONY: bootstrap
bootstrap: vendor