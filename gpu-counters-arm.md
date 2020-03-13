---
title: ARM Counters
layout: default
permalink: /gpu-counters/arm
---

* `MaliFragActiveCy`
  * Fragment active cycles. This counter increments every cycle where the shader core is processing some fragment workload. Active processing includes any cycle where fragment work is running anywhere in the fixed-function front-end, back-end, or the in the programmable core. Hardware name: FRAG_ACTIVE.
* `MaliGeomFaceCullPrim`
  * Facing test culled primitives. This counter increments for every primitive culled by the facing test. For an arbitrary 3D scene we would expect approximately half of the triangles to be back-facing; if you see a significantly lower percentage than this, check that the facing test is properly enabled. Hardware name: PRIM_CULLED.
* `MaliCoreFullQdWarp`
  * Full quad warps. This counter increments for every warp that is fully populated with quads. If there are many warps that are not full then performance can be lower, because thread slots in the warp are unused. Full warps are more likely if: * Compute shaders have work groups that are a multiple of warp size. * Draw calls use meshes without very small primitives. Hardware name: FULL_QUAD_WARPS.
* `MaliTilerUTLBLookup`
  * UTLB lookup requests. This counter increments for every address translation lookup into the tiler micro-TLB. Hardware name: UTLB_TRANS.
* `MaliTilerUTLBHit`
  * UTLB lookup hits. This counter increments for every address translation lookup into the tiler micro-TLB that hits a valid entry. Hardware name: UTLB_TRANS_HIT.
* `MaliTilerPtrCacheMiss`
  * Pointer cache misses. This counter increments every time a pointer lookup misses in the bin pointer cache. Hardware name: PCACHE_MISS.
* `MaliGPUActiveCy`
  * GPU active cycles. This counter increments every clock cycle where the GPU has any pending workload present in one of its processing queues, and therefore shows the overall GPU processing load requested by the application. This counter will increment every clock cycle where any workload is present in a processing queue, even if the GPU is stalled waiting for external memory to return data; this is still counted as active time even though no forward progress is being made. Hardware name: GPU_ACTIVE.
* `MaliCompActiveCy`
  * Compute active cycles. This counter increments every cycle that the shader core is processing some compute workload. Active processing includes any cycle where compute work is running anywhere in the fixed-function front-end or the in the programmable core. Hardware name: COMPUTE_ACTIVE.
* `MaliTilerOverflowBins`
  * Tile list overflow allocations. This counter increments for every overflow tiler bin allocation made. These bins are 512 bytes in size. Hardware name: BIN_ALLOC_OVERFLOW.
* `MaliSentMsg`
  * Sent messages. This counter increments for each message sent by the Job Manager to another internal GPU subsystem. Hardware name: MESSAGES_SENT.
* `MaliTilerWrBuffMissCy`
  * Write buffer misses. This counter increments every time a new write misses in the write buffer. Hardware name: WRBUF_MISS.
* `MaliEngInstr`
  * Executed instructions. This counter increments for every instruction that the execution engine processes per warp. All instructions are single cycle issue. Hardware name: EXEC_INSTR_COUNT.
* `MaliFragQueueWaitFinishCy`
  * Fragment job finish wait cycles. This counter increments any clock cycle where the GPU has run out of new fragment work to issue, and is waiting for remaining work to complete. Hardware name: JS0_WAIT_FINISH.
* `MaliVarInstr`
  * Varying instructions. This counter increments for every warp-width interpolation operation processed by the varying unit. Hardware name: VARY_INSTR.
* `MaliFragQueueWaitFlushCy`
  * Fragment cache flush wait cycles. This counter increments any clock cycle where the GPU has fragment work queued that can not run or retire due to a pending L2 cache flush. Hardware name: JS0_WAIT_FLUSH.
* `MaliTilerPtrMgrStallCy`
  * Pointer manager busy stall cycles. This counter increments every cycle where the write compressor has data that is ready to be sent, but the pointer manager is not ready. Hardware name: COMPRESS_STALL.
* `MaliTilerDescStallCy`
  * Loading descriptor wait cycles. The number of clock cycles where the tiler front-end is idle and waiting for a descriptor load. Hardware name: LOADING_DESC.
* `MaliTilerVarCacheDealloc`
  * Varying cache line deallocation requests. This counter increments every time a vertex varying cache line is deallocated in the vertex cache. Hardware name: IDVS_VBU_LINE_DEALLOCATE.
* `MaliCompTask`
  * Compute tasks. This counter increments for every compute task issued to the shader core. The size of these tasks is variable. Hardware name: COMPUTE_TASKS.
* `MaliSCBusFFERdBt`
  * Fragment front-end read beats from L2 cache. This counter increments for every read beat received by the fixed-function fragment front-end. Hardware name: BEATS_RD_FTC.
* `MaliSCBusOtherRdBt`
  * Miscellaneous read beats from L2 cache. This counter increments for every read beat received by any unit that is not identified as a specific data destination. Hardware name: BEATS_RD_OTHER.
* `MaliL2CacheIncSnpData`
  * Input external snoop transactions that hit with data response. This counter increments for every external snoop transaction that hits a dirty line in the GPU cache, and returns data. Hardware name: L2_EXT_SNOOP_RESP_DATA.
* `MaliFragPartWarp`
  * Partial fragment warps. This counter increments for every created fragment warp containing helper threads that do not correspond to a hit sample point. Partial coverage in a fragment quad occurs if any of its sample points span the edge of a triangle, or if one or more covered sample points fail an early ZS test. Partial coverage in a warp occurs if any quads it contains have partial coverage. Hardware name: FRAG_PARTIAL_WARPS.
* `MaliTilerCompressReset`
  * Compressor stream reset requests. This counter increments for every cache miss in the write compressor cache. Entries which hit in this cache can use efficient delta compression, but after a miss the compression scheme must restart with absolute values. Hardware name: COMPRESS_MISS.
* `MaliCoreAllRegsWarp`
  * Warps using more than 32 registers. This counter increments for every warp that requires more than 32 registers. Threads which require more than 32 registers consume two thread slots in the register file, halving the number of threads that can be concurrently active in the shader core. Reduction in thread count can impact the ability of the shader core to keep functional units busy, and means that performance is more likely to be impacted by stalls caused by cache misses. Aim to minimize the number of threads requiring more than 32 registers, by using simpler shader programs and lower precision data types. Hardware name: WARP_REG_SIZE_64.
* `MaliExtBusRdNoSnoop`
  * Output external ReadNoSnoop transactions. This counter increments for every non-coherent (ReadNoSnp) transaction. Hardware name: L2_EXT_READ_NOSNP.
* `MaliTilerUTLBLat`
  * UTLB lookup wait cycles. This counter increments every 64 cycle wait period that a micro-TLB lookup is waiting for a result from the main MMU. Hardware name: UTLB_TRANS_MISS_DELAY.
* `MaliNonFragQueueWaitIssueCy`
  * Non-fragment job issue wait cycles. This counter increments any clock cycle where the GPU has non-fragment work queued that can not run because all processor resources are busy. Hardware name: JS1_WAIT_ISSUE.
* `MaliTilerPtrCacheHit`
  * Pointer cache hits. This counter increments every time a pointer lookup  that result in a successful hit in the bin pointer cache. Hardware name: PCACHE_HIT.
* `MaliExtBusRdUnique`
  * Output external ReadUnique transactions. This counter increments for every coherent exclusive read (ReadUnique) transaction. Hardware name: L2_EXT_READ_UNIQUE.
* `MaliExtBusRdLat0`
  * Output external read latency 0-127 cycles. This counter increments for every data beat that is returned between 0 and 127 cycles after the read transaction started. Hardware name: L2_EXT_RRESP_0_127.
* `MaliL2CacheSnpStallCy`
  * Input internal snoop stall cycles. This counter increments for every clock cycle where an L2 cache coherency snoop request from an internal master is stalled. Hardware name: L2_SNP_MSG_IN_STALL.
* `MaliMMUL2Rd`
  * MMU L2 table read requests. This counter increments for every read of a level 2 MMU translation table entry. Each address translation at this level covers a 2MB section, which is typically broken down into further into 4KB pages via a subsequent level 3 translation table lookup. Hardware name: MMU_TABLE_READS_L2.
* `MaliL2CacheWr`
  * Input internal write requests. This counter increments for every write request received by the L2 cache from an internal master. Hardware name: L2_WR_MSG_IN.
* `MaliTilerVertLoadStallCy`
  * Vertex prefetcher stall cycles. This counter increments every cycle where the vertex prefetcher has valid data, but the vertex loader can not accept new requests. Hardware name: PREFETCH_STALL.
