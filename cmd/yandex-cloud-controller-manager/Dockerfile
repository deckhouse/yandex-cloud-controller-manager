FROM golang:1.15.5-alpine3.12 as builder

ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64

WORKDIR /go/src/github.com/flant/yandex-cloud-controller-manager

COPY go.mod  go.mod
COPY go.sum  go.sum
RUN go get -d -v ./...

COPY cmd/    cmd/
COPY pkg/    pkg/
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} go build -a -o yandex-cloud-controller-manager ./cmd/yandex-cloud-controller-manager


FROM alpine:3.12

RUN apk add --no-cache ca-certificates
COPY --from=builder /go/src/github.com/flant/yandex-cloud-controller-manager/yandex-cloud-controller-manager /bin/

CMD ["/bin/yandex-cloud-controller-manager"]
