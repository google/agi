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
	"fmt"

	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/memory"
)

// Melih TODO: it seems like those are never freed
func (s *commandSplitter) MustAllocReadDataForSubmit(ctx context.Context, g *api.GlobalState, v ...interface{}) api.AllocResult {
	allocateResult := g.AllocDataOrPanic(ctx, v...)
	s.readMemoriesForSubmit = append(s.readMemoriesForSubmit, &allocateResult)
	rng, id := allocateResult.Data()
	g.Memory.ApplicationPool().Write(rng.Base, memory.Resource(id, rng.Size))
	return allocateResult
}

func (s *commandSplitter) MustAllocReadDataForCmd(ctx context.Context, g *api.GlobalState, v ...interface{}) api.AllocResult {
	allocateResult := g.AllocDataOrPanic(ctx, v...)
	s.readMemoriesForCmd = append(s.readMemoriesForCmd, &allocateResult)
	rng, id := allocateResult.Data()
	g.Memory.ApplicationPool().Write(rng.Base, memory.Resource(id, rng.Size))
	return allocateResult
}

func (s *commandSplitter) MustAllocWriteDataForCmd(ctx context.Context, g *api.GlobalState, v ...interface{}) api.AllocResult {
	allocateResult := g.AllocDataOrPanic(ctx, v...)
	s.writeMemoriesForCmd = append(s.writeMemoriesForCmd, &allocateResult)
	return allocateResult
}

// Melih TODO: Check every occurance of this method
func (s *commandSplitter) observeCommand(ctx context.Context, cmd api.Cmd) {
	for i := range s.readMemoriesForCmd {
		cmd.Extras().GetOrAppendObservations().AddRead(s.readMemoriesForCmd[i].Data())
	}
	for i := range s.writeMemoriesForCmd {
		cmd.Extras().GetOrAppendObservations().AddWrite(s.writeMemoriesForCmd[i].Data())
	}
	s.readMemoriesForCmd = []*api.AllocResult{}
	s.writeMemoriesForCmd = []*api.AllocResult{}
}

// commandSplitter is a transform that will re-write command-buffers and insert replacement
// commands at the correct locations in the stream for downstream transforms to replace.
// See: https://www.khronos.org/registry/vulkan/specs/1.1-extensions/html/vkspec.html#renderpass
// and https://www.khronos.org/registry/vulkan/specs/1.1-extensions/html/vkspec.html#pipelines-graphics
// to understand how/why we have to split these.
type commandSplitter struct {
	lastRequest      api.SubCmdIdx
	requestsSubIndex []api.SubCmdIdx

	readMemoriesForSubmit []*api.AllocResult
	readMemoriesForCmd    []*api.AllocResult
	writeMemoriesForCmd   []*api.AllocResult
	pool                  VkCommandPool

	thisRenderPass    VkCmdBeginRenderPassArgsʳ
	currentRenderPass [][3]VkRenderPass
	thisSubpass       int

	splitRenderPasses      map[VkRenderPass][][3]VkRenderPass
	fixedGraphicsPipelines map[VkPipeline]VkPipeline

	pendingCommandBuffers []VkCommandBuffer
	cleanupFuncs          []func()
}

func NewCommandSplitter(ctx context.Context) *commandSplitter {
	return &commandSplitter{
		lastRequest:            api.SubCmdIdx{},
		requestsSubIndex:       make([]api.SubCmdIdx, 0),
		readMemoriesForSubmit:  make([]*api.AllocResult, 0),
		readMemoriesForCmd:     make([]*api.AllocResult, 0),
		writeMemoriesForCmd:    make([]*api.AllocResult, 0),
		pool:                   0,
		thisRenderPass:         NilVkCmdBeginRenderPassArgsʳ,
		currentRenderPass:      make([][3]VkRenderPass, 0),
		thisSubpass:            0,
		splitRenderPasses:      make(map[VkRenderPass][][3]VkRenderPass),
		fixedGraphicsPipelines: make(map[VkPipeline]VkPipeline),
		pendingCommandBuffers:  make([]VkCommandBuffer, 0),
		cleanupFuncs:           make([]func(), 0),
	}
}

func (splitTransform *commandSplitter) RequiresAccurateState() bool {
	return false
}

func (splitTransform *commandSplitter) BeginTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	return inputCommands, nil
}

func (splitTransform *commandSplitter) EndTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	return inputCommands, nil
}

func (splitTransform *commandSplitter) ClearTransformResources(ctx context.Context) {
	for _, f := range splitTransform.cleanupFuncs {
		f()
	}
}

// Melih TODO: This is a bit tricky, we don't know what to do with the added commands.
func (splitTransform *commandSplitter) TransformCommand(ctx context.Context, id api.CmdID, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	if len(inputCommands) == 0 {
		return inputCommands, nil
	}

	modifidCommands := splitTransform.modifyCommand(ctx, id, inputCommands[0], inputState)
	if modifidCommands == nil {
		return inputCommands, nil
	}

	return append(modifidCommands, inputCommands[1:]...), nil
}

