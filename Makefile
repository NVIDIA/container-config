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

CUDA_VERSION ?= 11.3.0
GOLANG_VERSION ?= 1.16.4
DOCKER   ?= docker
ifeq ($(IMAGE),)
REGISTRY ?= nvidia
IMAGE := $(REGISTRY)/container-toolkit
endif

# Must be set externally before invoking
VERSION ?= 1.5.0

# Fix the versions for the toolkit components
LIBNVIDIA_CONTAINER_VERSION=1.4.0
NVIDIA_CONTAINER_TOOLKIT_VERSION=1.5.0
NVIDIA_CONTAINER_RUNTIME_VERSION=3.5.0
NVIDIA_CONTAINER_RUNTIME_EXPERIMENTAL_VERSION=latest

##### Public rules #####
DEFAULT_PUSH_TARGET := ubuntu18.04
TARGETS := ubuntu20.04 ubuntu18.04 ubuntu16.04 ubi8 centos7 centos8

PUSH_TARGETS := $(patsubst %, push-%, $(TARGETS))
BUILD_TARGETS := $(patsubst %, build-%, $(TARGETS))
TEST_TARGETS := $(patsubst %, test-%, $(TARGETS))

.PHONY: $(TARGETS) $(PUSH_TARGETS) $(BUILD_TARGETS) $(TEST_TARGETS)

all: $(TARGETS)

push-all: $(PUSH_TARGETS)
build-all: $(BUILD_TARGETS)

$(PUSH_TARGETS): push-%:
	$(DOCKER) push "$(IMAGE):$(VERSION)-$(*)"

# For the default push target we also push a short tag equal to the version.
# We skip this for the development release
RELEASE_DEVEL_TAG ?= devel
ifneq ($(strip $(VERSION)),$(RELEASE_DEVEL_TAG))
push-$(DEFAULT_PUSH_TARGET): push-short
endif
push-short:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-$(DEFAULT_PUSH_TARGET)" "$(IMAGE):$(VERSION)"
	$(DOCKER) push "$(IMAGE):$(VERSION)"

build-ubuntu%: DOCKERFILE_SUFFIX := ubuntu
build-ubuntu16.04: BASE_DIST := ubuntu16.04
build-ubuntu18.04: BASE_DIST := ubuntu18.04
build-ubuntu20.04: BASE_DIST := ubuntu20.04

build-ubi8: DOCKERFILE_SUFFIX := ubi8
build-ubi8: BASE_DIST := ubi8

build-centos%: DOCKERFILE_SUFFIX := centos
build-centos7: BASE_DIST := centos7
build-centos8: BASE_DIST := centos8

# Both ubi8 and build-ubi8 trigger a build of the relevant image
$(TARGETS): %: build-%
$(BUILD_TARGETS): build-%:
	$(DOCKER) build --pull \
		--tag $(IMAGE):$(VERSION)-$(*) \
		--build-arg BASE_DIST="$(BASE_DIST)" \
		--build-arg CUDA_VERSION="$(CUDA_VERSION)" \
		--build-arg VERSION="$(VERSION)" \
		--build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
		--build-arg LIBNVIDIA_CONTAINER_VERSION="$(LIBNVIDIA_CONTAINER_VERSION)" \
		--build-arg NVIDIA_CONTAINER_RUNTIME_EXPERIMENTAL_VERSION="$(NVIDIA_CONTAINER_RUNTIME_EXPERIMENTAL_VERSION)" \
		--build-arg NVIDIA_CONTAINER_RUNTIME_VERSION="$(NVIDIA_CONTAINER_RUNTIME_VERSION)" \
		--build-arg NVIDIA_CONTAINER_TOOLKIT_VERSION="$(NVIDIA_CONTAINER_TOOLKIT_VERSION)" \
		--file docker/Dockerfile.$(DOCKERFILE_SUFFIX) .

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

# Targets for go development
IMAGE_TAG ?= $(GOLANG_VERSION)
BUILDIMAGE ?= $(IMAGE):$(IMAGE_TAG)-devel

MODULE := .

GO_TARGETS := binary build go-all check fmt assert-fmt lint lint-internal vet test examples coverage
DOCKER_TARGETS := $(patsubst %, docker-%, $(GO_TARGETS))
.PHONY: $(GO_TARGETS) $(DOCKER_TARGETS)

GOOS := linux

go-all: check test build binary
check: assert-fmt lint vet

binary:
	GOOS=$(GOOS) go build ./cmd/nvidia-container-runtime.experimental

build:
	GOOS=$(GOOS) go build ./...

examples:
	GOOS=$(GOOS) go build ./examples/...

.PHONY: fmt assert-fmt lint vet

# Apply go fmt to the codebase
fmt:
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs gofmt -s -l -w

assert-fmt:
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs gofmt -s -l > fmt.out
	@if [ -s fmt.out ]; then \
		echo "\nERROR: The following files are not formatted:\n"; \
		cat fmt.out; \
		rm fmt.out; \
		exit 1; \
	else \
		rm fmt.out; \
	fi

lint:
# We use `go list -f '{{.Dir}}' $(MODULE)/...` to skip the `vendor` folder.
	go list -f '{{.Dir}}' $(MODULE)/... | grep -v /internal/ | xargs golint -set_exit_status

vet:
	GOOS=$(GOOS) go vet $(MODULE)/...

COVERAGE_FILE := coverage.out
test: build
	GOOS=$(GOOS) go test -v -coverprofile=$(COVERAGE_FILE) $(MODULE)/...

coverage: test
	go tool cover -func=$(COVERAGE_FILE)

# Generate an image for containerized builds
# Note: This image is local only
.PHONY: .build-image .pull-build-image .push-build-image
.build-image: docker/Dockerfile.devel
	if [ x"$(SKIP_IMAGE_BUILD)" = x"" ]; then \
		$(DOCKER) build \
			--progress=plain \
			--build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
			--tag $(BUILDIMAGE) \
			-f $(^) \
			docker; \
	fi

.pull-build-image:
	$(DOCKER) pull $(BUILDIMAGE)

.push-build-image:
	$(DOCKER) push $(BUILDIMAGE)

$(DOCKER_TARGETS): docker-%: .build-image
	@echo "Running 'make $(*)' in docker container $(BUILDIMAGE)"
	$(DOCKER) run \
		--rm \
		-e GOCACHE=/tmp/.cache \
		-v $(PWD):$(PWD) \
		-w $(PWD) \
		--user $$(id -u):$$(id -g) \
		$(BUILDIMAGE) \
			make $(*)
