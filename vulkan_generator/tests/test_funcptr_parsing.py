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

from vulkan_generator.vulkan_parser import funcptr_parser
from vulkan_generator.vulkan_parser import types


def test_vulkan_func_pointer() -> None:
    """""Test the parsing of a function pointer"""
    xml = """<?xml version="1.0" encoding="UTF-8"?>
    <type category="funcpointer">typedef void (VKAPI_PTR *
    <name>PFN_vkInternalAllocationNotification</name>)(
    <type>void</type>*                                       pUserData,
    <type>size_t</type>                                      size,
    <type>VkInternalAllocationType</type>                    allocationType,
    <type>VkSystemAllocationScope</type>                     allocationScope);</type>
    """
    funcptr = funcptr_parser.parse(ET.fromstring(xml))

    assert isinstance(funcptr, types.VulkanFunctionPtr)

    assert funcptr.typename == "PFN_vkInternalAllocationNotification"
    assert funcptr.return_type == "void"
    assert len(funcptr.arguments) == 4

    assert funcptr.arguments[0].typename == "void*"
    assert funcptr.arguments[0].argument_name == "pUserData"

    assert funcptr.arguments[1].typename == "size_t"
    assert funcptr.arguments[1].argument_name == "size"

    assert funcptr.arguments[2].typename == "VkInternalAllocationType"
    assert funcptr.arguments[2].argument_name == "allocationType"

    assert funcptr.arguments[3].typename == "VkSystemAllocationScope"
    assert funcptr.arguments[3].argument_name == "allocationScope"


def test_vulkan_func_pointer_with_pointer_return_value() -> None:
    """""Test the parsing of a function pointer with a pointer return type"""
    xml = """<?xml version="1.0" encoding="UTF-8"?>
    <type category="funcpointer">typedef void* (VKAPI_PTR *
    <name>PFN_vkReallocationFunction</name>)(
    <type>void</type>*                                       pUserData,
    <type>void</type>*                                       pOriginal,
    <type>size_t</type>                                      size,
    <type>size_t</type>                                      alignment,
    <type>VkSystemAllocationScope</type>                     allocationScope);</type>
    """

    funcptr = funcptr_parser.parse(ET.fromstring(xml))

    assert isinstance(funcptr, types.VulkanFunctionPtr)
    assert funcptr.return_type == "void*"


def test_vulkan_func_pointer_with_const_member() -> None:
    """""Test the parsing of a function pointer with a const pointer argument"""

    xml = """<?xml version="1.0" encoding="UTF-8"?>
    <type category="funcpointer">typedef VkBool32 (VKAPI_PTR *
    <name>PFN_vkDebugReportCallbackEXT</name>)(
    <type>VkDebugReportFlagsEXT</type>                       flags,
    <type>VkDebugReportObjectTypeEXT</type>                  objectType,
    <type>uint64_t</type>                                    object,
    <type>size_t</type>                                      location,
    <type>int32_t</type>                                     messageCode,
    const <type>char</type>*                                 pLayerPrefix,
    const <type>char</type>*                                 pMessage,
    <type>void</type>*                                       pUserData);</type>
    """

    funcptr = funcptr_parser.parse(ET.fromstring(xml))

    assert funcptr.arguments[4].argument_name == "messageCode"
    assert funcptr.arguments[5].typename == "const char*"
