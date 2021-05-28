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
	"path/filepath"

	"github.com/pelletier/go-toml"
)

// configV2 represents a V2 containerd config
type configV2 struct {
	config
	containerdVersion string
}

func newConfigV2(cfg *toml.Tree) UpdateReverter {
	c := configV2{
		config: config{
			Tree:      cfg,
			version:   2,
			cri:       "io.containerd.grpc.v1.cri",
			binaryKey: "BinaryName",
		},
	}

	return &c
}

// Update performs an update specific to v2 of the containerd config
func (config *configV2) Update(o *options) error {
	setAsDefault := o.setAsDefault

	runtimePath := filepath.Join(o.runtimeDir, runtimeBinary)
	config.update(o.runtimeClass, o.runtimeType, runtimePath, setAsDefault)

	return nil
}

// Revert performs a revert specific to v2 of the containerd config
func (config *configV2) Revert(o *options) error {
	containerdPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
	}
	runtimeClassPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
	}

	config.DeletePath(runtimeClassPath)
	if runtime, ok := config.GetPath(append(containerdPath, "default_runtime_name")).(string); ok {
		if o.runtimeClass == runtime {
			config.DeletePath(append(containerdPath, "default_runtime_name"))
		}
	}

	for i := 0; i < len(runtimeClassPath); i++ {
		if runtimes, ok := config.GetPath(runtimeClassPath[:len(runtimeClassPath)-i]).(*toml.Tree); ok {
			if len(runtimes.Keys()) == 0 {
				config.DeletePath(runtimeClassPath[:len(runtimeClassPath)-i])
			}
		}
	}

	if len(config.Keys()) == 1 && config.Keys()[0] == "version" {
		config.Delete("version")
	}

	return nil
}
