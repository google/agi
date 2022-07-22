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

""" This module is responsible for parsing Vulkan commands and aliases of them"""

from typing import Dict

import xml.etree.ElementTree as ET

from vulkan_generator.vulkan_parser.internal import parser_utils
from vulkan_generator.vulkan_parser.internal import internal_types


def parse_arguments(command_elem: ET.Element) -> Dict[str, internal_types.VulkanCommandParam]:
    """Parses the arguments of Vulkan Commands"""

    parameters: Dict[str, internal_types.VulkanCommandParam] = {}

    for param_elem in command_elem:
        if param_elem.tag == "proto":
            # This tag is for function name and return type
            continue

        if param_elem.tag == "implicitexternsyncparams":
            # We do not need to worry about this tag
            continue

        if param_elem.tag != "param":
            raise SyntaxError(f"Unknown tag in function parameters: {param_elem}")

        parameter_type = parser_utils.get_text_from_tag_in_children(param_elem, "type")
        parameter_name = parser_utils.get_text_from_tag_in_children(param_elem, "name")

        # This part(attributes and pointers) is very similar to parsing typenames in struct
        # But as we cannot guarantee that it will stay similar
        # It's better to write separately to be able to handle nuances in between.

        # If type is const, it's written under the param tag's text instead of type.
        # e.g. <param>const <type>VkInstanceCreateInfo</type>* <name>pCreateInfo</name></param>
        type_attributes = param_elem.text
        # some times it's just empty space or endline character
        if type_attributes:
            type_attributes = parser_utils.clean_type_string(type_attributes)
            # it might be empty after cleaning
            if type_attributes:
                parameter_type = f"{type_attributes} {parameter_type}"

        pointers = parser_utils.try_get_tail_from_tag_in_children(param_elem, "type")
        # some times it's just empty space or endline character
        if pointers:
            pointers = parser_utils.clean_type_string(pointers)
            # it might be empty after cleaning
            if pointers:
                # Add space between "*" and "const"
                pointers = pointers.replace("const", " const")
                parameter_type = f"{parameter_type}{pointers}"

        if not parameter_type:
            raise SyntaxError(
                f"No parameter type found in : {ET.tostring(param_elem, 'utf-8')}")

        if not parameter_name:
            raise SyntaxError(
                f"No parameter name found in : {ET.tostring(param_elem, 'utf-8')}")

        # Is this parameter optional or has to be not null
        # When this field is "false, true"  it's always for the length of the array
        # Therefore it does not give any extra information.
        optional = param_elem.get("optional") == "true"

        # External syncronisation info for the parameter
        externally_synced_field = param_elem.get("externsync")
        externally_synced = externally_synced_field is not None
        # If the externally sync attribute says true, this means entire parameter is synced
        # not a specific field
        if externally_synced_field == "true":
            externally_synced_field = None

        # This is useful when the parameter is a pointer to an array
        # with a length given by another parameter
        array_size_reference = param_elem.get("len")
        if array_size_reference:
            # pointer to char array has this property, which is redundant
            array_size_reference = array_size_reference.replace("null-terminated", "")
            array_size_reference = parser_utils.clean_type_string(array_size_reference)

        parameters[parameter_name] = internal_types.VulkanCommandParam(
            parameter_name=parameter_name,
            parameter_type=parameter_type,
            optional=optional,
            externally_synced=externally_synced,
            externally_synced_field=externally_synced_field,
            array_size_reference=array_size_reference,
        )

    return parameters


def parse_command(command_elem: ET.Element) -> internal_types.VulkanCommand:
    """Returns a Vulkan command from the XML element that defines it.

    A sample Vulkan command:
    <command successcodes="VK_SUCCESS,VK_NOT_READY"
        errorcodes="VK_ERROR_OUT_OF_HOST_MEMORY,VK_ERROR_OUT_OF_DEVICE_MEMORY,VK_ERROR_DEVICE_LOST">

        <proto><type>VkResult</type> <name>vkGetQueryPoolResults</name></proto>
        <param><type>VkDevice</type> <name>device</name></param>
        <param><type>VkQueryPool</type> <name>queryPool</name></param>
        <param><type>uint32_t</type> <name>firstQuery</name></param>
        <param><type>uint32_t</type> <name>queryCount</name></param>
        <param><type>size_t</type> <name>dataSize</name></param>
        <param len="dataSize"><type>void</type>* <name>pData</name></param>
        <param><type>VkDeviceSize</type> <name>stride</name></param>
        <param optional="true"><type>VkQueryResultFlags</type> <name>flags</name></param>
    </command>
    """

    success_codes = parser_utils.try_get_attribute_as_list(command_elem, "successcodes")
    error_codes = parser_utils.try_get_attribute_as_list(command_elem, "errorcodes")
    queues = parser_utils.try_get_attribute_as_list(command_elem, "queues")
    command_buffer_levels = parser_utils.try_get_attribute_as_list(command_elem, "cmdbufferlevel")

    renderpass_allowance = command_elem.get("renderpass")

    name = parser_utils.get_text_from_tag_in_children(command_elem[0], "name")
    return_type = parser_utils.get_text_from_tag_in_children(command_elem[0], "type")

    parameters = parse_arguments(command_elem)
    return internal_types.VulkanCommand(
        name=name,
        return_type=return_type,
        renderpass_allowance=renderpass_allowance,
        success_codes=success_codes,
        queues=queues,
        command_buffer_levels=command_buffer_levels,
        error_codes=error_codes,
        parameters=parameters)


def parse_command_alias(command_elem: ET.Element) -> internal_types.VulkanCommandAlias:
    """Returns a Vulkan command alias from the XML element that defines it.

    A sample Vulkan command alias:
    <command name="vkResetQueryPoolEXT" alias="vkResetQueryPool"/>
    """
    alias = command_elem.attrib["alias"]
    name = command_elem.attrib["name"]
    return internal_types.VulkanCommandAlias(command_name=name, aliased_command_name=alias)


def parse(commands_elem: ET.Element) -> internal_types.AllVulkanCommands:
    """Parses all the Vulkan commands and aliases"""
    vulkan_commands = internal_types.AllVulkanCommands()

    for command_elem in commands_elem:
        if command_elem.tag != "command":
            raise SyntaxError("Unknown tag in commands: {command_elem}")

        if "alias" in command_elem.attrib:
            command_alias = parse_command_alias(command_elem)
            vulkan_commands.command_aliases[command_alias.command_name] = command_alias
            continue

        command = parse_command(command_elem)
        vulkan_commands.commands[command.name] = command

    return vulkan_commands
