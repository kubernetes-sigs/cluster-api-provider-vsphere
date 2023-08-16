#!/usr/bin/env bash
# Copyright 2023 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

MAKEFILE_PATH="$(dirname "${0}")/../Makefile"

MARKDOWNLINT_CLI2_VERSION="$(grep '^MARKDOWNLINT_CLI2_VERSION ' < "${MAKEFILE_PATH}" | awk '{print $NF}' | tr -d '"')"
TARGET_IMAGE_NAME="$(grep '^MARKDOWNLINT_CLI2_IMAGE_NAME ' < "${MAKEFILE_PATH}" | awk '{print $NF}' | tr -d '"')"

ORIGINAL_IMAGE="davidanson/markdownlint-cli2:${MARKDOWNLINT_CLI2_VERSION}"
TARGET_IMAGE="${TARGET_IMAGE_NAME}:${MARKDOWNLINT_CLI2_VERSION}"

echo docker pull "${ORIGINAL_IMAGE}"
echo docker tag "${ORIGINAL_IMAGE}" "${TARGET_IMAGE}"
echo docker push "${TARGET_IMAGE}"
