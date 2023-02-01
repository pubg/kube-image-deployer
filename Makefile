#!/usr/bin/make -f

BUILDDIR ?= $(CURDIR)/build

ldflags := -w -s
BUILD_FLAGS := -ldflags '$(ldflags)' -trimpath

.PHONY: all kube-image-deployer test

all: kube-image-deployer

kube-image-deployer:
	@go build -mod=readonly $(BUILD_FLAGS) -o $(BUILDDIR)/kube-image-deployer ./main.go

test:
	@TEST_DOCKER_ECR_SKIP=1 TEST_DOCKER_PRIVATE_SKIP=1 go test ./...