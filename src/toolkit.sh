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

packages=("/usr/bin/nvidia-container-runtime" \
	"/usr/bin/nvidia-container-toolkit" \
	"/usr/bin/nvidia-container-cli" \
	"/etc/nvidia-container-runtime/config.toml")

toolkit::remove() {
	local -r destination="${1:-"${SOURCE_DIR}"}"
	log INFO "${FUNCNAME[0]} $*"

	rm -rf "${destination}"
}

toolkit::install::packages() {
	local -r destination="${1:-"${SOURCE_DIR}"}"

	mkdir -p "${destination}"
	mkdir -p "${destination}/.config/nvidia-container-runtime"

	# Note: Bash arrays start at 0 (zsh arrays start at 1)
	for ((i=0; i < ${#packages[@]}; i++)); do
		packages[$i]=$(readlink -f "${packages[$i]}")
	done

	cp "${packages[@]}" "${destination}"
	mv "${destination}/config.toml" "${destination}/.config/nvidia-container-runtime/"
}

toolkit::setup::config() {
	local -r destination="${1:-"${SOURCE_DIR}"}"
	local -r config_path="${destination}/.config/nvidia-container-runtime/config.toml"
	log INFO "${FUNCNAME[0]} $*"

	sed -i 's/^#root/root/;' "${config_path}"
	sed -i "s@/run/nvidia/driver@${RUN_DIR}/driver@;" "${config_path}"
	sed -i "s;@/sbin/ldconfig.real;@${RUN_DIR}/driver/sbin/ldconfig.real;" "${config_path}"
}

toolkit::setup::cli_binary() {
	local -r destination="${1:-"${SOURCE_DIR}"}"
	log INFO "${FUNCNAME[0]} $*"

	# setup links to the real binaries to ensure that variables and configs
	# are pointing to the right path
	mv "${destination}/nvidia-container-cli" \
		"${destination}/nvidia-container-cli.real"

	# setup aliases so as to ensure that the path is correctly set
	cat <<- EOF | tr -s ' \t' > ${destination}/nvidia-container-cli
		#! /bin/sh
		LD_LIBRARY_PATH="${destination}" \
		PATH="${destination}:\$PATH" \
		${destination}/nvidia-container-cli.real \
			"\$@"
	EOF

	# Make sure that the alias files are executable
	chmod +x "${destination}/nvidia-container-cli"
}

toolkit::setup::toolkit_binary() {
	local -r destination="${1:-"${SOURCE_DIR}"}"
	log INFO "${FUNCNAME[0]} $*"

	mv "${destination}/nvidia-container-toolkit" \
		"${destination}/nvidia-container-toolkit.real"

	cat <<- EOF | tr -s ' \t' > ${destination}/nvidia-container-toolkit
		#! /bin/sh
		PATH="${destination}:\$PATH" \
		${destination}/nvidia-container-toolkit.real \
			-config "${destination}/.config/nvidia-container-runtime/config.toml" \
			"\$@"
	EOF

	chmod +x "${destination}/nvidia-container-toolkit"
}

toolkit::setup::runtime_binary() {
	local -r destination="${1:-"${SOURCE_DIR}"}"
	log INFO "${FUNCNAME[0]} $*"

	mv "${destination}/nvidia-container-runtime" \
		"${destination}/nvidia-container-runtime.real"

	cat <<- EOF | tr -s ' \t' > ${destination}/nvidia-container-runtime
		#! /bin/sh
		PATH="${destination}:\$PATH" \
		XDG_CONFIG_HOME="${destination}/.config" \
		${destination}/nvidia-container-runtime.real \
			"\$@"
	EOF

	chmod +x "${destination}/nvidia-container-runtime"
}

toolkit::usage() {
	cat >&2 <<EOF
Usage: $0 COMMAND [ARG...]

Commands:
  install DESTINATION

Description:
  DESTINATION	The path where the toolkit directory resides (e.g: /usr/local/nvidia/toolkit).
EOF
}

toolkit::install() {
	local destination="$1"; shift

	if [[ "$#" -ne 0 ]]; then toolkit::usage; exit 1; fi

	# Uninstall previous installation of the toolkit
	toolkit::remove "${destination}" || exit 1

	log INFO "${FUNCNAME[0]} $*"

	# Add one more according to Debian/Ubuntu or RHEL style multilib path
	if [ -e /etc/debian_version ]; then
		packages+=("/usr/lib/x86_64-linux-gnu/libnvidia-container.so.1")
	else
		packages+=("/usr/lib64/libnvidia-container.so.1")
	fi

	toolkit::install::packages "${destination}"

	toolkit::setup::config "${destination}"
	toolkit::setup::cli_binary "${destination}"
	toolkit::setup::toolkit_binary "${destination}"
	toolkit::setup::runtime_binary "${destination}"

	# The runtime shim is still looking for the old binary
	# Move to ${destination} to get expanded
	# Make symlinks local so that they still refer to the
	# local target when mounted on the host
	cd "${destination}"
	ln -s "./nvidia-container-toolkit" "${destination}/nvidia-container-runtime-hook"
	ln -s "./libnvidia-container.so.1."* "${destination}/libnvidia-container.so.1"
	cd -
}

if [ $# -eq 0 ]; then toolkit::usage; exit 1; fi
toolkit::install "$@"
