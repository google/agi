---
title: Troubleshooting
layout: default
permalink: /docs/troubleshooting
---

Please ensure that your setup meets all of the [requirements](requirements).

## GPU activity is missing from my OpenGL ES trace

Only GPU counters are supported if tracing an OpenGL ES application. GPU activity information for OpenGL ES applications is currently under development.

## Profiling on Android does not work

-   Stop any program that may interact with the device over ADB, such as Android
    Studio.

-   Enable the **Stay awake** option (under **Developer options** on Android) to
    prevent issues that arise when the device screen turns off due to sleep
    mode.

-   If a Vulkan application is being profiled, the target application must _not_ report any warning or error when run with
    [Vulkan validation layers](https://developer.android.com/ndk/guides/graphics/validation-layer)
    enabled.

## Android Vulkan layers issues

If a Vulkan capture on Android does not terminate properly, Android
GPU Inspector may leave some Android settings related to Vulkan layers
in a state that may pertubate subsequent runs of the app. These
settings are:

-   `enable_gpu_debug_layers`

-   `gpu_debug_app`

-   `gpu_debug_layers`

-   `gpu_debug_layer_app`

If your app has some Vulkan layer issues after you used Android GPU
Inspector, you may try and clear these settings with the following
`adb` commands:

```sh
adb shell settings delete global enable_gpu_debug_layers
adb shell settings delete global gpu_debug_app
adb shell settings delete global gpu_debug_layers
adb shell settings delete global gpu_debug_layer_app
```

See also the [Android documentation on how to use settings to enable Vulkan layers](https://developer.android.com/ndk/guides/graphics/validation-layer?release=r21#enable-layers-outside-app).
