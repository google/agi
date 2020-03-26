---
layout: home
title: Device Validation
permalink: /validation
---

# Device validation

Android GPU Inspector only works with GPU profiling compatible devices. In order to validate whether the connected device is GPU profiling compatible, the AGI collects a trace by running a [Vulkan sample application](https://github.com/google/agi/tree/master/cmd/vulkan_sample) and validates the trace values. This is called **validation**. A device that doesn't pass the validation is considered not GPU profiling compatible and hence can not be used with AGI.

Please <a href="https://services.google.com/fb/forms/androidgpuinspectordeveloperpreview/">sign up</a>
for the developer preview of Android GPU Inspector to get access to the device bits required to pass
validation.

## Technical details

In order to be considered GPU profiling compatible, a device must emit required trace packets, currently there are 3 categories that must be emitted:

* GPU slices
* GPU counters
* Vukan event slices

### GPU slices

Each GPU slice must contain below fields as part of the trace packet:

* `command_buffer_handle`
* `submission_id`

If a GPU slice is a non-compute GPU slice, below fields must also be emitted:

* `render_target_handle`
* `render_pass_handle`

### GPU counters

AGI picks a list of GPU counters based on the type of the GPU, and does a sanity check on the values of the counters. Currently, AGI checks GPU counters on Adreno GPUs and Mali GPUs.

### Vulkan Api Event slices

In order to properly correlate GPU work and CPU work, the GPU must emit what we call [Vulkan Api Event](https://android.googlesource.com/platform/external/perfetto/+/refs/heads/master/protos/perfetto/trace/gpu/vulkan_api_event.proto) slices for a Vulkan app. Currently, the required Vulkan Api Event is `vkQueueSubmit`.
