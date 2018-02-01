# Architectures supported: amd64, arm, arm64 and ppc64le
ARCH ?= amd64
BUILD_IMAGE ?= golang:1.9-alpine
BIN = tectonic-stats-extender
TEST_BIN = tectonic-stats-test
PKG = github.com/coreos/container-images/tectonic-stats-extender
REGISTRY ?= quay.io/coreos
IMAGE = $(REGISTRY)/$(BIN)
TEST_IMAGE = $(REGISTRY)/$(TEST_BIN)
VERSION :=
TAG := $(shell git describe --abbrev=0 --tags HEAD 2>/dev/null)
COMMIT := $(shell git rev-parse HEAD)
ifeq ($(VERSION),)
    ifeq ($(TAG),)
        VERSION := unknown-$(COMMIT)
    else
        ifeq ($(COMMIT), $(shell git rev-list -n1 $(TAG)))
            VERSION := $(TAG)
        else
            VERSION := $(TAG)-$(COMMIT)
        endif
    endif
endif
DIRTY := $(shell test -z "$$(git diff --shortstat 2>/dev/null)" || echo -dirty)
VERSION := $(VERSION)$(DIRTY)

.PHONY: build
build: bin/$(ARCH)/$(BIN)
.PHONY: test
test: bin/$(ARCH)/$(TEST_BIN)

BIN_SOURCES := $(shell find . -type f -name '*.go' | grep -v test/)
bin/$(ARCH)/$(BIN): build-dirs $(BIN_SOURCES)
	@echo "building: $@"
	@docker run                                                            \
	    -ti                                                                \
	    -u $$(id -u):$$(id -g)                                             \
	    -v $$(pwd)/.go:/go                                                 \
	    -v $$(pwd):/go/src/$(PKG)                                          \
	    -v $$(pwd)/bin/$(ARCH):/go/bin                                     \
	    -v $$(pwd)/.go/std/$(ARCH):/usr/local/go/pkg/linux_$(ARCH)_static  \
	    -w /go/src/$(PKG)                                                  \
	    $(BUILD_IMAGE)                                                     \
	    /bin/sh -c "                                                       \
	        ARCH=$(ARCH)                                                   \
	        VERSION=$(VERSION)                                             \
	        CGO_ENABLED=0                                                  \
		go build -o bin/$(ARCH)/$(BIN)                                 \
		    -ldflags='-w -X $(PKG)/pkg/version.Version=$(VERSION)'     \
		    $(PKG)/cmd/extender                                        \
	    "

TEST_SOURCES := $(shell find . -type f -name '*.go' | grep -v cmd/ | grep -v pkg/)
bin/$(ARCH)/$(TEST_BIN): build-dirs $(TEST_SOURCES)
	@echo "building: $@"
	@docker run                                                            \
	    -ti                                                                \
	    -u $$(id -u):$$(id -g)                                             \
	    -v $$(pwd)/.go:/go                                                 \
	    -v $$(pwd):/go/src/$(PKG)                                          \
	    -v $$(pwd)/bin/$(ARCH):/go/bin                                     \
	    -v $$(pwd)/.go/std/$(ARCH):/usr/local/go/pkg/linux_$(ARCH)_static  \
	    -w /go/src/$(PKG)                                                  \
	    $(BUILD_IMAGE)                                                     \
	    /bin/sh -c "                                                       \
	        ARCH=$(ARCH)                                                   \
	        VERSION=$(VERSION)                                             \
	        CGO_ENABLED=0                                                  \
		go test -c -o bin/$(ARCH)/$(TEST_BIN) ./test                   \
	    "

.PHONY: build-dirs
build-dirs:
	@mkdir -p bin/$(ARCH)
	@mkdir -p .go/src/$(PKG) .go/pkg .go/bin .go/std/$(ARCH)

.PHONY: clean
clean:
	rm -rf .go bin

.PHONY: container
container:
	@docker build -t $(IMAGE):$(VERSION) --build-arg ARCH=$(ARCH) .

.PHONY: container-test
container-test:
	@docker build -t $(TEST_IMAGE):$(VERSION) --build-arg ARCH=$(ARCH) --file Dockerfile.test .

.PHONY: vendor
vendor: glide.yaml
	@glide install -v
	@glide-vc --use-lock-file --no-tests --only-code
