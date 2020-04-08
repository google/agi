#!/bin/bash

# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script enables to run a Swarming test outside of the Kokoro workflow

set -e
set -x

SWARMING_TEST_DIR=$1
if [ -z "${SWARMING_TEST_DIR}" -o ! -d "${SWARMING_TEST_DIR}" ] ; then
  echo "Error: missing or invalid test directory argument."
  echo "Usage: `basename $0` tests/foobar"
  exit 1
fi

# Fake Swarming environment
export SWARMING_AUTH_FLAG=""
export SWARMING_BUILD_INFO="Manual"
export SWARMING_TRIGGERED_DIR="triggered-manual"
export SWARMING_RESULT_FILE="${SWARMING_TRIGGERED_DIR}/results.txt"

rm -rf ${SWARMING_TRIGGERED_DIR}
mkdir -p ${SWARMING_TRIGGERED_DIR}

./trigger.sh ${SWARMING_TEST_DIR}
for t in ${SWARMING_TRIGGERED_DIR}/*/*.json; do
  ./collect.sh ${t}
done