* `MaliTexCacheCompressFetch`
  * Compressed texture line fetch requests. This counter increments for every texture line fetch from the L2 cache that is a block compressed texture. Note that this counter excludes lines compressed using the AFBC lossless compression scheme. Hardware name: TEX_TFCH_NUM_LINES_FETCHED_BLOCK_COMPRESSED.
* `MaliFragEZSTestQd`
  * Early ZS tested quads. This counter increments for every quad undergoing early depth and stencil testing. For maximum performance, this number should be close to the total number of input quads. We want as many of the input quads as possible to be subject to early ZS testing as it is significantly more efficient than late ZS testing, which will only kill threads after they have been fragment shaded. Hardware name: FRAG_QUADS_EZS_TEST.
* `MaliMMUS2L3Rd`
  * MMU stage 2 L3 lookup requests. This counter increments for every read of a stage 2 level 3 MMU translation table entry. Each address translation at this level covers a single 4KB page. Hardware name: MMU_S2_TABLE_READS_L3.
* `MaliSCBusLSExtRdBt`
  * Load/store read beats from external memory. This counter increments for every read beat received by the load/store unit that required an external memory access due to an L2 cache miss. Hardware name: BEATS_RD_LSC_EXT.
* `MaliFragQueueWaitIssueCy`
  * Fragment job issue wait cycles. This counter increments any clock cycle where the GPU has fragment work queued that can not run because all processor resources are busy. Hardware name: JS0_WAIT_ISSUE.
* `MaliTilerPosRdCyStall`
  * Position fetcher read stall cycles. This counter increments every cycle where an index is ready to be sent to the primitive assembly, but its position data is not ready. Hardware name: VFETCH_VERTEX_WAIT.
* `MaliExtBusRdLat320`
  * Output external read latency 320-383 cycles. This counter increments for every data beat that is returned between 320 and 383 cycles after the read transaction started. Hardware name: L2_EXT_RRESP_320_383.
* `MaliTilerCompStallCy`
  * Compressor busy stall cycles. This counter increments every cycle where the tiler write iterator has valid data, but the write compressor can not accept new requests. Hardware name: ITER_STALL.
* `MaliMMUS2L2Rd`
  * MMU stage 2 L2 lookup requests. This counter increments for every read of a stage 2 level 2 MMU translation table entry. Each address translation at this level covers a 2MB section. Hardware name: MMU_S2_TABLE_READS_L2.
* `MaliSCBusFFEExtRdBt`
  * Fragment front-end read beats from external memory. This counter increments for every read beat received by the fixed-function fragment front-end that required an external memory access due to an L2 cache miss. Hardware name: BEATS_RD_FTC_EXT.
* `MaliResQueueWaitDepCy`
  * Reserved job dependency wait cycles. This counter increments any clock cycle where the GPU has reserved work queued that can not run until dependent work is completed. Hardware name: JS2_WAIT_DEPEND.
* `MaliResQueueWaitFlushCy`
  * Reserved cache flush wait cycles. This counter increments any clock cycle where the GPU has reserved work queued that can not run or retire due to a pending L2 cache flush. Hardware name: JS2_WAIT_FLUSH.
* `MaliTilerBinStallCy`
  * Tile binning unit busy stall cycles. This counter increments every cycle where a primitive has a bounding box and is ready for binning, but the binning unit is not ready. Hardware name: BBOX_GEN_STALL.
* `MaliSCBusLSWBWrBt`
  * Internal load/store writeback write beats. This counter increments for every write beat by the load/store unit that are due to writeback. Hardware name: BEATS_WR_LSC_WB.
* `MaliFragRdPrim`
  * Fragment read primitives. This counter increments for every primitive read from the tile list by the fragment front-end. Note that this counter will increment per tile, which means that a single primitive that spans multiple tiles will increment this counter multiple times. Hardware name: FRAG_PRIMITIVES.
* `MaliResQueueWaitRdCy`
  * Reserved job descriptor read wait cycles. This counter increments any clock cycle where the GPU has reserved work queued that can not run due to a pending descriptor load from memory. Hardware name: JS2_WAIT_READ.
* `MaliTilerMMULookup`
  * Main MMU lookup requests. This counter increments for every main MMU lookup made by the tiler micro-TLB. Hardware name: UTLB_MMU_REQ.
* `MaliFragQueueActiveCy`
  * Fragment queue active cycles. This counter increments every clock cycle where the GPU has any workload present in the fragment queue. For most graphics content there are significantly more fragments than vertices, so this queue will normally have the highest processing load. In content that is GPU bound by fragment processing it is normal for {{MaliFragQueueActiveCy}} to be approximately equal to {{MaliGPUActiveCy}}, with vertex and fragment processing running in parallel. This counter will increment any clock cycle where a workload is loaded into a queue even if the GPU is stalled waiting for external memory to return data; this is still counted as active time even though no forward progress is being made. Hardware name: JS0_ACTIVE.
* `MaliNonFragQueueWaitRdCy`
  * Non-fragment job descriptor read wait cycles. This counter increments any clock cycle where the GPU has non-fragment work queued that can not run due to a pending descriptor load from memory. Hardware name: JS1_WAIT_READ.
* `MaliMMUS2L3Hit`
  * MMU stage 2 L3 lookup TLB hits. This counter increments for every read of a stage 2 level 3 MMU translation table entry that results in a successful hit in the main MMU's TLB. Hardware name: MMU_S2_HIT_L3.
* `MaliExtBusWrSnoopPart`
  * Output external WriteSnoopPartial transactions. This counter increments for every external coherent partial write (WriteBackPtl or WriteUniquePtl) transaction. Hardware name: L2_EXT_WRITE_SNP_PTL.
* `MaliTilerPosRdCy`
  * Position fetcher read cycles. This counter increments every cycle where the tiler reads post-transform vertex position data. Hardware name: VFETCH_POS_READ_WAIT.
* `MaliL2CacheL1Rd`
  * Output internal read requests. This counter increments for every L1 cache read request or read response sent by the L2 cache to an internal master. Read requests are triggered by a snoop request from one master that needs data from another master's L1 to resolve. Read responses are standard responses back to a master in response to its own read requests. Hardware name: L2_RD_MSG_OUT.
* `MaliEngStarveCy`
  * Execution engine starvation cycles. This counter increments every cycle where the execution engine contains threads to execute, and is ready to accept a new instruction, but no new thread is available to execute. This typically occurs when all threads are blocked waiting for the result from an asynchronous processing operation, such as a texture filtering operation or a data fetch from memory. Hardware name: EXEC_INSTR_STARVING.
* `MaliTilerVarCacheMiss`
  * Varying cache misses. This counter increments every time a vertex varying lookup misses in the vertex cache. Cache misses at this stage will result in a varying shading request, although a single request may produce data to handle multiple cache misses. Hardware name: IDVS_VBU_MISS.
* `MaliCompWarp`
  * Compute warps. This counter increments for every created compute warp. To ensure full utilization of the warp capacity any compute workgroups should be a multiple of warp size. Note that the warp width varies between Mali devices. On this GPU, the number of threads in a single warp is 8. Hardware name: COMPUTE_WARPS.
* `MaliSCBusTileWrBt`
  * Tile buffer write beats to L2 memory system. This counter increments for every write beat sent by the tile buffer writeback unit. Hardware name: BEATS_WR_TIB.
* `MaliTilerPosShadStallCy`
  * IDVS position shading stall cycles. This counter increments every cycle where the tiler has a position shading request that it can not send to a shader core because the shading request queue is full. Hardware name: IDVS_POS_SHAD_STALL.
* `MaliExtBusWrOTQ3`
  * Output external outstanding writes 50-75%. This counter increments for every write transaction initiated when 50-75% of the available transaction IDs are in use. Hardware name: L2_EXT_AW_CNT_Q3.
* `MaliExtBusWrOTQ2`
  * Output external outstanding writes 25-50%. This counter increments for every write transaction initiated when 25-50% of the available transaction IDs are in use. Hardware name: L2_EXT_AW_CNT_Q2.
* `MaliExtBusWrOTQ1`
  * Output external outstanding writes 0-25%. This counter increments for every write transaction initiated when 0-25% of the available transaction IDs are in use. Hardware name: L2_EXT_AW_CNT_Q1.
* `MaliMMULookup`
  * MMU lookup requests. This counter increments for every address lookup made by the main GPU MMU. This occurs only if all lookups into a local TLB miss. Hardware name: MMU_REQUESTS.
* `MaliL2CacheRdLookup`
  * Read lookup requests. This counter increments for every L2 cache read lookup made. Hardware name: L2_READ_LOOKUP.
