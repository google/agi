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

"""This module is the entry point for the Vulkan parser that extracts information from Vulkan XML"""


from pathlib import Path
import sys

from vulkan_generator.vulkan_parser.api import types
from vulkan_generator.vulkan_parser.internal import parser as internal_parser
from vulkan_generator.vulkan_parser.postprocess import postprocess


def print_vulkan_metadata(vulkan_metadata: types.VulkanInfo) -> None:
    """Prints all the vulkan information that is extracted"""

    print(vulkan_metadata.platforms)
    print(vulkan_metadata.includes)
    print(vulkan_metadata.defines)

    vulkan_types = vulkan_metadata.types
    print(vulkan_types.handles)
    print(vulkan_types.handle_aliases)
    print(vulkan_types.bitmasks)
    print(vulkan_types.bitmask_aliases)
    print(vulkan_types.enums)
    print(vulkan_types.enum_aliases)
    print(vulkan_types.structs)
    print(vulkan_types.struct_aliases)
    print(vulkan_types.funcpointers)

    vulkan_commands = vulkan_metadata.commands
    print(vulkan_commands.commands)
    print(vulkan_commands.command_aliases)

    print(vulkan_metadata.core_versions)
    print(vulkan_metadata.extensions)
    print(vulkan_metadata.image_formats)

    spirv_metadata = vulkan_metadata.spirv_metadata
    print(spirv_metadata.extensions)
    print(spirv_metadata.capabilities)


def parse(filename: Path, dump: bool = False) -> types.VulkanInfo:
    metadata = postprocess.process(internal_parser.parse(filename))

    if dump:
        print_vulkan_metadata(metadata)

    return metadata


if __name__ == "__main__":
    if len(sys.argv) == 2:
        parse(Path(sys.argv[1]))
        sys.exit(0)

    if len(sys.argv) == 3:
        if sys.argv[2].strip().lower() == "dump":
            parse(Path(sys.argv[1]), True)
            sys.exit(0)

    print("Please use as <xml location> Optional[<dump>]")
