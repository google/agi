---
title: Tutorial - Profile a Vulkan Application on Android
layout: default
permalink: /docs/tutorial
---

> (Short walkthrough: download, install, capture system trace, capture API
> trace, replay, show counters)

## Install adb

Make sure the `adb` tool is installed on your host computer. See the
[adb documentation](https://developer.android.com/studio/command-line/adb) for
installation instructions.

## Profile a Vulkan application

TODO: add screen capture (no more than one per window, if possible)

Connect one of the
[supported devices](../requirements#supported-android-devices)
to your host computer with a USB cable. Your device must have
[`adb` debugging enabled](https://developer.android.com/studio/command-line/adb#Enabling)
such that it can be accessed via `adb`.

TODO: Tell how to launch AGI depending on OS

The first time you launch AGI, you face a pop-up windows prompting for some
options and the path to the `adb` tool. Fill this up and close the window.
Settings are saved in the `.agic` file in your HOME folder.

You now face the welcome window. Click on the "Capture a new trace" option,
which brings you to the capture option menu.

The first thing to do is to select the Android device you want to capture on. If
your device does not shows up in the "Device" list, you can refresh the device
list by clicking on the reload arrow next to the drop-down menu.

In "Type", select "System profile".

> If we support only this type, we should not expose it as an option

Next, you need to select which application you want to trace. AGI comes with a
simple Vulkan application that we are going to use as an example here. Click on
the "..." button right next to the "Application" text entry to list applications
that can be traces on the selected device. This pops-up the "Select an
Application to Trace" window. Type "agi" in the filter text box to filter down
applications to the ones that contain "agi" in their package name. Select the
package name and press "OK". This brings you back to the capture options window,
with the "Application" field populated by the Application you selected.

> TODO: clarify whether we need to select a particular activity or not, or which
> activity is selected by default when there is more than one activity by
> package.

In the "Application" section, you can leave the other fields empty.

Under "Start and Duration" section, set "Start at" to "Manual", and "Duration"
to 2.

In the "Trace Option" section, click on the "Configure" button to pop-up a
configuration window where you can select which can of performance counters you
want to be captured. We stick with the defaults in this walkthrough.

> TODO: Can't we have all these options as part of the capture menu?

In the "Output" section, please select an "Output Directory" where the capture
file will be stored. The "File Name" should be auto-filled, you can edit the
file name if need be.

Once all this is done, press "OK". This starts the selected app on the Android
device, and creates a pop-up window with a "Start" button. Click the "Start"
button to start the performance capture, and wait for a couple of seconds for
the trace to terminate.

Once the capture is completed, click on "Open Trace". This leads to a view
similar to [systrace](https://developer.android.com/studio/profile/systrace).

> TODO: Describe the interesting tracks: GPU, Memory, Battery

## Validate Vulkan on Android

The official Android documentation contains a section about
[Vulkan Validation layers](https://developer.android.com/ndk/guides/graphics/validation-layer).

If your app does not already have a setup to run with the validation layers, you
can use the following commands to force it to run with the validation layers
shipped in the GAPID apk:

```sh
app_package=<YOUR APP PACKAGE NAME HERE>
abi=arm64v8a # Possible values: arm64v8a, armeabi-v7a, x86

adb shell settings put global enable_gpu_debug_layers 1
adb shell settings put global gpu_debug_app ${app_package}
adb shell settings put global gpu_debug_layer_app com.google.android.gapid.${abi}
# The order of layers matter
adb shell settings put global gpu_debug_layers VK_LAYER_GOOGLE_threading:VK_LAYER_LUNARG_parameter_validation:VK_LAYER_LUNARG_object_tracker:VK_LAYER_LUNARG_core_validation:VK_LAYER_GOOGLE_unique_objects
# Once NDK r21 stable release is done, we can use the unified layer:
# adb shell settings put global gpu_debug_layers VK_LAYER_KHRONOS_validation
```

To disable validation layers:

```sh
adb shell settings delete global enable_gpu_debug_layers
adb shell settings delete global gpu_debug_app
adb shell settings delete global gpu_debug_layers
adb shell settings delete global gpu_debug_layer_app
```
