# Copyright (c) 2018-2019, NVIDIA CORPORATION. All rights reserved.

.PHONY: all build builder test
.DEFAULT_GOAL := all


##### Global variables #####

DOCKERFILE ?= $(CURDIR)/docker/Dockerfile

REGISTRY ?= nvidia
TAG      ?= 1.0.0-alpha1


##### Public rules #####

all: build

build:
	docker build --pull \
		--tag $(REGISTRY)/container-toolkit:$(TAG) \
		--file $(DOCKERFILE) .

push:
	docker push $(REGISTRY)/container-toolkit:$(TAG)
