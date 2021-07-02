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
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"
)

func TestV1ConfigSetDefaultRuntime(t *testing.T) {
	setAsDefaultFlag = true
	runtimeClassFlag = "runtime-class"

	testCases := []struct {
		containerdVersion      containerdVersion
		expectedDefaultRuntime string
	}{
		{
			containerdVersion:      containerdVersion("v1.2"),
			expectedDefaultRuntime: "",
		},
		{
			containerdVersion:      containerdVersion("v1.3"),
			expectedDefaultRuntime: "runtime-class",
		},
		{
			containerdVersion:      containerdVersion("v1.4"),
			expectedDefaultRuntime: "runtime-class",
		},
	}

	for _, tc := range testCases {

		config, _ := toml.TreeFromMap(map[string]interface{}{})

		err := UpdateV1Config(config, tc.containerdVersion)
		require.NoError(t, err)

		value := config.Get("plugins.cri.containerd.default_runtime_name")

		if tc.expectedDefaultRuntime == "" {
			require.Nil(t, value)
		} else {
			defaultRuntimeName, ok := value.(string)
			require.True(t, ok)
			require.Equal(t, tc.expectedDefaultRuntime, defaultRuntimeName)
		}
	}
}

func TestParseVersion(t *testing.T) {
	testCases := []struct {
		config            map[string]interface{}
		containerdVersion containerdVersion
		expectedVersion   int
	}{
		{
			config:            map[string]interface{}{},
			containerdVersion: containerdVersion("v1.2"),
			expectedVersion:   1,
		},
		{
			config:            map[string]interface{}{},
			containerdVersion: containerdVersion("v1.3"),
			expectedVersion:   2,
		},
		{
			config:            map[string]interface{}{},
			containerdVersion: containerdVersion("v1.4"),
			expectedVersion:   2,
		},
	}

	for _, tc := range testCases {
		config, err := toml.TreeFromMap(tc.config)
		require.NoError(t, err)

		version, err := ParseVersion(config, tc.containerdVersion)
		require.NoError(t, err)

		require.Equal(t, tc.expectedVersion, version)
	}
}

func TestNewContainerdVersion(t *testing.T) {
	testCases := []struct {
		version  string
		expected containerdVersion
		isError  bool
	}{
		{
			version:  "1.3",
			expected: containerdVersion("v1.3"),
		},
		{
			version:  "v1.3",
			expected: containerdVersion("v1.3"),
		},
		{
			version: "v",
			isError: true,
		},
		{
			version: "",
			isError: true,
		},
	}

	for _, tc := range testCases {
		c, err := newContainerdVersion(tc.version)

		if tc.isError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.expected, c)
		}
	}
}

