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
REGISTRY ?= nvidia
VERSION  ?= 1.0.0-beta.1

##### Public rules #####

all: ubuntu18.04 ubuntu16.04 ubi8

push:
	$(DOCKER) push "$(REGISTRY)/container-toolkit:$(VERSION)-ubuntu18.04"
	$(DOCKER) push "$(REGISTRY)/container-toolkit:$(VERSION)-ubuntu16.04"
	$(DOCKER) push "$(REGISTRY)/container-toolkit:$(VERSION)-ubi8"

push-short:
	$(DOCKER) tag "$(REGISTRY)/container-toolkit:$(VERSION)-ubuntu18.04" "$(REGISTRY)/container-toolkit:$(VERSION)"
	$(DOCKER) push "$(REGISTRY)/container-toolkit:$(VERSION)"

push-latest:
	$(DOCKER) tag "$(REGISTRY)/container-toolkit:$(VERSION)-ubuntu18.04" "$(REGISTRY)/container-toolkit:latest"
	$(DOCKER) push "$(REGISTRY)/container-toolkit:latest"

ubuntu18.04:
	$(DOCKER) build --pull \
		--tag $(REGISTRY)/container-toolkit:$(VERSION)-ubuntu18.04 \
		--file docker/Dockerfile.ubuntu18.04 .

ubuntu16.04:
	$(DOCKER) build --pull \
		--tag $(REGISTRY)/container-toolkit:$(VERSION)-ubuntu16.04 \
		--file docker/Dockerfile.ubuntu16.04 .

ubi8:
	$(DOCKER) build --pull \
		--tag $(REGISTRY)/container-toolkit:$(VERSION)-ubi8 \
		--build-arg VERSION="$(VERSION)" \
		--file docker/Dockerfile.ubi8 .

clean:
	bash $(CURDIR)/test/main.sh clean $(CURDIR)/shared

test: build
	bash -x $(CURDIR)/test/main.sh run $(CURDIR)/shared $(REGISTRY)/container-toolkit:$(VERSION) --no-cleanup-on-error
