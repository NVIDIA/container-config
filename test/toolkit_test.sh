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

testing::toolkit::main() {
	local -r uid=$(id -u)
	local -r gid=$(id -g)

	testing::docker_run::toolkit::shell 'toolkit /usr/local/nvidia/toolkit'
	docker run -v "${shared_dir}:/work" alpine sh -c "chown -R ${uid}:${gid} /work/"

	# Ensure toolkit dir is correctly setup
	test ! -z "$(ls -A "${shared_dir}/usr/local/nvidia/toolkit")"

	test -L "${shared_dir}/usr/local/nvidia/toolkit/libnvidia-container.so.1"
	test -e "$(readlink -f "${shared_dir}/usr/local/nvidia/toolkit/libnvidia-container.so.1")"

	test -e "${shared_dir}/usr/local/nvidia/toolkit/nvidia-container-cli"
	test -e "${shared_dir}/usr/local/nvidia/toolkit/nvidia-container-toolkit"
	test -e "${shared_dir}/usr/local/nvidia/toolkit/nvidia-container-runtime"

	test -e "${shared_dir}/usr/local/nvidia/toolkit/nvidia-container-cli.real"
	test -e "${shared_dir}/usr/local/nvidia/toolkit/nvidia-container-toolkit.real"
	test -e "${shared_dir}/usr/local/nvidia/toolkit/nvidia-container-runtime.real"

	test -e "${shared_dir}/usr/local/nvidia/toolkit/.config/nvidia-container-runtime/config.toml"
}

testing::toolkit::cleanup() {
	:
}
