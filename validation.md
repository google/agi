---
layout: default
title: Device Support Validation
# This URL is referenced from within the application.
permalink: /validation
---

<div class="info" markdown="span">
Do not disturb the device while device support validation is in progress as it may cause the device to fail the
validation checks. If a device fails validation, but is setup correctly, you can retry the
support validation by re-selecting the device.
</div>

## Why is my device not supported?

AGI needs a compatible GPU driver to run. To ensure the tool provides good and valid profiling data, we run a validation check when you first connect a new device.

If your device is listed in the [supported devices](docs/devices) page, you can expect AGI to pass validation.

If your device is not listed, then it is likely that your device does not have a compatible GPU driver.

## When will AGI support my device?

We will also periodically update our supported device page with devices that support AGI. We are working with our OEM partners to add support to more devices.

Please [file an issue] in our GitHub repository to request support for a device. We will connect you to the OEM in question.

## What should I expect? I use a supported device.

Device validation takes about 10 seconds, during which you should see a spinning cube on the device screen. Once a device passes the validation process, it can be used to profile Android applications. 

## How often do I need to validate?

Device validation is a one-time step and results are cached for future runs of AGI. If the device setup changes, such as updating to a different version of Android or the GPU driver, validation will run again. 

## I’m using a supported device and it still fails. What now?

Please [file an issue] in our GitHub repository describing the bug you’re encountering.

[file an issue]: https://github.com/google/agi/issues