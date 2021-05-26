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

	"github.com/stretchr/testify/require"
)

func TestUpdateConfig(t *testing.T) {
	runtimeDirnameArg = "/test/runtime/dir"
	testCases := []struct {
		config         map[string]interface{}
		setAsDefault   bool
		expectedConfig map[string]interface{}
	}{
		{
			config:       map[string]interface{}{},
			setAsDefault: false,
			expectedConfig: map[string]interface{}{
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
		},
		{
			config: map[string]interface{}{
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
			setAsDefault: false,
			expectedConfig: map[string]interface{}{
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
		},
		{
			config: map[string]interface{}{
				"runtimes": map[string]interface{}{
					"not-nvidia": map[string]interface{}{
						"path": "some-other-path",
						"args": []string{},
					},
				},
			},
			setAsDefault: false,
			expectedConfig: map[string]interface{}{
				"runtimes": map[string]interface{}{
					"not-nvidia": map[string]interface{}{
						"path": "some-other-path",
						"args": []string{},
					},
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
		},
		{
			config:       map[string]interface{}{},
			setAsDefault: true,
			expectedConfig: map[string]interface{}{
				"default-runtime": "nvidia",
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
		},
		{
			config: map[string]interface{}{
				"default-runtime": "runc",
			},
			setAsDefault: true,
			expectedConfig: map[string]interface{}{
				"default-runtime": "nvidia",
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
		},
		{
			config: map[string]interface{}{
				"exec-opts":  []string{"native.cgroupdriver=systemd"},
				"log-driver": "json-file",
				"log-opts": map[string]string{
					"max-size": "100m",
				},
				"storage-driver": "overlay2",
			},
			expectedConfig: map[string]interface{}{
				"exec-opts":  []string{"native.cgroupdriver=systemd"},
				"log-driver": "json-file",
				"log-opts": map[string]string{
					"max-size": "100m",
				},
				"storage-driver": "overlay2",
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		setAsDefaultFlag = tc.setAsDefault
		err := UpdateConfig(tc.config)

		require.NoError(t, err, "%d: %v", i, tc)
		require.EqualValues(t, tc.expectedConfig, tc.config, "%d: %v", i, tc)
	}
}

func TestRevertConfig(t *testing.T) {
	testCases := []struct {
		config         map[string]interface{}
		expectedConfig map[string]interface{}
	}{
		{
			config:         map[string]interface{}{},
			expectedConfig: map[string]interface{}{},
		},
		{
			config: map[string]interface{}{
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
			expectedConfig: map[string]interface{}{},
		},
		{
			config: map[string]interface{}{
				"default-runtime": "nvidia",
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
			expectedConfig: map[string]interface{}{
				"default-runtime": "runc",
			},
		},
		{
			config: map[string]interface{}{
				"default-runtime": "not-nvidia",
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
			expectedConfig: map[string]interface{}{
				"default-runtime": "not-nvidia",
			},
		},
		{
			config: map[string]interface{}{
				"exec-opts":  []string{"native.cgroupdriver=systemd"},
				"log-driver": "json-file",
				"log-opts": map[string]string{
					"max-size": "100m",
				},
				"storage-driver": "overlay2",
				"runtimes": map[string]interface{}{
					"nvidia": map[string]interface{}{
						"path": "/test/runtime/dir/nvidia-container-runtime",
						"args": []string{},
					},
				},
			},
			expectedConfig: map[string]interface{}{
				"exec-opts":  []string{"native.cgroupdriver=systemd"},
				"log-driver": "json-file",
				"log-opts": map[string]string{
					"max-size": "100m",
				},
				"storage-driver": "overlay2",
			},
		},
	}

	for i, tc := range testCases {
		err := RevertConfig(tc.config)

		require.NoError(t, err, "%d: %v", i, tc)
		require.EqualValues(t, tc.expectedConfig, tc.config, "%d: %v", i, tc)
	}
}