* `MaliTilerPtrCacheRdStallCy`
  * Pointer manager read stall cycles. This counter increments every cycle that the tiler is waiting for existing bin pointer data to be read from memory. Hardware name: PMGR_PTR_RD_STALL.
* `MaliTilerWrBuffActiveCy`
  * Write buffer active cycles. This counter increments every cycle that the tiler write buffer contains data still be written. Hardware name: WRBUF_ACTIVE.
* `MaliFragEZSKillQd`
  * Early ZS killed quads. This counter increments for every quad killed by early depth and stencil testing. It is common to see a proportion of quads killed at this point in the pipeline, because early ZS is effective at handling depth-based occlusion inside the view frustum, and can reduce the need for perfect culling in the application. However, if a very high percentage of quads are being killed at this stage, this can indicate that improvements in application culling are possible, such as the use of potential visibility sets or portal culling to cull objects in different rooms. Hardware name: FRAG_QUADS_EZS_KILL.
* `MaliNonFragQueueTask`
  * Non-fragment tasks. This counter increments for every non-fragment task that is processed by the GPU. Hardware name: JS1_TASKS.
* `MaliLSAtomic`
  * Load/store atomic issues. This counter increments for every load/store atomic access. Atomic memory accesses are typically multi-cycle operations per thread in the warp, so they are exceptionally expensive. Minimize the use of atomics in performance critical code. Hardware name: LS_MEM_ATOMIC.
* `MaliEngMulInstr`
  * Arithmetic multiplication instructions. This counter increments every instruction issue where the workload is a single fused multiply-accumulate pipe arithmetic operation and the operation requires use of the multiplier hardware. Hardware name: ARITH_INSTR_FP_MUL.
* `MaliExtBusWr`
  * Output external write transactions. This counter increments for every external write transaction made on the memory bus. These transactions will typically result in an external DRAM access, but some chips include a system cache which can provide some buffering. The longest memory transaction possible is 64 bytes in length, but shorter transactions can be generated in some circumstances. Hardware name: L2_EXT_WRITE.
* `MaliFragRastQd`
  * Rasterized quads. This counter increments for every quad generated by the rasterization phase. The quads generated have at least some coverage based on the current sample pattern, but may subsequently be killed by early ZS testing or forward pixel kill (FPK) hidden surface removal before they are shaded. Each 2x2 quad typically covers a 2x2 pixel screen region, but additional quads may be generated if the application is using sample-rate shading. Hardware name: FRAG_QUADS_RAST.
* `MaliL2CacheSnp`
  * Input internal snoop requests. This counter increments for every coherency snoop request received by the L2 cache from internal masters. Hardware name: L2_SNP_MSG_IN.
* `MaliL2CacheL1RdStallCy`
  * Output internal read stall cycles. This counter increments for every clock cycle where L1 cache read requests and responses sent by the L2 cache to an internal master are stalled. Hardware name: L2_RD_MSG_OUT_STALL.
* `MaliFragQueueWaitRdCy`
  * Fragment job descriptor read wait cycles. This counter increments any clock cycle where the GPU has fragment work queued that can not run due to a pending descriptor load from memory. Hardware name: JS0_WAIT_READ.
* `MaliGeomSampleCullPrim`
  * Sample test killed primitives. This counter increments for every primitive culled by the sample coverage test. It is expected that a small number of primitives will be small and fail the sample coverage test, as application mesh level-of-detail selection can never be perfect. If the number of primitives counted is more than than 5-10% of the total number, this might indicate that the application has a large number of very small triangles, which are very expensive for a GPU to process. Aim to keep triangle screen area above 10 pixels, using schemes such as mesh level-of-detail to select simplified meshes as objects move further away from the camera. Hardware name: PRIM_SAT_CULLED.
* `MaliL2CacheRd`
  * Input internal read requests. This counter increments for every read request received by the L2 cache from an internal master. Hardware name: L2_RD_MSG_IN.
* `MaliSCBusTexRdBt`
  * Texture read beats from L2 cache. This counter increments for every read beat received by the texture unit. Hardware name: BEATS_RD_TEX.
* `MaliResQueueTask`
  * Reserved queue tasks. This counter increments for every task that is processed by the GPU reserved queue. Hardware name: JS2_TASKS.
* `MaliTilerPosCacheMiss`
  * Position cache miss requests. This counter increments every time a vertex position lookup misses in the vertex cache. Cache misses at this stage will result in a position shading request, although a single request may produce data to handle multiple cache misses. Hardware name: VCACHE_MISS.
* `MaliFragOpaqueQd`
  * FPK occluder quads. This counter increments for every quad that is a valid occluder for hidden surface removal. To be a candidate occluder, a quad must be guaranteed to be opaque and resolvable at early ZS. Draw calls that use blending, shader discard, alpha-to-coverage, programmable depth, or programmable tile buffer access can not be occluders. Hardware name: QUAD_FPK_KILLER.
* `MaliTilerPtrCacheMissStallCy`
  * Pointer cache line miss stall cycles. This counter increments every cycle where the tiler is waiting for a free cache line to become available in the bin pointer cache. Hardware name: PCACHE_MISS_STALL.
* `MaliGeomFrustumCullPrim`
  * Frustum test culled primitives. This counter increments for every primitive culled by the frustum test. It is expected that a small number of primitives will be outside of the frustum, as application culling is never perfect and some models may intersect a frustum clip plane. A good estimation is that under 10% of the primitives should be culled by this stage. If the number of primitives being culled is significantly higher than this, check the efficiency of any software culling schemes in the application. Use draw call bounding box checks to cull draws that are completely out-of-frustum. If batched draw calls are complex and have a large bounding volume consider using smaller batches to reduce the bounding volume to enable better culling. Hardware name: PRIM_CLIPPED.
* `MaliTilerPosShadFIFOFullCy`
  * IDVS position FIFO full cycles. This counter increments every cycle where the tiler has a position shading request that it can not send to a shader core because the position buffer is full. Hardware name: IDVS_POS_FIFO_FULL.
* `MaliTilerWrBuffOTStallCy`
  * Write buffer outstanding transaction stall cycles. This counter increments every cycle where the tiler write buffer has data to write, but stalls because there are no available write IDs for the internal bus. Hardware name: WRBUF_NO_AXI_ID_STALL.
* `MaliGPUIRQActiveCy`
  * Interrupt pending cycles. This counter increments every cycle that the GPU has an interrupt pending and is waiting for the CPU to process it. Note that cycles with a pending interrupt do not necessarily indicate lost performance because the GPU can process other queued work in parallel. However if {{MaliGPUIRQActiveCy}} is a high percentage of {{MaliGPUActiveCy}}, there could be an underlying problem that is preventing the CPU from handling interrupts efficiently. The most common cause for this is a misbehaving device driver, which may not be the Mali device driver, that has masked interrupts for a long period of time. Hardware name: IRQ_ACTIVE.
* `MaliFragQueueTask`
  * Fragment tasks. This counter increments for every 32x32 pixel region of a render pass that is processed by the GPU. The processed region of a render pass may be smaller than the full size of the attached surfaces if the application's viewport and scissor settings prevent the whole image being rendered. Hardware name: JS0_TASKS.
* `MaliGeomTrianglePrim`
  * Triangle primitives. This counter increments for every input triangle primitive. The count is made before any culling or clipping. Hardware name: TRIANGLES.
* `MaliTexCacheFetch`
  * Texture line fetch requests. This counter increments for every texture line fetch from the L2 cache. Hardware name: TEX_TFCH_NUM_LINES_FETCHED.
* `MaliEngDivergedInstr`
  * Diverged instructions. This counter increments for every instruction the execution engine processes per warp where there is control flow divergence across the warp. Control flow divergence erodes arithmetic execution efficiency because it implies some threads in the warp are idle because they did not take the current control path through the code. Aim to minimize control flow divergence when designing shader effects. Hardware name: EXEC_INSTR_DIVERGED.
* `MaliCoreActiveCy`
  * Execution core active cycles. This counter increments every cycle that the shader core is processing at least one warp. Note that this counter does not provide detailed information about how the functional units are utilized inside the shader core, but simply gives an indication that something was running. Hardware name: EXEC_CORE_ACTIVE.
* `MaliCompStarveCy`
  * Compute front-end starvation cycles. This counter increments every cycle where the shader core is processing a compute workload (see {{MaliCompActiveCy}}), and the execution core can accept a new thread, but no new thread is available to execute. Hardware name: COMPUTE_STARVING.
* `MaliExtBusRdLat256`
  * Output external read latency 256-319 cycles. This counter increments for every data beat that is returned between 256 and 319 cycles after the read transaction started. Hardware name: L2_EXT_RRESP_256_319.
