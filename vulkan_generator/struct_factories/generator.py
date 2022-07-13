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
from typing import Dict

from textwrap import dedent
from vulkan_generator.vulkan_parser import types
from vulkan_generator.codegen_utils import codegen


def struct_factory_name(struct: str) -> str:
    return struct + "Factory"


def set_member_name(member: str) -> str:
    return "set" + member[0:1].upper() + member[1:]


def get_member_name(member: str) -> str:
    return "get" + member[0:1].upper() + member[1:]

def tokenize_typename(member_type: str) -> List[str]:

    tokens: List[str] = []
    current_token = ""

    for char in member_type:
        if char.isspace():
            if len(current_token) > 0:
                tokens += [current_token]
            current_token = ""
        elif char == "*":
            if len(current_token) > 0:
                tokens += [current_token]
            tokens += ["*"]
            current_token = ""
        else:
            current_token += char

    if len(current_token) > 0:
        tokens += [current_token]

    return tokens

def sort_struts_impl(struct :str,
                     processed_structs : Dict[str, bool],
                     sorted_structs : List[str],
                     vulkan_metadata: types.VulkanMetadata) :

    if struct in processed_structs:
        return

    for member in vulkan_metadata.types.structs[struct].members:
        tokens = tokenize_typename(vulkan_metadata.types.structs[struct].members[member].variable_type)
        for token in tokens:
            if token in vulkan_metadata.types.structs and token != struct:
                sort_struts_impl(token, processed_structs, sorted_structs, vulkan_metadata)

    processed_structs[struct] = True
    sorted_structs += [struct]


def sort_struts(vulkan_metadata: types.VulkanMetadata) -> List[str] :
    processed_structs : Dict[str, bool] = {}
    sorted_structs : List[str] = []

    for struct in vulkan_metadata.types.structs:
        sort_struts_impl(struct, processed_structs, sorted_structs, vulkan_metadata)

    return sorted_structs


def remap_member_type(member_type: str, member_name: str, vulkan_metadata: types.VulkanMetadata) -> str:

    if member_name == "pNext":
        return "std::shared_ptr<VkStructFactory>"

    tokens = tokenize_typename(member_type)
    stripped_tokens: List[str] = []
    ret = ""

    for token in tokens:
        if token in vulkan_metadata.types.structs:
            stripped_tokens += [struct_factory_name(token)]
        elif token != "const" and token != "struct" and token != "conststruct":  # TODO: conststruct is to work around a bug.remove when fixed!
            stripped_tokens += [token]

    if len(stripped_tokens) == 1:
        return stripped_tokens[0]

    if len(stripped_tokens) == 2:
        if stripped_tokens[1] == "*":
            if stripped_tokens[0] == "char":
                return "std::string"
            elif stripped_tokens[0] == "void":
                return "void*"
            else:
                return "std::vector<" + stripped_tokens[0] + ">"

    if len(stripped_tokens) == 3:
        if stripped_tokens[1] == "*" and stripped_tokens[2] == "*":
            if stripped_tokens[0] == "char":
                return "std::vector<std::string>"
            elif stripped_tokens[0] == "void":
                return "void**"
            else:
                return "std::vector<std::vector<" + stripped_tokens[0] + ">>"

    raise BaseException("TYPE ERROR: Cannot remap type " + member_type)


def generate_struct_member_setter(member: types.VulkanStructMember, vulkan_metadata: types.VulkanMetadata) -> str:
    return codegen.create_function_declaration(set_member_name(member.variable_name),
                                               arguments={"val": "const " + remap_member_type(member.variable_type, member.variable_name, vulkan_metadata) + "&"})


def generate_struct_member_getter(member: types.VulkanStructMember, vulkan_metadata: types.VulkanMetadata) -> str:
    return codegen.create_function_declaration(get_member_name(member.variable_name), return_type="const " + remap_member_type(member.variable_type, member.variable_name, vulkan_metadata) + "&")


def process_struct_member(member: types.VulkanStructMember,
                          public_members: List[str],
                          private_members: List[str],
                          vulkan_metadata: types.VulkanMetadata):
    public_members += [generate_struct_member_setter(member, vulkan_metadata),
                       generate_struct_member_getter(member, vulkan_metadata)]
    private_members += [remap_member_type(member.variable_type, member.variable_name,
                                          vulkan_metadata) + " " + member.variable_name + "_;"]


