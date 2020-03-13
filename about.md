---
title: About
layout: default
permalink: /about/
---

AGI is a developer tool for recording and inspecting calls made by an application to the graphics driver.

It is open-source: [{{site.project-url}}]({{site.project-url}})

[Download an AGI release]({{site.project-url}}/releases)

Unstable developer releases are also available [here](https://github.com/google/agi-dev-releases).

<div style="text-align: center;">
    <img src="../images/hero.gif" alt="AGI image" width="540" height="337">
    <figcaption>Using AGI to step through each individual draw call of a frame</figcaption>
</div>

Once a capture of a target application has been made, AGI lets you disconnect from the target and inspect all the graphics commands made by the application.

AGI is able to replay the command stream, letting you visualize the frame composition by stepping through each command and inspecting the driver state at any point in the stream. Replay also supports modifications, allowing you to adjust command parameters and shader source to instantly see what effect this would have on the frame.

AGI can also visualize the textures, shaders and draw call geometry used by the application.

## API support

|                              | Android | Windows | macOS  | Linux | Stadia
| ---------------------------- | ------- | ------- |------- | ----- | ------
| Vulkan - Trace               |   <i class="material-icons check">check</i>     |   <i class="material-icons check">check</i>     |        |   <i class="material-icons check">check</i>   |   <i class="material-icons check">check</i>
| Vulkan - Replay <sup>*</sup> |   <i class="material-icons check">check</i>     |   <i class="material-icons check">check</i>     |        |   <i class="material-icons check">check</i>   |   <i class="material-icons check">check</i>

<sup>* Vulkan replay currently needs to be performed on the same device used to trace.</sup>

While AGI is primarily targeted for games developers, it can also help developers to inspect low-level 2D graphics calls made by the Android graphics framework.

AGI is under active development and has some [known issues]({{site.project-url}}/issues). Your feedback is appreciated!
