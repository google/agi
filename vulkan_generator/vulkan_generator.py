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

from pathlib import Path
from pprint import PrettyPrinter
from vulkan_parser import parser as vulkan_parser
from vulkan_parser import type_parser

def print_vulkan_metaadata(vulkan_metadata: type_parser.AllVulkanTypes) -> None:
    """Prints all the vulkan information that is extracted"""

    pp = PrettyPrinter(depth=4)

    print("=== Vulkan Handles ===")
    pp.pprint(vulkan_metadata.handles)

    print("=== Vulkan Handle Aliases ===")
    pp.pprint(vulkan_metadata.handle_aliases)

    print("=== Vulkan Structs ===")
    pp.pprint(vulkan_metadata.structs)

    print("=== Vulkan Struct Aliases ===")
    pp.pprint(vulkan_metadata.struct_aliases)


def generate(vulkan_xml_path: Path) -> bool:
    """ Generator function """
    all_vulkan_types = vulkan_parser.parse(vulkan_xml_path)
    print_vulkan_metaadata(all_vulkan_types)
    return True
