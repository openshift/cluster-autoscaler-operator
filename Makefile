DBG         ?= 0
PROJECT     ?= cluster-autoscaler-operator
ORG_PATH    ?= github.com/openshift
REPO_PATH   ?= $(ORG_PATH)/$(PROJECT)
VERSION     ?= $(shell git describe --always --dirty --abbrev=7)
LD_FLAGS    ?= -X $(REPO_PATH)/pkg/version.Raw=$(VERSION)
BUILD_DEST  ?= bin/cluster-autoscaler-operator
MUTABLE_TAG ?= latest
IMAGE        = origin-cluster-autoscaler-operator

GO111MODULE = on
export GO111MODULE
GOFLAGS ?= -mod=vendor
export GOFLAGS
GOPROXY ?=
export GOPROXY

ifeq ($(DBG),1)
GOGCFLAGS ?= -gcflags=all="-N -l"
endif

.PHONY: all
all: build images check

NO_DOCKER ?= 0
ifeq ($(NO_DOCKER), 1)
  DOCKER_CMD =
  IMAGE_BUILD_CMD = imagebuilder
else
  DOCKER_CMD := docker run --rm --env GO111MODULE="$(GO111MODULE)" --env GOFLAGS="$(GOFLAGS)" --env GOPROXY="$(GOPROXY)" -v "$(PWD):/go/src/$(REPO_PATH):Z" -w "/go/src/$(REPO_PATH)" openshift/origin-release:golang-1.13
  IMAGE_BUILD_CMD = docker build
endif

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify


# This is a hack. The operator-sdk doesn't currently let you configure
# output paths for the generated CRDs.  It also requires that they
# already exist in order to regenerate the OpenAPI bits, so we do some
# copying around here.
.PHONY: generate
generate: ## Code generation (requires operator-sdk >= v0.5.0)
	mkdir -p deploy/crds

	cp install/01_clusterautoscaler.crd.yaml \
	  deploy/crds/autoscaling_v1_clusterautoscaler_crd.yaml
	cp install/02_machineautoscaler.crd.yaml \
	  deploy/crds/autoscaling_v1beta1_machineautoscaler_crd.yaml

	operator-sdk generate k8s
	operator-sdk generate openapi

	cp deploy/crds/autoscaling_v1_clusterautoscaler_crd.yaml \
	  install/01_clusterautoscaler.crd.yaml
	cp deploy/crds/autoscaling_v1beta1_machineautoscaler_crd.yaml \
	  install/02_machineautoscaler.crd.yaml

.PHONY: gen-crd
gen-crd:
	$(DOCKER_CMD) ./hack/gen-crd.sh

.PHONY: build
build: ## build binaries
	$(DOCKER_CMD) go build $(GOGCFLAGS) -ldflags "$(LD_FLAGS)" -o "$(BUILD_DEST)" "$(REPO_PATH)/cmd/manager"

.PHONY: images
images: ## Create images
	$(IMAGE_BUILD_CMD) -t "$(IMAGE):$(VERSION)" -t "$(IMAGE):$(MUTABLE_TAG)" ./

.PHONY: push
push:
	docker push "$(IMAGE):$(VERSION)"
	docker push "$(IMAGE):$(MUTABLE_TAG)"

.PHONY: check
check: fmt vet lint test ## Check your code

.PHONY: check-pkg
check-pkg:
	./hack/verify-actuator-pkg.sh

.PHONY: test
test: ## Run unit tests
	$(DOCKER_CMD) go test -race -cover ./...

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	hack/e2e.sh

.PHONY: lint
lint: ## Go lint your code
	hack/go-lint.sh -min_confidence 0.3 $(go list -f '{{ .ImportPath }}' ./...)

.PHONY: fmt
fmt: ## Go fmt your code
	hack/go-fmt.sh .

.PHONY: goimports
goimports: ## Go fmt your code
	hack/goimports.sh .

.PHONY: vet
vet: ## Apply go vet to all go files
	hack/go-vet.sh ./...

.PHONY: help
help:
	@grep -E '^[a-zA-Z/0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
