#!/bin/bash
# Copyright (C) 2019 Google Inc.
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

# Linux Build Script.
set -ex

BUILD_ROOT=$PWD
SRC=$PWD/github/agi/

# Get bazel
BAZEL_VERSION=2.0.0
curl -L -k -O -s https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh
mkdir bazel
bash bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh --prefix=$PWD/bazel

# Get GCC 8
sudo rm /etc/apt/sources.list.d/cuda.list*
sudo add-apt-repository -y ppa:ubuntu-toolchain-r/test
sudo apt-get -q update
sudo apt-get -qy install gcc-8 g++-8
export CC=/usr/bin/gcc-8

cd $SRC
BUILD_SHA=${KOKORO_GITHUB_COMMIT:-$KOKORO_GITHUB_PULL_REQUEST_COMMIT}

function bazel {
  echo $(date): Starting bazel command $@...
  $BUILD_ROOT/bazel/bin/bazel \
    --output_base="${TMP}/bazel_out_dbg" \
    "$@"
  echo $(date): bazel command completed.
}

function test {
    echo $(date): Starting test for $@...
    $BUILD_ROOT/bazel/bin/bazel \
        --output_base="${TMP}/bazel_out" \
        test -c opt --config symbols \
        --define AGI_BUILD_NUMBER="$KOKORO_BUILD_NUMBER" \
        --define AGI_BUILD_SHA="$BUILD_SHA" \
        --test_tag_filters=-needs_gpu \
        $@
    echo $(date): Tests completed.
}

# Install Vulkan loader and xvfb (X virtual framebuffer).
sudo apt-get -qy install libvulkan1 xvfb

# Download and extract prebuilt SwiftShader.
curl -fsSL -o swiftshader.zip https://github.com/google/gfbuild-swiftshader/releases/download/github%2Fgoogle%2Fgfbuild-swiftshader%2F0bbf7ba9f909092f0328b1d519d5f7db1773be57/gfbuild-swiftshader-0bbf7ba9f909092f0328b1d519d5f7db1773be57-Linux_x64_Debug.zip
unzip -d swiftshader swiftshader.zip

# Use SwiftShader.
export VK_ICD_FILENAMES="${SRC}/swiftshader/lib/vk_swiftshader_icd.json"
export VK_LOADER_DEBUG=all

# Build Vulkan sample binary.
bazel build -c dbg //cmd/vulkan_sample:vulkan_sample

# Run it.
SAMPLE_PATH="$(bazel run -c dbg --run_under "echo" //cmd/vulkan_sample:vulkan_sample)"
timeout --preserve-status -s INT -k 10 5 xvfb-run -e xvfb.log -a "${SAMPLE_PATH}"

# Running all the tests in one go leads to an out-of-memory error on Kokoro, hence the division in smaller test sets
test tests-core
test tests-gapis-api
test tests-gapis-replay-resolve
test tests-gapis-other
test tests-gapir
test tests-gapil
test tests-general
