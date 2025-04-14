# Copyright 2020 DigitalOcean
# Copyright 2020 Flant
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.24.2-alpine3.20@sha256:00f149d5963f415a8a91943531b9092fde06b596b276281039604292d8b2b9c8 as build

ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64

WORKDIR /go/src/app
ADD . /go/src/app

RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} \
    go build -a \
    -o /go/bin/yandex-cloud-controller-manager \
    ./cmd/yandex-cloud-controller-manager


FROM alpine:3.21.2@sha256:56fa17d2a7e7f168a043a2712e63aed1f8543aeafdcee47c58dcffe38ed51099

RUN apk add --no-cache ca-certificates
COPY --from=build /go/bin/yandex-cloud-controller-manager /bin/

CMD ["/bin/yandex-cloud-controller-manager"]
