DOCKER_REGISTRY	?= radixdev.azurecr.io

BINS	= radix-operator rx
IMAGES	= radix-operator rx

GIT_TAG		= $(shell git describe --tags --always 2>/dev/null)
VERSION		?= ${GIT_TAG}
IMAGE_TAG 	?= ${VERSION}
LDFLAGS		+= "-ldflags -X github.com/statoil/radix-operator/pkg/version.Version=$(shell cat VERSION)"


CX_OSES		= linux windows
CX_ARCHS	= amd64

.PHONY: build
build: $(BINS)

.PHONY: test
test:
	go test -cover `go list ./... | grep -v 'pkg/client\|apis/radix'`

.PHONY: $(BINS)
$(BINS): vendor
	go build -ldflags '$(LDFLAGS)' -o bin/$@ ./$@

build-docker-bins: $(addsuffix -docker-bin,$(BINS))
%-docker-bin: vendor
	GOOS=linux GOARCH=$(CX_ARCHS) CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o ./$*/rootfs/$* ./$*

.PHONY: docker-build
docker-build: build-docker-bins
docker-build: $(addsuffix -image,$(IMAGES))

%-image:
	docker build $(DOCKER_BUILD_FLAGS) -t $(DOCKER_REGISTRY)/$*:$(IMAGE_TAG) $*

.PHONY: docker-push
docker-push: $(addsuffix -push,$(IMAGES))

%-push:
	docker push $(DOCKER_REGISTRY)/$*:$(IMAGE_TAG)

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

ROOT_PACKAGE=github.com/statoil/radix-operator
CUSTOM_RESOURCE_NAME=radix
CUSTOM_RESOURCE_VERSION=v1

.PHONY: code-gen
code-gen: 
	vendor/k8s.io/code-generator/generate-groups.sh all $(ROOT_PACKAGE)/pkg/client $(ROOT_PACKAGE)/pkg/apis $(CUSTOM_RESOURCE_NAME):$(CUSTOM_RESOURCE_VERSION)

# make deploy VERSION=keaaa-v1
# VERSION variable is mandatory 
# need to connect to container registry first - docker login radixdev.azurecr.io -u radixdev -p <%password%>
deploy:
	make deploy-operator
	make deploy-brigade

deploy-operator:
	dep ensure
	# fixes error in dependency
	sed -i "" 's/spt.Token/spt.Token()/g' ./vendor/k8s.io/client-go/plugin/pkg/client/auth/azure/azure.go
	make docker-build
	make docker-push
	
	# update docker image version in deploy file - file name should be a variable
	kubectl get deploy radix-operator -o yaml > oldRadixOperatorDef.yaml 
	sed -E "s/(image: radixdev.azurecr.io\/radix-operator).*/\1:$(VERSION)/g" ./oldRadixOperatorDef.yaml > newRadixOperatorDef.yaml

	kubectl apply -f newRadixOperatorDef.yaml

	rm oldRadixOperatorDef.yaml newRadixOperatorDef.yaml

deploy-brigade:
	# update script based on the generic-build.js and put it into radix-script cm
	cp ./brigade-scripts/generic-build.js ./oldBuildScript.js
	sed -E "s/(radixdev.azurecr.io\/rx:).[a-z0-9]*/\1$(VERSION)/g" ./oldBuildScript.js > newBuildScript.js
	minify newBuildScript.js > minifiedBuildScript.js
	sed -i "" 's/"/\\"/g' minifiedBuildScript.js
	perl -pi -e 'chomp if eof' minifiedBuildScript.js
	
	# update kc cm
	kubectl get cm radix-script -o json > oldRadixScript.json
	sed -E 's/"brigade.js":.*/"brigade.js":"<inputscript>"/g' ./oldRadixScript.json > newRadixScript.json
	perl -pe 's/<inputscript>/`cat minifiedBuildScript.js`/ge' newRadixScript.json > finalRadixScript.json
	
	kubectl apply -f finalRadixScript.json

	rm oldRadixScript.json oldBuildScript.js newRadixScript.json newBuildScript.js minifiedBuildScript.js finalRadixScript.json