* `MaliLSFullRd`
  * Load/store full read issues. This counter increments for every full-width load/store cache read. Hardware name: LS_MEM_READ_FULL.
* `MaliTilerBBoxStallCy`
  * Bounding box generator busy stall cycles. This counter increments every cycle that a primitive is ready for bounding box construction, but the bounding box generator is not ready. Hardware name: PRIMASSY_STALL.
* `MaliMMUL3Hit`
  * MMU L3 lookup TLB hits. This counter increments for every read of a level 3 MMU translation table entry that results in a successful hit in the main MMU's TLB. Hardware name: MMU_HIT_L3.
* `MaliFragTileKill`
  * Constant tiles killed. This counter increments for every tile killed by a transaction elimination CRC check. Hardware name: FRAG_TRANS_ELIM.
* `MaliNonFragQueueWaitFinishCy`
  * Non-fragment job finish wait cycles. This counter increments any clock cycle where the GPU has run out of new non-fragment work to issue, and is waiting for remaining work to complete. Hardware name: JS1_WAIT_FINISH.
* `MaliL2CacheSnpLookup`
  * Input external snoop lookup requests. This counter increments for every coherency snoop lookup performed that is triggered by a master outside of the GPU. Hardware name: L2_EXT_SNOOP_LOOKUP.
* `MaliExtBusRdBt`
  * Output external read beats. This counter increments for every clock cycle where a data beat was read from the external memory bus. Most implementations use a 128-bit (16 byte) data bus, enabling a single 64 byte read transaction to be read using 4 bus cycles. Hardware name: L2_EXT_READ_BEATS.
* `MaliSCBusTexExtRdBt`
  * Texture read beats from external memory. This counter increments for every read beat received by the texture unit that required an external memory access due to an L2 cache miss. Hardware name: BEATS_RD_TEX_EXT.
* `MaliNonFragQueueActiveCy`
  * Non-fragment queue active cycles. This counter increments every clock cycle where the GPU has any workload present in the non-fragment queue. This queue can be used for vertex shaders, tessellation shaders, geometry shaders, fixed function tiling, and compute shaders. This counter can not disambiguate between these workloads. This counter will increment any clock cycle where a workload is loaded into a queue even if the GPU is stalled waiting for external memory to return data; this is still counted as active time even though no forward progress is being made. Hardware name: JS1_ACTIVE.
* `MaliNonFragQueueWaitFlushCy`
  * Non-fragment cache flush wait cycles. This counter increments any clock cycle where the GPU has non-fragment work queued that can not run or retire due to a pending L2 cache flush. Hardware name: JS1_WAIT_FLUSH.
* `MaliFragStarveCy`
  * Fragment front-end starvation cycles. This counter increments every cycle where the shader core is processing a fragment workload (see {{MaliFragActiveCy}}), and the execution core can accept a new thread, but no new thread is available to execute. Hardware name: FRAG_STARVING.
* `MaliSCBusLSOtherWrBt`
  * Internal load/store other write beats. This counter increments for every write beat by the load/store unit that are due to any reason other than writeback. Hardware name: BEATS_WR_LSC_OTHER.
* `MaliL2CacheRdStallCy`
  * Input internal read stall cycles. This counter increments for every cycle an L2 cache read request from an internal master is stalled. Hardware name: L2_RD_MSG_IN_STALL.
* `MaliExtBusRdOTQ2`
  * Output external outstanding reads 25-50%. This counter increments for every read transaction initiated when 25-50% of the available transaction IDs are in use. Hardware name: L2_EXT_AR_CNT_Q2.
* `MaliExtBusRdOTQ3`
  * Output external outstanding reads 50-75%. This counter increments for every read transaction initiated when 50-75% of the available transaction IDs are in use. Hardware name: L2_EXT_AR_CNT_Q3.
* `MaliExtBusRdOTQ1`
  * Output external outstanding reads 0-25%. This counter increments for every read transaction initiated when 0-25% of the available transaction IDs are in use. Hardware name: L2_EXT_AR_CNT_Q1.
* `MaliEngAMInstr`
  * Arithmetic + Message instructions. This counter increments every instruction issue where the workload is one fused multiply-accumulate pipe arithmetic operation and one add pipe message operation. Hardware name: ARITH_INSTR_MSG.
* `MaliTilerJob`
  * Tiler jobs. This counter increments for every job processed by the tiler. This includes both legacy tiler jobs, and index-driven vertex shading (IDVS) jobs. Hardware name: JOBS_PROCESSED.
* `MaliL2CacheIncSnp`
  * Input external snoop transactions. This counter increments for every coherency snoop transaction received from an external master. Hardware name: L2_EXT_SNOOP.
* `MaliTilerPosCacheHit`
  * Position cache hit requests. This counter increments every time a vertex position lookup hits in the vertex cache. Hardware name: VCACHE_HIT.
* `MaliResQueueWaitFinishCy`
  * Reserved job finish wait cycles. This counter increments any clock cycle where the GPU has run out of new reserved work to issue, and is waiting for remaining work to complete. Hardware name: JS2_WAIT_FINISH.
* `MaliTilerPosCacheWaitCy`
  * Position cache line availability stall cycles. This counter increments every cycle that the tiler is waiting for a cache line to become available in the vertex position cache. Hardware name: VCACHE_LINE_WAIT.
* `MaliFragFPKActiveCy`
  * Forward pixel kill buffer active cycles. This counter increments every cycle that the pre-pipe quad queue contains at least one quad waiting to run. If this queue completely drains, it will not be possible to spawn a fragment warp when space for new threads becomes available in the shader core. This can result in reduced performance because low thread occupancy can starve the functional units of work to process. Possible causes for this include: * Tiles which contain no geometry. This is commonly encountered when creating shadow maps, where many tiles will contain no shadow casters. * Tiles which contain a lot of geometry which can be killed by early ZS or hidden surface removal. Hardware name: FRAG_FPK_ACTIVE.
* `MaliMMUS2L2Hit`
  * MMU stage 2 L2 lookup TLB hits. This counter increments for every read of a stage 2 level 2 MMU translation table entry that results in a successful hit in the main MMU's TLB. Hardware name: MMU_S2_HIT_L2.
* `MaliGPUCy`
  * GPU cycles. This counter increments every clock cycle where the GPU is powered.
* `MaliTexQuadPass`
  * Texture quad issues. This counter increments for every quad-width filtering pass. A single texture operation can be split into multiple passes due to: * Use of trilinear filtering * Use of anisotropic filtering * Use of a 3D texture format * Use of a cubemap texture format that requires data from multiple cube faces. Hardware name: TEX_DFCH_NUM_PASSES.
* `MaliTilerWrBuffFullStallCy`
  * Write buffer no free line stall cycles. This counter increments every cycle that the tiler write buffer has new data, but stalls because there are no available lines to store the data in. Hardware name: WRBUF_NO_FREE_LINE_STALL.
* `MaliTilerPtrCachePtrWrStallCy`
  * Pointer manager pointer writeback stall cycles. This counter increments every cycle that the tiler is waiting for new bin pointer data to be written to memory. Hardware name: PMGR_PTR_WR_STALL.
* `MaliExtBusWrBt`
  * Output external write beats. This counter increments for every clock cycle where a data beat was written to the external memory bus. Most implementations use a 128-bit (16 byte) data bus, enabling a single 64 byte read transaction to be written using 4 bus cycles. Hardware name: L2_EXT_WRITE_BEATS.
* `MaliExtBusWrNoSnoopFull`
  * Output external WriteNoSnoopFull transactions. This counter increments for every external non-coherent full write (WriteNoSnpFull) transaction. Hardware name: L2_EXT_WRITE_NOSNP_FULL.
* `MaliTilerPrimAsmStallCy`
  * Primitive assembly busy stall cycles. This counter increments every cycle where a vertex is ready to be sent to the primitive assembly stage, but primitive assembly is not ready. Hardware name: VFETCH_STALL.
* `MaliExtBusRdLat192`
  * Output external read latency 192-255 cycles. This counter increments for every data beat that is returned between 192 and 255 cycles after the read transaction started. Hardware name: L2_EXT_RRESP_192_255.
* `MaliTilerActiveCy`
  * Tiler active cycles. This counter increments every cycle the tiler has a workload in its processing queue. The tiler can run in parallel to vertex shading and fragment shading so a high cycle count here does not necessarily imply a bottleneck, unless the {{MaliCompActiveCy}} counters in the shader cores are very low relative to this. Hardware name: TILER_ACTIVE.
* `MaliL2CacheLookup`
  * Any lookup requests. This counter increments for every L2 cache lookup made. This includes all reads, writes, coherency snoops, and cache flush operations. Hardware name: L2_ANY_LOOKUP.
