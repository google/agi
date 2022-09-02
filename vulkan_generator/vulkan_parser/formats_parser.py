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

""" This module is responsible for parsing Vulkan Image Formats"""

from typing import OrderedDict
from typing import List
from typing import Optional

import xml.etree.ElementTree as ET

from vulkan_generator.vulkan_parser import parser_utils
from vulkan_generator.vulkan_parser import types


def parse_format_planes(format_element: ET.Element) -> List[types.ImageFormatPlane]:
    """
    Parser for the image format planes if format is multi-planar

    Sample image format plane:
    <plane index="0" widthDivisor="1" heightDivisor="1" compatible="VK_FORMAT_R8_UNORM"/>
    """
    planes: List[types.ImageFormatPlane] = []

    for plane in format_element:
        if plane.tag == "spirvimageformat":
            # this is handled by image format parser
            continue

        if plane.tag == "component":
            # this is handled by image format component parser
            continue

        if plane.tag == "plane":
            index = int(plane.attrib["index"], 0)
            with_divisor = int(plane.attrib["widthDivisor"], 0)
            height_divisor = int(plane.attrib["heightDivisor"], 0)
            compatible = plane.attrib["compatible"]

            planes.append(types.ImageFormatPlane(
                index=index,
                width_divisor=with_divisor,
                height_divisor=height_divisor,
                compatible=compatible
            ))
        else:
            raise SyntaxError(f"Uknown image format element{ET.tostring(plane, 'utf-8')!r}")

    return planes


def parse_format_components(format_element: ET.Element) -> OrderedDict[str, types.ImageFormatComponent]:
    """
    Parser for the image formats components

    Sample image format component:
    <component name="R" bits="compressed" numericFormat="SRGB"/>
    """
    components: OrderedDict[str, types.ImageFormatComponent] = OrderedDict()

    for component in format_element:
        if component.tag == "spirvimageformat":
            # this is handled by image format parser
            continue

        if component.tag == "plane":
            # this is handled by image format planes parser
            continue

        if component.tag == "component":
            name = component.attrib["name"]
            numeric_format = component.attrib["numericFormat"]

            # if the image is not compressed then bits per component is available
            bits = None
            compressed = component.attrib["bits"] == "compressed"
            if not compressed:
                bits = int(component.attrib["bits"], 0)

            components[name] = types.ImageFormatComponent(
                name=name,
                numeric_format=numeric_format,
                bits=bits,
                compressed=compressed
            )
        else:
            raise SyntaxError(f"Uknown image format element{ET.tostring(component, 'utf-8')!r}")

    return components


def parse_format(format_element: ET.Element) -> types.ImageFormat:
    """
    Parser for the Vulkan image formats

    A sample Vulkan image format:
    <format name="VK_FORMAT_G8_B8_R8_3PLANE_420_UNORM" class="8-bit 3-plane 420"
        blockSize="3" texelsPerBlock="1" chroma="420">
        <component name="G" bits="8" numericFormat="UNORM" planeIndex="0"/>
        <component name="B" bits="8" numericFormat="UNORM" planeIndex="1"/>
        <component name="R" bits="8" numericFormat="UNORM" planeIndex="2"/>
        <plane index="0" widthDivisor="1" heightDivisor="1" compatible="VK_FORMAT_R8_UNORM"/>
        <plane index="1" widthDivisor="2" heightDivisor="2" compatible="VK_FORMAT_R8_UNORM"/>
        <plane index="2" widthDivisor="2" heightDivisor="2" compatible="VK_FORMAT_R8_UNORM"/>
    </format>
    """
    name = format_element.attrib["name"]
    format_class = format_element.attrib["class"]
    block_size = int(format_element.attrib["blockSize"], 0)
    texels_per_block = int(format_element.attrib["texelsPerBlock"], 0)

    # Pack size for the packed formats
    packed: Optional[int] = None
    packed_str = parser_utils.try_get_attribute(format_element, "packed")
    if packed_str:
        packed = int(format_element.attrib["packed"], 0)

    # if the image has a spirv equavelant, it's stated in a child node
    spirv_format: Optional[str] = None
    for child in format_element:
        if child.tag == "spirvimageformat":
            spirv_format = child.attrib["name"]

    components = parse_format_components(format_element)

    # Parse plane info for multi-planar formats. For non-planar formats
    # this list will be empty
    planes = parse_format_planes(format_element)

    return types.ImageFormat(
        name=name,
        format_class=format_class,
        block_size=block_size,
        texels_per_block=texels_per_block,
        packed=packed,
        spirv_format=spirv_format,
        components=components,
        planes=planes)


def parse(formats_element: ET.Element) -> types.ImageFormatMetadata:
    format_metadata = types.ImageFormatMetadata()

    for format_element in formats_element:
        image_format = parse_format(format_element)
        format_metadata.formats[image_format.name] = image_format

    return format_metadata
