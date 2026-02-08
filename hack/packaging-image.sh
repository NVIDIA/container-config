# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
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

SCRIPTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/../hack && pwd )"

DOCKERFILE_ROOT=${SCRIPTS_DIR}/../deployments/container

PACKAGING_IMAGE=$(grep -E "^FROM .* AS packagingdefault$" ${DOCKERFILE_ROOT}/Dockerfile | sed -e 's/FROM \(.*\) AS packagingdefault/\1/g')

echo $PACKAGING_IMAGE
