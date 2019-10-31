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
Usage: $0 DESTINATION [-n | --no-daemon]

Environment Variables:
  TOOLKIT_ARGS	Arguments to pass to the 'toolkit' command
  RUNTIME_ARGS	Arguments to pass to the 'docker', 'crio' or 'containerd' command

Description
  -n, --no-daemon	Set this flag if the run file should terminate immediatly after setting up the runtime. Note that no cleanup will be performed.
EOF
}


main() {
	local -r destination="${1}"
	shift

	options=$(getopt -l no-daemon -o n -- "$@")
	if [[ "$?" -ne 0 ]]; then usage; exit 1; fi

	# set options to positional parameters
	eval set -- "${options}"
	for opt in ${options}; do
		case "${opt}" in
		n | --no-daemon) DAEMON=1; shift;;
		--) shift; break;;
		esac
	done

	_init
	trap "_shutdown" EXIT

	log INFO "=================Starting the NVIDIA Container Toolkit================="

	toolkit "${destination}" ${TOOLKIT_ARGS}
	docker setup "${destination}" ${RUNTIME_ARGS}

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
		docker cleanup ${destination}; \
		{ kill $!; exit 0; }" HUP INT QUIT PIPE TERM
	trap - EXIT

	while true; do wait $! || continue; done
	exit 0
}

if [ $# -eq 0 ]; then usage; exit 1; fi
main "$@"
