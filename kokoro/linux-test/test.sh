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
CURL="curl -fksLS --http1.1 --retry 3"

# Get bazel.
BAZEL_VERSION=6.5.0
$CURL -O https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh
echo "c0161a346b9c0d00e6eb3d3e8f9c4dece32f6292520248c5ab2e3527265601c1  bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh" | sha256sum --check
mkdir bazel
bash bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh --prefix=$PWD/bazel

# Get Clang-12.
$CURL -O https://apt.llvm.org/llvm-snapshot.gpg.key
echo "ce6eee4130298f79b0e0f09a89f93c1bc711cd68e7e3182d37c8e96c5227e2f0  llvm-snapshot.gpg.key" | sha256sum --check
sudo apt-key add llvm-snapshot.gpg.key
sudo add-apt-repository 'deb http://apt.llvm.org/focal/ llvm-toolchain-focal main'
sudo apt-get -y update
sudo apt-get install -y clang-12
export CC=/usr/bin/clang-12

# Get the Android NDK.
$CURL -O https://dl.google.com/android/repository/android-ndk-r21d-linux-x86_64.zip
echo "dd6dc090b6e2580206c64bcee499bc16509a5d017c6952dcd2bed9072af67cbd  android-ndk-r21d-linux-x86_64.zip" | sha256sum --check
unzip -q android-ndk-r21d-linux-x86_64.zip
export ANDROID_NDK_HOME=$PWD/android-ndk-r21d

# Get recent build tools.
echo y | $ANDROID_HOME/cmdline-tools/latest/bin/sdkmanager --install 'build-tools;30.0.3' 'platforms;android-34'

# Get the JDK from our mirror.
JDK_BUILD=zulu11.39.15-ca
JDK_VERSION=11.0.7
JDK_NAME=$JDK_BUILD-jdk$JDK_VERSION-linux_x64
$CURL -O https://storage.googleapis.com/jdk-mirror/$JDK_BUILD/$JDK_NAME.zip
echo "afbaa594447596a7fcd78df4ee59436ee19b43e27111e2e5a21a3272a89074cf  $JDK_NAME.zip" | sha256sum --check
unzip -q $JDK_NAME.zip
export JAVA_HOME=$PWD/$JDK_NAME

cd $SRC
BUILD_SHA=${KOKORO_GITHUB_COMMIT:-$KOKORO_GITHUB_PULL_REQUEST_COMMIT}

function test {
    $BUILD_ROOT/bazel/bin/bazel \
        --output_base="${TMP}/bazel_out" \
        test -c opt --config symbols \
        --define AGI_BUILD_NUMBER="$KOKORO_BUILD_NUMBER" \
        --define AGI_BUILD_SHA="$BUILD_SHA" \
        --test_tag_filters=-needs_gpu \
        --show_timestamps \
        $@
}

 # If linting fails we do not need to wait entire AGI tests
test lint

# Test Vulkan Generator first as its much smaller and independent than other packages
test tests-vulkan-generator

# Running all the tests in one go leads to an out-of-memory error on Kokoro, hence the division in smaller test sets
test tests-core
test tests-gapis-api
test tests-gapis-replay-resolve
test tests-gapis-other
test tests-gapir
test tests-gapil
test tests-general
