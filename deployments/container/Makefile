# Copyright (c) 2021, NVIDIA CORPORATION.  All rights reserved.
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

BUILD_MULTI_ARCH_IMAGES ?= false
DOCKER ?= docker
REGCTL ?= regctl

BUILDX =
ifeq ($(BUILD_MULTI_ARCH_IMAGES),true)
BUILDX = buildx
endif

MKDIR    ?= mkdir
DIST_DIR ?= $(CURDIR)/dist

##### Global variables #####
include $(CURDIR)/versions.mk

GOPROXY ?= https://proxy.golang.org,direct

IMAGE_VERSION := $(VERSION)

IMAGE_TAG ?= $(VERSION)
IMAGE = $(IMAGE_NAME):$(IMAGE_TAG)

OUT_IMAGE_NAME ?= $(IMAGE_NAME)
OUT_IMAGE_VERSION ?= $(IMAGE_VERSION)
OUT_IMAGE_TAG = $(OUT_IMAGE_VERSION)
OUT_IMAGE = $(OUT_IMAGE_NAME):$(OUT_IMAGE_TAG)

##### Public rules #####
.PHONY: image build bush

ifneq ($(BUILD_MULTI_ARCH_IMAGES),true)
include $(CURDIR)/deployments/container/native-only.mk
else
include $(CURDIR)/deployments/container/multi-arch.mk
endif

# Define the push targets
push:
	$(CURDIR)/scripts/publish-image.sh $(IMAGE) $(OUT_IMAGE)

DOCKERFILE = $(CURDIR)/deployments/container/Dockerfile

# Build the container-toolkit image
image: $(ARTIFACTS_ROOT)
	DOCKER_BUILDKIT=1 \
		$(DOCKER) $(BUILDX) build --pull \
		--provenance=false --sbom=false \
		$(DOCKER_BUILD_OPTIONS) \
		$(DOCKER_BUILD_PLATFORM_OPTIONS) \
		$(INTERMEDIATE_TARGET) \
		--tag $(IMAGE) \
		--build-arg PACKAGING_IMAGE="$(PACKAGING_IMAGE)" \
		--build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
		--build-arg PACKAGE_DIST_DEB="$(PACKAGE_DIST_DEB)" \
		--build-arg PACKAGE_DIST_RPM="$(PACKAGE_DIST_RPM)" \
		--build-arg PACKAGE_VERSION="$(PACKAGE_VERSION)" \
		--build-arg VERSION="$(VERSION)" \
		--build-arg GIT_COMMIT="$(GIT_COMMIT)" \
		--build-arg GIT_COMMIT_SHORT="$(GIT_COMMIT_SHORT)" \
		--build-arg GIT_BRANCH="$(GIT_BRANCH)" \
		--build-arg SOURCE_DATE_EPOCH="$(SOURCE_DATE_EPOCH)" \
		--build-arg GOPROXY="$(GOPROXY)" \
		-f $(DOCKERFILE) \
		$(CURDIR)


PACKAGE_DIST_DEB = ubuntu18.04
PACKAGE_DIST_RPM = centos7
