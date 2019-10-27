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

readonly RUN_DIR="/run/nvidia"
readonly TOOLKIT_DIR="${RUN_DIR}/toolkit"
readonly PID_FILE="${RUN_DIR}/toolkit.pid"

readonly basedir="$(dirname "$(realpath "$0")")"

source "${basedir}/common.sh"
source "${basedir}/toolkit.sh"
source "${basedir}/docker.sh"

DAEMON=0

_shutdown() {
	log INFO "_shutdown"

	rm -f ${PID_FILE}
}

_init() {
	log INFO "_init"

	# Todo this probably just needs us to wait
	exec 3> ${PID_FILE}
	if ! flock -n 3; then
		log ERR "An instance of the NVIDIA toolkit Container is already running, aborting"
		exit 1
	fi

	echo $$ >&3
}

main() {
	local destination
	local docker_socket

	while [ $# -gt 0 ]; do
		case "$1" in
		--no-daemon)
			DAEMON=1
			shift
			;;
		--docker-socket|-d)
			docker_socket="$2"
			shift 2
			;;
		--destination|-o)
			destination="$2"/toolkit
			shift 2
			;;
		*)
			echo "Unknown argument $1"
			exit 1
		esac
	done

	log INFO "=================Starting the NVIDIA Container Toolkit================="


	_init
	trap "_shutdown" EXIT

	# Uninstall previous installation of the toolkit
	toolkit::remove "${destination}" || exit 1

	toolkit::install "${destination}"
	docker::setup "${destination}" "${docker_socket}"

	if [[ "$DAEMON" -ne 0 ]]; then
		exit 0
	fi

	log INFO "=================Done, Now Waiting for signal================="
	sleep infinity &

	# shellcheck disable=SC2064
	# We want the expand to happen now rather than at trap time
	# Setup a new signal handler and reset the EXIT signal handler
	trap "echo 'Caught signal'; \
		_shutdown; \
		{ kill $!; exit 0; }" HUP INT QUIT PIPE TERM
	trap - EXIT

	while true; do wait $! || continue; done
	exit 0
}

uninstall() {
	local destination="${1:-"${RUN_DIR}"}/toolkit"

	while [ $# -gt 0 ]; do
		case "$1" in
		--destination|-o)
			destination="$2"/toolkit
			shift 2
			;;
		*)
			echo "Unknown argument $1"
			exit 1
		esac
	done

	# Don't uninstall if another instance is already running
	_init
	trap "_shutdown" EXIT

	# Uninstall previous installation of the toolkit
	toolkit::remove "${destination}" || exit 1
	rm -rf "${destination}"
}

case "$1" in
run)
	shift
	main "$@"
	;;
uninstall)
	shift
	uninstall "$@"
	;;
*)
	echo "Unknown argument $1"
	exit 1
esac
