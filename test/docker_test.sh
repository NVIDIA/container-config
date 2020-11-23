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
	docker run -d --rm --privileged \
		-v "${shared_dir}/etc/docker:/etc/docker" \
		-v "${shared_dir}/run/nvidia:/run/nvidia" \
		-v "${shared_dir}/usr/local/nvidia:/usr/local/nvidia" \
		--name "${docker_dind_ctr}" \
		docker:stable-dind $*
}

testing::docker::main() {
	testing::dind -H unix://run/nvidia/docker.sock
	testing::dind::toolkit --no-daemon

	# Ensure that we haven't broken non GPU containers
	with_retry 3 5s testing::exec::dind docker run -t alpine echo foo
}
