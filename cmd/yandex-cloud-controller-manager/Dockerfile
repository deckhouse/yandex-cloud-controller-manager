FROM golang:1.11.4 as builder

WORKDIR /go/src/github.com/dlisin/yandex-cloud-controller-manager
COPY cmd/    cmd/
COPY pkg/    pkg/
COPY vendor  vendor/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o yandex-cloud-controller-manager ./cmd/yandex-cloud-controller-manager

FROM alpine:3.8

RUN apk add --no-cache ca-certificates
COPY --from=builder /go/src/github.com/dlisin/yandex-cloud-controller-manager/yandex-cloud-controller-manager /bin/

CMD ["/bin/yandex-cloud-controller-manager"]
