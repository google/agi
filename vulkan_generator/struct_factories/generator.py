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
    remapped_type = remap_member_type(member, vulkan_info)
    if "*" in remapped_type:
        return remapped_type
    else:
        return "const " + remapped_type + "&"


def get_member_name(member: str) -> str:
    return "get" + member[0:1].upper() + member[1:]


def get_member_return_type(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    remapped_type = remap_member_type(member, vulkan_info)
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

    #TODO: Move these to some common location for the whole project.
    # This whole function should move to that location, so we can ask what to generate.
    supported_versions : List[str] = ["VK_VERSION_1_0"]
    supported_extensions : List[str] = [] # TODO

    # VkBaseInStructure and VkBaseOutStructure are not used directly as complete types.
    # We'll ignore them by default.
    ignore_structs : List[str] = ["VkBaseInStructure", "VkBaseOutStructure"]

    for struct in vulkan_info.types.structs:
        supported : bool = False
        for version in supported_versions:
            features = vulkan_info.core_versions[version].features
            if struct in features.types or struct in features.type_aliases:
                supported = True
        if not supported:
            ignore_structs += [struct]

    return ignore_structs


def sort_structs_dep_order_impl(struct :str,
                     processed_structs : Dict[str, bool],
                     sorted_structs : List[str],
                     vulkan_info: types.VulkanInfo) :

    #TODO: Move these to some common location for the whole project.
    # This whole function should move to that location, so we can ask what to generate.

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

    #TODO: Move these to some common location for the whole project.
    # This whole function should move to that location, so we can ask what to generate.

    processed_structs : Dict[str, bool] = {}
    sorted_structs : List[str] = []

    for struct in vulkan_info.types.structs:
        sort_structs_dep_order_impl(struct, processed_structs, sorted_structs, vulkan_info)

    return sorted_structs


def remap_member_type(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:

    member_type = member.full_typename
    member_name = member.name

    if member_name == "pNext":
        return "std::shared_ptr<VkStructFactory>"

    if member.size and isinstance(member.size, list):
        return f"""std::array<{member.type.typename}, {str(member.size).replace("[", "").replace("]", "")}>""" #TODO: size

    if member.size and isinstance(member.size, types.VulkanDefine):
        return f"""std::array<{member.type.typename}, {member.size.name}>"""

    tokens = tokenize_typename(member_type)
    stripped_tokens: List[str] = []
    ret = ""

    for token in tokens:
        if token in vulkan_info.types.structs:
            stripped_tokens += [struct_factory_name(token)]
        elif token != "const" and token != "struct": 
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

    raise BaseException("remap_member_type(): cannot remap type " + member_type)


def generate_struct_member_setter(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    return codegen.create_function_declaration(set_member_name(member.name),
                                               arguments={"val": set_member_arg_type(member, vulkan_info)})


def generate_struct_member_getter(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    return codegen.create_function_declaration(get_member_name(member.name),
                                               return_type=get_member_return_type(member, vulkan_info),
                                               const_func=True)


def generate_struct_member(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:
    return remap_member_type(member, vulkan_info) + " " + member.name + "_;"


def generate_struct_factories_h(file_path: Path, vulkan_info: types.VulkanInfo):

    ''' Generates struct_factories.h '''
    with open(file_path, "w", encoding="ascii") as remapper_h:

        remapper_h.write(codegen.generated_license_header())

        remapper_h.write(dedent("""

            #include "replay2/struct_factories/vulkan/vulkan.h"
            #include "replay2/replay_context/replay_context.h"

            #include <memory>
            #include <string>
            #include <vector>
            #include <array>

            namespace agi {
            namespace replay2 {

            class VkStructSidecar {
            public:

                typedef uint64_t ALIGN_TYPE;

                VkStructSidecar() : scratchPtr_(nullptr) {}
                ~VkStructSidecar() {}

                void reset() {
                    scratch_.resize(0);
                }

                template<typename T>
                T* allocateSidecarData(size_t elements) {
                    size_t new_data = (sizeof(T) *elements +sizeof(ALIGN_TYPE) -1) /sizeof(ALIGN_TYPE);
                    if(scratchPtr_ != nullptr) {
                        scratchPtr_->resize(scratchPtr_->size() +new_data);
                        return ((T*)(&(*scratchPtr_)[scratchPtr_->size() -1 -new_data]));
                    }
                    else {
                        scratch_.resize(scratch_.size() +new_data);
                        return ((T*)(&scratch_[scratch_.size() -1 -new_data]));
                    }
                }

                std::unique_ptr<VkStructSidecar> CreateSubordinate() {
                    std::unique_ptr<VkStructSidecar> ret(new VkStructSidecar);
                    ret->scratchPtr_ = &scratch_;
                    return ret;
                }

                bool isSubordinate() const { return scratchPtr_ != nullptr; }

            private:
                std::vector<ALIGN_TYPE> *scratchPtr_;
                std::vector<ALIGN_TYPE> scratch_;
            };


            class VkStructFactory {
            public:
                virtual ~VkStructFactory() {}
                virtual size_t VkStructMemorySize() = 0;
                virtual VkBaseInStructure* GenerateAsPNext(const ReplayContext& context, std::unique_ptr<VkStructSidecar> sidecar) = 0;
            };

        """))

        all_vulkan_structs = vulkan_info.types.structs
        ignore_structs = generate_ignore_structs(vulkan_info)
        sorted_structs = sort_structs_dep_order(vulkan_info)

        for struct in sorted_structs:
            if not struct in ignore_structs:

                remapper_h.write(dedent(f"""
                    class Generated{struct} {{
                    public:
                        Generated{struct}(std::unique_ptr<VkStructSidecar> sidecar) {{ sidecar_ = std::move(sidecar); }}
                        {struct}& object() {{ return object_; }}

                    protected:
                        template<typename T> T* allocateSidecarData(size_t elements) {{ return sidecar_->allocateSidecarData<T>(elements); }}
                        std::unique_ptr<VkStructSidecar> CreateSubordinateSidecar() {{ return sidecar_->CreateSubordinate(); }}

                    private:
                        std::unique_ptr<VkStructSidecar> sidecar_;
                        {struct} object_;

                        friend class {struct}Factory;
                    }};

                """))

        for struct in sorted_structs:
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
                                   "",
                                   f"""Generated{struct} Generate(const ReplayContext& context);""",
                                   f"""Generated{struct} Generate(const ReplayContext& context, std::unique_ptr<VkStructSidecar> sidecar);""",
                                   f"""VkBaseInStructure* GenerateAsPNext(const ReplayContext& context, std::unique_ptr<VkStructSidecar> sidecar) override;"""]

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


def get_derived_trivial_types(trivial_types : List[str], vulkan_info : types.VulkanInfo) -> List[str]:

    derived_trivial_types : List[str] = []
    for base_type in vulkan_info.types.basetypes:
        real_type = vulkan_info.types.basetypes[base_type].basetype
        if real_type:
            if real_type in trivial_types:
                if not base_type in derived_trivial_types:
                    derived_trivial_types += [base_type]

    return derived_trivial_types


def get_derived_pointer_types(vulkan_info : types.VulkanInfo) -> List[str]:

    derived_pointer_types : List[str] = []
    for base_type in vulkan_info.types.basetypes:
        real_type = vulkan_info.types.basetypes[base_type].basetype
        if real_type:
            if "*" in real_type:
                if not real_type in derived_pointer_types:
                    derived_pointer_types += [real_type]

    return derived_pointer_types


def zero_value_for_type(mapped_type : str, member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:

    expected_value = member.expected_value
    if expected_value:
        return expected_value.name

    trivial_types = ["int64_t", "uint64_t", "int32_t", "uint32_t", "int16_t", "uint16_t", "int8_t", "uint8_t", "char", "bool", "size_t", "float"]
    if mapped_type in trivial_types:
        return "0"

    derived_trivial_types = get_derived_trivial_types(trivial_types, vulkan_info)
    if mapped_type in derived_trivial_types:
        return "0"

    if mapped_type.endswith("Factory"):
        return ""
    if mapped_type.startswith("std::vector<"):
        return ""

    if mapped_type.startswith("std::array<"):
        return "{}"

    if mapped_type.startswith("std::string"):
        return "\"\""

    if mapped_type in vulkan_info.types.handles or mapped_type in vulkan_info.types.handle_aliases:
        return "VK_NULL_HANDLE"

    if mapped_type in vulkan_info.types.enums or mapped_type in vulkan_info.types.enum_aliases:
        return f"""(({mapped_type})0)"""

    if mapped_type in vulkan_info.types.bitmasks or mapped_type in vulkan_info.types.bitmask_aliases:
        return "0"

    if mapped_type in vulkan_info.types.unions:
        raise Exception("zero_value_for_type() cannot provide zero values for vk unions. Please use memset().")

    if mapped_type == "void*":
        return "nullptr"
    if mapped_type == "void**":
        return "nullptr"
    if mapped_type.startswith("std::shared_ptr<"):
        return "nullptr"

    derived_pointer_types = get_derived_pointer_types(vulkan_info)
    if mapped_type in derived_pointer_types:
        return "nullptr"

    if mapped_type in vulkan_info.types.funcpointers:
        return "nullptr"

    raise Exception("Cannot create zero value for unknown type: " + mapped_type + " (name: " + member.name + ")")


def set_member_to_zero_value(member: types.VulkanStructMember, vulkan_info: types.VulkanInfo) -> str:

    unmapped_type = member.full_typename
    mapped_type = remap_member_type(member, vulkan_info)
    if mapped_type in vulkan_info.types.unions:
        #TODO: Is this the right thing to do with unions? Maybe we should pick one value type and zero that one?
        return f"""{codegen.indent_characters()}memset(&{member.name}_, 0, sizeof({mapped_type}));""" 
    else:
        zero_value = zero_value_for_type(mapped_type, member, vulkan_info)
        if zero_value != "":
            return f"""{codegen.indent_characters()}{member.name}_ = {zero_value};"""
        else:
            return f"""{codegen.indent_characters()}//Using default constructor for this.{member.name}_""" 


def generate_factory_ctor_def(struct : str, vulkan_info: types.VulkanInfo) -> str :
    head = dedent(f"""
                  {struct_factory_name(struct)}::{struct_factory_name(struct)}() {{
                  """)

    struct_data = vulkan_info.types.structs[struct]
    middle = ""
    for member in struct_data.members:
        member_data = struct_data.members[member]
        middle += set_member_to_zero_value(member_data, vulkan_info) +"\n"

    tail = dedent(f"""}}
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


def generate_factory_argless_generate_def(struct : str, vulkan_info: types.VulkanInfo) -> str :

    return dedent(f"""
                    Generated{struct} {struct_factory_name(struct)}::Generate(const ReplayContext& context) {{
                        std::unique_ptr<VkStructSidecar> sidecar(new VkStructSidecar());
                        return Generate(context, std::move(sidecar));
                    }}""")

def generate_factory_generate_def(struct : str, vulkan_info: types.VulkanInfo) -> str :

    head = dedent(f"""
                  Generated{struct} {struct_factory_name(struct)}::Generate(const ReplayContext& context, std::unique_ptr<VkStructSidecar> sidecar) {{
                      Generated{struct} ret(std::move(sidecar));
                  """)

    struct_data = vulkan_info.types.structs[struct]
    middle = ""

    for member in struct_data.members:

        member_data = struct_data.members[member]

        struct_data = member_data.parent
        unmapped_type = member_data.full_typename
        mapped_type = remap_member_type(member_data, vulkan_info)

        middle += f"""\n{codegen.indent_characters()}"""

        if member == "pNext":
            middle += f"""ret.object().{member} = {member}_->GenerateAsPNext(context, ret.CreateSubordinateSidecar());"""
        elif mapped_type.endswith("Factory"):
            middle += f"""auto Generated{member} = {member}_.Generate(context, ret.CreateSubordinateSidecar());\n"""
            middle += f"""ret.object().{member} = Generated{member}.object();""";
        elif mapped_type.startswith("std::vector<"):
            vector_type = mapped_type[len("std::vector<"): -len(">")]
            if vector_type.startswith("std::vector<"):
                raise Exception("generate_factory_generate_def() does not yet implement vectors of vectors.")
            elif vector_type == "std::string":
                middle += f"""char **{member}Scratch = ret.allocateSidecarData<char*>({member}_.size());\n"""
                middle += f"""for(int i = 0; i < {member}_.size(); ++i){{\n"""
                middle += f"""{member}Scratch[i] = {member}_[i].data();\n"""
                middle += f"""}}\n"""
                middle += f"""ret.object().{member} = {member}Scratch;"""
            elif vector_type.endswith("Factory"):
                element_type = vector_type[0:-len("Factory")]
                middle += f"""{element_type} *{member}Scratch = ret.allocateSidecarData<{element_type}>({member}_.size());\n"""
                middle += f"""for(int i = 0; i < {member}_.size(); ++i){{\n"""
                middle += f"""auto Generated{member} = {member}_[i].Generate(context, ret.CreateSubordinateSidecar());\n"""
                middle += f"""{member}Scratch[i] = Generated{member}.object();//HELLO\n"""
                middle += f"""}}\n"""
                middle += f"""ret.object().{member} = {member}Scratch;"""
            else:
                middle += f"""ret.object().{member} = {member}_.data();"""
        elif mapped_type.startswith("std::array<"):
            if member_data.size and isinstance(member_data.size, list):
                middle += f"""memcpy(&ret.object().{member}, {member}_.data(), {str(member_data.size).replace("[", "").replace("]", "")} * sizeof({unmapped_type}));""" #TODO: size
            elif member_data.size and isinstance(member_data.size, types.VulkanDefine):
                middle += f"""memcpy(&ret.object().{member}, {member}_.data(), {member_data.size.name} * sizeof({unmapped_type}));"""
            else:
                raise Exception("")
        elif mapped_type.startswith("std::shared_ptr<"):
            middle += f"""ret.object().{member} = {member}_.get();"""
        elif mapped_type.startswith("std::string"):
            middle += f"""ret.object().{member} = {member}_.c_str();"""
        elif mapped_type in vulkan_info.types.handles or mapped_type in vulkan_info.types.handle_aliases:
            middle += f"""ret.object().{member} = {member}_;/* TODO: REMAP THIS VALUE */"""
        else:
            middle += f"""ret.object().{member} = {member}_;"""

        middle += f"""   /*|unmapped_type = {unmapped_type}| mapped_type = {mapped_type}|*/"""

    tail = dedent(f"""
                      return ret;
                  }}
                  """)

    return head + middle + tail


def generate_factory_pnext_generate_def(struct : str, vulkan_info: types.VulkanInfo) -> str :

    return dedent(f"""
                    VkBaseInStructure* {struct_factory_name(struct)}::GenerateAsPNext(const ReplayContext& context, std::unique_ptr<VkStructSidecar> sidecar) {{
                        {struct} *{struct}Ptr = sidecar->allocateSidecarData<{struct}>(sizeof({struct}));
                        Generated{struct} pNext = Generate(context, sidecar->CreateSubordinate());
                        *{struct}Ptr = pNext.object();
                        return ((VkBaseInStructure*){struct}Ptr);
                    }}""")


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
        sorted_structs = sort_structs_dep_order(vulkan_info)
        
        for struct in sorted_structs:
            if not struct in ignore_structs:

                remapper_cpp.write(generate_factory_ctor_def(struct, vulkan_info))
                remapper_cpp.write(generate_factory_dtor_def(struct, vulkan_info))

                for member in all_vulkan_structs[struct].members:
                    member_data = all_vulkan_structs[struct].members[member]
                    remapper_cpp.write(generate_factory_setter_def(struct, member, member_data, vulkan_info))
                    remapper_cpp.write(generate_factory_getter_def(struct, member, member_data, vulkan_info))

                remapper_cpp.write(generate_factory_size_def(struct))
                remapper_cpp.write(generate_factory_argless_generate_def(struct, vulkan_info))
                remapper_cpp.write(generate_factory_generate_def(struct, vulkan_info))
                remapper_cpp.write(generate_factory_pnext_generate_def(struct, vulkan_info))
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
