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
	*toml.Tree
	containerdVersion string
}

func newConfigV2(cfg *toml.Tree) UpdateReverter {
	c := configV2{
		Tree: cfg,
	}

	return &c
}

// Update performs an update specific to v2 of the containerd config
func (config *configV2) Update(o *options) error {
	runtimePath := filepath.Join(o.runtimeDir, runtimeBinary)

	// We ensure that the version is set to 2. This handles the case where the config was empty and
	// the config version was determined from the containerd version.
	config.Set("version", int64(2))

	containerdPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
	}
	runcPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		"runc",
	}
	runtimeClassPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
	}
	runtimeClassOptionsPath := []string{
		"plugins",
		"io.containerd.grpc.v1.cri",
		"containerd",
		"runtimes",
		o.runtimeClass,
		"options",
	}

	switch runc := config.GetPath(runcPath).(type) {
	case *toml.Tree:
		runc, _ = toml.Load(runc.String())
		config.SetPath(runtimeClassPath, runc)
	default:
		config.SetPath(append(runtimeClassPath, "runtime_type"), o.runtimeType)
		config.SetPath(append(runtimeClassPath, "runtime_root"), "")
		config.SetPath(append(runtimeClassPath, "runtime_engine"), "")
		config.SetPath(append(runtimeClassPath, "privileged_without_host_devices"), false)
	}
	config.SetPath(append(runtimeClassOptionsPath, "BinaryName"), runtimePath)

	if o.setAsDefault {
		config.SetPath(append(containerdPath, "default_runtime_name"), o.runtimeClass)
	}

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
