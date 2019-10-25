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

set -euo pipefail
shopt -s lastpipe

readonly basedir="$(dirname "$(realpath "$0")")"
readonly dind="nvidia-container-runtime-dind"

source "${basedir}/../src/common.sh"

testing::cleanup() {
	docker kill "${dind}" || true &> /dev/null
	docker rm "${dind}" || true &> /dev/null

	mkdir -p "${shared_dir}/run"
	docker run -it --privileged \
		-v "${shared_dir}/run/nvidia:/run/nvidia:shared" \
		"${toolkit}" \
		bash -x -c "/work/run.sh uninstall /run/nvidia"
	rm -rf shared

	return
}

testing::setup() {
	mkdir -p "${shared_dir}"
	mkdir -p "${shared_dir}"/etc/docker
	mkdir -p "${shared_dir}"/run/nvidia
	mkdir -p "${shared_dir}"/etc/nvidia-container-runtime
}

testing::run::dind() {
	# Docker creates /etc/docker when starting
	# by default there isn't any config in this directory (even after the daemon starts)
	docker run --privileged \
		-v "${shared_dir}/etc/docker:/etc/docker" \
		-v "${shared_dir}/run/nvidia:/run/nvidia:shared" \
		--name "${dind}" -d docker:stable-dind $*
}

testing::exec::dind() {
	docker exec -it "${dind}" sh -c "$*"
}

testing::run::toolkit() {
	# Share the volumes so that we can edit the config file and point to the new runtime
	# Share the pid so that we can ask docker to reload its config
	docker run -it --privileged \
		--volumes-from "${dind}" \
		--pid "container:${dind}" \
		"${toolkit}" \
		bash -x -c "/work/run.sh run /run/nvidia /run/nvidia/docker.sock $*"
}

testing::uninstall::toolkit() {
	# Share the volumes so that we can edit the config file and point to the new runtime
	# Share the pid so that we can ask docker to reload its config
	docker run -it --privileged \
		--volumes-from "${dind}" \
		--pid "container:${dind}" \
		"${toolkit}" \
		bash -x -c "/work/run.sh uninstall /run/nvidia $*"
}

testing::main() {
	testing::setup

	testing::run::dind -H unix://run/nvidia/docker.sock
	testing::run::toolkit --no-daemon

	# Ensure that we haven't broken non GPU containers
	with_retry 3 5s testing::exec::dind docker run -it alpine echo foo

	# Ensure toolkit dir is not empty
	test ! -z "$(ls -A "${shared_dir}"/run/nvidia/toolkit)"

	testing::cleanup
}

readonly shared_dir="${1:-"./shared"}"
readonly toolkit="${2:-"UNKNOWN"}"

trap testing::cleanup ERR

testing::cleanup
testing::main "$@"
