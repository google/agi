---
title: Getting Started
layout: default
permalink: /docs/getting-started
---

## Install adb

Make sure the `adb` tool is installed on your host computer. See the [adb documentation](https://developer.android.com/studio/command-line/adb) for installation instructions.

## Prepare your device

As long as a [supported device](devices) is being used and is running Android 11 or later, you should be good to go.

If you are using beta GPU drivers, instructions on how to set up your device will be e-mailed to you.

## Prepare your application

* Both Vulkan and OpenGL ES applications are supported. 

* The target application must be debuggable; the [debuggable attribute] in the Android manifest of the application must be set to `true`. This attribute enables proper instrumentations from the graphics driver. For Vulkan applications, this attribute enables AGI to add its own Vulkan layer when the application starts.
  ```xml
  <application [...] android:debuggable="true">
  ```
[debuggable attribute]: https://developer.android.com/guide/topics/manifest/application-element#debug

* If you are using beta GPU drivers, the following piece of metadata must be included in the `<application>` tag of the APK manifest:
  ```xml
  <meta-data 
            android:name="com.android.graphics.developerdriver.enable" 
            android:value="true" />
  ```

## Capturing a systems profile of an Android application

1. Connect one of the [supported devices](devices) to your host computer with a USB cable. Your device must have [`adb` debugging enabled](https://developer.android.com/studio/command-line/adb#Enabling)
such that it can be accessed via `adb`.

2. The first time you launch AGI, you will see a pop-up window prompting for some options and the path to the `adb` tool. Fill this and close the window. Settings are saved in the `.agic` file in your HOME folder.

    <img src="../images/system_profiler/startup.png" width="500px">

3. You will now see the welcome window. Click on the "Capture a new trace" option, which will then bring you to the capture option menu.

    <img src="../images/system_profiler/welcome.png" width="500px">

4. The first thing to do is to select the Android device you want to capture on. If your device does not show up in the "Device" list, you can refresh the device list by clicking on the reload arrow next to the drop-down menu.

    <img src="../images/system_profiler/capture-dialog.png" width="500px">

5. In "Type", select "System profile".

6. Select which application you want to trace. 

    <div class="info" markdown="span">	
    AGI comes with a simple Vulkan application that we are going to use as an example.<br><br>
    1. Click on the "..." button right next to the "Application" text entry to list applications that can be traced on the selected device.<br>
    2. This pops-up the "Select an Application to Trace" window.<br>
    3. Type "gapid" in the filter text box to filter down applications to the ones that contain "gapid" in their package name.<br>
    4. Expand the package, select "com.google.android.gapid.VkSampleActivity", and press "OK".<br>
    5. This brings you back to the capture options window, with the "Application" field populated by the Application you selected.<br>
    </div>

    You may also choose another application if you prefer.  If there is only one activity in the package, you can simply select the package instead of the Activity.

7. In the "Application" section, you can leave the other fields empty.

8. Under "Start and Duration" section, set "Start at" to "Manual", and "Duration" to 2.

9. In the "Trace Option" section, click on the "Configure" button to pop-up a configuration window where you can select which profiling data you want to be captured.

    <img src="../images/system_profiler/capture-config.png" width="500px">

10. In the GPU section, click on "Select" to bring up the counter selection dialog. Click on "default" to select the set of default counters and press "OK".

    <img src="../images/system_profiler/counter-config.png" width="500px">

11. In the "Output" section, please select an "Output Directory" where the capture files will be stored. The "File Name" should be auto-filled. You can also edit the file name.

12. Once all this is done, press "OK". This starts the selected app on the Android device, and creates a pop-up window with a "Start" button. Click the "Start" button to start the performance capture, and wait for a couple of seconds for
the trace to terminate.

13. Once the capture is completed, click on "Open Trace". This leads to a view similar to [systrace](https://developer.android.com/studio/profile/systrace).

    <img src="../images/system_profiler/system-profile.png" width="500px">

    In addition to the data available in systrace, AGI also shows GPU performance information. The GPU Queue section shows when GPU commands are submitted and when the GPU actually starts work.  The GPU Counters shows various performance counters to help optimize the graphics works in the app.

## Validate Vulkan on Android

The official Android documentation contains a section about [Vulkan Validation layers](https://developer.android.com/ndk/guides/graphics/validation-layer).

If your app does not already have a setup to run with the validation layers, you can use the following commands to force it to run with the validation layers shipped in the AGI apk (named `com.google.android.gapid.<abi>` for historical reasons):

```sh
app_package=<YOUR APP PACKAGE NAME HERE>
abi=arm64v8a # Possible values: arm64v8a, armeabi-v7a, x86

adb shell settings put global enable_gpu_debug_layers 1
adb shell settings put global gpu_debug_app ${app_package}
adb shell settings put global gpu_debug_layer_app com.google.android.gapid.${abi}
adb shell settings put global gpu_debug_layers VK_LAYER_KHRONOS_validation
```

To disable validation layers:

```sh
adb shell settings delete global enable_gpu_debug_layers
adb shell settings delete global gpu_debug_app
adb shell settings delete global gpu_debug_layers
adb shell settings delete global gpu_debug_layer_app
```