* `MaliVar16ActiveCy`
  * 16-bit interpolation active. This counter increments for every 16-bit interpolation cycle processed by the varying unit. Hardware name: VARY_SLOT_16.
* `MaliLSPartWr`
  * Load/store partial write issues. This counter increments for every partial-width load/store cache write. Partial data accesses do not make full use of the load/store cache capability, so efficiency can be improved by merging short accesses together to make fewer larger access requests. To do this in shader code: * Use vector data loads * Avoid padding in strided data accesses * Write compute shaders so that adjacent threads in a warp access adjacent addresses in memory. Hardware name: LS_MEM_WRITE_SHORT.
* `MaliExtBusWrSnoopFull`
  * Output external WriteSnoopFull transactions. This counter increments for every external coherent full write (WriteBackFull or WriteUniqueFull) transaction. Hardware name: L2_EXT_WRITE_SNP_FULL.
* `MaliExtBusRdLat128`
  * Output external read latency 128-191 cycles. This counter increments for every data beat that is returned between 128 and 191 cycles after the read transaction started. Hardware name: L2_EXT_RRESP_128_191.
* `MaliL2CacheIncSnpClean`
  * Input external snoop transactions that hit with no data response. This counter increments for every external snoop transaction that hits a clean line in the GPU cache, and does not return any data. Hardware name: L2_EXT_SNOOP_RESP_CLEAN.
* `MaliFragLZSTestQd`
  * Late ZS tested quads. This counter increments for every quad undergoing late depth and stencil testing. Hardware name: FRAG_LZS_TEST.
* `MaliGeomFrontFacePrim`
  * Visible front-facing primitives. This counter increments for every visible front-facing triangle that survives culling. Hardware name: FRONT_FACING.
* `MaliGeomBackFacePrim`
  * Visible back-facing primitives. This counter increments for every visible back-facing triangle that survives culling. Hardware name: BACK_FACING.
* `MaliFragQueueWaitDepCy`
  * Fragment job dependency wait cycles. This counter increments any clock cycle where the GPU has fragment work queued that can not run until dependent work has completed. Hardware name: JS0_WAIT_DEPEND.
* `MaliTilerPtrCacheEvictStallCy`
  * Pointer cache line evict stall cycles. This counter increments every cycle where the tiler is waiting for an old cache line to be evicted from the bin pointer cache. Hardware name: PCACHE_EVICT_STALL.
* `MaliFragLZSKillQd`
  * Late ZS killed quads. This counter increments for every quad killed by late depth and stencil testing. Hardware name: FRAG_LZS_KILL.
* `MaliMMUL2Hit`
  * MMU L2 lookup TLB hits. This counter increments for every read of a level 2 MMU translation table entry that results in a successful hit in the main MMU's TLB. Hardware name: MMU_HIT_L2.
* `MaliLSFullWr`
  * Load/store full write issues. This counter increments for every full-width load/store cache write. Hardware name: LS_MEM_WRITE_FULL.
* `MaliSCBusLSRdBt`
  * Load/store read beats from L2 cache. This counter increments for every read beat received by the load/store unit. Hardware name: BEATS_RD_LSC.
* `MaliTexCacheLookup`
  * Texture cache lookup requests. This counter increments for every texture cache lookup cycle. A high number of cache lookups per pass can indicate a poor access pattern, that sparsely samples texels from a high resolution texture. This normally occurs for 3D content that has failed to use mipmaps, and so accesses the high resolution level 0 surface when an object is distant from the camera. Hardware name: TEX_TFCH_NUM_OPERATIONS.
* `MaliTilerPosShadWaitCy`
  * IDVS position shading outstanding cycles. This counter increments every cycle where the tiler has an outstanding position shading request that is still being processed by a shader core. Hardware name: IDVS_POS_SHAD_WAIT.
* `MaliL2CacheL1Wr`
  * Output internal write requests. This counter increments for every L1 cache write response sent by the L2 cache to an internal master. Write responses are standard responses back to a master in response to its own write requests. Hardware name: L2_WR_MSG_OUT.
* `MaliFragQueueJob`
  * Fragment jobs. This counter increments for every job processed by the GPU fragment queue. Hardware name: JS0_JOBS.
* `MaliL2CacheWrLookup`
  * Write lookup requests. This counter increments for every L2 cache write lookup made. Hardware name: L2_WRITE_LOOKUP.
* `MaliResQueueWaitIssueCy`
  * Reserved job issue wait cycles. This counter increments any clock cycle where the GPU has reserved work queued that can not run because all processor resources are busy. Hardware name: JS2_WAIT_ISSUE.
* `MaliTilerWrBuffHitCy`
  * Write buffer hits. This counter increments every time a new write hits in the write buffer, and is combined with existing data. Hardware name: WRBUF_HIT.
* `MaliExtBusWrNoSnoopPart`
  * Output external WriteNoSnoopPartial transactions. This counter increments for every external non-coherent partial write (WriteNoSnpPtl) transaction. Hardware name: L2_EXT_WRITE_NOSNP_PTL.
* `MaliGeomVisiblePrim`
  * Visible primitives. This counter increments for every visible primitive that survives culling. There may still be some forms of redundancy present in the set of visible primitives. For example, a complex mesh that is inside the frustum but occluded by a wall is classified as visible by this counter. Software techniques such as zone-based portal culling can be used to effectively cull objects inside the frustum, as they can provide guarantees about visibility between rooms in a game level. Hardware name: PRIM_VISIBLE.
* `MaliFragTile`
  * Tiles. This counter increments for every tile processed by the shader core. Note that tiles are normally 16x16 pixels but can vary depending on per-pixel storage requirements and the tile buffer size of the current GPU. The number of bits of color storage per pixel available when using a 16x16 tile size on this GPU is 256. Using more storage than this - whether for multi-sampling, wide color formats, or multiple render targets - will result in the driver dynamically reducing tile size until sufficient storage is available. The most accurate way to get the total pixel count rendered by the application is to use the job manager {{MaliFragQueueTask}} counter, because it will always count 32x32 pixel regions. Hardware name: FRAG_PTILES.
* `MaliGeomLinePrim`
  * Line primitives. This counter increments for every input line primitive. The count is made before any culling or clipping. Hardware name: LINES.
* `MaliExtBusRdStallCy`
  * Output external read stall transactions. This counter increments for every stall cycle on the AXI bus where the GPU has a valid read transaction to send, but is awaiting a ready signal from the bus. Hardware name: L2_EXT_AR_STALL.
* `MaliL2CacheIncSnpStallCy`
  * Input external snoop stall cycles. This counter increments for every clock cycle where a coherency snoop transaction received from an external master is stalled by the L2 cache. Hardware name: L2_EXT_SNOOP_STALL.
* `MaliTilerInitBins`
  * Tile list initial allocations. This counter increments for every initial tiler bin allocation made. These bins are 128 bytes in size. Hardware name: BIN_ALLOC_INIT.
* `MaliTexFiltActiveCy`
  * Texture filtering cycles. This counter increments for every texture filtering issue cycle. Some instructions take more than one cycle due to multi-cycle data access and filtering operations: * 2D bilinear filtering takes two cycles per quad. * 2D trilinear filtering takes four cycles per quad. * 3D bilinear filtering takes four cycles per quad. * 3D trilinear filtering takes eight cycles per quad. Hardware name: TEX_FILT_NUM_OPERATIONS.
* `MaliGeomPosShadTask`
  * IDVS position shading requests. This counter increments for every position shading request in the index-driven vertex shading (IDVS) flow. Position shading executes the first part of the vertex shader, computing the position required to execute clipping and culling. The same vertex may be shaded multiple times if it has been evicted from the post-transform cache, so keep good spatial locality of index reuse in your index buffers. Each request contains 4 vertices. Note that not all types of draw call can use the IDVS flow, so this may not account for all submitted geometry. Hardware name: IDVS_POS_SHAD_REQ.
* `MaliLSPartRd`
  * Load/store partial read issues. This counter increments for every partial-width load/store cache read. Partial data accesses do not make full use of the load/store cache capability, so efficiency can be improved by merging short accesses together to make fewer larger access requests. To do this in shader code: * Use vector data loads * Avoid padding in strided data accesses * Write compute shaders so that adjacent threads in a warp access adjacent addresses in memory. Hardware name: LS_MEM_READ_SHORT.
* `MaliTilerUTLBStallCy`
  * UTLB lookup cache full stall cycles. This counter increments every cycle where a micro-TLB lookup is stalled because the buffer is full. Hardware name: UTLB_TRANS_STALL.
