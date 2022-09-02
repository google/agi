# Copyright (C) 2022 Google Inc.
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

load("@gapid//tools/build/rules:repository.bzl", "maybe_repository")
load("@rules_fuchsia//fuchsia:deps.bzl", "fuchsia_clang_repository", "fuchsia_sdk_repository", "rules_fuchsia_deps")

def fuchsia_sdk_dependencies(locals = {}):
  rules_fuchsia_deps()

  maybe_repository(
    fuchsia_sdk_repository,
    name = "fuchsia_sdk_dynamic",
    locals = locals,
  )
  native.register_toolchains("@fuchsia_sdk_dynamic//:fuchsia_toolchain_sdk")

  maybe_repository(
    fuchsia_clang_repository,
    name = "fuchsia_clang",
    locals = locals,
    sdk_root_label = "@fuchsia_sdk_dynamic",
  )
