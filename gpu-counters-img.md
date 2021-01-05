---
title: IMG GPU Counters
layout: default
permalink: /docs/gpu-counters/img
---

* `Triangles In`
  * Number of triangles submitted to the tiling hardware.
* `Triangles Input per Draw`
  * Average number of triangles input to the tiling hardware per draw.
* `Triangles Out`
  * Number of triangles output after back-face, off-screen and sub-pixel culling.
* `Triangle Ratio`
  * Ratio of the number of triangles output over input, note that while culling removes triangles reducing the ratio, clipping may increase the ratio.
* `Vertex Sharing`
  * Percentage average of the number of transformed vertices being shared across primitives, a higher value indicates more efficient vertex sharing, resulting in more efficient cache utilisation.
* `Tiles per Triangle`
  * Average number of tiles a triangle touches, this indicates how many tiles need to be processed per primitive. A large value may increase the amount of work required during rasterization.
* `Geometry Load`
  * Percentage of time the tile binning hardware spent processing primitives.
* `HSR Efficiency`
  * Percentage of pixels submitted to the Hidden Surface Removal hardware compared to the pixels that were actually shaded. For example a value of 50% means that half of the pixels were rejected before fragment processing, saving valuable processing resources.
* `Overdraw`
  * Percentage of on-screen pixels that were drawn more than once. For example a value of 0% means that every on-screen pixel was shaded exactly once, while a value of 100% means that every on-screen pixel was shaded exactly twice.
* `ISP Pixel Load`
  * Percentage of time the rasterization hardware spent processing pixels.
* `Z/Stencil Load/Store`
  * Percentage of time the rasterization hardware spent loading and storing the Z/Stencil buffer.
* `ISP Throughput`
  * Percentage of tiles that the rasterization hardware is processing concurrently during a 3D workload, a higher value implies better rasterization HW utilisation. A low value could be the result of the renderpass requiring more memory per pixel due to MRT/PLS setup.
* `Pixels Out`
  * Number of pixels written by the GPU to main memory.
* `PBE Load`
  * Percentage of time the pixel back-end spent processing pixels. A high value indicates the application is using high resolution render surface(s).
* `Vertex Shader Load`
  * Percentage average of the time spent processing vertex shaders.
* `Fragment Shader Load`
  * Percentage average of the time spent processing fragment shaders.
* `Compute Shader Load`
  * Percentage average of the time spent processing compute shaders.
* `Shaded Vertices`
  * Number of vertices shaded by the shader core.
* `Shaded Fragments`
  * Number of fragments shaded by the shader core.
* `Processed Kernels`
  * Number of compute kernels processed by the shader core.
* `Cycles per Vertex`
  * Average number of cycles the shader core spent processing vertices.
* `Cycles per Pixel`
  * Average number of cycles the shader core spent processing pixels.
* `Cycles per Kernel`
  * Average number of cycles the shader core spent processing compute kernels.
* `Register Overload`
  * Percentage of time the shader core was stalled due to temporary register pressure.
* `Texturing Unit Active`
  * Percentage of time the texturing unit spent processing texture requests.
* `Texturing Unit Overload`
  * Percentage of time the shader core was stalled waiting for a texture request.
* `Memory Interface Load`
  * Percentage utilisation of the interface between the GPU and main memory.
* `GPU Clock Speed`
  * Current GPU clock in Hz. This may change during a profiling session due to DVFS etc.
