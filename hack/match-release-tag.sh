#!/bin/bash

# Copyright 2019 The Kubernetes Authors.
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

# Change directories to the parent directory of the one in which this
# script is located.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

usage() {
  cat <<EOF
usage: ${0} [TAG]
  Verifies the provided tag is a release tag.

TAG
  If no tag is provided then "git describe --dirty" is used to obtain the tag.

FLAGS
  -h    prints this help screen
  -x    run the examples
EOF
}

while getopts ':hx' opt; do
  case "${opt}" in
  h)
    usage 1>&2; exit 1
    ;;
  x)
    EXAMPLES=1
    ;;
  \?)
    { echo "invalid option: -${OPTARG}"; usage; } 1>&2; exit 1
    ;;
  :)
    echo "option -${OPTARG} requires an argument" 1>&2; exit 1
    ;;
  esac
done
shift $((OPTIND-1))

# The regular expression matches the following strings:
#   * v1.0.0-alpha.0
#   * v1.0.0-beta.0
#   * v1.0.0-rc.0
#   * v1.0.0
# Any occurence of a digit in the above examples may be multiple digits.
REGEX='^[[:space:]]{0,}v[[:digit:]]{1,}\.[[:digit:]]{1,}\.[[:digit:]]{1,}(-(alpha|beta|rc)\.[[:digit:]]{1,}){0,1}[[:space:]]{0,}$'

# Match the tag against the regular expression for a release tag.
match() {
  if [[ ${1} =~ ${REGEX} ]]; then
    echo "yay: ${1}"
  else
    exit_code="${?}"
    echo "nay: ${1}"
    return "${exit_code}"
  fi
}

# Run examples to illustrate valid and invalid values.
examples() {
  local semvers=" \
    v1.0.0-alpha.0 \
    v1.0.0-beta.0 \
    v1.0.0-rc.0 \
    v1.0.0 \
    v10.0.0 \
    v1.10.0 \
    v1.0.10 \
    v10.0.0-alpha.10 \
    v1.10.0-beta.10 \
    v1.0.10-rc.10 \
    1.0.0 \
    v1.0.0+rc.0 \
    v10a.0.0 \
    1.1.0-alpha.1 \
    v1.0.0-alpha.0a"
  set +o errexit
  for v in ${semvers}; do match "${v}"; done
  return 0
}

main() {
  # Get the tag from the remaining arguments or from "git describe --dirty"
  [ "${#}" -eq "0" ] || tag="${1}"
  [ -n "${tag-}" ] || tag="$(git describe --dirty)"

  # Match the tag against the regular expression for a release tag.
  match "${tag}" 1>&2
}

{ [ "${EXAMPLES-}" ] && examples; } || main "${@-}"
