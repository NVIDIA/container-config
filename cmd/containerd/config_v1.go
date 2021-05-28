/**
# Copyright (c) 2020-2021, NVIDIA CORPORATION.  All rights reserved.
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
	"path"
	"path/filepath"

	"github.com/pelletier/go-toml"
	log "github.com/sirupsen/logrus"
)

// configV1 represents a V1 containerd config
type configV1 struct {
	config
	containerdVersion containerdVersion
}

func newConfigV1(cfg *toml.Tree, containerdVersion containerdVersion) UpdateReverter {
	c := configV1{
		config: config{
			Tree:      cfg,
			version:   1,
			cri:       "cri",
			binaryKey: "Runtime",
		},
		containerdVersion: containerdVersion,
	}

	return &c
}

// Update performs an update specific to v1 of the containerd config
func (config *configV1) Update(o *options) error {
	// For v1 config, the `default_runtime_name` setting is only supported
	// for containerd version at least v1.3
	setAsDefault := o.setAsDefault && config.containerdVersion.atLeast(containerdVersion1dot3)

	runtimePath := filepath.Join(o.runtimeDir, runtimeBinary)
	config.update(o.runtimeClass, o.runtimeType, runtimePath, setAsDefault)

	if !o.setAsDefault {
		return nil
	}

	if config.containerdVersion.atLeast(containerdVersion1dot3) {
		defaultRuntimePath := append(config.containerdPath(), "default_runtime")
		if config.GetPath(defaultRuntimePath) != nil {
			log.Warnf("The setting of default_runtime (%v) in containerd is deprecated", defaultRuntimePath)
		}
		return nil
	}

	log.Warnf("Support for containerd version %v is deprecated", containerdVersion1dot3)
	defaultRuntimePath := append(config.containerdPath(), "default_runtime")
	config.initRuntime(defaultRuntimePath, o.runtimeType, runtimePath)

	return nil
}

// Revert performs a revert specific to v1 of the containerd config
func (config *configV1) Revert(o *options) error {
	runtimeClassPath := []string{
		"plugins",
		"cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
	}
	defaultRuntimePath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime",
	}
	defaultRuntimeOptionsPath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime",
		"options",
	}
	defaultRuntimeNamePath := []string{
		"plugins",
		"cri",
		"containerd",
		"default_runtime_name",
	}

	config.DeletePath(runtimeClassPath)
	if runtime, ok := config.GetPath(append(defaultRuntimeOptionsPath, "Runtime")).(string); ok {
		if runtimeBinary == path.Base(runtime) {
			config.DeletePath(append(defaultRuntimeOptionsPath, "Runtime"))
		}
	}

	if defaultRuntimeName, ok := config.GetPath(defaultRuntimeNamePath).(string); ok {
		if o.runtimeClass == defaultRuntimeName {
			config.DeletePath(defaultRuntimeNamePath)
		}
	}

	for i := 0; i < len(runtimeClassPath); i++ {
		if runtimes, ok := config.GetPath(runtimeClassPath[:len(runtimeClassPath)-i]).(*toml.Tree); ok {
			if len(runtimes.Keys()) == 0 {
				config.DeletePath(runtimeClassPath[:len(runtimeClassPath)-i])
			}
		}
	}

	if options, ok := config.GetPath(defaultRuntimeOptionsPath).(*toml.Tree); ok {
		if len(options.Keys()) == 0 {
			config.DeletePath(defaultRuntimeOptionsPath)
		}
	}

	if runtime, ok := config.GetPath(defaultRuntimePath).(*toml.Tree); ok {
		fields := []string{"runtime_type", "runtime_root", "runtime_engine", "privileged_without_host_devices"}
		if len(runtime.Keys()) <= len(fields) {
			matches := []string{}
			for _, f := range fields {
				e := runtime.Get(f)
				if e != nil {
					matches = append(matches, f)
				}
			}
			if len(matches) == len(runtime.Keys()) {
				for _, m := range matches {
					runtime.Delete(m)
				}
			}
		}
	}

	for i := 0; i < len(defaultRuntimePath); i++ {
		if runtimes, ok := config.GetPath(defaultRuntimePath[:len(defaultRuntimePath)-i]).(*toml.Tree); ok {
			if len(runtimes.Keys()) == 0 {
				config.DeletePath(defaultRuntimePath[:len(defaultRuntimePath)-i])
			}
		}
	}

	if len(config.Keys()) == 1 && config.Keys()[0] == "version" {
		config.Delete("version")
	}

	return nil
}
