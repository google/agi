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

"""This is the entry point for Vulkan Code Generator"""

import abc

from pathlib import Path
from typing import List

from textwrap import dedent
from vulkan_generator.vulkan_parser import types
from vulkan_generator.codegen_utils import codegen

def generate_struct_factories_h(file_path: Path, vulkan_metadata: types.VulkanMetadata):
    ''' Generates struct_factories.h '''
    with open(file_path, "w", encoding="ascii") as remapper_h:

        remapper_h.write(codegen.generated_license_header())

        remapper_h.write(dedent("""
            namespace agi {
            namespace replay2 {

        """))

        remapper_h.write(dedent("""
            }
            }

        """))


def generate_struct_factories_cpp(file_path: Path, vulkan_metadata: types.VulkanMetadata):
    ''' Generates struct_factories.cc '''
    with open(file_path, "w", encoding="ascii") as remapper_cpp:

        remapper_cpp.write(codegen.generated_license_header())

        remapper_cpp.write(dedent("""
            #include "struct_factories.h"

            namespace agi {
            namespace replay2 {

        """))

        remapper_cpp.write(dedent("""
            }
            }

        """))


def generate_struct_factories_tests(file_path: Path, vulkan_metadata: types.VulkanMetadata):
    ''' Generates struct_factories_tests.cc '''
    with open(file_path, "w", encoding="ascii") as tests_cpp:

        tests_cpp.write(codegen.generated_license_header())

        tests_cpp.write(dedent("""
            #include "struct_factories.h"
            #include <gtest/gtest.h>

            using namespace agi::replay2;
            """))

        tests_cpp.write(dedent("""
            TEST(VulkanStructFactories, AAA) {{
                EXPECT_TRUE(false);
            }}"""))

        tests_cpp.write(dedent("""
        int main(int argc, char **argv) {
            ::testing::InitGoogleTest(&argc, argv);
            return RUN_ALL_TESTS();
        }
        """))
