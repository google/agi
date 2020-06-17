---
title: Minimum System Requirements
layout: default
permalink: /docs/requirements
---

## Android

Profiling a Vulkan application on an Android device requires the following:

* A [supported device](#supported-android-devices)
* The device must run Android 10 or later. Previous versions are not supported.
* The device must have [adb debugging enabled], and be accessible via adb.
* The target application must be debuggable; the [debuggable attribute] in the
  Android manifest of the application must be set to `true`. This attribute
  enables AGI to add its own Vulkan layer when the application starts.
  ```xml
  <application [...] android:debuggable="true">
  ```
* The target application must not report any warning/error when it runs with
  the [Vulkan validation layers] enabled. Fix any Vulkan validation error before
  profiling. The replay profiler assumes that the application uses the Vulkan
  API correctly. Invalid use of the Vulkan API is considered undefined behavior.

### Supported Android devices {#supported-android-devices}

The following Android devices are supported by AGI:

Device name                         | GPU name
----------------------------------- | -------------------
Google Pixel 4 (standard and XL)    | Qualcomm® Adreno™ 640
Samsung Galaxy Note 10 (Exynos)     | Arm® Mali™-G76
Samsung Galaxy Note 10 (Snapdragon) | Qualcomm® Adreno™ 640
Samsung Galaxy S10 (Exynos)         | Arm® Mali™-G76
Samsung Galaxy S10 (Snapdragon)     | Qualcomm® Adreno™ 640

Android emulators are not supported.

## System

The following are the minimum system requirements for profiling an Android
application using a desktop or laptop:

### Windows

* Windows 7 or later
* [Vulkan] GPU drivers for desktop frame profiling

### macOS

* El Capitan (10.11) or later

### Linux

* Ubuntu 'Trusty Tahr' (14.04) or later recommended
* Java 64-bit JDK or JRE 11 (or newer)
* [Vulkan] GPU drivers for desktop frame profiling


[adb debugging enabled]: https://developer.android.com/studio/command-line/adb#Enabling
[Vulkan]: https://en.wikipedia.org/wiki/Vulkan_(API)#Compatibility
[debuggable attribute]: https://developer.android.com/guide/topics/manifest/application-element#debug
[Vulkan validation layers]: https://developer.android.com/ndk/guides/graphics/validation-layer
