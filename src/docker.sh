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

readonly DOCKER_CONFIG="/etc/docker/daemon.json"

docker::info() {
	local -r docker_socket="${1:-/var/run/docker.sock}"

	if [[ ! -e ${docker_socket} ]]; then
		log ERR "Docker socket doesn't exist"
		exit 1
	fi

	curl --unix-socket "${docker_socket}" 'http://v1.40/info'
}

docker::ensure::mounted() {
	mount | grep /etc/docker
	if [[ ! $? ]]; then
		log ERROR "Docker directory isn't mounted in container"
		log ERROR "Ensure that you have correctly mounted the docker directoy"
		exit 1
	fi
}

docker::ensure::config_dir() {
	# Ensure that the docker config path exists
	if [[ ! -d "/etc/docker" ]]; then
		log ERROR "Docker directory doesn't exist in container"
		log ERROR "Ensure that you have correctly mounted the docker directoy"
		exit 1
	fi

}

docker::config::backup() {
	if [[ -f "${DOCKER_CONFIG}" ]]; then
		mv "${DOCKER_CONFIG}" "${DOCKER_CONFIG}.bak"
	fi
}

docker::config::restore() {
	if [[ -f "${DOCKER_CONFIG}.bak" ]]; then
		mv "${DOCKER_CONFIG}.bak" "${DOCKER_CONFIG}"
	else
		if [[ -f "${DOCKER_CONFIG}" ]]; then
			rm "${DOCKER_CONFIG}"
		fi
	fi
}

docker::config::add_runtime() {
	local -r destination="${1:-/run/nvidia}"
	local -r nvcr="${destination}/nvidia-container-runtime"

	cat - | \
		jq -r ".runtimes = {}" | \
		jq -r ".runtimes += {\"nvidia\": {\"path\": \"${nvcr}\"}}" | \
		jq -r '. += {"default-runtime": "nvidia"}'
}

docker::config() {
	([[ -f "${DOCKER_CONFIG}" ]] && cat "${DOCKER_CONFIG}") || echo {}
}

docker::config::refresh() {
	log INFO "Refreshing the docker daemon configuration"
	pkill -SIGHUP dockerd
}

docker::config::restart() {
	log INFO "restarting the docker daemon"
	pkill -SIGTERM dockerd
}

docker::config::get_nvidia_runtime() {
	cat - | jq -r '.runtimes.nvidia'
}

docker::setup() {
	docker::ensure::mounted
	docker::ensure::config_dir

	log INFO "Setting up the configuration for the docker daemon"

	local -r destination="${1:-/run/nvidia}"
	local -r docker_socket="${2:-"/var/run/docker.socket"}"
	local updated_config

	local -r config="$(with_retry 5 5s docker::info "${docker_socket}"))"
	local -r nvidia_runtime="$(echo "${config}" | docker::config::get_nvidia_runtime)"
	local -r default_runtime="$(echo "${config}" | jq -r '.DefaultRuntime')"

	# This is a no-op
	if [[ "${nvidia_runtime}" = "${destination}/nvidia-container-runtime" ]] && \
		[[ "${default_runtime}" = "nvidia" ]]; then
		log INFO "Noop, docker is arlready setup with the runtime container"
		return
	fi

	local -r config_file=$(docker::config)
	log INFO "content of docker's config file : ${config_file}"

	# First try to update the existing config file
	updated_config=$(echo "${config_file}" | docker::config::add_runtime "${destination}")

	# If there was an error while parsing the file catch it here
	local -r config_runtime=$(echo "${updated_config}" | docker::config::get_nvidia_runtime)
	if [[ "${config_runtime}" == "null" ]]; then
		updated_config=$(echo "{}" | docker::config::add_runtime "${destination}")
	fi

	docker::config::backup
	echo "${updated_config}" > /etc/docker/daemon.json
	docker::config::refresh
}

docker::uninstall() {
	local -r docker_socket="${1:-"/var/run/docker.socket"}"
	docker::config::restore
}
