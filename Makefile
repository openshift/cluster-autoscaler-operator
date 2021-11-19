DBG         ?= 0
PROJECT     ?= cluster-autoscaler-operator
ORG_PATH    ?= github.com/openshift
REPO_PATH   ?= $(ORG_PATH)/$(PROJECT)
VERSION     ?= $(shell git describe --always --dirty --abbrev=7)
LD_FLAGS    ?= -X $(REPO_PATH)/pkg/version.Raw=$(VERSION)
BUILD_DEST  ?= bin/cluster-autoscaler-operator
MUTABLE_TAG ?= latest
IMAGE        = origin-cluster-autoscaler-operator
BUILD_IMAGE ?= registry.ci.openshift.org/openshift/release:golang-1.17

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
  DOCKER_CMD = GOFLAGS="$(GOFLAGS)" GOPROXY="$(GOPROXY)"
  IMAGE_BUILD_CMD = imagebuilder
else
  DOCKER_CMD := docker run --rm --env GOFLAGS="$(GOFLAGS)" --env GOPROXY="$(GOPROXY)" -v "$(PWD):/go/src/$(REPO_PATH):Z" -w "/go/src/$(REPO_PATH)" $(BUILD_IMAGE)
  IMAGE_BUILD_CMD = docker build
endif

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: generate
generate: gen-deepcopy gen-crd goimports
	./hack/verify-diff.sh

.PHONY: gen-deepcopy
gen-deepcopy:
	$(DOCKER_CMD) go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen \
		paths=./pkg/apis/... object:headerFile=./hack/boilerplate.go.txt,year=2020

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
