## Dependency versions

CONTROLLER_RUNTIME_VERSION=$(shell awk '/sigs\.k8s\.io\/controller-runtime/ {print substr($$2, 2)}' go.mod)
CONTROLLER_TOOLS_VERSION=$(shell awk '/sigs\.k8s\.io\/controller-tools/ {print substr($$2, 2)}' go.mod)
CSI_VERSION=1.5.0
PROTOC_VERSION=3.19.3
KIND_VERSION=v0.11.1
HELM_VERSION=3.8.0
HELM_DOCS_VERSION=1.7.0
YQ_VERSION=4.18.1
GINKGO_VERSION := $(shell awk '/github.com\/onsi\/ginkgo/ {print substr($$2, 2)}' go.mod)

SUDO=sudo
CURL=curl -Lsf
BINDIR := $(shell pwd)/bin
CONTROLLER_GEN := $(BINDIR)/controller-gen
STATICCHECK := $(BINDIR)/staticcheck
NILERR := $(BINDIR)/nilerr
PROTOC := PATH=$(BINDIR):$(PATH) $(BINDIR)/protoc -I=$(shell pwd)/include:.
PACKAGES := unzip lvm2 xfsprogs
ENVTEST_ASSETS_DIR := $(shell pwd)/testbin

GO_FILES=$(shell find -name '*.go' -not -name '*_test.go')
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
GOFLAGS =
export GOFLAGS

BUILD_TARGET=hypertopolvm
TOPOLVM_VERSION ?= devel
IMAGE_TAG ?= latest

## for custom build kind node
## ignore if you do not need a custom image
KIND_NODE_VERSION=v1.21.1

ENVTEST_KUBERNETES_VERSION=1.23

PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' go.mod)
PROTOC_GEN_DOC_VERSION := $(shell awk '/github.com\/pseudomuto\/protoc-gen-doc/ {print substr($$2, 2)}' go.mod)
PROTOC_GEN_GO_GRPC_VERSION := $(shell awk '/google.golang.org\/grpc\/cmd\/protoc-gen-go-grpc/ {print substr($$2, 2)}' go.mod)

# Set the shell used to bash for better error handling.
SHELL = /bin/bash
.SHELLFLAGS = -e -o pipefail -c

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

csi.proto:
	$(CURL) -o $@ https://raw.githubusercontent.com/container-storage-interface/spec/v$(CSI_VERSION)/csi.proto
	sed -i 's,^option go_package.*$$,option go_package = "github.com/topolvm/topolvm/csi";,' csi.proto
	sed -i '/^\/\/ Code generated by make;.*$$/d' csi.proto

csi/csi.pb.go: csi.proto
	mkdir -p csi
	$(PROTOC) --go_out=module=github.com/topolvm/topolvm:. $<

csi/csi_grpc.pb.go: csi.proto
	mkdir -p csi
	$(PROTOC) --go-grpc_out=module=github.com/topolvm/topolvm:. $<

lvmd/proto/lvmd.pb.go: lvmd/proto/lvmd.proto
	$(PROTOC) --go_out=module=github.com/topolvm/topolvm:. $<

lvmd/proto/lvmd_grpc.pb.go: lvmd/proto/lvmd.proto
	$(PROTOC) --go-grpc_out=module=github.com/topolvm/topolvm:. $<

docs/lvmd-protocol.md: lvmd/proto/lvmd.proto
	$(PROTOC) --doc_out=./docs --doc_opt=markdown,$@ $<

PROTOBUF_GEN = csi/csi.pb.go csi/csi_grpc.pb.go \
	lvmd/proto/lvmd.pb.go lvmd/proto/lvmd_grpc.pb.go docs/lvmd-protocol.md

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
		crd:crdVersions=v1 \
		rbac:roleName=topolvm-controller \
		webhook \
		paths="./api/...;./controllers;./hook;./driver/k8s;./pkg/..." \
		output:crd:artifacts:config=config/crd/bases
	$(BINDIR)/yq eval 'del(.status)' config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml > charts/topolvm/crds/topolvm.cybozu.com_logicalvolumes.yaml

