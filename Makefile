# Copyright (c) 2018-2019, NVIDIA CORPORATION. All rights reserved.

.PHONY: all build builder test
.DEFAULT_GOAL := all


##### Global variables #####

DOCKERFILE ?= $(CURDIR)/Dockerfile

REGISTRY ?= nvidia
TAG      ?= 1.0.0-alpha1


##### Public rules #####

all: build

build: clean
	docker build --pull \
		--tag $(REGISTRY)/container-toolkit:$(TAG) \
		--file $(DOCKERFILE) .

push:
	docker push $(REGISTRY)/container-toolkit:$(TAG)

clean:
	bash $(CURDIR)/test/main.sh clean $(CURDIR)/shared

test: build
	bash -x $(CURDIR)/test/main.sh run $(CURDIR)/shared $(REGISTRY)/container-toolkit:$(TAG) --no-cleanup-on-error
