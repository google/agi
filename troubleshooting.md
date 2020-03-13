---
title: Troubleshooting
layout: default
permalink: /docs/troubleshooting
---

Please ensure that your setup meets all of the [requirements](../requirements).

## Profiling on Android does not work

-   Stop any program that may interact with the device over ADB, such as Android
    Studio.

-   Enable the **Stay awake** option (under **Developer options** on Android) to
    prevent issues that arise when the device screen turns off due to sleep
    mode.

-   The target application must _not_ report any warning or error when run with
    [Vulkan validation layers](https://developer.android.com/ndk/guides/graphics/validation-layer)
    enabled.