// Add adds the command with identifier id to the set of commands that will be split.
func (t *commandSplitter) Split(ctx context.Context, id api.SubCmdIdx) error {
	t.requestsSubIndex = append(t.requestsSubIndex, append(api.SubCmdIdx{}, id...))
	if t.lastRequest.LessThan(id) {
		t.lastRequest = append(api.SubCmdIdx{}, id...)
	}

	return nil
}

func (t *commandSplitter) getCommandPool(ctx context.Context, queueSubmit *VkQueueSubmit, inputState *api.GlobalState) (api.Cmd, VkCommandPool) {
	if t.pool != 0 {
		return nil, t.pool
	}

	cb := CommandBuilder{Thread: queueSubmit.Thread(), Arena: inputState.Arena}
	queue := GetState(inputState).Queues().Get(queueSubmit.Queue())

	t.pool = VkCommandPool(newUnusedID(false, func(x uint64) bool {
		return GetState(inputState).CommandPools().Contains(VkCommandPool(x))
	}))

	poolCreateInfo := NewVkCommandPoolCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,                                 // sType
		NewVoidᶜᵖ(memory.Nullptr),                                                                  // pNext
		VkCommandPoolCreateFlags(VkCommandPoolCreateFlagBits_VK_COMMAND_POOL_CREATE_TRANSIENT_BIT), // flags
		queue.Family(), // queueFamilyIndex
	)

	newCmd := cb.VkCreateCommandPool(
		queue.Device(),
		t.MustAllocReadDataForCmd(ctx, inputState, poolCreateInfo).Ptr(),
		memory.Nullptr,
		t.MustAllocWriteDataForCmd(ctx, inputState, t.pool).Ptr(),
		VkResult_VK_SUCCESS,
	)

	t.observeCommand(ctx, newCmd)
	return newCmd, t.pool
}

func (t *commandSplitter) getStartedCommandBuffer(ctx context.Context, queueSubmit *VkQueueSubmit, inputState *api.GlobalState) ([]api.Cmd, VkCommandBuffer) {
	cb := CommandBuilder{Thread: queueSubmit.Thread(), Arena: inputState.Arena}
	queue := GetState(inputState).Queues().Get(queueSubmit.Queue())

	outputCmds := make([]api.Cmd, 0)
	commandPoolCmd, commandPoolID := t.getCommandPool(ctx, queueSubmit, inputState)
	if commandPoolCmd != nil {
		outputCmds = append(outputCmds, commandPoolCmd)
	}

	commandBufferAllocateInfo := NewVkCommandBufferAllocateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO, // sType
		NewVoidᶜᵖ(memory.Nullptr),                                      // pNext
		commandPoolID,                                                  // commandPool
		VkCommandBufferLevel_VK_COMMAND_BUFFER_LEVEL_PRIMARY,           // level
		1, // commandBufferCount
	)
	commandBufferID := VkCommandBuffer(newUnusedID(true, func(x uint64) bool {
		return GetState(inputState).CommandBuffers().Contains(VkCommandBuffer(x))
	}))

	allocateCmd := cb.VkAllocateCommandBuffers(
		queue.Device(),
		t.MustAllocReadDataForCmd(ctx, inputState, commandBufferAllocateInfo).Ptr(),
		t.MustAllocWriteDataForCmd(ctx, inputState, commandBufferID).Ptr(),
		VkResult_VK_SUCCESS,
	)

	t.observeCommand(ctx, allocateCmd)
	outputCmds = append(outputCmds, allocateCmd)

	beginInfo := NewVkCommandBufferBeginInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO, // sType
		NewVoidᶜᵖ(memory.Nullptr),                                   // pNext
		0,                                                           // flags
		NewVkCommandBufferInheritanceInfoᶜᵖ(memory.Nullptr), // pInheritanceInfo
	)
	beginCommandBufferCmd := cb.VkBeginCommandBuffer(
		commandBufferID,
		t.MustAllocReadDataForCmd(ctx, inputState, beginInfo).Ptr(),
		VkResult_VK_SUCCESS,
	)

	t.observeCommand(ctx, beginCommandBufferCmd)
	outputCmds = append(outputCmds, beginCommandBufferCmd)

	return outputCmds, commandBufferID
}

const VK_ATTACHMENT_UNUSED = uint32(0xFFFFFFFF)

type commandSplitterTransformWriter struct {
	state            *api.GlobalState
	statebuilderCmds []api.Cmd
}

func newCommandSplitterTransformWriter(state *api.GlobalState) *commandSplitterTransformWriter {
	return &commandSplitterTransformWriter{
		state:            state,
		statebuilderCmds: make([]api.Cmd, 0),
	}
}

