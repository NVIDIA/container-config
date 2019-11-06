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

testing::dind() {
	# Docker creates /etc/docker when starting
	# by default there isn't any config in this directory (even after the daemon starts)
	docker run --privileged \
		-v "${shared_dir}/etc/docker:/etc/docker" \
		-v "${shared_dir}/run/nvidia:/run/nvidia:shared" \
		-v "${shared_dir}/usr/local/nvidia:/usr/local/nvidia:shared" \
		--name "${dind}" -d docker:stable-dind $*
}

testing::dind() {
	docker exec -it "${dind}" sh -c "$*"
}

testing::dind::toolkit() {
	# Share the volumes so that we can edit the config file and point to the new runtime
	# Share the pid so that we can ask docker to reload its config
	docker run -it --privileged \
		--volumes-from "${dind}" \
		--pid "container:${dind}" \
		-e 'TOOLKIT_ARGS=--symlink /usr/local/nvidia' \
		-e 'RUNTIME_ARGS=--socket /run/nvidia/docker.sock' \
		"${toolkit}" "/run/nvidia" "$*"
}

testing::docker::main() {
	testing::dind -H unix://run/nvidia/docker.sock
	testing::dind::toolkit --no-daemon

	# Ensure that we haven't broken non GPU containers
	with_retry 3 5s testing::exec::dind docker run -it alpine echo foo

	# Ensure toolkit dir is not empty
	test ! -z "$(ls -A "${shared_dir}"/run/nvidia/toolkit)"
}