* `MaliTexQuads`
  * Texture quads. This counter increments for every quad-width texture operation processed by the texture unit. Hardware name: TEX_MSGI_NUM_QUADS.
* `MaliL2CacheWrStallCy`
  * Input internal write stall cycles. This counter increments for every clock cycle where an L2 cache write request from an internal master is stalled. Hardware name: L2_WR_MSG_IN_STALL.
* `MaliFragRastPrim`
  * Rasterized primitives. This counter increments for every primitive entering the rasterization unit. This counter will increment once per primitive per tile; if you want to know the total number of primitives in the scene refer to the {{MaliGeomTotalPrim}} expression. Hardware name: FRAG_PRIM_RAST.
* `MaliGeomPointPrim`
  * Point primitives. This counter increments for every input point primitive. The count is made before any culling or clipping. Hardware name: POINTS.
* `MaliTilerWrBt`
  * Internal write beats. This counter increments for every data write cycle the tiler uses on the internal bus. Hardware name: BUS_WRITE.
* `MaliFragEZSUpdateQd`
  * Early ZS updated quads. This counter increments for every quad undergoing early depth and stencil testing that can update the framebuffer. Quads that have a depth value that depends on shader execution, or those that have indeterminate coverage due to use of alpha-to-coverage or discard statements in the shader, might be early ZS tested but can not do an early ZS update. For maximum performance, this number should be close to the total number of input quads. Aim to maximize the number of quads that are capable of doing an early ZS update. Hardware name: FRAG_QUADS_EZS_UPDATE.
* `MaliTexQuadPassTri`
  * Trilinear filtered texture quad issues. This counter increments for every quad-width filtering pass that uses a trilinear filter. Hardware name: TEX_TIDX_NUM_SPLIT_MIP_MAP.
* `MaliTilerBinWriteStallCy`
  * Tile iterator busy stall cycles. This counter increments every cycle where the tiler binning unit has valid data, but the write iterator can not accept new requests. Hardware name: BINNER_STALL.
* `MaliNonFragQueueWaitDepCy`
  * Non-fragment job dependency wait cycles. This counter increments any clock cycle where the GPU has non-fragment work queued that can not run until dependent work has completed. Hardware name: JS1_WAIT_DEPEND.
* `MaliTilerVarShadStallCy`
  * IDVS varying shading stall cycles. This counter increments every cycle where the tiler has a varying shading request that it can not send to a shader core because the shading request queue is full. Hardware name: IDVS_VAR_SHAD_STALL.
* `MaliMMUS2Lookup`
  * MMU stage 2 lookup requests. This counter increments for every stage 2 lookup made by the main GPU MMU. This occurs only if all lookups in to a local TLB miss. Stage 2 address translation is used when the operating system using the GPU is a guest in a virtualized environment. The guest operating system controls the stage 1 MMU, translating virtual addresses into intermediate physical addresses. The hypervisor controls the stage 2 MMU, translating intermediate physical addresses into physical addresses. Hardware name: MMU_S2_REQUESTS.
* `MaliAttrInstr`
  * Attribute instructions. This counter increments for every instruction executed by the attribute unit. Each instruction converts a logical attribute access into a pointer-based access, which can be progressed by the load/store unit. Hardware name: ATTR_INSTR.
* `MaliEngActiveCy`
  * Execution engine active cycles. This counter increments every cycle where the execution engine unit is processing at least one thread. Hardware name: EXEC_ACTIVE.
* `MaliResQueueActiveCy`
  * Reserved active cycles. This counter increments any clock cycle where the GPU has any workload present in the reserved processing queue. Hardware name: JS2_ACTIVE.
* `MaliGeomVarShadTask`
  * IDVS varying shading requests. This counter increments for every varying shading request in the index-driven vertex shading (IDVS) flow. Varying shading executes the second part of the vertex shader, for any primitive that survives clipping and culling. The same vertex may be shaded multiple times if it has been evicted from the post-transform cache, so keep good spatial locality of index reuse in your index buffers. Each request contains 4 vertices. Note that not all types of draw call can use the IDVS flow, so this may not account for all submitted geometry. Hardware name: IDVS_VAR_SHAD_REQ.
* `MaliEngNMInstr`
  * Message instructions. This counter increments every instruction issue where the workload is a single add pipe message operation, with no fused multiply-accumulate pipe operation. Hardware name: ARITH_INSTR_MSG_ONLY.
* `MaliVar32ActiveCy`
  * 32-bit interpolation active. This counter increments for every 32-bit interpolation cycle processed by the varying unit. 32-bit interpolation is half the performance of 16-bit interpolation, so if content is varying bound and this counter is high consider reducing precision of varying inputs to fragment shaders. Hardware name: VARY_SLOT_32.
* `MaliL2CacheIncSnpSnp`
  * Input external snoop transactions triggering L1 snoop. This counter increments for every external snoop transaction that triggers a snoop into the GPU L1 cache hierarchy. Hardware name: L2_EXT_SNOOP_INTERNAL.
* `MaliTexQuadPassDescMiss`
  * Texture quad descriptor misses. This counter increments for every quad-width filtering pass that misses in the resource or sampler descriptor cache. A high miss rate in the descriptor cache can indicate: * Cache thrashing due to too many unique texture or sampler descriptors * A high memory fetch latency causing multiple sampling operations to miss on the same descriptor Hardware name: TEX_DFCH_NUM_PASSES_MISS.
* `MaliNonFragQueueJob`
  * Non-fragment jobs. This counter increments for every job processed by the GPU non-fragment queue. Hardware name: JS1_JOBS.
* `MaliMMUL3Rd`
  * MMU L3 table read requests. This counter increments for every read of a level 3 MMU translation table entry. Each address translation at this level covers a single 4KB page. Hardware name: MMU_TABLE_READS_L3.
* `MaliFragWarp`
  * Fragment warps. This counter increments for every created fragment warp. Note that the warp width varies between Mali devices. On this GPU, the number of threads in a single warp is 8. Hardware name: FRAG_WARPS.
* `MaliExtBusRd`
  * Output external read transactions. This counter increments for every external read transaction made on the memory bus. These transactions will typically result in an external DRAM access, but some designs include a system cache which can provide some buffering. The longest memory transaction possible is 64 bytes in length, but shorter transactions can be generated in some circumstances. Hardware name: L2_EXT_READ.
* `MaliExtBusWrStallCy`
  * Output external write stall cycles. This counter increments for every stall cycle on the external bus where the GPU has a valid write transaction to send, but is awaiting a ready signal from the external bus. Hardware name: L2_EXT_W_STALL.
* `MaliJMReceivedMsg`
  * Received messages. This counter increments for each message received by the Job Manager from another internal GPU subsystem. Hardware name: MESSAGES_RECEIVED.
* `MaliEngANInstr`
  * Arithmetic instructions. This counter increments every instruction issue where the workload is a single fused multiply-accumulate pipe arithmetic operation, with no add pipe operation. Hardware name: ARITH_INSTR_SINGLE_FMA.
* `MaliTilerWrBuffWrStallCy`
  * Write buffer write stall cycles. This counter increments every cycle where the tiler write buffer has data to write, but stalls because the internal bus is not ready to accept it. Hardware name: WRBUF_AXI_STALL.
* `MaliEngAAInstr`
  * Arithmetic + Arithmetic instructions. This counter increments every instruction issue where the workload is one fused multiply-accumulate pipe arithmetic operation and one add pipe arithmetic operation. Hardware name: ARITH_INSTR_DOUBLE.
* `MaliL2CacheFlush`
  * L2 cache flush requests. This counter increments for every L2 cache flush that is performed. Hardware name: CACHE_FLUSH.
* `MaliTilerPtrCacheDataWrStallCy`
  * Pointer manager data writeback stall cycles. This counter increments every cycle that the pointer manager has valid bin update data, but the write is stalled by the write buffer. Hardware name: PMGR_CMD_WR_STALL.
* `MaliTilerRdBt`
  * Output internal read beats. This counter increments for every data read cycle the tiler uses on the internal bus. Hardware name: BUS_READ.
* `MaliTexQuadPassMip`
  * Mipmapped texture quad issues. This counter increments for every quad-width filtering pass that uses a mipmapped texture. For applications rendering a 3D scene, you should aim to use mipmaps as much as possible to improve both image quality and performance. Hardware name: TEX_DFCH_NUM_PASSES_MIP_MAP.
* `MaliResQueueJob`
  * Reserved queue jobs. This counter increments for every job processed by the GPU reserved queue. Hardware name: JS2_JOBS.
* `MaliTilerVarCacheHit`
  * Varying cache hits. This counter increments every time a vertex varying lookup results in a successful hit in the vertex cache. Hardware name: IDVS_VBU_HIT.
