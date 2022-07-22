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

""" This module is responsible for parsing Vulkan function pointers"""

from typing import Dict
import xml.etree.ElementTree as ET

from vulkan_generator.vulkan_parser.internal import internal_types
from vulkan_generator.vulkan_parser.internal import parser_utils


def parse_arguments(function_ptr_elem: ET.Element) -> Dict[str, internal_types.VulkanFunctionArgument]:
    """Parses the arguments of a Vulkan Function Pointer"""
    arguments: Dict[str, internal_types.VulkanFunctionArgument] = {}

    # In the XML const modifier of the type is part of the
    # previous argument of the function
    # <type>int32_t</type> messageCode,
    # const <type> char </type> * pLayerPrefix,
    is_next_type_const = False
    for elem in function_ptr_elem:
        # Skip the name tag
        if elem.tag != "type":
            continue

        if not elem.text:
            raise SyntaxError(f"Argument type could not found: {ET.tostring(elem, 'utf-8')!r}")

        if not elem.tail:
            raise SyntaxError(f"Argument name could not found: {ET.tostring(elem, 'utf-8')!r}")

        argument_type = parser_utils.clean_type_string(elem.text)
        argument_name = parser_utils.clean_type_string(elem.tail)

        # Multiple const are not supported
        if argument_name.count("const") > 1:
            raise SyntaxError(f"Double const are not supported: {argument_type} {argument_name}")

        # Multiple pointers are not supported
        if argument_name.count("*") > 1:
            raise SyntaxError(f"Double pointers are not supported: {argument_type} {argument_name}")

        # This means previous argument has the const modifier for this type
        if is_next_type_const:
            argument_type = f"const {argument_type}"
            is_next_type_const = False

        if "const" in argument_name:
            if not argument_name.endswith("const"):
                raise SyntaxError(f"""This is probably a const pointer which is not supported:
                    {argument_type} {argument_name}""")

            is_next_type_const = True
            argument_name = argument_name.replace("const", "")

        # Pointers of the type is actually in the argument name
        if "*" in argument_name:
            argument_type = argument_type + "*"
            argument_name = argument_name[1:]

        arguments[argument_name] = internal_types.VulkanFunctionArgument(
            argument_type=argument_type,
            argument_name=argument_name,
        )

    return arguments


def parse(func_ptr_elem: ET.Element) -> internal_types.VulkanFunctionPtr:
    """Returns a Vulkan function pointer from the XML element that defines it.

    A sample Vulkan function_pointer:
    < type category="funcpointer" > typedef void(VKAPI_PTR *
    <name > PFN_vkInternalAllocationNotification < /name > )(
    < type > void < /type > *                                       pUserData,
    < type > size_t < /type > size,
    < type > VkInternalAllocationType < /type > allocationType,
    < type > VkSystemAllocationScope < /type > allocationScope); < /type >
    """

    function_name = parser_utils.get_text_from_tag_in_children(func_ptr_elem, "name")

    # Return type is in the type tag's text field with some extra information
    # e.g typedef void (VKAPI_PTR *
    return_type = func_ptr_elem.text
    if not return_type:
        raise SyntaxError(f"No return type found for the function pointer: {ET.tostring(func_ptr_elem, 'utf-8')!r}")

    # remove the function pointer boilers around type
    return_type = return_type.split("(")[0]
    return_type = return_type.replace("typedef", "")
    return_type = parser_utils.clean_type_string(return_type)

    arguments = parse_arguments(func_ptr_elem)
    return internal_types.VulkanFunctionPtr(
        typename=function_name,
        return_type=return_type,
        arguments=arguments)
