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
	"fmt"
	"path/filepath"
)

const (
	nvidiaExperimentalContainerRuntimeSource = "nvidia-container-runtime.experimental"
)

// installContainerRuntimes sets up the NVIDIA container runtimes, copying the executables
// and implementing the required wrapper
func installContainerRuntimes(toolkitDir string) error {
	r := newNvidiaContainerRuntimeInstaller()

	_, err := r.install(toolkitDir)
	if err != nil {
		return fmt.Errorf("error installing NVIDIA container runtime: %v", err)
	}

	er := newNvidiaContainerRuntimeExperimentalInstaller()
	_, err = er.install(toolkitDir)
	if err != nil {
		return fmt.Errorf("error installing experimental NVIDIA Container Runtime: %v", err)
	}

	return nil
}

func newNvidiaContainerRuntimeInstaller() *executable {
	target := executableTarget{
		dotfileName: "nvidia-container-runtime.real",
		wrapperName: "nvidia-container-runtime",
	}
	return newRuntimeInstaller(nvidiaContainerRuntimeSource, target)
}

func newNvidiaContainerRuntimeExperimentalInstaller() *executable {
	target := executableTarget{
		dotfileName: "nvidia-container-runtime.experimental",
		wrapperName: "nvidia-container-runtime-experimental",
	}
	return newRuntimeInstaller(nvidiaExperimentalContainerRuntimeSource, target)
}

func newRuntimeInstaller(source string, target executableTarget) *executable {
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
		source:   source,
		target:   target,
		env:      env,
		preLines: preLines,
	}

	return &r
}
