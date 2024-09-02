# Copyright (C) 2018 Google Inc.
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

# Defines macros to be called from a WORKSPACE file to setup the GAPID
# dependencies and toolchains.

load("@gapid//tools/build:cc_toolchain.bzl", "cc_configure")
load("@gapid//tools/build/rules:android.bzl", "android_native_app_glue", "ndk_vk_validation_layer", "ndk_version_check")
load("@gapid//tools/build/rules:repository.bzl", "github_repository", "maybe_repository")
load("@gapid//tools/build/third_party:breakpad.bzl", "breakpad")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository", "new_git_repository")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Defines the repositories for GAPID's dependencies, excluding the
# go dependencies, which require @io_bazel_rules_go to be setup.
#  android - if false, the Android NDK/SDK are not initialized.
#  mingw - if false, our cc toolchain, which uses MinGW on Windows is not initialized.
#  locals - can be used to provide local path overrides for repos:
#     {"foo": "/path/to/foo"} would cause @foo to be a local repo based on /path/to/foo.
def gapid_dependencies(android = True, mingw = True, locals = {}):
    #####################################################
    # Get repositories with workspace rules we need first

    maybe_repository(
        http_archive,
        name = "io_bazel_rules_go",
        locals = locals,
        sha256 = "6dc2da7ab4cf5d7bfc7c949776b1b7c733f05e56edc4bcd9022bb249d2e2a996",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.39.1/rules_go-v0.39.1.zip",
            "https://github.com/bazelbuild/rules_go/releases/download/v0.39.1/rules_go-v0.39.1.zip",
        ],
    )

    maybe_repository(
        http_archive,
        name = "rules_python",
        locals = locals,
        sha256 = "be04b635c7be4604be1ef20542e9870af3c49778ce841ee2d92fcb42f9d9516a",
        strip_prefix = "rules_python-0.35.0",
        url = "https://github.com/bazelbuild/rules_python/releases/download/0.35.0/rules_python-0.35.0.tar.gz",
    )

    maybe_repository(
        http_archive,
        name = "bazel_gazelle",
        locals = locals,
        sha256 = "727f3e4edd96ea20c29e8c2ca9e8d2af724d8c7778e7923a854b2c80952bc405",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.30.0/bazel-gazelle-v0.30.0.tar.gz",
            "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.30.0/bazel-gazelle-v0.30.0.tar.gz",
        ],
    )

    maybe_repository(github_repository,
        name = "net_zlib", # name used by rules_go
        locals = locals,
        organization = "madler",
        project = "zlib",
        commit = "21767c654d31d2dccdde4330529775c6c5fd5389",  # 1.2.12
        build_file = "@gapid//tools/build/third_party:zlib.BUILD",
        sha256 = "b860a877983100f28c7bcf2f3bb7abbca8833e7ce3af79edfda21358441435d3",
    )

    maybe_repository(
        github_repository,
        name = "com_google_protobuf",
        locals = locals,
        organization = "google",
        project = "protobuf",
        repo_mapping = {"@zlib": "@net_zlib"},
        commit = "e915ce24b3d43c0fffcbf847354288c07dda1de0",  # 3.25.4
        sha256 = "cfaf4871b55a86a5d04f19bacd64e940c2e2e015dbfa27d951bf63283dc8ee4e",
        patches = [
            "@gapid//tools/build/third_party:com_google_protobuf.patch",
        ],
    )

    maybe_repository(
        github_repository,
        name = "com_google_protobuf_3_21_5",
        locals = locals,
        organization = "google",
        project = "protobuf",
        commit = "ab840345966d0fa8e7100d771c92a73bfbadd25c",  # 3.21.5
        sha256 = "0025119f5c97871436b4b271fee48bd6bfdc99956023e0d4fd653dd8eaaeff52",
        repo_mapping = {"@zlib": "@net_zlib"},
    )

    maybe_repository(
        github_repository,
        name = "com_github_grpc_grpc",
        locals = locals,
        organization = "grpc",
        project = "grpc",
        commit = "aef0f0ccc3d21a328282144b8aa666f3c570dfb9",  # 1.64.3
        sha256 = "7e586cf8d6e386227ef779b91b9e91874bbf012de4442de047ded053e114a8da",
        repo_mapping = {"@zlib": "@net_zlib"},
        patches = [
            # Remove calling the go dependencies, since we do that ourselves.
            "@gapid//tools/build/third_party:com_github_grpc_grpc.patch",
        ],
    )

    ###########################################
    # Now get all our other non-go dependencies

    maybe_repository(
        github_repository,
        name = "com_google_googletest",
        locals = locals,
        organization = "google",
        project = "googletest",
        commit = "58d77fa8070e8cec2dc1ed015d66b454c8d78850",  # 1.12.1
        sha256 = "ab78fa3f912d44d38b785ec011a25f26512aaedc5291f51f3807c592b506d33a",
    )

    maybe_repository(
        github_repository,
        name = "astc_encoder",
        locals = locals,
        organization = "ARM-software",
        project = "astc-encoder",
        commit = "f6236cf158a877b3279a2090dbea5e9a4c105d64",  # 4.0.0
        build_file = "@gapid//tools/build/third_party:astc-encoder.BUILD",
        sha256 = "28305281b0fe89b0e57c61f684ed7f6145a5079a3f4f03a4fd3fe0c27df0bb45",
    )

    maybe_repository(
        github_repository,
        name = "etc2comp",
        locals = locals,
        organization = "google",
        project = "etc2comp",
        commit = "9cd0f9cae0f32338943699bb418107db61bb66f2", # 2017/04/24
        build_file = "@gapid//tools/build/third_party:etc2comp.BUILD",
        sha256 = "0ddcf7484c0d55bc5a3cb92edb4812dc932ac9f73b4641ad2843fec82ae8cf90",
    )

    maybe_repository(
        breakpad,
        name = "breakpad",
        locals = locals,
        commit = "57bed07ad46f46ae575d1e38bf07c4d3137bbf53",
        build_file = "@gapid//tools/build/third_party/breakpad:breakpad.BUILD",
    )

    maybe_repository(
        github_repository,
        name = "cityhash",
        locals = locals,
        organization = "google",
        project = "cityhash",
        commit = "f5dc54147fcce12cefd16548c8e760d68ac04226",
        build_file = "@gapid//tools/build/third_party:cityhash.BUILD",
        sha256 = "20ab6da9929826c7c81ea3b7348190538a23f823a8b749c2da9715ecb7a6b545",
    )

    # Override the gRPC abseil dependency, so we can patch it.
    maybe_repository(
        github_repository,
        name = "com_google_absl",
        locals = locals,
        organization = "abseil",
        project = "abseil-cpp",
        commit = "4a2c63365eff8823a5221db86ef490e828306f9d",  # Abseil LTS 20240116.0
        sha256 = "f49929d22751bf70dd61922fb1fd05eb7aec5e7a7f870beece79a6e28f0a06c1",
    )

    maybe_repository(
        github_repository,
        name = "glslang",
        locals = locals,
        organization = "KhronosGroup",
        project = "glslang",
        commit = "73c9630da979017b2f7e19c6549e2bdb93d9b238",  # 11.11.0
        sha256 = "9304cb73d86fc8e3f1cbcdbd157cd2750baad10cb9e3a798986bca3c3a1be1f0",
        patches = [
            "@gapid//tools/build/third_party:glslang.patch",
        ]
    )

    maybe_repository(
        github_repository,
        name = "stb",
        locals = locals,
        organization = "nothings",
        project = "stb",
        commit = "af1a5bc352164740c1cc1354942b1c6b72eacb8a",
        sha256 = "e3d0edbecd356506d3d69b87419de2f9d180a98099134c6343177885f6c2cbef",
        build_file = "@gapid//tools/build/third_party:stb.BUILD",
    )

    maybe_repository(
        new_git_repository,
        name = "lss",
        locals = locals,
        remote = "https://chromium.googlesource.com/linux-syscall-support",
        commit = "c0c9689369b4c5e46b440993807ce4b0a7c9af8a",
        build_file = "@gapid//tools/build/third_party:lss.BUILD",
        shallow_since = "1660655052 +0000",
    )

    maybe_repository(
        github_repository,
        name = "perfetto",
        locals = locals,
        organization = "google",
        project = "perfetto",
        commit = "0ff403688efce9d5de43d69cae3c835e993e4730",  # 29+
        sha256 = "e609a91a6d64caf9a4e4b64f1826d160eba8fd84f7e5e94025ba287374e78e30",
        repo_mapping = {"@com_google_protobuf": "@com_google_protobuf_3_21_5"},
        patches = [
            "@gapid//tools/build/third_party:perfetto.patch",
        ]
    )

    maybe_repository(
        http_archive,
        name = "sqlite",
        locals = locals,
        url = "https://storage.googleapis.com/perfetto/sqlite-amalgamation-3440200.zip",
        sha256 = "833be89b53b3be8b40a2e3d5fedb635080e3edb204957244f3d6987c2bb2345f",
        strip_prefix = "sqlite-amalgamation-3440200",
        build_file = "@perfetto//bazel:sqlite.BUILD",
    )

    maybe_repository(
        http_archive,
        name = "sqlite_src",
        locals = locals,
        url = "https://storage.googleapis.com/perfetto/sqlite-src-3440200.zip",
        sha256 = "73187473feb74509357e8fa6cb9fd67153b2d010d00aeb2fddb6ceeb18abaf27",
        strip_prefix = "sqlite-src-3440200",
        build_file = "@perfetto//bazel:sqlite.BUILD",
    )

    maybe_repository(
        native.new_local_repository,
        name = "perfetto_cfg",
        locals = locals,
        path = "tools/build/third_party/perfetto",
        build_file = "@gapid//tools/build/third_party/perfetto:BUILD.bazel",
    )

    maybe_repository(
        github_repository,
        name = "spirv_headers",
        locals = locals,
        organization = "KhronosGroup",
        project = "SPIRV-Headers",
        commit = "b2a156e1c0434bc8c99aaebba1c7be98be7ac580",  # 1.3.216.0
        sha256 = "fbb4e256c2e9385169067d3b6f2ed3800f042afac9fb44a348b619aa277bb1fd",
    )

    maybe_repository(
        github_repository,
        name = "spirv_cross",
        locals = locals,
        organization = "KhronosGroup",
        project = "SPIRV-Cross",
        commit = "0e2880ab990e79ce6cc8c79c219feda42d98b1e8",  # 2021-08-30
        build_file = "@gapid//tools/build/third_party:spirv-cross.BUILD",
        sha256 = "7ae1069c29f507730ffa5143ac23a5be87444d18262b3b327dfb00ca53ae07cd",
        patches = [
            "@gapid//tools/build/third_party:spirv_cross.patch",
        ]
    )

    maybe_repository(
        github_repository,
        name = "spirv_tools",
        locals = locals,
        organization = "KhronosGroup",
        project = "SPIRV-Tools",
        commit = "b930e734ea198b7aabbbf04ee1562cf6f57962f0",  # 1.3.216.0
        sha256 = "2d956e7d49a8795335d13c3099c44aae4fe501eb3ec0dbf7e1bfa28df8029b43",
    )

    maybe_repository(
        github_repository,
        name = "spirv_reflect",
        locals = locals,
        organization = "KhronosGroup",
        project = "SPIRV-Reflect",
        commit = "0f142bbfe9bd7aeeae6b3c703bcaf837dba8df9d",  # 1.3.216.0
        sha256 = "8eae9dcd2f6954b452a9a53b02ce7507dd3dcd02bacb678c4316f336dab79d86",
    )

    maybe_repository(
        http_archive,
        name = "vscode-languageclient",
        locals = locals,
        url = "https://registry.npmjs.org/vscode-languageclient/-/vscode-languageclient-2.6.3.tgz",
        build_file = "@gapid//tools/build/third_party:vscode-languageclient.BUILD",
        sha256 = "42ad6dc73bbf24a067d1e21038d35deab975cb207ac2d63b81c37a977d431d8f",
    )

    maybe_repository(
        http_archive,
        name = "vscode-jsonrpc",
        locals = locals,
        url = "https://registry.npmjs.org/vscode-jsonrpc/-/vscode-jsonrpc-2.4.0.tgz",
        build_file = "@gapid//tools/build/third_party:vscode-jsonrpc.BUILD",
        sha256= "bed9b2facb7d179f14c8a710db8e613be56bd88b2a75443143778813048b5c89",
    )

    maybe_repository(
        http_archive,
        name = "vscode-languageserver-types",
        locals = locals,
        url = "https://registry.npmjs.org/vscode-languageserver-types/-/vscode-languageserver-types-1.0.4.tgz",
        build_file = "@gapid//tools/build/third_party:vscode-languageserver-types.BUILD",
        sha256 = "0cd219ac388c41a70c3ff4f72d25bd54fa351bc0850196c25c6c3361e799ac79",
    )

    maybe_repository(
        github_repository,
        name = "vulkan-headers",
        locals = locals,
        organization = "KhronosGroup",
        project = "Vulkan-Headers",
        commit = "3ef4c97fd6ea001d75a8e9da408ee473c180e456",  # 1.3.216
        build_file = "@gapid//tools/build/third_party:vulkan-headers.BUILD",
        sha256 = "64a7fc6994501b36811af47b21385251a56a136a3ed3cf92673465c9d62985a1",
    )

    if android:
        maybe_repository(
            native.android_sdk_repository,
            name = "androidsdk",
            locals = locals,
            api_level = 26, # This is the target API
            build_tools_version = "30.0.3",
        )

        maybe_repository(
            native.android_ndk_repository,
            name = "androidndk",
            locals = locals,
            api_level = 23, # This is the minimum API
        )

        maybe_repository(
            android_native_app_glue,
            name = "android_native_app_glue",
            locals = locals,
        )

        maybe_repository(
            ndk_vk_validation_layer,
            name = "ndk_vk_validation_layer",
            locals = locals,
        )

        maybe_repository(
            ndk_version_check,
            name = "ndk_version_check",
            locals = locals,
        )

    if mingw:
        cc_configure()
