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

crio::usage() {
	cat >&2 <<EOF
Usage: $0 COMMAND [ARG...]

Commands:
  setup DESTINATION [-d | --hooks-dir HOOKS_DIRECTORY] [-c | --no-check]
  cleanup [-d | --hooks-dir HOOKS_DIRECTORY]

Description:
  -d, --hooks-dir	The path to the hooks directory. By default it points to '${CRIO_HOOKS_DIR}'.
  -c, --no-check	Specify this option if you want to disable the different checks.
  DESTINATION		The path where the toolkit directory resides (e.g: /usr/local/nvidia/toolkit).
EOF
}


crio::setup() {
	if [ $# -eq 0 ]; then crio::usage; exit 1; fi

	local hooksd="${CRIO_HOOKS_DIR}"
	local ensure="TRUE"
	local -r destination="${1}"; shift

	options=$(getopt -l hooks-dir:,no-check -o d:c -- "$@")
	if [[ "$?" -ne 0 ]]; then crio::usage; exit 1; fi

	# set options to positional parameters
	eval set -- "${options}"
	for opt in ${options}; do
		case "${opt}" in
		-d | --hooks-dir) hooksd="$2"; shift 2;;
		-c | --no-check) ensure="FALSE"; shift;;
		--) shift; break;;
		esac
	done

	# Make some checks
	[[ "${ensure}" = "TRUE" ]] && ensure::mounted ${hooksd}

	if [[ "${destination}" == *\#* ]]; then
		log ERROR "DESTINATION '${destination}' contains forbidden character '#'"
		exit 1;
	fi

	mkdir -p ${hooksd}
	cp "${basedir}/${CRIO_HOOK_FILENAME}" "${hooksd}"
	sed -i "s#@DESTINATION@#${destination}#" "${hooksd}/${CRIO_HOOK_FILENAME}"
}

crio::cleanup() {
	local hooksd="${CRIO_HOOKS_DIR}"
	local ensure="TRUE"

	if [ $# -eq 0 ]; then crio::usage; exit 1; fi

	options=$(getopt -l hooks-dir:,no-check -o d:c -- "$@")
	if [[ "$?" -ne 0 ]]; then crio::usage; exit 1; fi

	# set options to positional parameters
	eval set -- "${options}"
	for opt in ${options}; do
		case "${opt}" in
		-d | --hooks-dir) hooksd="$2"; shift 2;;
		-c | --no-check) ensure="FALSE"; shift;;
		--) shift; break;;
		esac
	done

	# Make some checks
	[[ "${ensure}" = "TRUE" ]] && ensure::mounted ${hooksd}

	rm -f "${hooksd}/${CRIO_HOOK_FILENAME}"
}

if [ $# -eq 0 ]; then docker::usage; exit 1; fi

command=$1; shift
case "${command}" in
	setup)   crio::setup "$@";;
	cleanup) crio::cleanup "$@";;
	*)       crio::usage;;
esac
