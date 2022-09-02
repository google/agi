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
This module is responsible for testing Vulkan platforms

Examples in this files stems from vk.xml that relesed by Khronos.
Anytime the particular xml updated, test should be checked
if they reflect the new XML
"""

import xml.etree.ElementTree as ET

from vulkan_generator.vulkan_parser.internal import platforms_parser


def test_vulkan_include_with_directive() -> None:
    """""Test the case if the handle name is in an XML tag"""

    xml = """<?xml version="1.0" encoding="UTF-8"?>
    <platforms comment="Vulkan platform names, reserved for use with platform- and window system-specific extensions">
        <platform name="android" protect="VK_USE_PLATFORM_ANDROID_KHR" comment="Android OS"/>
    </platforms>"""

    platforms = platforms_parser.parse(ET.fromstring(xml))

    assert "android" in platforms

    android = platforms["android"]
    assert android.name == "android"
    assert android.protect == "VK_USE_PLATFORM_ANDROID_KHR"
