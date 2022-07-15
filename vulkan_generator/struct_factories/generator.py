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


def set_member_arg_type(member: types.VulkanStructMember, vulkan_metadata: types.VulkanMetadata) -> str:
    remapped_type = remap_member_type(member.variable_type, member.variable_name, vulkan_metadata)
    if "*" in remapped_type:
        return remapped_type
    else:
        return "const " + remapped_type + "&"


def get_member_name(member: str) -> str:
    return "get" + member[0:1].upper() + member[1:]


def get_member_return_type(member: types.VulkanStructMember, vulkan_metadata: types.VulkanMetadata) -> str:
    remapped_type = remap_member_type(member.variable_type, member.variable_name, vulkan_metadata)
    if "*" in remapped_type:
        return remapped_type
    else:
        return "const " + remapped_type + "&"


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


def sort_structs_dep_order_impl(struct :str,
                     processed_structs : Dict[str, bool],
                     sorted_structs : List[str],
                     vulkan_metadata: types.VulkanMetadata) :

    if struct in processed_structs:
        return

    for member in vulkan_metadata.types.structs[struct].members:
        tokens = tokenize_typename(vulkan_metadata.types.structs[struct].members[member].variable_type)
        for token in tokens:
            if token in vulkan_metadata.types.structs and token != struct:
                sort_structs_dep_order_impl(token, processed_structs, sorted_structs, vulkan_metadata)

    processed_structs[struct] = True
    sorted_structs += [struct]


def sort_structs_dep_order(vulkan_metadata: types.VulkanMetadata) -> List[str] :
    processed_structs : Dict[str, bool] = {}
    sorted_structs : List[str] = []

    for struct in vulkan_metadata.types.structs:
        sort_structs_dep_order_impl(struct, processed_structs, sorted_structs, vulkan_metadata)

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
                                               arguments={"val": set_member_arg_type(member, vulkan_metadata)})


def generate_struct_member_getter(member: types.VulkanStructMember, vulkan_metadata: types.VulkanMetadata) -> str:
    return codegen.create_function_declaration(get_member_name(member.variable_name),
                                               return_type=get_member_return_type(member, vulkan_metadata),
                                               const_func=True)


def generate_struct_member(member: types.VulkanStructMember, vulkan_metadata: types.VulkanMetadata) -> str:
    return remap_member_type(member.variable_type, member.variable_name,
                             vulkan_metadata) + " " + member.variable_name + "_;"


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

            class VkStructFactory {
            public:
                virtual ~VkStructFactory() {}
                virtual size_t VkStructMemorySize() = 0;
                virtual size_t PNextChainMemorySize() = 0;
            };

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

        for struct in vulkan_metadata.types.structs:
            remapper_h.write("class " + struct + " {};\n")
        remapper_h.write("\n")

        all_vulkan_structs = vulkan_metadata.types.structs
        for struct in sort_structs_dep_order(vulkan_metadata):

            public_members: List[str] = [f"""{struct_factory_name(struct)}();""",
                                         f"""virtual ~{struct_factory_name(struct)}();"""
                                         "\n"]

            private_members: List[str] = []

            for member in all_vulkan_structs[struct].members:
                member_data = all_vulkan_structs[struct].members[member]
                public_members += [generate_struct_member_setter(member_data, vulkan_metadata),
                                   generate_struct_member_getter(member_data, vulkan_metadata)]
                private_members += [generate_struct_member(member_data, vulkan_metadata)]

            public_members += ["",
                               "size_t VkStructMemorySize() override;",
                               "size_t PNextChainMemorySize() override;",
                               "",
                               f"""{struct} Generate();"""]

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

def generate_factory_ctor_def(struct : str) -> str :
    return dedent(f"""
                  {struct_factory_name(struct)}::{struct_factory_name(struct)}() {{
                      //TODO: Zero out all plain ordinary member types
                  }}

                  """)


def generate_factory_dtor_def(struct : str) -> str :
    return dedent(f"""
                  {struct_factory_name(struct)}::~{struct_factory_name(struct)}() {{
                  }}

                  """)


def generate_factory_setter_def(struct : str,
                                member : str,
                                member_data: types.VulkanStructMember,
                                vulkan_metadata: types.VulkanMetadata) -> str :
    return dedent(f"""
                  void {struct_factory_name(struct)}::{set_member_name(member)}({set_member_arg_type(member_data, vulkan_metadata)} val) {{
                      {member}_ = val;
                  }}
                  """)


def generate_factory_getter_def(struct : str,
                                member : str,
                                member_data: types.VulkanStructMember,
                                vulkan_metadata: types.VulkanMetadata) -> str :
    return dedent(f"""
                  {get_member_return_type(member_data, vulkan_metadata)} {struct_factory_name(struct)}::{get_member_name(member)}() const {{
                      return {member}_;
                  }}
                  """)


def generate_factory_size_def(struct : str) -> str :
    return dedent(f"""
                  size_t {struct_factory_name(struct)}::VkStructMemorySize() {{
                      return sizeof({struct});
                  }}
                  """)


def generate_factory_chain_size_def(struct : str, vulkan_metadata: types.VulkanMetadata) -> str :
    if "pNext" in vulkan_metadata.types.structs[struct].members:
        return dedent(f"""
                      size_t {struct_factory_name(struct)}::PNextChainMemorySize() {{
                          if(pNext_ != nullptr) {{
                              size_t directNextSize = pNext_->VkStructMemorySize();
                              if(directNextSize % sizeof(void*) != 0) {{
                                  directNextSize += sizeof(void*) -(directNextSize % sizeof(void*));
                              }}
                              return directNextSize +pNext_->PNextChainMemorySize();
                          }}
                          return 0;
                      }}
                      """)
    else:
        return dedent(f"""
                      size_t {struct_factory_name(struct)}::PNextChainMemorySize() {{
                          return 0;
                      }}
                      """)


def generate_factory_generate_def(struct : str) -> str :
    return dedent(f"""
                  {struct} {struct_factory_name(struct)}::Generate() {{
                      {struct} ret;
                      //TODO: Populate the fields of ret here.
                      return ret;
                  }}
                  """)


def generate_struct_factories_cpp(file_path: Path, vulkan_metadata: types.VulkanMetadata):
    ''' Generates struct_factories.cc '''
    with open(file_path, "w", encoding="ascii") as remapper_cpp:

        remapper_cpp.write(codegen.generated_license_header())

        remapper_cpp.write(dedent("""
            #include "struct_factories.h"

            namespace agi {
            namespace replay2 {

        """))

        all_vulkan_structs = vulkan_metadata.types.structs
        for struct in sort_structs_dep_order(vulkan_metadata):

            remapper_cpp.write(generate_factory_ctor_def(struct))
            remapper_cpp.write(generate_factory_dtor_def(struct))

            for member in all_vulkan_structs[struct].members:
                member_data = all_vulkan_structs[struct].members[member]
                remapper_cpp.write(generate_factory_setter_def(struct, member, member_data, vulkan_metadata))
                remapper_cpp.write(generate_factory_getter_def(struct, member, member_data, vulkan_metadata))

            remapper_cpp.write(generate_factory_size_def(struct))
            remapper_cpp.write(generate_factory_chain_size_def(struct, vulkan_metadata))
            remapper_cpp.write(generate_factory_generate_def(struct))
            remapper_cpp.write("\n")

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
