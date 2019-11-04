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
set -euxo pipefail
shopt -s lastpipe

readonly basedir="$(dirname "$(realpath "$0")")"
source "${basedir}/common.sh"

readonly DOCKER_CONFIG="/etc/docker/daemon.json"

docker::info() {
	local -r docker_socket="${1:-/var/run/docker.sock}"

	if [[ ! -e ${docker_socket} ]]; then
		log ERR "Docker socket doesn't exist"
		exit 1
	fi

	curl --unix-socket "${docker_socket}" 'http://v1.40/info'
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

docker::config::is_configured() {
	local -r destination="${1}"
	local -r docker_socket="${2}"

	local -r config="$(with_retry 5 5s docker::info "${docker_socket}")"
	local -r nvidia_runtime="$(echo "${config}" | docker::config::get_nvidia_runtime)"
	local -r default_runtime="$(echo "${config}" | jq -r '.DefaultRuntime')"

	[[ "${nvidia_runtime}" = "${destination}/nvidia-container-runtime" ]] && \
		[[ "${default_runtime}" = "nvidia" ]];
}

toolkit::usage() {
	cat >&2 <<EOF
Usage: $0 COMMAND [ARG...]

Commands:
  setup DESTINATION [-s | --socket DOCKER_SOCKET_PATH]
  cleanup

Description
  -s, --socket	The path to the docker socket
EOF
}


docker::setup() {
	if [ $# -eq 0 ]; then docker::usage; exit 1; fi

	local -r destination="${1}/toolkit"; shift
	local docker_socket="/var/run/docker.sock"

	options=$(getopt -l socket: -o s: -- "$@")
	if [[ "$?" -ne 0 ]]; then toolkit::usage; exit 1; fi

	# set options to positional parameters
	eval set -- "${options}"
	for opt in ${options}; do
		case "${opt}" in
		-s | --socket) docker_socket="$2"; shift 2;;
		--) shift; break;;
		esac
	done

	# Make some checks
	ensure::mounted /etc/docker

	# This is a no-op
	if docker::config::is_configured "${docker_socket}" "${docker_socket}"; then
		log INFO "Noop, docker is arlready setup with the runtime container"
		return
	fi

	# First try to update the existing config file
	local updated_config
	local -r config_file=$(docker::config)
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

docker::cleanup() {
	docker::config::restore
}

if [ $# -eq 0 ]; then docker::usage; exit 1; fi

command=$1; shift
case "${command}" in
	setup)   docker::setup "$@";;
	cleanup) docker::cleanup "$@";;
	*)       docker::usage;;
esac
