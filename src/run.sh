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

DAEMON=0

_shutdown() {
	log INFO "_shutdown"

	rm -f "${PID_FILE}"
}

_init() {
	log INFO "_init"

	# Todo this probably just needs us to wait
	exec 3> "${PID_FILE}"
	if ! flock -n 3; then
		log ERR "An instance of the NVIDIA toolkit Container is already running, aborting"
		exit 1
	fi

	echo $$ >&3
}

usage() {
	cat >&2 <<EOF
Usage: $0 DESTINATION [-n | --no-daemon] [-t | --toolkit-args TOOLKIT_ARGS] [-r | --runtime RUNTIME] [-u | --runtime-args RUNTIME_ARGS]

Environment Variables:
  TOOLKIT_ARGS	Arguments to pass to the 'toolkit' command.
  RUNTIME		The runtime to setup on this node. One of {'docker', 'crio', 'containerd'}, defaults to 'docker'.
  RUNTIME_ARGS	Arguments to pass to 'docker', 'crio', or 'containerd' setup command.

Description
  -n, --no-daemon	Set this flag if the run file should terminate immediatly after setting up the runtime. Note that no cleanup will be performed.
  -t, --toolkit-args	Arguments to pass to the 'toolkit' command.
  -r, --runtime-args	Arguments to pass to the 'docker', 'crio', or 'containerd'.
EOF
}


main() {
	local -r destination="${1}"
	shift

	RUNTIME=${RUNTIME:-"docker"}
	TOOLKIT_ARGS=${TOOLKIT_ARGS:-""}
	RUNTIME_ARGS=${RUNTIME_ARGS:-""}


	options=$(getopt -l no-daemon,toolkit-args:,runtime:,runtime-args: -o nt:r:u: -- "$@")
	if [[ "$?" -ne 0 ]]; then usage; exit 1; fi

	# set options to positional parameters
	eval set -- "${options}"
	for opt in ${options}; do
		case "${opt}" in
		-n | --no-daemon)    DAEMON=1;          shift;;
		-t | --toolkit-args) TOOLKIT_ARGS="$2"; shift 2;;
		-r | --runtime)      RUNTIME="$2";      shift 2;;
		-u | --runtime-args) RUNTIME_ARGS="$2"; shift 2;;
		--) shift; break;;
		*) echo "Unknown argument ${opt}" && exit 1;;
		esac
	done

	# Validate arguments
	echo "${RUNTIME}" | ensure::oneof "docker" "crio" "containerd"

	_init
	trap "_shutdown" EXIT

	log INFO "=================Starting the NVIDIA Container Toolkit================="

	toolkit "${destination}/toolkit" ${TOOLKIT_ARGS}
	if [ "${RUNTIME}" = "containerd" ]; then
		${RUNTIME} setup ${RUNTIME_ARGS} "${destination}/toolkit"
	else
		${RUNTIME} setup "${destination}/toolkit" ${RUNTIME_ARGS}
	fi

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
		if [ \"${RUNTIME}\" = \"containerd\" ]; then \
			${RUNTIME} cleanup ${RUNTIME_ARGS} \"${destination}/toolkit\"; \
		else \
			${RUNTIME} cleanup \"${destination}/toolkit\"; \
		fi; \
		{ kill $!; exit 0; }" HUP INT QUIT PIPE TERM
	trap - EXIT

	while true; do wait $! || continue; done
	exit 0
}

if [ $# -eq 0 ]; then usage; exit 1; fi
main "$@"