func (writer *commandSplitterTransformWriter) State() *api.GlobalState {
	return writer.state
}

func (writer *commandSplitterTransformWriter) MutateAndWrite(ctx context.Context, id api.CmdID, cmd api.Cmd) error {
	writer.statebuilderCmds = append(writer.statebuilderCmds, cmd)
	return nil
}

func (t *commandSplitter) splitRenderPass(ctx context.Context, rp RenderPassObjectʳ, inputState *api.GlobalState) ([]api.Cmd, [][3]VkRenderPass) {
	st := GetState(inputState)

	if rp, ok := t.splitRenderPasses[rp.VulkanHandle()]; ok {
		return nil, rp
	}

	handles := make([][3]VkRenderPass, 0)
	currentLayouts := make(map[uint32]VkImageLayout)
	for i := uint32(0); i < uint32(rp.AttachmentDescriptions().Len()); i++ {
		currentLayouts[i] = rp.AttachmentDescriptions().Get(i).InitialLayout()
	}

	tempTransformWriter := newCommandSplitterTransformWriter(inputState)
	sb := st.newStateBuilder(ctx, newTransformerOutput(tempTransformWriter))

	for i := uint32(0); i < uint32(rp.SubpassDescriptions().Len()); i++ {
		subpassHandles := [3]VkRenderPass{}
		lastSubpass := (i == uint32(rp.SubpassDescriptions().Len()-1))
		firstSubpass := (i == 0)

		patchFinalLayout := func(rpo RenderPassObjectʳ, ar U32ːVkAttachmentReferenceᵐ) {
			for k := 0; k < len(ar.All()); k++ {
				ia := ar.Get(uint32(k))
				if ia.Attachment() != VK_ATTACHMENT_UNUSED {
					currentLayouts[ia.Attachment()] = ia.Layout()
					ad := rpo.AttachmentDescriptions().Get(ia.Attachment())
					ad.SetFinalLayout(ia.Layout())
					rpo.AttachmentDescriptions().Add(ia.Attachment(), ad)
				}
			}
		}

		const (
			PATCH_LOAD uint32 = 1 << iota
			PATCH_STORE
			PATCH_FINAL_LAYOUT
		)

		patchAllDescriptions := func(rpo RenderPassObjectʳ, patch uint32) {
			for j := uint32(0); j < uint32(len(currentLayouts)); j++ {
				ad := rpo.AttachmentDescriptions().Get(j)
				ad.SetInitialLayout(currentLayouts[j])
				if 0 != (patch & PATCH_FINAL_LAYOUT) {
					ad.SetFinalLayout(currentLayouts[j])
				}
				if 0 != (patch & PATCH_LOAD) {
					ad.SetLoadOp(VkAttachmentLoadOp_VK_ATTACHMENT_LOAD_OP_LOAD)
					ad.SetStencilLoadOp(VkAttachmentLoadOp_VK_ATTACHMENT_LOAD_OP_LOAD)
				}
				if 0 != (patch & PATCH_STORE) {
					ad.SetStoreOp(VkAttachmentStoreOp_VK_ATTACHMENT_STORE_OP_STORE)
					ad.SetStencilStoreOp(VkAttachmentStoreOp_VK_ATTACHMENT_STORE_OP_STORE)
				}
				rpo.AttachmentDescriptions().Add(j, ad)
			}
		}

		{
			rp1 := rp.Clone(inputState.Arena, api.CloneContext{})
			rp1.SetVulkanHandle(
				VkRenderPass(newUnusedID(true, func(x uint64) bool {
					return st.RenderPasses().Contains(VkRenderPass(x))
				})))

			spd := rp1.SubpassDescriptions().Get(i)
			patch := uint32(0)
			if !firstSubpass {
				patch = PATCH_LOAD
			}
			patchAllDescriptions(rp1, patch|PATCH_STORE|PATCH_FINAL_LAYOUT)
			patchFinalLayout(rp1, spd.InputAttachments())
			patchFinalLayout(rp1, spd.ColorAttachments())
			spd.ResolveAttachments().Clear()
			if !spd.DepthStencilAttachment().IsNil() {
				ia := spd.DepthStencilAttachment()
				if ia.Attachment() != VK_ATTACHMENT_UNUSED {
					currentLayouts[ia.Attachment()] = ia.Layout()
					ad := rp1.AttachmentDescriptions().Get(ia.Attachment())
					ad.SetFinalLayout(ia.Layout())
					rp1.AttachmentDescriptions().Add(ia.Attachment(), ad)
				}
			}
			spd.PreserveAttachments().Clear()

			rp1.SubpassDescriptions().Clear()
			rp1.SubpassDescriptions().Add(0, spd)
			rp1.SubpassDependencies().Clear()
			sb.createRenderPass(rp1)
			subpassHandles[0] = rp1.VulkanHandle()
		}

		{
			rp2 := rp.Clone(inputState.Arena, api.CloneContext{})
			rp2.SetVulkanHandle(
				VkRenderPass(newUnusedID(true, func(x uint64) bool {
					return st.RenderPasses().Contains(VkRenderPass(x))
				})))
			spd := rp2.SubpassDescriptions().Get(i)
			patchAllDescriptions(rp2, PATCH_LOAD|PATCH_STORE|PATCH_FINAL_LAYOUT)
			patchFinalLayout(rp2, spd.InputAttachments())
			patchFinalLayout(rp2, spd.ColorAttachments())
			spd.ResolveAttachments().Clear()
			if !spd.DepthStencilAttachment().IsNil() {
				ia := spd.DepthStencilAttachment()
				if ia.Attachment() != VK_ATTACHMENT_UNUSED {
					currentLayouts[ia.Attachment()] = ia.Layout()
					ad := rp2.AttachmentDescriptions().Get(ia.Attachment())
					ad.SetFinalLayout(ia.Layout())
					rp2.AttachmentDescriptions().Add(ia.Attachment(), ad)
				}
			}
			spd.PreserveAttachments().Clear()
			rp2.SubpassDescriptions().Clear()
			rp2.SubpassDescriptions().Add(0, spd)
			rp2.SubpassDependencies().Clear()
			sb.createRenderPass(rp2)
			subpassHandles[1] = rp2.VulkanHandle()
		}

		{
			rp3 := rp.Clone(inputState.Arena, api.CloneContext{})
			rp3.SetVulkanHandle(
				VkRenderPass(newUnusedID(true, func(x uint64) bool {
					return st.RenderPasses().Contains(VkRenderPass(x))
				})))
			spd := rp3.SubpassDescriptions().Get(i)
			patch := PATCH_LOAD
			if !lastSubpass {
				patch |= PATCH_STORE | PATCH_FINAL_LAYOUT
			}
			patchAllDescriptions(rp3, patch)
			if !lastSubpass {
				patchFinalLayout(rp3, spd.InputAttachments())
				patchFinalLayout(rp3, spd.ColorAttachments())
				if !spd.DepthStencilAttachment().IsNil() {
					ia := spd.DepthStencilAttachment()
					if ia.Attachment() != VK_ATTACHMENT_UNUSED {
						currentLayouts[ia.Attachment()] = ia.Layout()
						ad := rp3.AttachmentDescriptions().Get(ia.Attachment())
						ad.SetFinalLayout(ia.Layout())
						rp3.AttachmentDescriptions().Add(ia.Attachment(), ad)
					}
				}
			}
			spd.PreserveAttachments().Clear()
			rp3.SubpassDescriptions().Clear()
			rp3.SubpassDescriptions().Add(0, spd)
			rp3.SubpassDependencies().Clear()
			sb.createRenderPass(rp3)
			subpassHandles[2] = rp3.VulkanHandle()
		}

		handles = append(handles, subpassHandles)
	}
	t.splitRenderPasses[rp.VulkanHandle()] = handles
	return tempTransformWriter.statebuilderCmds, handles
}

