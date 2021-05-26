/**
# Copyright (c) 2021, NVIDIA CORPORATION.  All rights reserved.
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
*/

package main

import (
	"path/filepath"
)

func newNvidiaContainerRuntimeInstaller() *executable {
	preLines := []string{
		"",
		"cat /proc/modules | grep -e \"^nvidia \" >/dev/null 2>&1",
		"if [ \"${?}\" != \"0\" ]; then",
		"	echo \"nvidia driver modules are not yet loaded, invoking runc directly\"",
		"	exec runc \"$@\"",
		"fi",
		"",
	}
	env := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(destDirPattern, ".config"),
	}

	r := executable{
		source: nvidiaContainerRuntimeSource,
		target: executableTarget{
			dotfileName: "nvidia-container-runtime.real",
			wrapperName: "nvidia-container-runtime",
		},
		env:      env,
		preLines: preLines,
	}

	return &r
}
