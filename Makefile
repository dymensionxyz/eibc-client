export GO111MODULE = on

###########
# Install #
###########

all: install

.PHONY: install
install: build
	@echo "--> installing order-client"
	mv build/order-client $(GOPATH)/bin/order-client


.PHONY: build
build: go.sum ## Compiles the rollapd binary
	@echo "--> Ensure dependencies have not been modified"
	@go mod verify
	@echo "--> building order-client"
	@go build  -o build/order-client ./

###########
# Docker  #
###########

.PHONY: docker-build
docker-build:
	@echo "--> Building Docker image"
	docker build -t order-client:latest .