func (t *commandSplitter) rewriteGraphicsPipeline(ctx context.Context, graphicsPipeline VkPipeline, queueSubmit *VkQueueSubmit, inputState *api.GlobalState) ([]api.Cmd, VkPipeline) {
	if gp, ok := t.fixedGraphicsPipelines[graphicsPipeline]; ok {
		return nil, gp
	}

	st := GetState(inputState)

	tempTransformWriter := newCommandSplitterTransformWriter(inputState)
	sb := st.newStateBuilder(ctx, newTransformerOutput(tempTransformWriter))
	newGp := st.GraphicsPipelines().Get(graphicsPipeline).Clone(inputState.Arena, api.CloneContext{})
	newGp.SetRenderPass(st.RenderPasses().Get(t.currentRenderPass[t.thisSubpass][0]))
	newGp.SetSubpass(0)
	newGp.SetVulkanHandle(
		VkPipeline(newUnusedID(true, func(x uint64) bool {
			return st.GraphicsPipelines().Contains(VkPipeline(x)) ||
				st.ComputePipelines().Contains(VkPipeline(x))
		})))
	sb.createGraphicsPipeline(newGp)
	t.fixedGraphicsPipelines[graphicsPipeline] = newGp.VulkanHandle()
	return tempTransformWriter.statebuilderCmds, newGp.VulkanHandle()
}

