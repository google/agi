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

cc_library(
    name = "astc-encoder",
    srcs = [
        "Source/astcenc.h",
        "Source/astcenc_averages_and_directions.cpp",
        "Source/astcenc_block_sizes2.cpp",
        "Source/astcenc_color_quantize.cpp",
        "Source/astcenc_color_unquantize.cpp",
        "Source/astcenc_compress_symbolic.cpp",
        "Source/astcenc_compute_variance.cpp",
        "Source/astcenc_decompress_symbolic.cpp",
        "Source/astcenc_encoding_choice_error.cpp",
        "Source/astcenc_entry.cpp",
        "Source/astcenc_find_best_partitioning.cpp",
        "Source/astcenc_ideal_endpoints_and_weights.cpp",
        "Source/astcenc_image.cpp",
        "Source/astcenc_integer_sequence.cpp",
        "Source/astcenc_internal.h",
        "Source/astcenc_kmeans_partitioning.cpp",
        "Source/astcenc_mathlib.cpp",
        "Source/astcenc_mathlib.h",
        "Source/astcenc_mathlib_softfloat.cpp",
        "Source/astcenc_partition_tables.cpp",
        "Source/astcenc_percentile_tables.cpp",
        "Source/astcenc_pick_best_endpoint_format.cpp",
        "Source/astcenc_quantization.cpp",
        "Source/astcenc_symbolic_physical.cpp",
        "Source/astcenc_vecmathlib.h",
        "Source/astcenc_weight_align.cpp",
        "Source/astcenc_weight_quant_xfer_tables.cpp",
        "Source/astcenccli_image.cpp",
        "Source/astcenccli_internal.h",
        "Source/astcenccli_platform_dependents.cpp",
    ],
    hdrs = [
        "Source/astcenc.h",
        "Source/astcenc_mathlib.h",
        "Source/astcenccli_internal.h",
    ],
    copts = [
        "-Wno-c++11-narrowing",
        "-mfpmath=sse",
        "-msse2",
        "-DASTCENC_SSE=20",
        "-DASTCENC_POPCNT=0",
        "-DASTCENC_AVX=0",
        "-DASTCENC_ISA_INVARIANCE=0",
        "-DASTCENC_VECALIGN=16",
    ],
    include_prefix = "third_party/astc-encoder",
    visibility = ["//visibility:public"],
)
