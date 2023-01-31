BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUILD_VERSION ?= $(shell cat VERSION)
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || true)
GIT_TREE_STATE ?= $(shell if git_status=$$(git status --porcelain 2>/dev/null) && test -z "$$git_status"; then echo clean; else echo dirty; fi)

DOCKER_TAG ?= dev
DOCKER_IMG ?= deckhouse/yandex-cloud-controller-manager:${DOCKER_TAG}

all: test

docker-push: docker-build
	docker push ${DOCKER_IMG}
.PHONY: docker-push

docker-build:
	docker build --build-arg BUILD_DATE=${BUILD_DATE} \
				 --build-arg BUILD_VERSION=${BUILD_VERSION} \
				 --build-arg GIT_COMMIT=${GIT_COMMIT} \
				 --build-arg GIT_TREE_STATE=${GIT_TREE_STATE} \
				 -t ${DOCKER_IMG} .
.PHONY: docker-build

test: build
	go test -v -cover -coverprofile=coverage.out -covermode=atomic $(shell go list ./... | grep -v vendor)
.PHONY: test

build: dep lint
	go build ./cmd/yandex-cloud-controller-manager
.PHONY: build

gofmt:
	gofmt -s -w $(shell go list -f {{.Dir}} ./... | grep -v vendor)
.PHONY: gofmt

lint: $(GOPATH)/bin/golangci-lint
	golangci-lint run
.PHONY: lint

goimports: $(GOPATH)/bin/goimports
	goimports -w $(shell go list -f {{.Dir}} ./... | grep -v vendor)
.PHONY: goimports

dep:
	go get -d -v ./...
.PHONY: dep

$(GOPATH)/bin/goimports:
	go install golang.org/x/tools/cmd/goimports@latest

$(GOPATH)/bin/golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1