func (t *commandSplitter) splitCommandBuffer(ctx context.Context, embedBuffer VkCommandBuffer, commandBuffer CommandBufferObjectʳ, queueSubmit *VkQueueSubmit, id api.SubCmdIdx, cuts []api.SubCmdIdx, inputState *api.GlobalState) ([]api.Cmd, VkCommandBuffer) {
	cb := CommandBuilder{Thread: queueSubmit.Thread(), Arena: inputState.Arena}
	st := GetState(inputState)

	outputCmds := make([]api.Cmd, 0)
	for i := 0; i < commandBuffer.CommandReferences().Len(); i++ {
		splitAfterCommand := false
		replaceCommand := false
		newCuts := []api.SubCmdIdx{}
		for _, s := range cuts {
			if s[0] == uint64(i) {
				if len(s) == 1 {
					splitAfterCommand = true
					continue
				} else {
					newCuts = append(newCuts, s[1:])
				}
			}
		}

		cr := commandBuffer.CommandReferences().Get(uint32(i))
		extraArgs := make([]interface{}, 0)
		args := GetCommandArgs(ctx, cr, st)
		switch ar := args.(type) {
		case VkCmdBeginRenderPassArgsʳ:
			rp := ar.RenderPass()
			rpo := st.RenderPasses().Get(rp)
			var splitCmds []api.Cmd
			splitCmds = nil
			splitCmds, t.currentRenderPass = t.splitRenderPass(ctx, rpo, inputState)
			if splitCmds != nil {
				outputCmds = append(outputCmds, splitCmds...)
			}
			t.thisSubpass = 0
			args = NewVkCmdBeginRenderPassArgsʳ(inputState.Arena, VkSubpassContents_VK_SUBPASS_CONTENTS_INLINE,
				t.currentRenderPass[t.thisSubpass][0], ar.Framebuffer(), ar.RenderArea(), ar.ClearValues(),
				ar.DeviceGroupBeginInfo())
			t.thisRenderPass = ar
		case VkCmdNextSubpassArgsʳ:
			args = NewVkCmdEndRenderPassArgsʳ(inputState.Arena)
			extraArgs = append(extraArgs,
				NewVkCmdBeginRenderPassArgsʳ(
					inputState.Arena, VkSubpassContents_VK_SUBPASS_CONTENTS_INLINE,
					t.currentRenderPass[t.thisSubpass][2], t.thisRenderPass.Framebuffer(), t.thisRenderPass.RenderArea(), t.thisRenderPass.ClearValues(),
					t.thisRenderPass.DeviceGroupBeginInfo()))
			extraArgs = append(extraArgs, NewVkCmdEndRenderPassArgsʳ(inputState.Arena))

			t.thisSubpass++
			extraArgs = append(extraArgs,
				NewVkCmdBeginRenderPassArgsʳ(
					inputState.Arena, VkSubpassContents_VK_SUBPASS_CONTENTS_INLINE,
					t.currentRenderPass[t.thisSubpass][0], t.thisRenderPass.Framebuffer(), t.thisRenderPass.RenderArea(), t.thisRenderPass.ClearValues(),
					t.thisRenderPass.DeviceGroupBeginInfo()))
		case VkCmdEndRenderPassArgsʳ:
			extraArgs = append(extraArgs,
				NewVkCmdBeginRenderPassArgsʳ(
					inputState.Arena, VkSubpassContents_VK_SUBPASS_CONTENTS_INLINE,
					t.currentRenderPass[t.thisSubpass][2], t.thisRenderPass.Framebuffer(), t.thisRenderPass.RenderArea(), t.thisRenderPass.ClearValues(),
					t.thisRenderPass.DeviceGroupBeginInfo()))
			extraArgs = append(extraArgs, NewVkCmdEndRenderPassArgsʳ(inputState.Arena))
			t.thisRenderPass = NilVkCmdBeginRenderPassArgsʳ
			t.thisSubpass = 0
		case VkCmdBindPipelineArgsʳ:
			if ar.PipelineBindPoint() == VkPipelineBindPoint_VK_PIPELINE_BIND_POINT_GRAPHICS {
				// Graphics pipeline, must be split (maybe)
				if st.RenderPasses().Get(t.thisRenderPass.RenderPass()).SubpassDescriptions().Len() > 1 {
					// If we have more than one renderpass, then we should replace
					pipelineCmds, newPipeline := t.rewriteGraphicsPipeline(ctx, ar.Pipeline(), queueSubmit, inputState)
					if pipelineCmds != nil {
						outputCmds = append(outputCmds, pipelineCmds...)
					}
					np := ar.Clone(inputState.Arena, api.CloneContext{})
					np.SetPipeline(newPipeline)
					args = np
				}
			}
		case VkCmdExecuteCommandsArgsʳ:
			replaceCommand = true
			for j := 0; j < ar.CommandBuffers().Len(); j++ {
				splitAfterExecute := false
				newSubCuts := []api.SubCmdIdx{}
				for _, s := range newCuts {
					if s[0] == uint64(j) {
						if len(s) == 1 {
							splitAfterExecute = true
							continue
						} else {
							newSubCuts = append(newSubCuts, s[1:])
						}
					}
				}

				cbo := st.CommandBuffers().Get(ar.CommandBuffers().Get(uint32(j)))
				newCmds, _ := t.splitCommandBuffer(ctx, embedBuffer, cbo, queueSubmit, append(id, uint64(i), uint64(j)), newSubCuts, inputState)
				if newCmds != nil {
					outputCmds = append(outputCmds, newCmds...)
				}

				if splitAfterExecute {
					insertionCmd := &InsertionCommand{
						embedBuffer,
						append([]VkCommandBuffer{}, t.pendingCommandBuffers...),
						append(id, uint64(i), uint64(j)),
						queueSubmit,
					}
					t.observeCommand(ctx, insertionCmd)
					outputCmds = append(outputCmds, insertionCmd)
				}
			}
		}
		if splitAfterCommand {
			// If we are inside a renderpass, then drop out for this call.
			// If we were not in a renderpass then we do not need to drop out
			// of it.
			if t.thisRenderPass != NilVkCmdBeginRenderPassArgsʳ {
				extraArgs = append(extraArgs, NewVkCmdEndRenderPassArgsʳ(inputState.Arena))
			}
			extraArgs = append(extraArgs, &InsertionCommand{
				embedBuffer,
				append([]VkCommandBuffer{}, t.pendingCommandBuffers...),
				append(id, uint64(i)),
				queueSubmit,
			})
			// If we were inside a renderpass, then we have to get back
			// into a renderpass
			if t.thisRenderPass != NilVkCmdBeginRenderPassArgsʳ {
				extraArgs = append(extraArgs,
					NewVkCmdBeginRenderPassArgsʳ(
						inputState.Arena, VkSubpassContents_VK_SUBPASS_CONTENTS_INLINE,
						t.currentRenderPass[t.thisSubpass][1], t.thisRenderPass.Framebuffer(), t.thisRenderPass.RenderArea(), t.thisRenderPass.ClearValues(),
						t.thisRenderPass.DeviceGroupBeginInfo()))
			}
		}
		if !replaceCommand {
			cleanup, cmd, err := AddCommand(ctx, cb, embedBuffer, inputState, inputState, args)
			if err != nil {
				panic(fmt.Errorf("Invalid command-buffer detected %+v", err))
			}
			t.observeCommand(ctx, cmd)
			outputCmds = append(outputCmds, cmd)
			t.cleanupFuncs = append(t.cleanupFuncs, cleanup)
		}
		for _, ea := range extraArgs {
			if ins, ok := ea.(api.Cmd); ok {
				t.observeCommand(ctx, ins)
				outputCmds = append(outputCmds, ins)
			} else {
				cleanup, cmd, err := AddCommand(ctx, cb, embedBuffer, inputState, inputState, ea)
				if err != nil {
					panic(fmt.Errorf("Invalid command-buffer detected %+v", err))
				}
				t.observeCommand(ctx, cmd)
				outputCmds = append(outputCmds, cmd)
				t.cleanupFuncs = append(t.cleanupFuncs, cleanup)
			}
		}
	}

	return outputCmds, embedBuffer
}

