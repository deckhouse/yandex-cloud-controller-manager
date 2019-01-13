IMG ?= dlisin/yandex-cloud-controller-manager:latest

all: build

$(GOPATH)/bin/goimports:
	go get golang.org/x/tools/cmd/goimports

$(GOPATH)/bin/dep:
	go get -u github.com/golang/dep/cmd/dep

build: fmt
	go build ./cmd/yandex-cloud-controller-manager

dep: $(GOPATH)/bin/dep
	dep ensure -v

fmt: vet imports
	gofmt -s -w cmd pkg

vet:
	go vet ./...

imports: $(GOPATH)/bin/goimports
	goimports -w cmd pkg

docker-build: build
	docker build -t ${IMG} -f ./cmd/yandex-cloud-controller-manager/Dockerfile .

docker-push: docker-build
	docker push ${IMG}
