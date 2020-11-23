#! /bin/bash
# Copyright (c) 2019, NVIDIA CORPORATION.  All rights reserved.
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

readonly containerd_dind_ctr="container-config-containerd-dind-ctr-name"
readonly containerd_test_ctr="container-config-containerd-test-ctr-name"
readonly containerd_dind_socket="/run/nvidia/docker.sock"
readonly containerd_dind_containerd_dir="/run/docker/containerd"

testing::containerd::dind::setup() {
	# Docker creates /etc/docker when starting
	# by default there isn't any config in this directory (even after the daemon starts)
	docker run -d --rm --privileged \
		-v "${shared_dir}/etc/docker:/etc/docker" \
		-v "${shared_dir}/run/nvidia:/run/nvidia" \
		-v "${shared_dir}/usr/local/nvidia:/usr/local/nvidia" \
		-v "${shared_dir}/run/docker/containerd:/run/docker/containerd" \
		--name "${containerd_dind_ctr}" \
		docker:stable-dind -H unix://${containerd_dind_socket}
}

testing::containerd::dind::exec() {
	docker exec "${containerd_dind_ctr}" sh -c "$*"
}

testing::containerd::toolkit::run() {
	local version=${1}

	# Ensure that we can run some non GPU containers from within dind
	with_retry 3 5s testing::containerd::dind::exec " \
		ctr --address=${containerd_dind_containerd_dir}/containerd.sock image pull nvcr.io/nvidia/cuda:11.1-base; \
		ctr --address=${containerd_dind_containerd_dir}/containerd.sock run --rm --runtime=io.containerd.runtime.v1.linux nvcr.io/nvidia/cuda:11.1-base cuda echo foo"

	# Share the volumes so that we can edit the config file and point to the new runtime
	# Share the pid so that we can ask docker to reload its config
	docker run --rm --privileged \
		--volumes-from "${containerd_dind_ctr}" \
		-v "${shared_dir}/etc/containerd/config_${version}.toml:${containerd_dind_containerd_dir}/containerd.toml" \
		--pid "container:${containerd_dind_ctr}" \
		-e "RUNTIME=containerd" \
		-e "RUNTIME_ARGS=--config=/${containerd_dind_containerd_dir}/containerd.toml --socket=${containerd_dind_containerd_dir}/containerd.sock" \
		--name "${containerd_test_ctr}" \
		"${toolkit_container_image}" "/usr/local/nvidia" "--no-daemon"

	# Ensure that we haven't broken non GPU containers
	with_retry 3 5s testing::containerd::dind::exec " \
		ctr --address=${containerd_dind_containerd_dir}/containerd.sock image pull nvcr.io/nvidia/cuda:11.1-base; \
		ctr --address=${containerd_dind_containerd_dir}/containerd.sock run --rm --runtime=io.containerd.runtime.v1.linux nvcr.io/nvidia/cuda:11.1-base cuda echo foo"
}

testing::containerd::main() {
	testing::containerd::dind::setup
	testing::containerd::toolkit::run v1
	testing::containerd::cleanup

	testing::containerd::dind::setup
	testing::containerd::toolkit::run v2
	testing::containerd::cleanup
}

testing::containerd::cleanup() {
	docker kill "${containerd_dind_ctr}" &> /dev/null || true
	docker kill "${containerd_test_ctr}" &> /dev/null || true
}