def generate_struct_factories_h(file_path: Path, vulkan_metadata: types.VulkanMetadata):
    ''' Generates struct_factories.h '''
    with open(file_path, "w", encoding="ascii") as remapper_h:

        remapper_h.write(codegen.generated_license_header())

        remapper_h.write(dedent("""

            #include <memory>
            #include <string>
            #include <vector>

            // Temporary typedefs for window system stuff to let us compile.
            typedef uint64_t wl_display;
            typedef uint64_t wl_surface;
            typedef uint64_t HINSTANCE;
            typedef uint64_t HWND;
            typedef uint64_t Display;
            typedef uint64_t Window;
            typedef uint64_t xcb_connection_t;
            typedef uint64_t xcb_window_t;
            typedef uint64_t IDirectFB;
            typedef uint64_t IDirectFBSurface;
            typedef uint64_t zx_handle_t;
            typedef uint64_t GgpStreamDescriptor;
            typedef uint64_t _screen_context;
            typedef uint64_t _screen_window;
            typedef uint64_t HANDLE;
            typedef uint64_t SECURITY_ATTRIBUTES;
            typedef uint64_t DWORD;
            typedef uint64_t LPCWSTR;
            typedef uint64_t GgpFrameToken;
            typedef uint64_t HMONITOR;

            // Temporary typedefs for vulkan union stuff to let us compile.
            typedef uint64_t VkClearValue;
            typedef uint64_t VkPerformanceValueDataINTEL;
            typedef uint64_t VkPipelineExecutableStatisticValueKHR;
            typedef uint64_t VkClearColorValue;
            typedef uint64_t VkDeviceOrHostAddressConstKHR;
            typedef uint64_t VkAccelerationStructureGeometryDataKHR;
            typedef uint64_t VkDeviceOrHostAddressKHR;
            typedef uint64_t StdVideoH264ProfileIdc;
            typedef uint64_t StdVideoH264Level;
            typedef uint64_t StdVideoH264SequenceParameterSet;
            typedef uint64_t StdVideoH264PictureParameterSet;
            typedef uint64_t StdVideoDecodeH264PictureInfo;
            typedef uint64_t StdVideoDecodeH264ReferenceInfo;
            typedef uint64_t StdVideoDecodeH264Mvc;
            typedef uint64_t StdVideoH265ProfileIdc;
            typedef uint64_t StdVideoH265Level;
            typedef uint64_t StdVideoH265VideoParameterSet;
            typedef uint64_t StdVideoH265SequenceParameterSet;
            typedef uint64_t StdVideoDecodeH265PictureInfo;
            typedef uint64_t StdVideoDecodeH265ReferenceInfo;
            typedef uint64_t StdVideoEncodeH264RefMemMgmtCtrlOperations;
            typedef uint64_t StdVideoEncodeH264SliceHeader;
            typedef uint64_t StdVideoH265PictureParameterSet;
            typedef uint64_t StdVideoEncodeH265PictureInfo;
            typedef uint64_t StdVideoEncodeH265SliceSegmentHeader;
            typedef uint64_t StdVideoEncodeH264ReferenceInfo;
            typedef uint64_t StdVideoEncodeH264PictureInfo;
            typedef uint64_t StdVideoEncodeH265ReferenceModifications;
            typedef uint64_t VkAccelerationStructureMotionInstanceDataNV;
            typedef uint64_t StdVideoEncodeH265ReferenceInfo;

            namespace agi {
            namespace replay2 {

            class VkStructFactory {};

        """))

        for type_name in vulkan_metadata.types.basetypes:
            remapper_h.write("typedef uint64_t " + type_name + ";\n")
        remapper_h.write("\n")

        for type_name in vulkan_metadata.types.handles:
            remapper_h.write("typedef uint64_t " + type_name + ";\n")
        remapper_h.write("\n")

        for type_name in vulkan_metadata.types.handle_aliases:
            remapper_h.write("typedef uint64_t " + type_name + ";\n")
        remapper_h.write("\n")

        for type_name in vulkan_metadata.types.funcpointers:
            remapper_h.write("typedef uint64_t " + type_name + ";\n")
        remapper_h.write("\n")

        for type_name in vulkan_metadata.types.bitmasks:
            remapper_h.write("typedef uint64_t " + type_name + ";\n")
        remapper_h.write("\n")

        for type_name in vulkan_metadata.types.bitmask_aliases:
            remapper_h.write("typedef uint64_t " + type_name + ";\n")
        remapper_h.write("\n")

        for type_name in vulkan_metadata.types.enums:
            remapper_h.write("typedef uint64_t " + type_name + ";\n")
        remapper_h.write("\n")

        for type_name in vulkan_metadata.types.enum_aliases:
            remapper_h.write("typedef uint64_t " + type_name + ";\n")
        remapper_h.write("\n")

        all_vulkan_structs = vulkan_metadata.types.structs
        for struct in sort_struts(vulkan_metadata):

            public_members: List[str] = []
            private_members: List[str] = []

            for member in all_vulkan_structs[struct].members:
                process_struct_member(all_vulkan_structs[struct].members[member],
                                      public_members, private_members, vulkan_metadata)

            class_def = codegen.create_class_definition(struct_factory_name(struct),
                                                        public_inheritance=["VkStructFactory"],
                                                        public_members=public_members,
                                                        private_members=private_members)

            remapper_h.write(class_def)
            remapper_h.write("\n")

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
            TEST(VulkanStructFactories, Pass) {{
                EXPECT_TRUE(true);
            }}"""))

        tests_cpp.write(dedent("""
        int main(int argc, char **argv) {
            ::testing::InitGoogleTest(&argc, argv);
            return RUN_ALL_TESTS();
        }
        """))
