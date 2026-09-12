BINARY ?= manager
IMG ?= ghcr.io/thalassa-cloud/thalassa-workload-identity-controller:local

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.11.0

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: generate
generate: ## Generate deepcopy methods and CRDs.
	go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0 object paths=./api/...
	go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0 crd:crdVersions=v1 paths=./api/... output:crd:dir=./chart/thalassa-workload-identity-controller/crds

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run unit tests.
	go test ./... -coverprofile cover.out

.PHONY: lint
lint: golangci-lint ## Run golangci-lint.
	"$(GOLANGCI_LINT)" run

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(LOCALBIN)/$(BINARY) ./cmd

.PHONY: docker-build
docker-build: ## Build a local container image.
	docker build -t $(IMG) .

.PHONY: snapshot
snapshot: ## Create a local GoReleaser snapshot (no publish).
	goreleaser release --clean --snapshot --skip=validate

##@ Dependencies

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef
