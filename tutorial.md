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

Connect one of the
[supported devices](../requirements#supported-android-devices)
to your host computer with a USB cable. Your device must have
[`adb` debugging enabled](https://developer.android.com/studio/command-line/adb#Enabling)
such that it can be accessed via `adb`.

The first time you launch AGI, you face a pop-up windows prompting for some
options and the path to the `adb` tool. Fill this up and close the window.
Settings are saved in the `.agic` file in your HOME folder.

<img src="../images/system_profiler/startup.png" width="500px">

You now face the welcome window. Click on the "Capture a new trace" option,
which brings you to the capture option menu.

<img src="../images/system_profiler/welcome.png" width="500px">

The first thing to do is to select the Android device you want to capture on. If
your device does not shows up in the "Device" list, you can refresh the device
list by clicking on the reload arrow next to the drop-down menu.

<img src="../images/system_profiler/capture-dialog.png" width="500px">

In "Type", select "System profile".

Next, you need to select which application you want to trace. AGI comes with a
simple Vulkan application that we are going to use as an example here. Click on
the "..." button right next to the "Application" text entry to list applications
that can be traces on the selected device. This pops-up the "Select an
Application to Trace" window. Type "gapid" in the filter text box to filter down
applications to the ones that contain "gapid" in their package name. Expand the
package, select "com.google.android.gapid.VkSampleActivity", and press "OK". This
brings you back to the capture options window, with the "Application" field
populated by the Application you selected.

You may also choose another Vulkan application if you prefer.  If there is only
one activity in the package, you can simply select the package instead of the Activity.

In the "Application" section, you can leave the other fields empty.

Under "Start and Duration" section, set "Start at" to "Manual", and "Duration"
to 2.

In the "Trace Option" section, click on the "Configure" button to pop-up a
configuration window where you can select which can of performance counters you
want to be captured.

<img src="../images/system_profiler/capture-config.png" width="500px">

In the "Output" section, please select an "Output Directory" where the capture
file will be stored. The "File Name" should be auto-filled, you can edit the
file name if need be.

In the GPU section, click on "Select" to bring up the counter selection dialog.
Click on "default" to select the set of default counters and press "OK".

<img src="../images/system_profiler/counter-config.png" width="500px">

Once all this is done, press "OK". This starts the selected app on the Android
device, and creates a pop-up window with a "Start" button. Click the "Start"
button to start the performance capture, and wait for a couple of seconds for
the trace to terminate.

Once the capture is completed, click on "Open Trace". This leads to a view
similar to [systrace](https://developer.android.com/studio/profile/systrace).

<img src="../images/system_profiler/system-profile.png" width="500px">

Compare to systrace, there are now additional information relating to the GPU.
The GPU Queue section shows when GPU commands are submitted and when the GPU
actually starts work.  The GPU Counters shows various performance counters to
help optimize the graphics works in the app.

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
