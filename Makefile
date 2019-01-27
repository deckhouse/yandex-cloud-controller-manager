DOCKER_TAG ?=dev
DOCKER_IMG ?= dlisin/yandex-cloud-controller-manager:${DOCKER_TAG}

all: test

docker-push: docker-build
	docker push ${DOCKER_IMG}
.PHONY: docker-push

docker-build: test
	docker build -t ${DOCKER_IMG} -f ./cmd/yandex-cloud-controller-manager/Dockerfile .
.PHONY: docker-build

test: build
	go test -v -cover -coverprofile=coverage.out -covermode=atomic $(shell go list ./... | grep -v vendor)
.PHONY: test

build: gofmt goimports golint govet
	go build ./cmd/yandex-cloud-controller-manager
.PHONY: build

gofmt:
	gofmt -s -w $(shell go list -f {{.Dir}} ./... | grep -v vendor)
.PHONY: gofmt

govet:
	go vet $(shell go list ./... | grep -v vendor)
.PHONY: govet

golint: $(GOPATH)/bin/golint
	golint $(shell go list ./... | grep -v vendor)
.PHONY: golint

goimports: $(GOPATH)/bin/goimports
	goimports -w $(shell go list -f {{.Dir}} ./... | grep -v vendor)
.PHONY: goimports

dep: $(GOPATH)/bin/dep
	dep ensure -v
.PHONY: dep

$(GOPATH)/bin/goimports:
	go get -u golang.org/x/tools/cmd/goimports

$(GOPATH)/bin/golint:
	go get -u golang.org/x/lint/golint

$(GOPATH)/bin/dep:
	go get -u github.com/golang/dep/cmd/dep
