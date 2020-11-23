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

testing::setup() {
	cp -Rp ${basedir}/shared ${shared_dir}
	mkdir -p "${shared_dir}/etc/containerd"
	mkdir -p "${shared_dir}/etc/docker"
	mkdir -p "${shared_dir}/run/docker/containerd"
	mkdir -p "${shared_dir}/run/nvidia"
	mkdir -p "${shared_dir}/usr/local/nvidia"
	mkdir -p "${shared_dir}/${CRIO_HOOKS_DIR}"
}

testing::cleanup() {
	if [[ -e "${shared_dir}" ]]; then
		docker run --rm \
			-v "${shared_dir}:/work" \
			alpine sh -c 'rm -rf /work/*'
		rmdir "${shared_dir}"
	fi

	for tc in ${test_cases}; do
		testing::${tc}::cleanup
	done
}

testing::docker_run::toolkit::shell() {
	docker run --rm --privileged \
		--entrypoint sh \
		-v "${shared_dir}/etc/containerd:/etc/containerd" \
		-v "${shared_dir}/etc/docker:/etc/docker" \
		-v "${shared_dir}/run/docker/containerd:/run/docker/containerd" \
		-v "${shared_dir}/run/nvidia:/run/nvidia" \
		-v "${shared_dir}/usr/local/nvidia:/usr/local/nvidia" \
		-v "${shared_dir}/${CRIO_HOOKS_DIR}:${CRIO_HOOKS_DIR}" \
		"${toolkit_container_image}" "-c" "$*"
}
