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

""" This module is responsible for parsing Vulkan Handles and aliases of them"""

from typing import Dict
import xml.etree.ElementTree as ET

from vulkan_utils import parsing_utils
from vulkan_parser import types


def parse_struct_members(struct_element: ET.Element) -> Dict[str, types.VulkanStructMember]:
    """Parses a Vulkan Struct member

    This is a bit of an irregular code because the XML itself has quite irregularities that
    makes is hard to parse type and variable easily.

    For example a const pointer member is defined as:
     <member optional="true">const <type>void</type>*     <name>pNext</name></member>

    Where as a static array defined as:
    <member limittype="noauto">
        <type>char</type>
        <name>deviceName</name>[<enum>VK_MAX_PHYSICAL_DEVICE_NAME_SIZE</enum>]
    </member>
    """

    #  This is a bit of an irregular code because the XML itself has quite irregularities that
    # makes is hard to parse type and variable easily.
    #
    # This is not the code we wanted but it's the code that we needed and it's contained in a
    # small place so that XML irregularities does not leak into the rest of the code.

    members = {}
    for member_element in struct_element:
        if member_element.tag == "comment":
            # Melih TODO: We may want to support comments in the future
            continue

        if member_element.tag != "member":
            raise SyntaxError(f"No member tag found in : {ET.tostring(member_element, 'utf-8')}")

        typename = parsing_utils.get_text_from_tag(member_element, "type")
        variable_name = parsing_utils.get_text_from_tag(member_element, "name")

        # Type attributes(const, struct) and pointer attributes(*, const*, *const,*const*)
        # are usually in the text field of the member tag.
        #
        # Below is the example for a const pointer:
        #
        # <member optional="true">const <type>void</type>*     <name>pNext</name></member>
        #
        # In the below type is "const char* const*" but only "char" is in the type tag.
        # The rest of the information is scattered around the member's text
        # Therefore we need to retrieve and clean it so we can add it to the type.
        #
        # <member len="enabledLayerCount,null-terminated">const <type>char</type>
        # * const*      <name>ppEnabledLayerNames</name>
        #
        type_attributes = parsing_utils.try_get_text_from_tag(member_element, "member")
        # some times it's just empty space or endline character
        if type_attributes is not None:
            type_attributes = parsing_utils.clean_type_string(type_attributes)

        if type_attributes and len(type_attributes) > 0:
            typename = f"{type_attributes} {typename}"

        pointers = parsing_utils.try_get_tail_from_tag(member_element, "type")
        if pointers is not None:
            pointers = parsing_utils.clean_type_string(pointers)
            # Add space between "*" and "const"
            pointers = pointers.replace("const", " const")
            typename = f"{typename}{pointers}"

        if typename is None:
            raise SyntaxError(f"No typename found in : {ET.tostring(member_element, 'utf-8')}")

        if variable_name is None:
            raise SyntaxError(f"No variable name found in : {ET.tostring(member_element, 'utf-8')}")

        # Variable size is optional
        variable_size = parsing_utils.try_get_text_from_tag(member_element, "enum")

        # Currently if this attribute exists, it's always true
        no_auto_validity = parsing_utils.try_get_attribute(member_element, "noautovalidity")

        # This is useful for the sType where the correct value is already known
        expected_value = parsing_utils.try_get_attribute(member_element, "values")

        # Is this field optional or has to be set
        optional = parsing_utils.try_get_attribute(member_element, "optional")

        # This is useful when the member is an pointer to an array
        # with a length given by another member
        array_reference = parsing_utils.try_get_attribute(member_element, "len")
        if array_reference is not None:
            # pointer to char array has this property, which is redundant
            array_reference = array_reference.replace("null-terminated", "")
            array_reference = parsing_utils.clean_type_string(array_reference)

        members[variable_name] = types.VulkanStructMember(
            typename=typename,
            variable_name=variable_name,
            varible_size= variable_size,
            no_auto_validity=no_auto_validity,
            expected_value=expected_value,
            array_reference=array_reference,
            optional=optional
        )

    return members


def parse(struct_elem: ET.Element) -> types.VulkanType:
    """Returns a Vulkan struct or alias from the XML element that defines it.

    A sample Vulkan struct:
    <type category="struct" name="VkDevicePrivateDataCreateInfo"
        allowduplicate="true" structextends="VkDeviceCreateInfo">

        <member values="VK_STRUCTURE_TYPE_DEVICE_PRIVATE_DATA_CREATE_INFO">
            <type>VkStructureType</type> <name>sType</name>
        </member>
        <member optional="true">const <type>void</type>*<name>pNext</name></member>
        <member><type>uint32_t</type> <name>privateDataSlotRequestCount</name></member>
    </type>

    A sample Vulkan struct alias:
    <type category="struct" name="VkDevicePrivateDataCreateInfoEXT"
        alias="VkDevicePrivateDataCreateInfo"/>
    """

    struct_name = struct_elem.attrib["name"]

    if "alias" in struct_elem.attrib:
        return types.VulkanStructAlias(typename=struct_name,
            aliased_typename=struct_elem.attrib["alias"])


    members = parse_struct_members(struct_elem)
    return types.VulkanStruct(struct_name, members)
