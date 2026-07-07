# syntax=docker/dockerfile:1

##
## Build
## Go 크로스컴파일을 사용해 빌드는 항상 호스트 아키텍처에서 수행한다.
## (QEMU 에뮬레이션 없이 멀티아치 이미지를 빠르게 빌드)
##
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . ./

ARG TARGETOS TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags '-w -s' -trimpath -o kube-image-deployer

##
## Deploy
## non-root 사용자로 실행한다. Docker credential은 DOCKER_CONFIG 경로에 마운트한다.
##
FROM alpine:3.22

RUN adduser -D -u 65532 kube-image-deployer

WORKDIR /app

COPY --from=build /build/kube-image-deployer kube-image-deployer

ENV DOCKER_CONFIG=/home/kube-image-deployer/.docker

USER 65532:65532

ENTRYPOINT ["/app/kube-image-deployer"]
