FROM golang:1.13.6 as builder

ARG BUILD_VERSION
ARG BUILD_DATE
ARG GIT_COMMIT
ARG GIT_TREE_STATE
ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64

ENV LDFLAGS="-X github.com/flant/yandex-cloud-controller-manager/vendor/k8s.io/kubernetes/pkg/version.buildDate=${BUILD_DATE} \
             -X github.com/flant/yandex-cloud-controller-manager/vendor/k8s.io/kubernetes/pkg/version.gitVersion=${BUILD_VERSION} \
             -X github.com/flant/yandex-cloud-controller-manager/vendor/k8s.io/kubernetes/pkg/version.gitCommit=${GIT_COMMIT} \
             -X github.com/flant/yandex-cloud-controller-manager/vendor/k8s.io/kubernetes/pkg/version.gitTreeState=${GIT_TREE_STATE}"

WORKDIR /go/src/github.com/flant/yandex-cloud-controller-manager

COPY go.mod  go.mod
COPY go.sum  go.sum
RUN go get -d -v ./...

COPY cmd/    cmd/
COPY pkg/    pkg/
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags "${LDFLAGS}" -a -o yandex-cloud-controller-manager ./cmd/yandex-cloud-controller-manager


FROM alpine:3.11

RUN apk add --no-cache ca-certificates
COPY --from=builder /go/src/github.com/flant/yandex-cloud-controller-manager/yandex-cloud-controller-manager /bin/

CMD ["/bin/yandex-cloud-controller-manager"]
