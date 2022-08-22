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
from vulkan_generator.vulkan_parser.api import types
from vulkan_generator.codegen_utils import codegen

def struct_factory_name(struct: str) -> str:
    return struct + "Factory"


def set_member_name(member: str) -> str:
    return "set" + member[0:1].upper() + member[1:]


def set_member_arg_type(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    remapped_type = remap_member_type(member.full_typename, member.name, vulkan_info)
    if "*" in remapped_type:
        return remapped_type
    else:
        return "const " + remapped_type + "&"


def get_member_name(member: str) -> str:
    return "get" + member[0:1].upper() + member[1:]


def get_member_return_type(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    remapped_type = remap_member_type(member.full_typename, member.name, vulkan_info)
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


def generate_ignore_structs(vulkan_info : types.VulkanInfo) -> List[str]:
    ignore_structs : List[str] = []

    for extension_name in vulkan_info.extensions:
        extension = vulkan_info.extensions[extension_name]

        if extension.platform and extension.platform != "":

            for requirement in extension.requirements:

                for feature_name in requirement.features.types:
                    ignore_structs += [feature_name]

    return ignore_structs


def sort_structs_dep_order_impl(struct :str,
                     processed_structs : Dict[str, bool],
                     sorted_structs : List[str],
                     vulkan_info: types.VulkanInfo) :

    if struct in processed_structs:
        return

    for member in vulkan_info.types.structs[struct].members:
        tokens = tokenize_typename(vulkan_info.types.structs[struct].members[member].full_typename)
        for token in tokens:
            if token in vulkan_info.types.structs and token != struct:
                sort_structs_dep_order_impl(token, processed_structs, sorted_structs, vulkan_info)

    processed_structs[struct] = True
    sorted_structs += [struct]


def sort_structs_dep_order(vulkan_info: types.VulkanInfo) -> List[str] :
    processed_structs : Dict[str, bool] = {}
    sorted_structs : List[str] = []

    for struct in vulkan_info.types.structs:
        sort_structs_dep_order_impl(struct, processed_structs, sorted_structs, vulkan_info)

    return sorted_structs


def remap_member_type(member_type: str, member_name: str, vulkan_info: types.VulkanInfo) -> str:

    if member_name == "pNext":
        return "std::shared_ptr<VkStructFactory>"

    tokens = tokenize_typename(member_type)
    stripped_tokens: List[str] = []
    ret = ""

    for token in tokens:
        if token in vulkan_info.types.structs:
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


def generate_struct_member_setter(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    return codegen.create_function_declaration(set_member_name(member.name),
                                               arguments={"val": set_member_arg_type(member, vulkan_info)})


def generate_struct_member_getter(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    return codegen.create_function_declaration(get_member_name(member.name),
                                               return_type=get_member_return_type(member, vulkan_info),
                                               const_func=True)


def generate_struct_member(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    return remap_member_type(member.full_typename, member.name,
                             vulkan_info) + " " + member.name + "_;"


def generate_struct_factories_h(file_path: Path, vulkan_info: types.VulkanInfo):

    ''' Generates struct_factories.h '''
    with open(file_path, "w", encoding="ascii") as remapper_h:

        remapper_h.write(codegen.generated_license_header())

        remapper_h.write(dedent("""

            #include <memory>
            #include <string>
            #include <vector>

            #include "replay2/struct_factories/vulkan/vulkan.h"

            namespace agi {
            namespace replay2 {

            class VkStructFactory {
            public:
                virtual ~VkStructFactory() {}
                virtual size_t VkStructMemorySize() = 0;
                virtual size_t PNextChainMemorySize() = 0;
            };

        """))

        all_vulkan_structs = vulkan_info.types.structs
        ignore_structs = generate_ignore_structs(vulkan_info)

        for struct in sort_structs_dep_order(vulkan_info):
            if not struct in ignore_structs:

                public_members: List[str] = [f"""{struct_factory_name(struct)}();""",
                                             f"""virtual ~{struct_factory_name(struct)}();"""
                                             "\n"]

                private_members: List[str] = []

                for member in all_vulkan_structs[struct].members:
                    member_data = all_vulkan_structs[struct].members[member]
                    public_members += [generate_struct_member_setter(member_data, vulkan_info),
                                       generate_struct_member_getter(member_data, vulkan_info)]
                    private_members += [generate_struct_member(member_data, vulkan_info)]

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

#for handle in vulkan_metadata.types.handles:
def zero_value_for_type(type : str, vulkan_info : types.VulkanInfo) -> str:
    if type.endswith("Factory"):
        return ""
    if type.startswith("std::vector<"):
        return ""
    if type.startswith("std::shared_ptr<"):
        return "nullptr"
    if type.startswith("std::string"):
        return "\"\""
    if type == "VkStructureType":
        return "VK_STRUCTURE_TYPE_APPLICATION_INFO"
    if type in vulkan_info.types.handles or type in vulkan_info.types.handle_aliases:
        return "VK_NULL_HANDLE"
    if type in vulkan_info.types.enums or type in vulkan_info.types.enum_aliases:
        return f"""(({type})0)"""
    if type in vulkan_info.types.unions:
        return "" #TODO
    return "0"


def generate_factory_ctor_def(struct : str, vulkan_info: types.VulkanInfo) -> str :
    head = dedent(f"""
                  {struct_factory_name(struct)}::{struct_factory_name(struct)}() {{""")

    struct_data = vulkan_info.types.structs[struct]
    middle = ""
    for member in struct_data.members:
        unmapped_type = struct_data.members[member].full_typename
        mapped_type = remap_member_type(unmapped_type, member, vulkan_info)
        if mapped_type in vulkan_info.types.unions:
            middle += f"""\n{codegen.indent_characters()}memset(&{member}_, 0, sizeof({mapped_type}));""" 
        else:
            zero_value = zero_value_for_type(mapped_type, vulkan_info)
            print(mapped_type + " -> " + zero_value)
            if zero_value != "":
                middle += f"""\n{codegen.indent_characters()}{member}_ = {zero_value};""" 

    tail = dedent(f"""
                  }}
                  """)

    return head + middle + tail


def generate_factory_dtor_def(struct : str, vulkan_info: types.VulkanInfo) -> str :
    return dedent(f"""
                  {struct_factory_name(struct)}::~{struct_factory_name(struct)}() {{
                  }}

                  """)


def generate_factory_setter_def(struct : str,
                                member : str,
                                member_data: types.VulkanStructMember,
                                vulkan_info: types.VulkanInfo) -> str :
    return dedent(f"""
                  void {struct_factory_name(struct)}::{set_member_name(member)}({set_member_arg_type(member_data, vulkan_info)} val) {{
                      {member}_ = val;
                  }}
                  """)


def generate_factory_getter_def(struct : str,
                                member : str,
                                member_data: types.VulkanStructMember,
                                vulkan_info: types.VulkanInfo) -> str :
    return dedent(f"""
                  {get_member_return_type(member_data, vulkan_info)} {struct_factory_name(struct)}::{get_member_name(member)}() const {{
                      return {member}_;
                  }}
                  """)


def generate_factory_size_def(struct : str) -> str :
    return dedent(f"""
                  size_t {struct_factory_name(struct)}::VkStructMemorySize() {{
                      return sizeof({struct});
                  }}
                  """)


def generate_factory_chain_size_def(struct : str, vulkan_info: types.VulkanInfo) -> str :
    if "pNext" in vulkan_info.types.structs[struct].members:
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
                      //    This involves recursing though the factory's pNext_ (if it exists as a member)
                      //    and calling generate on that nested factory and its pNext_->pNext_->... as well,
                      //    generating all vulkan structs in the pNext chain. 
                      //    VkStructFactory::PNextChainMemorySize() will tell you how much memory
                      //    will be needed to store all of these generated structures. This memory will
                      //    need to be allocated (or taken from a pool in the replay context).
                      //    The actual vulkan pNext fields will need to point into this allocation
                      //    after the appropriate data has been generated into that allocation.
                      return ret;
                  }}
                  """)


def generate_struct_factories_cpp(file_path: Path, vulkan_info: types.VulkanInfo):
    ''' Generates struct_factories.cc '''
    with open(file_path, "w", encoding="ascii") as remapper_cpp:

        remapper_cpp.write(codegen.generated_license_header())

        remapper_cpp.write(dedent("""
            #include "struct_factories.h"

            namespace agi {
            namespace replay2 {

        """))

        all_vulkan_structs = vulkan_info.types.structs
        ignore_structs = generate_ignore_structs(vulkan_info)

        for struct in sort_structs_dep_order(vulkan_info):
            if not struct in ignore_structs:

                remapper_cpp.write(generate_factory_ctor_def(struct, vulkan_info))
                remapper_cpp.write(generate_factory_dtor_def(struct, vulkan_info))

                for member in all_vulkan_structs[struct].members:
                    member_data = all_vulkan_structs[struct].members[member]
                    remapper_cpp.write(generate_factory_setter_def(struct, member, member_data, vulkan_info))
                    remapper_cpp.write(generate_factory_getter_def(struct, member, member_data, vulkan_info))

                remapper_cpp.write(generate_factory_size_def(struct))
                remapper_cpp.write(generate_factory_chain_size_def(struct, vulkan_info))
                remapper_cpp.write(generate_factory_generate_def(struct))
                remapper_cpp.write("\n")

        remapper_cpp.write(dedent("""
            }
            }

        """))


def generate_struct_factories_tests(file_path: Path, vulkan_info: types.VulkanInfo):
    ''' Generates struct_factories_tests.cc '''
    with open(file_path, "w", encoding="ascii") as tests_cpp:

        tests_cpp.write(codegen.generated_license_header())

        tests_cpp.write(dedent("""
            #include "struct_factories.h"
            #include <gtest/gtest.h>

            using namespace agi::replay2;
            """))

        tests_cpp.write(dedent("""
            TEST(VulkanStructFactories, Pass) {
                EXPECT_TRUE(true);
            }"""))

        all_vulkan_structs = vulkan_info.types.structs
        ignore_structs = generate_ignore_structs(vulkan_info)

        for struct in all_vulkan_structs:
            if not struct in ignore_structs:
                tests_cpp.write(dedent(f"""
                    TEST(VulkanStructFactories, DefaultConstruct_{struct}) {{
                        {struct} s;
                        EXPECT_TRUE(&s != nullptr);
                    }}"""))

        tests_cpp.write(dedent("""
        int main(int argc, char **argv) {
            ::testing::InitGoogleTest(&argc, argv);
            return RUN_ALL_TESTS();
        }
        """))