func (t *commandSplitter) splitSubmit(ctx context.Context, submit VkSubmitInfo, idx api.SubCmdIdx, cuts []api.SubCmdIdx, queueSubmit *VkQueueSubmit, inputState *api.GlobalState) ([]api.Cmd, VkSubmitInfo) {
	newSubmit := MakeVkSubmitInfo(inputState.Arena)
	newSubmit.SetSType(submit.SType())
	newSubmit.SetPNext(submit.PNext())
	newSubmit.SetWaitSemaphoreCount(submit.WaitSemaphoreCount())
	newSubmit.SetPWaitSemaphores(submit.PWaitSemaphores())
	newSubmit.SetPWaitDstStageMask(submit.PWaitDstStageMask())
	newSubmit.SetCommandBufferCount(submit.CommandBufferCount())

	layout := inputState.MemoryLayout
	// pCommandBuffers
	commandBuffers := submit.PCommandBuffers().Slice(0, uint64(submit.CommandBufferCount()), layout).MustRead(ctx, queueSubmit, inputState, nil)
	newCommandBuffers := make([]VkCommandBuffer, 0)

	outputCmds := make([]api.Cmd, 0)
	cb := CommandBuilder{Thread: queueSubmit.Thread(), Arena: inputState.Arena}

	for i := range commandBuffers {
		splitAfterCommandBuffer := false
		newCuts := []api.SubCmdIdx{}
		for _, s := range cuts {
			if s[0] == uint64(i) {
				if len(s) == 1 {
					splitAfterCommandBuffer = true
					continue
				} else {
					newCuts = append(newCuts, s[1:])
				}
			}
		}
		if len(newCuts) > 0 {
			cbuff := commandBuffers[i]
			cbo := GetState(inputState).CommandBuffers().Get(cbuff)
			newCmds, commandBuffer := t.getStartedCommandBuffer(ctx, queueSubmit, inputState)
			if newCmds != nil {
				outputCmds = append(outputCmds, newCmds...)
			}

			splitCmds, splitCommandBuffers := t.splitCommandBuffer(ctx, commandBuffer, cbo, queueSubmit, append(idx, uint64(i)), newCuts, inputState)
			if splitCmds != nil {
				outputCmds = append(outputCmds, splitCmds...)
			}
			newCommandBuffers = append(newCommandBuffers, splitCommandBuffers)

			endCmd := cb.VkEndCommandBuffer(commandBuffer, VkResult_VK_SUCCESS)
			outputCmds = append(outputCmds, endCmd)
			t.observeCommand(ctx, endCmd)
		} else {
			newCommandBuffers = append(newCommandBuffers, commandBuffers[i])
		}
		if splitAfterCommandBuffer {
			newCmds, commandBuffer := t.getStartedCommandBuffer(ctx, queueSubmit, inputState)
			if newCmds != nil {
				outputCmds = append(outputCmds, newCmds...)
			}

			outputCmds = append(outputCmds, &InsertionCommand{
				commandBuffer,
				append([]VkCommandBuffer{}, t.pendingCommandBuffers...),
				append(idx, uint64(i)),
				queueSubmit,
			})

			endCmd := cb.VkEndCommandBuffer(commandBuffer, VkResult_VK_SUCCESS)
			outputCmds = append(outputCmds, endCmd)
			t.observeCommand(ctx, endCmd)

			newCommandBuffers = append(newCommandBuffers, commandBuffer)
		}
	}
	t.pendingCommandBuffers = append(t.pendingCommandBuffers, newCommandBuffers...)
	newCbs := t.MustAllocReadDataForSubmit(ctx, inputState, newCommandBuffers)
	newSubmit.SetPCommandBuffers(NewVkCommandBufferᶜᵖ(newCbs.Ptr()))
	newSubmit.SetCommandBufferCount(uint32(len(newCommandBuffers)))
	newSubmit.SetSignalSemaphoreCount(submit.SignalSemaphoreCount())
	newSubmit.SetPSignalSemaphores(submit.PSignalSemaphores())

	return outputCmds, newSubmit
}

