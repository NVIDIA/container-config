# Copyright (c) 2020, NVIDIA CORPORATION.  All rights reserved.
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


.PHONY: all build builder test
.DEFAULT_GOAL := all

##### Global variables #####

DOCKER   ?= docker
ifeq ($(IMAGE),)
REGISTRY ?= nvidia
IMAGE := $(REGISTRY)/container-toolkit
endif

# Must be set externally before invoking
VERSION ?= 1.4.1

# Fix the versions for the toolkit components
LIBNVIDIA_CONTAINER_VERSION=1.3.1
NVIDIA_CONTAINER_TOOLKIT_VERSION=1.4.0
NVIDIA_CONTAINER_RUNTIME_VERSION=3.4.0

##### Public rules #####

all: ubuntu18.04 ubuntu16.04

push:
	$(DOCKER) push "$(IMAGE):$(VERSION)-ubuntu18.04"
	$(DOCKER) push "$(IMAGE):$(VERSION)-ubuntu16.04"

push-short:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-ubuntu18.04" "$(IMAGE):$(VERSION)"
	$(DOCKER) push "$(IMAGE):$(VERSION)"

push-latest:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-ubuntu18.04" "$(IMAGE):latest"
	$(DOCKER) push "$(IMAGE):latest"

ubuntu18.04:
	$(DOCKER) build --pull \
		--tag $(IMAGE):$(VERSION)-ubuntu18.04 \
		--build-arg VERSION="$(VERSION)" \
		--build-arg LIBNVIDIA_CONTAINER_VERSION="$(LIBNVIDIA_CONTAINER_VERSION)" \
		--build-arg NVIDIA_CONTAINER_TOOLKIT_VERSION="$(NVIDIA_CONTAINER_TOOLKIT_VERSION)" \
		--build-arg NVIDIA_CONTAINER_RUNTIME_VERSION="$(NVIDIA_CONTAINER_RUNTIME_VERSION)" \
		--file docker/Dockerfile.ubuntu18.04 .

ubuntu16.04:
	$(DOCKER) build --pull \
		--tag $(IMAGE):$(VERSION)-ubuntu16.04 \
		--build-arg VERSION="$(VERSION)" \
		--build-arg LIBNVIDIA_CONTAINER_VERSION="$(LIBNVIDIA_CONTAINER_VERSION)" \
		--build-arg NVIDIA_CONTAINER_TOOLKIT_VERSION="$(NVIDIA_CONTAINER_TOOLKIT_VERSION)" \
		--build-arg NVIDIA_CONTAINER_RUNTIME_VERSION="$(NVIDIA_CONTAINER_RUNTIME_VERSION)" \
		--file docker/Dockerfile.ubuntu16.04 .

ubi8:
	$(DOCKER) build --pull \
		--tag $(IMAGE):$(VERSION)-ubi8 \
		--build-arg VERSION="$(VERSION)" \
		--build-arg LIBNVIDIA_CONTAINER_VERSION="$(LIBNVIDIA_CONTAINER_VERSION)" \
		--build-arg NVIDIA_CONTAINER_TOOLKIT_VERSION="$(NVIDIA_CONTAINER_TOOLKIT_VERSION)" \
		--build-arg NVIDIA_CONTAINER_RUNTIME_VERSION="$(NVIDIA_CONTAINER_RUNTIME_VERSION)" \
		--file docker/Dockerfile.ubi8 .

clean:
	bash $(CURDIR)/test/main.sh clean $(CURDIR)/shared

test: build
	bash -x $(CURDIR)/test/main.sh run $(CURDIR)/shared $(IMAGE):$(VERSION) --no-cleanup-on-error