* `MaliFragThread`
  * Fragment threads. This expression defines the number of fragment threads started. This is an approximation, based on the assumption that all warps are fully populated with threads. The {{MaliFragPartWarp}} and {{MaliCoreFullQdWarp}} counters can give some indication of how close this approximation is.
* `MaliExtBusRdBy`
  * Output external read bytes. This expression defines the output read bandwidth for the GPU.
* `MaliGeomPosShadThread`
  * Position shader thread invocations. This expression defines the number of position shader thread invocations.
* `MaliGeomFaceCullRate`
  * Input primitives to facing test killed by it. This expression defines the percentage of primitives entering the facing test that are killed by it.
* `MaliGeomVarShadThread`
  * Varying shader thread invocations. This expression defines the number of varying shader thread invocations.
* `MaliSCBusLSL2RdBy`
  * Load/store read bytes from L2 cache. This expression defines the total number of bytes read from the L2 memory system by the load/store unit.
* `MaliExtBusWrBy`
  * Output external write bytes. This expression defines the output write bandwidth for the GPU.
* `MaliFragEZSUpdateRate`
  * Early ZS updated quad percentage. This expression defines the percentage of rasterized quads that update the framebuffer during early depth and stencil testing.
* `MaliLSIssueCy`
  * Load/store total issues. This expression defines the total number of load/store issue cycles. Note that this counter ignores secondary effects such as cache misses, so this counter provides the best case cycle usage.
* `MaliCompUtil`
  * Compute utilization. This expression defines the percentage utilization of the shader core compute path.
* `MaliL2CacheRdMissRate`
  * L2 cache read miss rate. This expression defines the percentage of internal L2 cache reads that result in an external read.
* `MaliExtBusRdStallRate`
  * Output external read stall rate. This expression defines the percentage of read transactions that stall waiting for the external memory interface.
* `MaliGeomVisibleRate`
  * Visible primitives after culling. This expression defines the percentage of primitives that are visible after culling.
* `MaliCompThroughputCy`
  * Compute cycles per thread. This expression defines the average number of shader core cycles per compute thread. Note that this measurement captures the average throughput, which may not be a direct measure of processing cost for content that is sensitive to memory access latency. In addition there will be some crosstalk caused by compute and fragment workloads running concurrently on the same hardware. This expression is therefore indicative of cost, but does not reflect precise costing.
* `MaliNonFragQueueUtil`
  * Non-fragment queue utilization. This expression defines the non-fragment queue utilization compared against the GPU active cycles. For GPU bound content it is expected that the GPU queues process work in parallel, so the dominant queue should be close to 100% utilized. If no queue is dominant, but the GPU is close to 100% utilized, then there could be a serialization or dependency problem preventing better overlap across the queues.
* `MaliTexMipInstrRate`
  * Texture accesses using mipmapped texture percentage. This expression defines the percentage of texture operations accessing mipmapped textures.
* `MaliL2CacheWrMissRate`
  * L2 cache write miss rate. This expression defines the percentage of internal L2 cache writes that result in an external write.
* `MaliFragOverdraw`
  * Fragments per pixel. This expression computes the number of fragments shaded per output pixel. High levels of overdraw can be a significant processing cost, especially when rendering to a high-resolution framebuffer. Note that this expression assumes a 16x16 tile size is used during shading. For render passes using more than 256 bits per pixel the tile size will be dynamically reduced and this assumption will be invalid.
* `MaliFragEZSTestRate`
  * Early ZS tested quad percentage. This expression defines the percentage of rasterized quads that were subjected to early depth and stencil testing.
* `MaliSCBusTexExtRdBy`
  * Texture read bytes from external memory. This expression defines the total number of bytes read from the external memory system by the texture unit.
* `MaliVarIssueCy`
  * Varying cycles. This expression defines the total number of cycles where the varying interpolator is active.
* `MaliGeomSampleCullRate`
  * Input primitives to sample test killed by it. This expression defines the percentage of primitives entering the sample coverage test that are killed by it.
* `MaliEngDivergedInstrRate`
  * Diverged instruction issue rate. This expression defines the percentage of instructions that have control flow divergence across the warp.
* `MaliExtBusRdLat384`
  * Output external read latency 384+ cycles. This expression increments for every read beat that is returned at least 384 cycles after the transaction started.
* `MaliSCBusTexL2RdBy`
  * Texture read bytes from L2 cache. This expression defines the total number of bytes read from the L2 memory system by the texture unit.
* `MaliTilerUtil`
  * Tiler utilization. This expression defines the tiler utilization compared to the total GPU active cycles. Note that this measures the overall processing time for index-driven vertex shading (IDVS) workloads as well as fixed function tiling, so is not necessarily indicative of the runtime of the fixed-function tiling process itself.
* `MaliFragPartWarpRate`
  * Partial coverage rate. This expression defines the percentage of warps that contain samples with no coverage. A high percentage can indicate that your content has a high density of small triangles, which is expensive. To avoid this, use mesh level-of-detail algorithms to select simpler meshes as objects move further from the camera.
* `MaliFragFPKKillRate`
  * FPK killed quad percentage. This expression defines the percentage of rasterized quads that are killed by forward pixel kill (FPK) hidden surface removal.
* `MaliFragFPKBUtil`
  * Fragment FPK buffer active percentage. This expression defines the percentage of cycles where the forward pixel kill (FPK) quad buffer, before the execution core, contains at least one quad.
* `MaliTexTriInstrRate`
  * Texture accesses using trilinear filter percentage. This expression defines the percentage of texture operations using trilinear filtering.
* `MaliEngUtil`
  * Execution engine utilization. This expression defines the percentage utilization of the execution engine.
* `MaliTexCPI`
  * Texture filtering cycles per instruction. This expression defines the average number of texture filtering cycles per instruction. For texture unit limited content that has a CPI lower than 2, review any use of multi-cycle operations and consider using simpler texture filters. See {{MaliTexFiltActiveCy}} for details of the expected performance for different types of operation.
* `MaliSCBusTexL2RdByPerRd`
  * Texture bytes read from L2 per texture cycle. This expression defines the average number of bytes read from the L2 memory system by the texture unit per filtering cycle. This metric indicates how well textures are being cached in the L1 texture cache. If a high number of bytes are being requested per access, where high depends on the texture formats you are using, it can be worth reviewing texture settings: * Enable mipmaps for offline generated textures * Use ASTC or ETC compression for offline generated textures * Replace run-time generated framebuffer and texture formats with a narrower format * Reduce any use of negative LOD bias used for texture sharpening * Reduce the MAX_ANISOTROPY level for anisotropic filtering
* `MaliExtBusRdOTQ4`
  * Output external outstanding reads 75-100%. This expression increments for every read transaction initiated when 75-100% of transaction IDs are in use.
* `MaliFragUtil`
  * Fragment utilization. This expression defines the percentage utilization of the shader core fragment path.
* `MaliGPUPix`
  * Pixels. This expression defines the total number of pixels that are shaded for any render pass. Note that this can be a slight overestimate because the underlying hardware counter rounds the width and height values of the rendered surface to be 32-pixel aligned, even if those pixels are not actually processed during shading because they are out of the active viewport and/or scissor region.
* `MaliTexSample`
  * Texture samples. This expression defines the number of texture samples made.
* `MaliGeomTotalCullPrim`
  * Total culled primitives. This expression defines the number of primitives that were culled during the rendering process, for any reason.
* `MaliLSWrCy`
  * Load/store write issues. This expression defines the total number of load/store write cycles.
* `MaliTexCacheCompFetchRate`
  * Texture data fetches form compressed lines. This expression defines the percentage of texture line fetches that are from block compressed textures.
* `MaliSCBusTileWrBy`
  * Tile buffer write bytes. This expression defines the total number of bytes written to the L2 memory system by the tile buffer writeback unit.
* `MaliExtBusWrStallRate`
  * Output external write stall rate. This expression defines the percentage of write transactions that stall waiting for the external memory interface.
* `MaliFragEZSKillRate`
  * Early ZS killed quad percentage. This expression defines the percentage of rasterized quads that are killed by early depth and stencil testing.
* `MaliFragQueueUtil`
  * Fragment queue utilization. This expression defines the fragment queue utilization compared against the GPU active cycles. For GPU bound content it is expected that the GPU queues will process work in parallel, so the dominant queue should be close to 100% utilized. If no queue is dominant, but the GPU is close to 100% utilized, then there could be a serialization or dependency problem preventing better overlap across the queues.
* `MaliSCBusLSExtRdByPerRd`
  * Load/store bytes read from external memory per access cycle. This expression defines the average number of bytes read from the external memory system by the load/store unit per read cycle. This metric indicates how well data is being cached in the L2 cache. If a high number of bytes are being requested per access, where high depends on the texture formats you are using, it can be worth reviewing data formats and access patterns.
