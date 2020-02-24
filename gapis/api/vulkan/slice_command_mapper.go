// Copyright (C) 2020 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vulkan

import (
	"context"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/transform"
)

type beginRenderPassIndex struct {
	index       uint64
	renderPass  uint64
	framebuffer uint64
}

type sliceCommandMapper struct {
	subCommandIndicesMap *map[api.CommandSubmissionKey][][]uint64
	submissionCount      uint64
	commandBuffersMap    map[uint64]uint64
	beginRenderPassMap   map[uint64][]beginRenderPassIndex
}

func (t *sliceCommandMapper) Transform(ctx context.Context, id api.CmdID, cmd api.Cmd, out transform.Writer) error {
	ctx = log.Enter(ctx, "Slice Command Mapper")
	s := out.State()
	switch cmd := cmd.(type) {
	case *VkBeginCommandBuffer:
		cb := uint64(cmd.commandBuffer)
		t.commandBuffersMap[cb] = 0
		t.beginRenderPassMap[cb] = []beginRenderPassIndex{}
		return out.MutateAndWrite(ctx, id, cmd)
	case *VkCmdBeginRenderPass:
		cb := uint64(cmd.commandBuffer)
		beginInfo := cmd.PRenderPassBegin().MustRead(ctx, cmd, s, nil)
		rp := uint64(beginInfo.RenderPass())
		fb := uint64(beginInfo.Framebuffer())
		t.beginRenderPassMap[cb] = append(t.beginRenderPassMap[cb], beginRenderPassIndex{t.commandBuffersMap[cb], rp, fb})
		t.commandBuffersMap[cb]++
		return out.MutateAndWrite(ctx, id, cmd)
	case *VkQueueSubmit:
		if id.IsReal() {
			submitInfoCount := cmd.SubmitCount()
			submitInfos := cmd.pSubmits.Slice(0, uint64(submitInfoCount), s.MemoryLayout).MustRead(ctx, cmd, s, nil)
			for i := uint32(0); i < submitInfoCount; i++ {
				si := submitInfos[i]
				cmdBufferCount := si.CommandBufferCount()
				cmdBuffers := si.PCommandBuffers().Slice(0, uint64(cmdBufferCount), s.MemoryLayout).MustRead(ctx, cmd, s, nil)
				for j := uint32(0); j < cmdBufferCount; j++ {
					cb := uint64(cmdBuffers[j])
					rps := t.beginRenderPassMap[cb]
					for _, rp := range rps {
						commandIndex := []uint64{uint64(id), uint64(i), uint64(j), rp.index}
						key := api.CommandSubmissionKey{SubmissionOrder: t.submissionCount,
							CommandBuffer: cb,
							RenderPass:    rp.renderPass,
							Framebuffer:   rp.framebuffer}
						if _, ok := (*t.subCommandIndicesMap)[key]; ok {
							(*t.subCommandIndicesMap)[key] = append((*t.subCommandIndicesMap)[key], commandIndex)
						} else {
							(*t.subCommandIndicesMap)[key] = [][]uint64{commandIndex}
						}
					}
				}
			}
			t.submissionCount++
		}
		return out.MutateAndWrite(ctx, id, cmd)
	default:
		name := cmd.CmdName()
		if len(name) >= 5 && name[0:5] == "vkCmd" {
			ps := cmd.CmdParams()
			if len(ps) > 0 {
				cb := cmd.CmdParams()[0].Get().(VkCommandBuffer)
				t.commandBuffersMap[uint64(cb)]++
			}
		}
		return out.MutateAndWrite(ctx, id, cmd)
	}
}

func (t *sliceCommandMapper) PreLoop(ctx context.Context, out transform.Writer) {
	out.NotifyPreLoop(ctx)
}
func (t *sliceCommandMapper) PostLoop(ctx context.Context, out transform.Writer) {
	out.NotifyPostLoop(ctx)
}
func (t *sliceCommandMapper) Flush(ctx context.Context, out transform.Writer) error { return nil }
func (t *sliceCommandMapper) BuffersCommands() bool {
	return false
}

func newSliceCommandMapper(subCommandIndicesMap *map[api.CommandSubmissionKey][][]uint64) *sliceCommandMapper {
	return &sliceCommandMapper{subCommandIndicesMap: subCommandIndicesMap,
		submissionCount:    0,
		commandBuffersMap:  make(map[uint64]uint64),
		beginRenderPassMap: make(map[uint64][]beginRenderPassIndex)}
}