.PHONY: generate
generate: $(PROTOBUF_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="./hack/boilerplate.go.txt" paths="./api/..."

.PHONY: check-uncommitted
check-uncommitted: ## Check if latest generated artifacts are committed.
	$(MAKE) manifests
	find . -name "*.pb.go" -delete
	$(MAKE) generate
	./bin/helm-docs -c charts/topolvm/
	git diff --exit-code --name-only

.PHONY: lint
lint: ## Run lint
	test -z "$$(gofmt -s -l . | grep -vE '^vendor|^api/v1/zz_generated.deepcopy.go' | tee /dev/stderr)"
	$(STATICCHECK) ./...
	test -z "$$($(NILERR) ./... 2>&1 | tee /dev/stderr)"
	go vet ./...
	test -z "$$(go vet ./... | grep -v '^vendor' | tee /dev/stderr)"

.PHONY: test
test: lint ## Run lint and unit tests.
	go install ./...

	mkdir -p $(ENVTEST_ASSETS_DIR)
	source <($(BINDIR)/setup-envtest use $(ENVTEST_KUBERNETES_VERSION) --bin-dir=$(ENVTEST_ASSETS_DIR) -p env); GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn go test -race -v ./...

.PHONY: clean
clean: ## Clean working directory.
	rm -rf build/
	rm -rf bin/
	rm -rf include/
	rm -rf testbin/

##@ Build

.PHONY: build
build: build-topolvm csi-sidecars ## Build binaries.

.PHONY: build-topolvm
build-topolvm: build/hypertopolvm build/lvmd

build/hypertopolvm: $(GO_FILES)
	mkdir -p build
	go build -o $@ -ldflags "-w -s -X github.com/topolvm/topolvm.Version=$(TOPOLVM_VERSION)" ./pkg/hypertopolvm

build/lvmd:
	mkdir -p build
	CGO_ENABLED=0 go build -o $@ -ldflags "-w -s -X github.com/topolvm/topolvm.Version=$(TOPOLVM_VERSION)" ./pkg/lvmd

.PHONY: csi-sidecars
csi-sidecars: ## Build sidecar images.
	mkdir -p build
	make -f csi-sidecars.mk OUTPUT_DIR=build

.PHONY: image
image: ## Build topolvm images.
	docker build --no-cache -t $(IMAGE_PREFIX)topolvm:devel --build-arg TOPOLVM_VERSION=$(TOPOLVM_VERSION) .
	docker build --no-cache -t $(IMAGE_PREFIX)topolvm-with-sidecar:devel --build-arg IMAGE_PREFIX=$(IMAGE_PREFIX) -f Dockerfile.with-sidecar .

.PHONY: tag
tag: ## Tag topolvm images.
	docker tag $(IMAGE_PREFIX)topolvm:devel $(IMAGE_PREFIX)topolvm:$(IMAGE_TAG)
	docker tag $(IMAGE_PREFIX)topolvm-with-sidecar:devel $(IMAGE_PREFIX)topolvm-with-sidecar:$(IMAGE_TAG)

.PHONY: push
push: ## Push topolvm images.
	docker push $(IMAGE_PREFIX)topolvm:$(IMAGE_TAG)
	docker push $(IMAGE_PREFIX)topolvm-with-sidecar:$(IMAGE_TAG)

##@ Setup

.PHONY: install-kind
install-kind:
	GOBIN=$(BINDIR) go install sigs.k8s.io/kind@$(KIND_VERSION)

.PHONY: tools
tools: install-kind ## Install development tools.
	GOBIN=$(BINDIR) go install honnef.co/go/tools/cmd/staticcheck@latest
	GOBIN=$(BINDIR) go install github.com/gostaticanalysis/nilerr/cmd/nilerr@latest
	# Follow the official documentation to install the `latest` version, because explicitly specifying the version will get an error.
	GOBIN=$(BINDIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	GOBIN=$(BINDIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION)

	curl -sfL -o protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-linux-x86_64.zip
	unzip -o protoc.zip bin/protoc 'include/*'
	rm -f protoc.zip
	GOBIN=$(BINDIR) go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(BINDIR) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$(PROTOC_GEN_GO_GRPC_VERSION)
	GOBIN=$(BINDIR) go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@v$(PROTOC_GEN_DOC_VERSION)

	GOBIN=$(BINDIR) go install github.com/onsi/ginkgo/ginkgo@v$(GINKGO_VERSION)

	GOBIN=$(BINDIR) go install github.com/norwoodj/helm-docs/cmd/helm-docs@v$(HELM_DOCS_VERSION)
	curl -L -sS https://get.helm.sh/helm-v$(HELM_VERSION)-linux-amd64.tar.gz \
		| tar xvz -C $(BINDIR) --strip-components 1 linux-amd64/helm
	wget https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64 -O $(BINDIR)/yq \
		&& chmod +x $(BINDIR)/yq

.PHONY: setup
setup: ## Setup local environment.
	$(SUDO) apt-get update
	$(SUDO) apt-get -y install --no-install-recommends $(PACKAGES)
	$(MAKE) tools

.PHONY: kind-node
kind-node:
	git clone --depth 1 https://github.com/kubernetes/kubernetes.git -b $(KIND_NODE_VERSION) /tmp/kind-node
	$(BINDIR)/kind build node-image --kube-root /tmp/kind-node --image quay.io/topolvm/kind-node:$(KIND_NODE_VERSION)
