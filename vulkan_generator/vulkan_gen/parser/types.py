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

"""This module contains the Vulkan meta data definitions to define Vulkan types and functions"""

from dataclasses import dataclass
from enum import Enum


# Melih TODO: Complete
class VulkanTypeCategories(Enum):
    """Enum to define different kinds of Vulkan types"""
    VULKAN_HANDLE = 1
    VULKAN_HANDLE_ALIAS = 2
    VULKAN_ENUM = 3
    VULKAN_STRUCT = 4


@dataclass
class VulkanType:
    """Base class for a Vulkan Type. All the other Types should inherit from this class"""
    category: VulkanTypeCategories
    typename: str

    def __init__(self, category: VulkanTypeCategories, typename: str):
        self.category = category
        self.typename = typename


@dataclass
class VulkanHandle(VulkanType):
    """The meta data defines a Vulkan Handle"""
    # Melih TODO: Vulkan Handles have object type in the XML that might be required in the future

    def __init__(self, typename: str):
        VulkanType.__init__(self, VulkanTypeCategories.VULKAN_HANDLE, typename)


@dataclass
class VulkanHandleAlias(VulkanType):
    """The meta data defines a Vulkan Handle alias"""
    alias_handle: VulkanHandle

    def __init__(self, typename: str, alias_typename: str):
        VulkanType.__init__(self, VulkanTypeCategories.VULKAN_HANDLE_ALIAS, typename)
        self.alias_handle = VulkanHandle(alias_typename)
