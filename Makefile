export GO111MODULE = on

###########
# Install #
###########

all: install

.PHONY: install
install: build
	@echo "--> installing eibc-client"
	mv build/eibc $(GOPATH)/bin/eibc-client

.PHONY: build
build: go.sum ## Compiles the rollapd binary
	@echo "--> Ensure dependencies have not been modified"
	@go mod verify
	@echo "--> building eibc-client"
	@go build  -o build/eibc-client ./

###########
# Docker  #
###########

.PHONY: docker-build
docker-build:
	@echo "--> Building Docker image"
	docker build -t eibc-client:latest .
