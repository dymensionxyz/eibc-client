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

###############################################################################
###                                Releasing                                ###
###############################################################################

PACKAGE_NAME:=github.com/dymensionxyz/eibc-client
GOLANG_CROSS_VERSION  = v1.23
GOPATH ?= '$(HOME)/go'
release-dry-run:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-v ${GOPATH}/pkg:/go/pkg \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		--clean --skip=validate --skip=publish --snapshot

release:
	@if [ ! -f ".release-env" ]; then \
		echo "\033[91m.release-env is required for release\033[0m";\
		exit 1;\
	fi
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		--env-file .release-env \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --clean --skip=validate

.PHONY: release-dry-run release