func (t *commandSplitter) splitAfterSubmit(ctx context.Context, id api.SubCmdIdx, queueSubmit *VkQueueSubmit, inputState *api.GlobalState) ([]api.Cmd, VkSubmitInfo) {
	outputCmds := make([]api.Cmd, 0)

	cb := CommandBuilder{Thread: queueSubmit.Thread(), Arena: inputState.Arena}
	newCmds, commandBuffer := t.getStartedCommandBuffer(ctx, queueSubmit, inputState)
	if newCmds != nil {
		outputCmds = append(outputCmds, newCmds...)
	}
	t.pendingCommandBuffers = append(t.pendingCommandBuffers, commandBuffer)

	outputCmds = append(outputCmds, &InsertionCommand{
		commandBuffer,
		append([]VkCommandBuffer{}, t.pendingCommandBuffers...),
		id,
		queueSubmit,
	})

	endCmd := cb.VkEndCommandBuffer(commandBuffer, VkResult_VK_SUCCESS)
	t.observeCommand(ctx, endCmd)
	outputCmds = append(outputCmds, endCmd)

	info := NewVkSubmitInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_SUBMIT_INFO, // sType
		NewVoidᶜᵖ(memory.Nullptr),                     // pNext
		0,                                             // waitSemaphoreCount,
		NewVkSemaphoreᶜᵖ(memory.Nullptr),              // pWaitSemaphores
		NewVkPipelineStageFlagsᶜᵖ(memory.Nullptr), // pWaitDstStageMask
		1, // commandBufferCount
		NewVkCommandBufferᶜᵖ(t.MustAllocReadDataForSubmit(ctx, inputState, commandBuffer).Ptr()),
		0,                                // signalSemaphoreCount
		NewVkSemaphoreᶜᵖ(memory.Nullptr), // pSignalSemaphores
	)

	return outputCmds, info
}

