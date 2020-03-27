---
title: Qualcomm Counters
layout: default
permalink: /docs/gpu-counters/qualcomm
---

* `% Anisotropic Filtered`
  * Percent of texels filtered using the 'Anisotropic' sampling method.
* `% Linear Filtered`
  * Percent of texels filtered using the 'Linear' sampling method.
* `% Nearest Filtered`
  * Percent of texels filtered using the 'Nearest' sampling method.
* `% Non-Base Level Textures`
  * Percent of texels coming from a non-base MIP level.
* `% Prims Clipped`
  * Percentage of primitives clipped by the GPU (where new primitives are generated).
For a primitive to be clipped, it has to have a visible portion inside the viewport but extend outside the 'guardband' - an area that surrounds the viewport and significantly reduces the number of primitives the hardware has to clip.
* `% Prims Trivially Rejected`
  * Percentage of primitives that are trivially rejected.
A primitive can be trivially rejected if it is outside the visible region of the render surface.  These primitives are ignored by the rasterizer.
* `% Shader ALU Capacity Utilized`
  * Percent of maximum shader capacity (ALU operations) utilized.
For each cycle that the shaders are working, the average percentage of the total shader ALU capacity that is utilized for that cycle.
* `% Shaders Busy`
  * Percentage of time that all Shader cores are busy.
* `% Stalled on System Memory`
  * Percentage of cycles the L2 cache is stalled waiting for data from system memory.
* `% Texture Fetch Stall`
  * Percentage of clock cycles where the shader processors cannot make any more requests for texture data.
A high value for this metric implies the shaders cannot get texture data from the texture pipe (L1, L2 cache or memory) fast enough, and rendering performance may be negatively affected.
* `% Texture L1 Miss`
  * Number of L1 texture cache misses divided by L1 texture cache requests.
This metric does not consider how many texture requests are made per time period (like the '% GPU L1 Texture cache miss' metric), but is simple miss to request ratio.
* `% Texture L2 Miss`
  * Number of L2 texture cache misses divided by L2 texture cache requests.
This metric does not consider how many texture requests are made per time period, but is simple miss to request ratio.
* `% Time ALUs Working`
  * Percentage of time the ALUs are working while the Shaders are busy.
* `% Time Compute`
  * Amount of time spent in compute work compared to the total time spent shading everything.
* `% Time EFUs Working`
  * Percentage of time the EFUs are working while the Shaders are busy.
* `% Time Shading Fragments`
  * Amount of time spent shading fragments compared to the total time spent shading everything.
* `% Time Shading Vertices`
  * Amount of time spent shading vertices compared to the total time spent shading everything.
* `% Vertex Fetch Stall`
  * Percentage of clock cycles where the GPU cannot make any more requests for vertex data.
A high value for this metric implies the GPU cannot get vertex data from memory fast enough, and rendering performance may be negatively affected.   
* `ALU / Fragment`
  * Average number of scalar fragment shader ALU instructions issued per shaded fragment, expressed as full precision ALUs (2 mediump = 1 fullp).
Includes interpolation instruction.  Does not include vertex shader instructions.
* `ALU / Vertex`
  * Average number of vertex scalar shader ALU instructions issued per shaded vertex.
Does not include fragment shader instructions.
* `Average Polygon Area`
  * Average number of pixels per polygon.
Adreno's binning architecture will count a primitive for each 'bin' it covers, so this metric may not exactly match expectations.
* `Average Vertices / Polygon`
  * Average number of vertices per polygon.
This will be around 3 for triangles, and close to 1 for triangle strips.
* `Avg Bytes / Fragment`
  * Average number of bytes transferred from main memory for each fragment.
* `Avg Bytes / Vertex`
  * Average number of bytes transferred from main memory for each vertex.
* `Avg Preemption Delay`
  * Average time (us) from the preemption request to preemption start.
* `Clocks / Second`
  * Number of GPU clocks per second.
* `EFU / Fragment`
  * Average number of scalar fragment shader EFU instructions issued per shaded fragment.
Does not include Vertex EFU instructions
* `EFU / Vertex`
  * Average number of scalar vertex shader EFU instructions issued per shaded vertex.
Does not include fragment EFU instructions
* `Fragment ALU Instructions / Sec (Full)`
  * Total number of full precision fragment shader instructions issued, per second.
Does not include medium precision instructions or texture fetch instructions.
* `Fragment ALU Instructions / Sec (Half)`
  * Total number of half precision Scalar fragment shader instructions issued, per second.
Does not include full precision instructions or texture fetch instructions.
* `Fragment EFU Instructions / Second`
  * Total number of Scalar fragment shader Elementary Function Unit (EFU) instructions issued, per second.
These include math functions like sin, cos, pow, etc.
* `Fragment Instructions / Second`
  * Total number of fragment shader instructions issued, per second.
Reported as full precision scalar ALU instructions - 2 medium precision instructions equal 1 full precision instruction. Also includes interpolation instructions (which are executed on the ALU hardware) and EFU (Elementary Function Unit) instructions. Does not include texture fetch instructions.
* `Fragments Shaded / Second`
  * Number of fragments submitted to the shader engine, per second.
* `GPU % Bus Busy`
  * Approximate Percentage of time the GPU's bus to system memory is busy.
* `GPU % Utilization`
  * Percentage of GPU utilized as measured at peak GPU clock(585Mhz) and capacity
* `GPU Frequency`
  * GPU frequency in Hz
* `L1 Texture Cache Miss Per Pixel`
  * Average number of Texture L1 cache misses per pixel.
Lower values for this metric imply better memory coherency.  If this value is high, consider using compressed textures, reducing texture usage, etc.
* `Pre-clipped Polygons/Second`
  * Number of polygons submitted to the GPU, per second, before any hardware clipping.
* `Preemptions / second`
  * The number of GPU preemptions that occurred, per second.
* `Read Total (Bytes/sec)`
  * Total number of bytes read by the GPU from memory, per second.
* `Reused Vertices / Second`
  * Number of vertices used from the post-transform vertex buffer cache.
A vertex may be used in multiple primitives; a high value for this metric (compared to number of vertices shaded) indicates good re-use of transformed vertices, reducing vertex shader workload.
* `SP Memory Read (Bytes/Second)`
  * Bytes of data read from memory by the Shader Processors, per second.
* `Texture Memory Read BW (Bytes/Second)`
  * Bytes of texture data read from memory per second.
Includes bytes of platform compressed texture data read from memory.
* `Textures / Fragment`
  * Average number of textures referenced per fragment.
* `Textures / Vertex`
  * Average number of textures referenced per vertex.
* `Vertex Instructions / Second`
  * Total number of scalar vertex shader instructions issued, per second.
Includes full precision ALU vertex instructions and EFU vertex instructions.  Does not include medium precision instructions (since they are not used for vertex shaders). Does not include vertex fetch or texture fetch instructions.
* `Vertex Memory Read (Bytes/Second)`
  * Bytes of vertex data read from memory per second.
* `Vertices Shaded / Second`
  * Number of vertices submitted to the shader engine, per second.
* `Write Total (Bytes/sec)`
  * Total number of bytes written by the GPU to memory, per second.
