DOCKER_IMG ?= dlisin/yandex-cloud-controller-manager:latest

all: test

docker-push: docker-build
	docker push ${DOCKER_IMG}

docker-build: test
	docker build -t ${DOCKER_IMG} -f ./cmd/yandex-cloud-controller-manager/Dockerfile .

test: build
	go test $(shell go list ./... | grep -v vendor)

build: gofmt goimports golint govet
	go build ./cmd/yandex-cloud-controller-manager

gofmt:
	gofmt -s -w $(shell go list -f {{.Dir}} ./... | grep -v vendor)

govet:
	go vet $(shell go list ./... | grep -v vendor)

golint: $(GOPATH)/bin/golint
	golint $(shell go list ./... | grep -v vendor)

goimports: $(GOPATH)/bin/goimports
	goimports -w $(shell go list -f {{.Dir}} ./... | grep -v vendor)

$(GOPATH)/bin/goimports:
	go get golang.org/x/tools/cmd/goimports

$(GOPATH)/bin/golint:
	go get -u golang.org/x/lint/golint