func (t *commandSplitter) rewriteQueueSubmit(ctx context.Context, id api.CmdID, cuts []api.SubCmdIdx, queueSubmit *VkQueueSubmit, inputState *api.GlobalState) []api.Cmd {
	layout := inputState.MemoryLayout
	cb := CommandBuilder{Thread: queueSubmit.Thread(), Arena: inputState.Arena}
	queueSubmit.Extras().Observations().ApplyReads(inputState.Memory.ApplicationPool())

	submitInfos := queueSubmit.PSubmits().Slice(0, uint64(queueSubmit.SubmitCount()), layout).MustRead(ctx, queueSubmit, inputState, nil)
	newSubmitInfos := []VkSubmitInfo{}

	newSubmit := cb.VkQueueSubmit(queueSubmit.Queue(), queueSubmit.SubmitCount(), queueSubmit.PSubmits(), queueSubmit.Fence(), queueSubmit.Result())
	newSubmit.Extras().MustClone(queueSubmit.Extras().All()...)

	outputCmds := make([]api.Cmd, 0)
	for i := 0; i < len(submitInfos); i++ {
		subIdx := api.SubCmdIdx{uint64(id), uint64(i)}
		newCuts := []api.SubCmdIdx{}
		addAfterSubmit := false
		for _, s := range cuts {
			if s[0] == uint64(i) {
				if len(s) == 1 {
					addAfterSubmit = true
				} else {
					newCuts = append(newCuts, s[1:])
				}
			}
		}
		newSubmitInfo := submitInfos[i]
		if len(newCuts) != 0 {
			var newCmds []api.Cmd
			newCmds = nil
			newCmds, newSubmitInfo = t.splitSubmit(ctx, submitInfos[i], subIdx, newCuts, queueSubmit, inputState)
			if newCmds != nil {
				outputCmds = append(outputCmds, newCmds...)
			}
		} else {
			commandBuffers := submitInfos[i].PCommandBuffers().Slice(0, uint64(submitInfos[i].CommandBufferCount()), layout).MustRead(ctx, queueSubmit, inputState, nil)
			t.pendingCommandBuffers = append(t.pendingCommandBuffers, commandBuffers...)
		}
		newSubmitInfos = append(newSubmitInfos, newSubmitInfo)
		if addAfterSubmit {
			newCmds, s := t.splitAfterSubmit(ctx, subIdx, queueSubmit, inputState)
			if newCmds != nil {
				outputCmds = append(outputCmds, newCmds...)
			}
			newSubmitInfos = append(newSubmitInfos, s)
		}
	}
	newSubmit.SetSubmitCount(uint32(len(newSubmitInfos)))
	newSubmit.SetPSubmits(NewVkSubmitInfoᶜᵖ(t.MustAllocReadDataForSubmit(ctx, inputState, newSubmitInfos).Ptr()))

	for x := range t.readMemoriesForSubmit {
		newSubmit.AddRead(t.readMemoriesForSubmit[x].Data())
	}
	t.readMemoriesForSubmit = []*api.AllocResult{}
	outputCmds = append(outputCmds, newSubmit)
	return outputCmds
}

// Melih TODO: Can this be only have to run for the authentic commands?
// We don't have a consistent way to detect it
func (t *commandSplitter) modifyCommand(ctx context.Context, id api.CmdID, cmd api.Cmd, inputState *api.GlobalState) []api.Cmd {
	inRange := false
	var topCut api.SubCmdIdx
	cuts := []api.SubCmdIdx{}
	thisID := api.SubCmdIdx{uint64(id)}
	for _, r := range t.requestsSubIndex {
		if thisID.Contains(r) {
			inRange = true
			if thisID.Equals(r) {
				topCut = r
			} else {
				cuts = append(cuts, r[1:])
			}
		}
	}

	if !inRange {
		return []api.Cmd{cmd}
	}

	if len(cuts) == 0 {
		outputCmds := make([]api.Cmd, 0)
		if cmd.CmdFlags().IsEndOfFrame() {
			outputCmds = append(outputCmds, &InsertionCommand{
				VkCommandBuffer(0),
				append([]VkCommandBuffer{}, t.pendingCommandBuffers...),
				topCut,
				cmd,
			})
			outputCmds = append(outputCmds, cmd)
		} else {
			outputCmds = append(outputCmds, cmd)
			outputCmds = append(outputCmds, &InsertionCommand{
				VkCommandBuffer(0),
				append([]VkCommandBuffer{}, t.pendingCommandBuffers...),
				topCut,
				cmd,
			})
		}

		return outputCmds
	}

	// Actually do the cutting here:
	queueSubmit, ok := cmd.(*VkQueueSubmit)
	// If this is not a queue submit it has no business having
	// subcommands.
	if !ok {
		return []api.Cmd{cmd}
	}
	rewriteCommands := t.rewriteQueueSubmit(ctx, id, cuts, queueSubmit, inputState)
	if len(topCut) == 0 {
		return rewriteCommands
	}
	insertionCmd := &InsertionCommand{
		VkCommandBuffer(0),
		append([]VkCommandBuffer{}, t.pendingCommandBuffers...),
		topCut,
		cmd,
	}
	t.pendingCommandBuffers = []VkCommandBuffer{}
	return append(rewriteCommands, insertionCmd)
}
