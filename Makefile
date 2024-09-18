export GO111MODULE = on

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git log -1 --format='%H' | cut -c 1-8)

# don't override user values
ifeq (,$(VERSION))
  VERSION := $(shell git describe --tags)
  # if VERSION is empty, then populate it with branch's name and raw commit hash
  ifeq (,$(VERSION))
    VERSION := $(BRANCH)-$(COMMIT)
  endif
endif

ldflags = -X github.com/dymensionxyz/eibc-client/version.BuildVersion=$(VERSION)

BUILD_FLAGS := -ldflags '$(ldflags)'

###########
# Install #
###########

all: install

.PHONY: install
install: build
	@echo "--> installing eibc-client"
	mv build/eibc-client $(GOPATH)/bin/eibc-client

.PHONY: build
build: go.sum ## Compiles the eibc binary
	@echo "--> Ensure dependencies have not been modified"
	@go mod verify
	@echo "--> building eibc-client"
	@go build  -o build/eibc-client $(BUILD_FLAGS) ./

###########
# Docker  #
###########

.PHONY: docker-build
docker-build:
	@echo "--> Building Docker image"
	docker build -t eibc-client:latest .
