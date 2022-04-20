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

"""
This package is responsible for testing Vulkan Parser

Examples in this files stems from vk.xml that relesed by Khronos.
Anytime the particular xml updated, test should be checked
if they reflect the new XML
"""

import xml.etree.ElementTree as ET

from vulkan_parser import handle_parser
from vulkan_parser.types import VulkanHandle, VulkanHandleAlias


def test_vulkan_handle_by_tag() -> None:
    """""Test the case if the handle name is in an XML tag"""

    xml = """<?xml version="1.0" encoding="UTF-8"?>
    <type category="handle" parent="VkDevice" objtypeenum="VK_OBJECT_TYPE_QUEUE">
    <type>VK_DEFINE_HANDLE</type>(<name>VkQueue</name>)</type>"""

    handle = handle_parser.parse_handle_by_tag(ET.fromstring(xml))

    assert isinstance(handle, VulkanHandle)
    assert handle.typename == "VkQueue"


def test_vulkan_handle_by_attribute() -> None:
    """""Test the case if the handle name is in an XML attribute"""

    xml = """<?xml version="1.0" encoding="UTF-8"?>
        <type category="handle" name="VkDescriptorUpdateTemplateKHR"
        alias="VkDescriptorUpdateTemplate"/>
    """

    handle = handle_parser.parse_handle_by_attribute(ET.fromstring(xml))

    assert isinstance(handle, VulkanHandleAlias)
    assert handle.typename == "VkDescriptorUpdateTemplateKHR"
    assert handle.aliased_typename == "VkDescriptorUpdateTemplate"