func TestUpdateV1Config(t *testing.T) {
	const runtimeClass = "runtime-class"
	const runtimeDir = "/test/runtime/dir"
	runtimeClassFlag = runtimeClass
	runtimeTypeFlag = "runtime_type"
	runtimeDirnameArg = runtimeDir

	testCases := []struct {
		config            map[string]interface{}
		containerdVersion containerdVersion
		setAsDefault      bool
		expected          map[string]interface{}
	}{
		{
			config: map[string]interface{}{},
			expected: map[string]interface{}{
				"version": int64(1),
				"plugins": map[string]interface{}{
					"cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runtime_type",
									"runtime_root":                    "",
									"runtime_engine":                  "",
									"privileged_without_host_devices": false,
									"options": map[string]interface{}{
										"Runtime": "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			config: map[string]interface{}{
				"plugins": map[string]interface{}{
					"cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								"runc": map[string]interface{}{
									"runtime_type":                    "runc_runtime_type",
									"runtime_root":                    "runc_runtime_root",
									"runtime_engine":                  "runc_runtime_engine",
									"privileged_without_host_devices": true,
									"options": map[string]interface{}{
										"runc-option": "value",
										"Runtime":     "/runc-binary",
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"version": int64(1),
				"plugins": map[string]interface{}{
					"cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								"runc": map[string]interface{}{
									"runtime_type":                    "runc_runtime_type",
									"runtime_root":                    "runc_runtime_root",
									"runtime_engine":                  "runc_runtime_engine",
									"privileged_without_host_devices": true,
									"options": map[string]interface{}{
										"runc-option": "value",
										"Runtime":     "/runc-binary",
									},
								},
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runc_runtime_type",
									"runtime_root":                    "runc_runtime_root",
									"runtime_engine":                  "runc_runtime_engine",
									"privileged_without_host_devices": true,
									"options": map[string]interface{}{
										"runc-option": "value",
										"Runtime":     "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			config:            map[string]interface{}{},
			containerdVersion: containerdVersion("v1.3"),
			setAsDefault:      true,
			expected: map[string]interface{}{
				"version": int64(1),
				"plugins": map[string]interface{}{
					"cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runtime_type",
									"runtime_root":                    "",
									"runtime_engine":                  "",
									"privileged_without_host_devices": false,
									"options": map[string]interface{}{
										"Runtime": "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
							"default_runtime": map[string]interface{}{
								"runtime_type":                    "runtime_type",
								"runtime_root":                    "",
								"runtime_engine":                  "",
								"privileged_without_host_devices": false,
								"options": map[string]interface{}{
									"Runtime": "/test/runtime/dir/nvidia-container-runtime",
								},
							},
							"default_runtime_name": runtimeClass,
						},
					},
				},
			},
		},
		{
			config:            map[string]interface{}{},
			containerdVersion: containerdVersion("v1.2"),
			setAsDefault:      true,
			expected: map[string]interface{}{
				"version": int64(1),
				"plugins": map[string]interface{}{
					"cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runtime_type",
									"runtime_root":                    "",
									"runtime_engine":                  "",
									"privileged_without_host_devices": false,
									"options": map[string]interface{}{
										"Runtime": "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
							"default_runtime": map[string]interface{}{
								"runtime_type":                    "runtime_type",
								"runtime_root":                    "",
								"runtime_engine":                  "",
								"privileged_without_host_devices": false,
								"options": map[string]interface{}{
									"Runtime": "/test/runtime/dir/nvidia-container-runtime",
								},
							},
						},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		config, err := toml.TreeFromMap(tc.config)
		require.NoError(t, err, "%d: %v", i, tc)

		expected, err := toml.TreeFromMap(tc.expected)
		require.NoError(t, err, "%d: %v", i, tc)

		setAsDefaultFlag = tc.setAsDefault
		err = UpdateV1Config(config, tc.containerdVersion)
		require.NoError(t, err, "%d: %v", i, tc)

		configContents, _ := toml.Marshal(config)
		expectedContents, _ := toml.Marshal(expected)

		require.Equal(t, string(expectedContents), string(configContents), "%d: %v", i, tc)
	}

}

func TestRevertV1Config(t *testing.T) {
	const runtimeClass = "runtime-class"
	runtimeClassFlag = runtimeClass
	runtimeTypeFlag = "runtime_type"

	testCases := []struct {
		config map[string]interface {
		}
		expected map[string]interface{}
	}{
		{},
		{
			config: map[string]interface{}{
				"version": int64(1),
			},
		},
		{
			config: map[string]interface{}{
				"version": int64(1),
				"plugins": map[string]interface{}{
					"cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runtime_type",
									"runtime_root":                    "",
									"runtime_engine":                  "",
									"privileged_without_host_devices": false,
									"options": map[string]interface{}{
										"Runtime": "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			config: map[string]interface{}{
				"version": int64(1),
				"plugins": map[string]interface{}{
					"cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runtime_type",
									"runtime_root":                    "",
									"runtime_engine":                  "",
									"privileged_without_host_devices": false,
									"options": map[string]interface{}{
										"Runtime": "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
							"default_runtime": map[string]interface{}{
								"runtime_type":                    "runtime_type",
								"runtime_root":                    "",
								"runtime_engine":                  "",
								"privileged_without_host_devices": false,
								"options": map[string]interface{}{
									"Runtime": "/test/runtime/dir/nvidia-container-runtime",
								},
							},
							"default_runtime_name": runtimeClass,
						},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		config, err := toml.TreeFromMap(tc.config)
		require.NoError(t, err, "%d: %v", i, tc)

		expected, err := toml.TreeFromMap(tc.expected)
		require.NoError(t, err, "%d: %v", i, tc)

		err = RevertV1Config(config)
		require.NoError(t, err, "%d: %v", i, tc)

		configContents, _ := toml.Marshal(config)
		expectedContents, _ := toml.Marshal(expected)

		require.Equal(t, string(expectedContents), string(configContents), "%d: %v", i, tc)
	}
}

func TestUpdateV2Config(t *testing.T) {
	const runtimeClass = "runtime-class"
	const runtimeDir = "/test/runtime/dir"
	runtimeClassFlag = runtimeClass
	runtimeTypeFlag = "runtime_type"
	runtimeDirnameArg = runtimeDir

	testCases := []struct {
		config       map[string]interface{}
		setAsDefault bool
		expected     map[string]interface{}
	}{
		{
			config: map[string]interface{}{},
			expected: map[string]interface{}{
				"version": int64(2),
				"plugins": map[string]interface{}{
					"io.containerd.grpc.v1.cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runtime_type",
									"runtime_root":                    "",
									"runtime_engine":                  "",
									"privileged_without_host_devices": false,
									"options": map[string]interface{}{
										"BinaryName": "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			config: map[string]interface{}{
				"plugins": map[string]interface{}{
					"io.containerd.grpc.v1.cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								"runc": map[string]interface{}{
									"runtime_type":                    "runc_runtime_type",
									"runtime_root":                    "runc_runtime_root",
									"runtime_engine":                  "runc_runtime_engine",
									"privileged_without_host_devices": true,
									"options": map[string]interface{}{
										"runc-option": "value",
										"BinaryName":  "/runc-binary",
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"version": int64(2),
				"plugins": map[string]interface{}{
					"io.containerd.grpc.v1.cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								"runc": map[string]interface{}{
									"runtime_type":                    "runc_runtime_type",
									"runtime_root":                    "runc_runtime_root",
									"runtime_engine":                  "runc_runtime_engine",
									"privileged_without_host_devices": true,
									"options": map[string]interface{}{
										"runc-option": "value",
										"BinaryName":  "/runc-binary",
									},
								},
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runc_runtime_type",
									"runtime_root":                    "runc_runtime_root",
									"runtime_engine":                  "runc_runtime_engine",
									"privileged_without_host_devices": true,
									"options": map[string]interface{}{
										"runc-option": "value",
										"BinaryName":  "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			config:       map[string]interface{}{},
			setAsDefault: true,
			expected: map[string]interface{}{
				"version": int64(2),
				"plugins": map[string]interface{}{
					"io.containerd.grpc.v1.cri": map[string]interface{}{
						"containerd": map[string]interface{}{
							"runtimes": map[string]interface{}{
								runtimeClass: map[string]interface{}{
									"runtime_type":                    "runtime_type",
									"runtime_root":                    "",
									"runtime_engine":                  "",
									"privileged_without_host_devices": false,
									"options": map[string]interface{}{
										"BinaryName": "/test/runtime/dir/nvidia-container-runtime",
									},
								},
							},
							"default_runtime_name": runtimeClass,
						},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		config, err := toml.TreeFromMap(tc.config)
		require.NoError(t, err, "%d: %v", i, tc)

		expected, err := toml.TreeFromMap(tc.expected)
		require.NoError(t, err, "%d: %v", i, tc)

		setAsDefaultFlag = tc.setAsDefault
		err = UpdateV2Config(config)
		require.NoError(t, err, "%d: %v", i, tc)

		configContents, _ := toml.Marshal(config)
		expectedContents, _ := toml.Marshal(expected)

		require.Equal(t, string(expectedContents), string(configContents), "%d: %v", i, tc)
	}
}

func TestRevertV2Config(t *testing.T) {
	const runtimeClass = "runtime-class"
	runtimeClassFlag = runtimeClass

	testCases := []struct {
		config       map[string]interface{}
		setAsDefault bool
		expected     map[string]interface{}
	}{
		{},
		{
			config: map[string]interface{}{
				"version": int64(2),
			},
		},
	}

	for i, tc := range testCases {
		config, err := toml.TreeFromMap(tc.config)
		require.NoError(t, err, "%d: %v", i, tc)

		expected, err := toml.TreeFromMap(tc.expected)
		require.NoError(t, err, "%d: %v", i, tc)

		setAsDefaultFlag = tc.setAsDefault
		err = RevertV2Config(config)
		require.NoError(t, err, "%d: %v", i, tc)

		configContents, _ := toml.Marshal(config)
		expectedContents, _ := toml.Marshal(expected)

		require.Equal(t, string(expectedContents), string(configContents), "%d: %v", i, tc)
	}
}
