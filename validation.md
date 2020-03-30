---
layout: default
title: Device Validation
# This URL is referenced from within the application.
permalink: /validation
---

Before Android GPU Inspector uses a devices for tracing and profiling, it validates the device setup
and that the device driver provides good and valid profiling data. Device validation takes about 10
seconds, during which you should see a spinning cube on the device screen.

Once a device passes the validation process, it can be used to trace and profile Vulkan
applications. Device validation is a one-time step and results are cached for future runs of AGI. If
the device setup changes, such as updating to a different version of Android or the GPU driver,
validation will run again.

Please <a href="https://services.google.com/fb/forms/androidgpuinspectordeveloperpreview/">sign up</a>
for the developer preview of Android GPU Inspector to get access to the device bits required to pass
validation. Device validation will only pass on a [supported device](docs/requirements#supported-android-devices) that has been setup correctly.

<div class="info" markdown="span">
Do not disturb the device while validation is in progress as it may cause the device to fail the
validation checks. If a device fails validation, but is setup correctly, you can retry the
validation by re-selecting the device.
</div>
