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

"""This is the top level point for Vulkan Code Generator"""

import os

from pathlib import Path
import pprint

from vulkan_generator.vulkan_parser import parser as vk_parser
from vulkan_generator.vulkan_parser.api import types
from vulkan_generator.handle_remapper import generator as handle_remapper_generator


def print_vulkan_metadata(vulkan_metadata: types.VulkanInfo) -> None:
    """Prints all the vulkan information that is extracted"""

    pretty_printer = pprint.PrettyPrinter(depth=4)

    print("=== Vulkan Platforms ===")
    pretty_printer.pprint(vulkan_metadata.platforms)

    print("=== Vulkan Includes ===")
    pretty_printer.pprint(vulkan_metadata.includes)

    print("=== Vulkan Defines ===")
    pretty_printer.pprint(vulkan_metadata.defines)

    vulkan_types = vulkan_metadata.types

    print("=== Vulkan External Types ===")
    pretty_printer.pprint(vulkan_types.external_types)

    print("=== Vulkan Bitmasks ===")
    pretty_printer.pprint(vulkan_types.bitmasks)

    print("=== Vulkan Bitmask Aliases ===")
    pretty_printer.pprint(vulkan_types.bitmask_aliases)

    print("=== Vulkan Enums ===")
    pretty_printer.pprint(vulkan_types.enums)

    print("=== Vulkan Enums Aliases ===")
    pretty_printer.pprint(vulkan_types.enum_aliases)

    print("=== Vulkan Handles ===")
    pretty_printer.pprint(vulkan_types.handles)

    print("=== Vulkan Handle Aliases ===")
    pretty_printer.pprint(vulkan_types.handle_aliases)

    print("=== Vulkan Structs ===")
    pretty_printer.pprint(vulkan_types.structs)

    print("=== Vulkan Struct Aliases ===")
    pretty_printer.pprint(vulkan_types.struct_aliases)

    print("=== Vulkan Function Pointers ===")
    pretty_printer.pprint(vulkan_types.funcpointers)

    vulkan_commands = vulkan_metadata.commands

    print("=== Vulkan Commands ===")
    pretty_printer.pprint(vulkan_commands.commands)

    print("=== Vulkan Command Aliases ===")
    pretty_printer.pprint(vulkan_commands.command_aliases)

    versions = vulkan_metadata.core_versions
    print("=== Vulkan 1.0 ===")
    pretty_printer.pprint(versions["VK_VERSION_1_0"])

    print("=== Vulkan 1.1 ===")
    pretty_printer.pprint(versions["VK_VERSION_1_1"])

    print("=== Vulkan 1.2 ===")
    pretty_printer.pprint(versions["VK_VERSION_1_2"])

    print("=== Vulkan 1.3 ===")
    pretty_printer.pprint(versions["VK_VERSION_1_3"])

    extension = vulkan_metadata.extensions
    print("=== Vulkan Extensions ===")
    pretty_printer.pprint(extension)

    print("=== Vulkan Image Formats ===")
    pretty_printer.pprint(vulkan_metadata.image_formats)

    spirv_metadata = vulkan_metadata.spirv_metadata

    print("=== Spirv Extensions ===")
    pretty_printer.pprint(spirv_metadata.extensions)

    print("=== Spirv Capabilities ===")
    pretty_printer.pprint(spirv_metadata.capabilities)


def basic_generate(target: str,
                   output_dir: Path,
                   all_vulkan_types: types.VulkanInfo,
                   generate_header,
                   generate_cpp,
                   generate_test):

    generate_header(os.path.join(output_dir, target + ".h"), all_vulkan_types)
    generate_cpp(os.path.join(output_dir, target + ".cc"), all_vulkan_types)
    generate_test(os.path.join(output_dir, target + "_tests.cc"), all_vulkan_types)


def generate(target: str, output_dir: Path, vulkan_xml_path: Path) -> bool:
    """ Generator function """
    vulkan_metadata = vk_parser.parse(vulkan_xml_path)
    print_vulkan_metadata(vulkan_metadata)

    if output_dir == "":
        print("No output directory specified. Not generating anything.")
    elif target == "":
        print("No generate target specified. Not generating anything.")
    else:
        os.makedirs(output_dir, exist_ok=True)

        # Switch table for generate target. Add new targets here and throw exception for unknown targets
        if target == "handle_remapper":
            basic_generate(target, output_dir, vulkan_metadata,
                           handle_remapper_generator.generate_handle_remapper_h,
                           handle_remapper_generator.generate_handle_remapper_cpp,
                           handle_remapper_generator.generate_handle_remapper_tests)
        else:
            raise Exception("unknown generate target: " + target)

    return True
