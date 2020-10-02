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

type framegraph struct {
}

var _ transform.Transform = &framegraph{}

func newFrameGraph() (*framegraph, error) {
	return &framegraph{}, nil
}

func (fg *framegraph) BeginTransform(ctx context.Context, inputState *api.GlobalState) error {
	return nil
}

func (fg *framegraph) EndTransform(ctx context.Context, inputState *api.GlobalState) ([]api.Cmd, error) {
	return nil, nil
}

func (fg *framegraph) TransformCommand(ctx context.Context, id transform.CommandID, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	st := GetState(inputState)
	for _, cmd := range inputCommands {
		if queueSubmit, ok := cmd.(*VkQueueSubmit); ok {
			log.W(ctx, "HUGUES saw queuesubmit: %v", queueSubmit)
			queueSubmit.Extras().Observations().ApplyReads(inputState.Memory.ApplicationPool())
			layout := inputState.MemoryLayout
			pSubmits := queueSubmit.PSubmits().Slice(0, uint64(queueSubmit.SubmitCount()), layout).MustRead(ctx, queueSubmit, inputState, nil)
			for _, submit := range pSubmits {
				commandBuffers := submit.PCommandBuffers().Slice(0, uint64(submit.CommandBufferCount()), layout).MustRead(ctx, queueSubmit, inputState, nil)
				for _, cb := range commandBuffers {
					commandBuffer := st.CommandBuffers().Get(cb)
					for i := 0; i < commandBuffer.CommandReferences().Len(); i++ {
						cr := commandBuffer.CommandReferences().Get(uint32(i))
						args := GetCommandArgs(ctx, cr, st)
						switch ar := args.(type) {
						case VkCmdBeginRenderPassArgsʳ:
							rp := ar.RenderPass()
							rpo := st.RenderPasses().Get(rp)
							log.W(ctx, "HUGUES renderpassobject: %v", rpo)
							for i := uint32(0); i < uint32(rpo.AttachmentDescriptions().Len()); i++ {
								attachment := rpo.AttachmentDescriptions().Get(i)
								log.W(ctx, "HUGUES attachment loadop: %v", attachment.LoadOp())
								log.W(ctx, "HUGUES attachment storeop: %v", attachment.StoreOp())
							}
						}
					}
				}
			}

		}
	}
	return inputCommands, nil
}

func (fg *framegraph) ClearTransformResources(ctx context.Context) {
}

func (fg *framegraph) RequiresAccurateState() bool {
	return false
}

func (fg *framegraph) RequiresInnerStateMutation() bool {
	return false
}

func (fg *framegraph) SetInnerStateMutationFunction(stateMutator transform.StateMutator) {}
