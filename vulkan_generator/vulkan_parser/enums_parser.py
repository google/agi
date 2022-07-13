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

""" This module is responsible for parsing Vulkan enums"""

from typing import Optional
from typing import NamedTuple
from typing import Union
from typing import OrderedDict
from typing import Dict

import xml.etree.ElementTree as ET

from vulkan_generator.vulkan_parser import parser_utils
from vulkan_generator.vulkan_parser import types


class EnumInformation(NamedTuple):
    """Temporary class to return argument information"""
    fields: OrderedDict[str, types.VulkanEnumField]
    aliases: Dict[str, str]


def parse_value_fields(enum_elem: ET.Element) -> EnumInformation:
    """Parses the fields of an enum which is defined by values

    A sample Vulkan enum
    <enums name="VkSubpassContents" type="enum">
        <enum value="0"     name="VK_SUBPASS_CONTENTS_INLINE"/>
        <enum value="1"     name="VK_SUBPASS_CONTENTS_SECONDARY_COMMAND_BUFFERS"/>
    </enums>
    """

    fields: OrderedDict[str, types.VulkanEnumField] = OrderedDict()
    aliases: Dict[str, str] = {}

    for field_element in enum_elem:
        if field_element.tag == "comment":
            # We are not interested in comments
            continue

        if field_element.tag == "unused":
            # We are not interested in unused values reserved
            # for future
            continue

        name = field_element.attrib["name"]
        alias = parser_utils.try_get_attribute(field_element, "alias")
        if alias:
            aliases[name] = alias
            continue

        field = parser_utils.get_enum_field_from_value(field_element.attrib["value"])

        fields[name] = types.VulkanEnumField(
            name=name,
            value=field.value,
            representation=field.representation)

    return EnumInformation(
        fields=fields,
        aliases=aliases
    )


class BitfieldInfo(NamedTuple):
    value: int
    representation: str


def get_bitfield_info(field_elem: ET.Element, bit64: bool) -> BitfieldInfo:
    """Parses the value and representation of a bitfield in an enum"""

    # Sometimes instead of a bitpos, bitfield has a direct value
    value_string = parser_utils.try_get_attribute(field_elem, "value")
    if value_string:
        return BitfieldInfo(
            value=int(value_string, 0),
            representation=value_string
        )

    field = parser_utils.get_enum_field_from_bitpos(field_elem.attrib["bitpos"], bit64)

    return BitfieldInfo(
        value=field.value,
        representation=field.representation
    )


def parse_bitmask_fields(enum_elem: ET.Element, bit64: bool) -> EnumInformation:
    """Parses the fields of a bitmask enum

    A sample Vulkan bitmask enum
    <enums name="VkMemoryHeapFlagBits" type="bitmask">
        <enum bitpos="0"    name="VK_MEMORY_HEAP_DEVICE_LOCAL_BIT"
                           comment="If set, heap represents device memory"/>
    </enums>
    """
    fields: OrderedDict[str, types.VulkanEnumField] = OrderedDict()
    aliases: Dict[str, str] = {}

    for field_element in enum_elem:
        name = field_element.attrib["name"]

        alias = parser_utils.try_get_attribute(field_element, "alias")
        if alias:
            aliases[name] = alias
            continue

        bitfield_info = get_bitfield_info(field_element, bit64)

        fields[name] = types.VulkanEnumField(
            name=name,
            value=bitfield_info.value,
            representation=bitfield_info.representation
        )

    return EnumInformation(
        fields=fields,
        aliases=aliases
    )


def parse_api_constants(enum_elem: ET.Element) -> OrderedDict[str, types.VulkanDefine]:
    constants: OrderedDict[str, types.VulkanDefine] = OrderedDict()

    for enum in enum_elem:
        name = enum.attrib["name"]
        value = enum.attrib["value"] if "value" in enum.attrib else enum.attrib["alias"]
        constants[name] = types.VulkanDefine(variable_name=name, value=value)

    return constants


# We have to return union because api constants are defined under Enums, even though they are not enum
def parse(enum_elem: ET.Element) -> Union[OrderedDict[str, types.VulkanDefine], Optional[types.VulkanEnum]]:
    """Returns a Vulkan enum from the XML element that defines it"""

    enum_name = enum_elem.attrib["name"]

    if enum_name == "API Constants":
        return parse_api_constants(enum_elem)

    enum_type = enum_elem.attrib["type"]
    if enum_type not in ("enum", "bitmask"):
        raise SyntaxError(f"Unknown enum type : {ET.tostring(enum_elem)!r}")

    bitwidth = parser_utils.try_get_attribute(enum_elem, "bitwidth")
    if bitwidth and bitwidth != "64":
        raise SyntaxError(f"Unknown bitwidth: {ET.tostring(enum_elem)!r}")

    bitmask = (enum_type == "bitmask")
    bit64 = (bitwidth == "64")

    enum_info: EnumInformation
    if bitmask:
        enum_info = parse_bitmask_fields(enum_elem, bit64)
    else:
        enum_info = parse_value_fields(enum_elem)

    enum = types.VulkanEnum(
        typename=enum_name,
        fields=enum_info.fields,
        aliases=enum_info.aliases,
        bitmask=bitmask,
        bit64=bit64)

    return enum