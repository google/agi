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

""" This module is responsible for parsing features for each Vulkan version"""

from typing import Dict
from typing import Optional
import xml.etree.ElementTree as ET

from vulkan_generator.vulkan_utils import parsing_utils
from vulkan_generator.vulkan_parser import types


def parse_enum_extension(enum_element: ET.Element) -> Optional[types.VulkanFeatureExtensionEnum]:
    """Parses the enum field extension added by the core version"""
    basetype = parsing_utils.try_get_attribute(enum_element, "extends")
    if not basetype:
        return None

    alias = parsing_utils.try_get_attribute(enum_element, "alias")
    extnumber = parsing_utils.try_get_attribute(enum_element, "extnumber")
    offset = parsing_utils.try_get_attribute(enum_element, "offset")
    bitpos = parsing_utils.try_get_attribute(enum_element, "bitpos")
    value = parsing_utils.try_get_attribute(enum_element, "value")

    return types.VulkanFeatureExtensionEnum(
        basetype=basetype,
        alias=alias,
        extnumber=extnumber,
        offset=offset,
        bitpos=bitpos,
        value=value)


def parse(feature_element: ET.Element) -> types.VulkanCoreVersion:
    """Parses features required by a specific Vulkan version"""
    if feature_element.attrib["api"] != "vulkan":
        raise SyntaxError(f"Unknown API {ET.tostring(feature_element, 'utf-8')!r}")

    version_name = feature_element.attrib["name"]
    version_number = feature_element.attrib["number"]

    features: Dict[str, types.VulkanFeature] = {}
    for require_element in feature_element:
        if require_element.tag != "require":
            raise SyntaxError(f"Unknown Tag in Vulkan features {ET.tostring(require_element, 'utf-8')!r}")

        for required_feature_element in require_element:
            if required_feature_element.tag == "comment":
                continue

            feature_name = required_feature_element.attrib["name"]
            feature_type = required_feature_element.tag

            feature_extension: Optional[types.VulkanFeatureExtension] = None
            if required_feature_element.tag == "enum":
                # Enums are expanded in core versions
                feature_extension = parse_enum_extension(required_feature_element)
                if not feature_extension:
                    # If there is no enum override, then enum is actually a Vulkan API constant(C++ define)
                    feature_type = "type"

            features[feature_name] = types.VulkanFeature(
                name=feature_name,
                feature_type=feature_type,
                feature_extension=feature_extension)

    return types.VulkanCoreVersion(
        name=version_name,
        number=version_number,
        features=features
    )
