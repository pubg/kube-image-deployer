#!/usr/bin/make -f

BUILDDIR ?= $(CURDIR)/build

ldflags := -w -s
BUILD_FLAGS := -ldflags '$(ldflags)' -trimpath

.PHONY: all kube-image-deployer

all: kube-image-deployer

kube-image-deployer:
	@go build -mod=readonly $(BUILD_FLAGS) -o $(BUILDDIR)/kube-image-deployer ./main.go
