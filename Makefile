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
VERSION ?= 1.4.7

# Fix the versions for the toolkit components
LIBNVIDIA_CONTAINER_VERSION=1.4.0
NVIDIA_CONTAINER_TOOLKIT_VERSION=1.5.0
NVIDIA_CONTAINER_RUNTIME_VERSION=3.5.0

##### Public rules #####
DEFAULT_PUSH_TARGET := ubuntu18.04
TARGETS := ubuntu18.04 ubuntu16.04 ubi8

PUSH_TARGETS := $(patsubst %, push-%, $(TARGETS))
BUILD_TARGETS := $(patsubst %, build-%, $(TARGETS))
TEST_TARGETS := $(patsubst %, test-%, $(TARGETS))

.PHONY: $(TARGETS) $(PUSH_TARGETS) $(BUILD_TARGETS) $(TEST_TARGETS)

all: $(TARGETS)

push-all: $(PUSH_TARGETS)
build-all: $(BUILD_TARGETS)

$(PUSH_TARGETS): push-%:
	$(DOCKER) push "$(IMAGE):$(VERSION)-$(*)"

# For the default push target we also push the short and latest tags
push-$(DEFAULT_PUSH_TARGET): push-short push-latest
push-short:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-$(DEFAULT_PUSH_TARGET)" "$(IMAGE):$(VERSION)"
	$(DOCKER) push "$(IMAGE):$(VERSION)"

push-latest:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-$(DEFAULT_PUSH_TARGET)" "$(IMAGE):latest"
	$(DOCKER) push "$(IMAGE):latest"

# Both ubi8 and build-ubi8 trigger a build of the relevant image
$(TARGETS): %: build-%
$(BUILD_TARGETS): build-%:
	$(DOCKER) build --pull \
		--tag $(IMAGE):$(VERSION)-$(*) \
		--build-arg VERSION="$(VERSION)" \
		--build-arg LIBNVIDIA_CONTAINER_VERSION="$(LIBNVIDIA_CONTAINER_VERSION)" \
		--build-arg NVIDIA_CONTAINER_TOOLKIT_VERSION="$(NVIDIA_CONTAINER_TOOLKIT_VERSION)" \
		--build-arg NVIDIA_CONTAINER_RUNTIME_VERSION="$(NVIDIA_CONTAINER_RUNTIME_VERSION)" \
		--file docker/Dockerfile.$(*) .

clean-%:
	bash $(CURDIR)/test/main.sh clean $(CURDIR)/shared-$(*)

TEST_CASES ?= toolkit docker crio containerd
$(TEST_TARGETS): test-%:
	TEST_CASES="$(TEST_CASES)" bash -x $(CURDIR)/test/main.sh run $(CURDIR)/shared-$(*) $(IMAGE):$(VERSION)-$(*) --no-cleanup-on-error

.PHONY: bump-commit
BUMP_COMMIT := Bump to version v$(VERSION)
bump-commit:
	@git log | if [ ! -z "$$(grep -o '$(BUMP_COMMIT)' | sort -u)" ]; then \
		echo "\nERROR: '$(BUMP_COMMIT)' already committed\n"; \
		exit 1; \
	fi
	@git add Makefile
	@git commit -m "$(BUMP_COMMIT)"
	@echo "Applied the diff:"
	@git --no-pager diff HEAD~1
