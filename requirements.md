---
title: Minimum System Requirements
layout: default
permalink: /docs/requirements
---

## Android

Profiling an application on an Android device requires the following:

* A [supported device](devices).
* The device must run Android 11 or later. Previous versions are not supported.
* The device must have [adb debugging enabled], and be accessible via adb.
* The target application must be debuggable; the [debuggable attribute] in the
  Android manifest of the application must be set to `true`. This attribute
  enables proper instrumentations from the graphics driver. For Vulkan applications,
  this attribute enables AGI to add its own Vulkan layer when the application starts.
  ```xml
  <application [...] android:debuggable="true">
  ```

For Vulkan applications:
* The target application must not report any warning/error when it runs with
  the [Vulkan validation layers] enabled. Fix any Vulkan validation error before
  profiling. The frame profiler assumes that the application uses the Vulkan
  API correctly. Invalid use of the Vulkan API is considered undefined behavior.

## System

The following are the minimum system requirements for profiling an Android
application using a desktop or laptop:

### Windows

* Windows 7 or later.

### macOS

* El Capitan (10.11) or later.

### Linux

* Ubuntu 'Trusty Tahr' (14.04) or later recommended.
* Java 64-bit JDK or JRE 8 (or newer).

[adb debugging enabled]: https://developer.android.com/studio/command-line/adb#Enabling
[Vulkan]: https://en.wikipedia.org/wiki/Vulkan_(API)#Compatibility
[debuggable attribute]: https://developer.android.com/guide/topics/manifest/application-element#debug
[Vulkan validation layers]: https://developer.android.com/ndk/guides/graphics/validation-layer