* `MaliFragTileKillRate`
  * Constant tile kill rate. This expression defines the percentage of tiles that are killed by the transaction elimination CRC check. If a high percentage of tile writes are being killed, this indicates that a significant part of the framebuffer is static from frame to frame. Consider using scissor rectangles to reduce the area that is redrawn. To help manage the partial frame updates for window surfaces consider using the EGL extensions such as: * EGL_KHR_partial_update * EGL_EXT_swap_buffers_with_damage
* `MaliGeomFrustumCullRate`
  * Input primitives to frustum test killed by it. This expression defines the percentage of primitives entering the frustum test that are killed by it.
* `MaliExtBusWrOTQ4`
  * Output external outstanding writes 75-100%. This expression increments for every write transaction initiated when 75-100% of transaction IDs are in use.
* `MaliSCBusLSWrByPerWr`
  * Load/store bytes written to L2 per access cycle. This expression defines the average number of bytes written to the L2 memory system by the load/store unit per write cycle.
* `MaliGeomTotalPrim`
  * Total input primitives. This expression defines the total number of input primitives to the rendering process.
* `MaliGPUIRQUtil`
  * Interrupt pending utilization. This expression defines the IRQ pending utilization compared against the GPU active cycles. In a well-functioning system this expression should be less than 1% of the total cycles. If the value is much higher than this then there may be a system issue preventing the CPU from efficiently handling interrupts.
* `MaliLSRdCy`
  * Load/store read issues. This expression defines the total number of load/store read cycles.
* `MaliSCBusLSL2RdByPerRd`
  * Load/store bytes read from L2 per access cycle. This expression defines the average number of bytes read from the L2 memory system by the load/store unit per read cycle. This metric gives some idea how well data is being cached in the L1 load/store cache. If a high number of bytes are being requested per access, where high depends on the buffer formats you are using, it can be worth reviewing data formats and access patterns.
* `MaliSCBusLSExtRdBy`
  * Load/store read bytes from external memory. This expression defines the total number of bytes read from the external memory system by the load/store unit.
* `MaliCompThread`
  * Compute threads. This expression defines the number of compute threads started. This is an approximation, based on the assumption that all warps are fully populated with threads. The {{MaliCoreFullQdWarp}} counter can give some indication of warp occupancy.
* `MaliFragShadedQd`
  * Fragment quads started. This expression defines the number of 2x2 fragment quads that are spawned as executing threads in the shader core. This is an approximation assuming that all spawned fragment warps contain a full set of quads. Comparing the total number of warps against the {{MaliCoreFullQdWarp}} counter can indicate how close this approximation is.
* `MaliSCBusLSWrBy`
  * Load/store write bytes. This expression defines the total number of bytes written to the L2 memory system by the load/store unit.
* `MaliUtilization`
  * This counter defines the total usage of the GPU relative to the maximum GPU frequency supported by the device. GPU-limited applications should achieve a utilization of around 98%. Lower utilization than this typically indicates one of the following scenarios: Content hitting vsync related limits. Content which is CPU limited so the GPU is running out of work to process. Content which is not pipelining well, so busy periods oscillate between CPU and the GPU. Remember most modern devices support Dynamic Voltage and Frequency Scaling (DVFS) which adjusts voltage and frequency to match the workload requirements. This means that the GPU frequency can change during game play.
* `MaliTexUtil`
  * Texture unit utilization. This expression defines the percentage utilization of the texturing unit.
* `MaliSCBusTexExtRdByPerRd`
  * Texture bytes read from external memory per texture cycle. This expression defines the average number of bytes read from the external memory system by the texture unit per filtering cycle. This metric indicates how well textures are being cached in the L2 cache. If a high number of bytes are being requested per access, where high depends on the texture formats you are using, it can be worth reviewing texture settings: * Enable mipmaps for offline generated textures * Use ASTC or ETC compression for offline generated textures * Replace run-time generated framebuffer and texture formats with a narrower format * Reduce any use of negative LOD bias used for texture sharpening * Reduce the MAX_ANISOTROPY level for anisotropic filtering
* `MaliFragThroughputCy`
  * Fragment cycles per thread. This expression defines the average number of shader core cycles per fragment thread. Note that this measurement captures the average throughput, which may not be a direct measure of processing cost for content which is sensitive to memory access latency. In addition there will be some crosstalk caused by compute and fragment workloads running concurrently on the same hardware. This expression is therefore indicative of cost, but does not reflect precise costing.
* `MaliCoreUtil`
  * Execution core utilization. This expression defines the percentage utilization of the programmable execution core. A low utilization indicates possible lost performance, because there are spare shader core cycles that could be used if they were accessible. In some use cases this is unavoidable, because there are regions in a render pass where there is no shader workload to process. For example, a clear color tile that contains no shaded geometry, or a shadow map that can be resolved entirely using early ZS depth updates. Aim to optimize screen regions that contain high volumes of redundant geometry, causing the programmable core to run out of work to process because the fragment front-end can not generate warps fast enough. This can be caused by a high percentage of triangles that are killed by ZS testing or FPK hidden surface removal, or by a very high density of micro-triangles which each generate low numbers of threads.
* `MaliGPUCyPerPix`
  * Cycles per pixel. This expression defines the average number of GPU cycles being spent per pixel rendered, including any vertex shading cost. It can be a useful exercise to set a cycle budget for each render pass in your game, based on the target resolution and frame rate you want to achieve. Rendering 1080p at 60 FPS is possible in a mass-market device, but the number of cycles per pixel you have to work with can be small, especially if you have multiple render passes per frame, so those cycles must be used wisely.
* `MaliLSUtil`
  * Load/store unit utilization. This expression defines the percentage utilization of the load/store unit.
* `MaliVarUtil`
  * Varying unit utilization. This expression defines the percentage utilization of the varying unit.
* `MaliFragTransparentQd`
  * FPK non-occluder quads. This expression defines the number of quads that are not candidates for being hidden surface removal occluders. To be eligible, a quad must be guaranteed to be opaque and resolvable at early ZS. Draw calls that use blending, shader discard, alpha-to-coverage, programmable depth, or programmable tile buffer access can not be occluders. Aim to minimize the number of transparent quads by disabling blending when it is not required.
* `MaliCoreAllRegsWarpRate`
  * All registers warp rate. This expression defines the percentage of warps that require more than 32 registers. If this number is high, the lack of running threads will start to impact the ability for the GPU to stay busy, especially under conditions of high memory latency.
* `MaliFragLZSKillRate`
  * Late ZS killed quad percentage. This expression defines the percentage of rasterized quads that are killed by late depth and stencil testing. Quads killed by late ZS testing will execute some of their fragment program before being killed, so a significant number of quads being killed at late ZS testing indicates a significant performance overhead and/or wasted energy. You should minimize the number of quads using and being killed by late ZS testing. The main causes of late ZS usage are where fragment shader programs: * use explicit discard statements * use implicit discard (alpha-to-coverage) * use a shader-created fragment depth * cause side-effects on shared resources, such as shader storage buffer objects, images, or atomics. In addition to the application use cases, the driver will generate warps for preloading the ZS values in the framebuffer if an attached depth or stencil attachment is not cleared or invalidated at the start of a render pass. These will be reported as quads killed by late ZS testing in the counter values. Always clear or invalidate all attached framebuffer surface attachments unless the algorithm requires the value to be preserved.
* `MaliFragFPKKillQd`
  * FPK killed quads. This expression defines the number of quads that are killed by the forward pixel kill (FPK) hidden surface removal scheme. The FPK scheme is efficient at killing occluded quads before they are spawned in a fragment warp, but a high percentage of FPK killed quads may indicate further potential application optimization opportunities. It is good practice to sort opaque geometry so that it is rendered front-to-back with depth testing enabled. This enables more geometry to be killed by early ZS testing instead of FPK, which removes the work earlier in the pipeline. If a very high percentage of quads are being killed by hidden surface removal, this can indicate that improvements in higher level application culling, such as the use of potential visibility sets or portal culling, are possible.
* `MaliCoreFullQdWarpRate`
  * Full quad warp rate. This expression defines the percentage of warps that are fully populated with quads. If there are many warps that are not full then performance may be lower, because thread slots in the warp are unused. Full warps are more likely if: * Compute shaders have work groups that are a multiple of warp size. * Draw calls avoid high numbers of small primitives.
* `MaliFragLZSTestRate`
  * Late ZS tested quad percentage. This expression defines the percentage of rasterized quads that are tested by late depth and stencil testing.
