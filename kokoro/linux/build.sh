#!/bin/bash
# Copyright (C) 2017 Google Inc.
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

mkdir -p $BUILD_ROOT/out/dist
echo "test" > $BUILD_ROOT/out/dist/test.gfxtrace

# Get bazel.
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

# Get the Android NDK
curl -L -k -O -s https://dl.google.com/android/repository/android-ndk-r21-linux-x86_64.zip
unzip -q android-ndk-r21-linux-x86_64.zip
export ANDROID_NDK_HOME=$PWD/android-ndk-r21

# Get recent build tools
echo y | $ANDROID_HOME/tools/bin/sdkmanager --install 'build-tools;29.0.2'

cd $SRC
BUILD_SHA=${DEV_PREFIX}${KOKORO_GITHUB_COMMIT:-$KOKORO_GITHUB_PULL_REQUEST_COMMIT}

function build {
  echo $(date): Starting build for $@...
  $BUILD_ROOT/bazel/bin/bazel \
    --output_base="${TMP}/bazel_out" \
    build -c opt --config symbols \
    --define AGI_BUILD_NUMBER="$KOKORO_BUILD_NUMBER" \
    --define AGI_BUILD_SHA="$BUILD_SHA" \
    $@
  echo $(date): Build completed.
}

# Build each API package separately first, as the go-compiler needs ~8GB of
# RAM for each of the big API packages.
# for api in gles vulkan gvr; do
#   build //gapis/api/$api:go_default_library
# done

# # Build the package and symbol file.
# build //:pkg //:symbols

# Build the Vulkan sample
build //cmd/vulkan_sample:vulkan_sample

# # Build and run the smoketests.
# set +e
# build //cmd/smoketests:smoketests
# echo $(date): Run smoketests...
# # Using "bazel run //cmd/smoketests seems to make 'bazel-bin/pkg/gapit'
# # disappear, hence we call the binary directly
# bazel-bin/cmd/smoketests/linux_amd64_stripped/smoketests -gapit bazel-bin/pkg/gapit -traces test/traces
# for i in smoketests.*/*/*.log
# do
#   echo "============================================================"
#   echo $i
#   cat $i
# done

ARTIFACTS_PREFIX=$BUILD_ROOT/out/dist
mkdir -p $ARTIFACTS_PREFIX

curl -fsSL -o agi.zip https://github.com/google/agi-dev-releases/releases/download/v0.9.0-dev-20200317/agi-0.9.0-dev-20200317-linux.zip
unzip agi.zip
# mkdir -p bazel-bin/pkg
# cp -r agi/* bazel-bin/pkg/

##
## Test capture and replay of the Vulkan Sample App.
##

# Install the Vulkan loader and xvfb (X virtual framebuffer).
sudo apt-get -qy install libvulkan1 xvfb

# Get prebuilt SwiftShader.
# This is the latest commit at the time of writing.
# Should be updated periodically.
curl -fsSL -o swiftshader.zip https://github.com/google/gfbuild-swiftshader/releases/download/github%2Fgoogle%2Fgfbuild-swiftshader%2F0bbf7ba9f909092f0328b1d519d5f7db1773be57/gfbuild-swiftshader-0bbf7ba9f909092f0328b1d519d5f7db1773be57-Linux_x64_Debug.zip
unzip -d swiftshader swiftshader.zip

# Use SwiftShader.
export VK_ICD_FILENAMES="$(pwd)/swiftshader/lib/vk_swiftshader_icd.json"
export VK_LOADER_DEBUG=all

# Just try running the app first.

# Allow non-zero exit status.
set +e

xvfb-run -e xvfb.log -a timeout --preserve-status -s INT -k 5 5 bazel-bin/cmd/vulkan_sample/vulkan_sample

APP_EXIT_STATUS="${?}"

set -e

if test -f xvfb.log; then
  cat xvfb.log
fi

# This line will exit with status 1 if the app's exit status
# was anything other than 130 (128+SIGINT).
test "${APP_EXIT_STATUS}" -eq 130

# TODO(https://github.com/google/gapid/issues/3163): The coherent memory
#  tracker must be disabled with SwiftShader for now.
xvfb-run -e xvfb.log -a agi/gapit trace -device host -disable-coherentmemorytracker -disable-pcs -disable-unknown-extensions -record-errors -no-buffer -api vulkan -start-at-frame 5 -capture-frames 10 -observe-frames 1 -out ${ARTIFACTS_PREFIX}/vulkan_sample.gfxtrace bazel-bin/cmd/vulkan_sample/vulkan_sample

sync
ls ${ARTIFACTS_PREFIX}

xvfb-run -e xvfb.log -a agi/gapit video -gapir-nofallback -type sxs -frames-minimum 10 -out vulkan_sample.mp4  ${ARTIFACTS_PREFIX}/vulkan_sample.gfxtrace

