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
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/api/terminator"
	"github.com/google/gapid/gapis/api/transform"
	"github.com/google/gapid/gapis/resolve"
	"github.com/google/gapid/gapis/service/path"
)

// vulkanTerminator is very similar to EarlyTerminator.
// It has 2 additional properties.
//  1. If a VkQueueSubmit is found, and it contains an event that will be
//     signaled after the final request, we remove the event from the
//     command-list, and remove any subsequent events
//  2. If a request is made to replay until the MIDDLE of a vkQueueSubmit,
//     then it will patch that command-list to remove any commands after
//     the command in question.
//     Furthermore it will continue the replay until that command can be run
//     i.e. it will make sure to continue to mutate the trace until
//     all pending events have been successfully completed.
//     TODO(awoloszyn): Handle #2
//
// This takes advantage of the fact that all commands will be in order.
type vulkanTerminator struct {
	lastRequest       api.CmdID
	realCommandOffset api.CmdID
	requestSubIndex   []uint64
	terminated        bool
	syncData          *sync.Data
	allocations       *allocationTracker
	cleanupFunctions  []func()
}

var _ terminator.Terminator = &vulkanTerminator{}

func newVulkanTerminator(ctx context.Context, capture *path.Capture, realCommandOffset api.CmdID) (*vulkanTerminator, error) {
	s, err := resolve.SyncData(ctx, capture)
	if err != nil {
		return nil, err
	}

	return &vulkanTerminator{
		lastRequest:       api.CmdID(0),
		realCommandOffset: realCommandOffset,
		requestSubIndex:   make([]uint64, 0),
		terminated:        false,
		syncData:          s,
		allocations:       nil,
		cleanupFunctions:  make([]func(), 0),
	}, nil
}

// Add adds the command with identifier id to the set of commands that must be
// seen before the vulkanTerminator will consume all commands (excluding the EOS
// command).
func (vtTransform *vulkanTerminator) Add(ctx context.Context, id api.CmdID, subcommand api.SubCmdIdx) error {
	if len(vtTransform.requestSubIndex) != 0 {
		return log.Errf(ctx, nil, "Cannot handle multiple requests when requesting a subcommand")
	}

	id += vtTransform.realCommandOffset

	if id > vtTransform.lastRequest {
		vtTransform.lastRequest = id
	}

	// If we are not trying to index a subcommand, then just continue on our way.
	if len(subcommand) == 0 {
		return nil
	}

	vtTransform.requestSubIndex = append([]uint64{uint64(id)}, subcommand...)
	vtTransform.lastRequest = id

	return nil
}

func (vtTransform *vulkanTerminator) RequiresAccurateState() bool {
	return false
}

func (vtTransform *vulkanTerminator) RequiresInnerStateMutation() bool {
	return false
}

func (vtTransform *vulkanTerminator) SetInnerStateMutationFunction(mutator transform.StateMutator) {
	// This transform do not require inner state mutation
}

func (vtTransform *vulkanTerminator) BeginTransform(ctx context.Context, inputState *api.GlobalState) error {
	vtTransform.allocations = NewAllocationTracker(inputState)
	return nil
}

func (vtTransform *vulkanTerminator) EndTransform(ctx context.Context, inputState *api.GlobalState) ([]api.Cmd, error) {
	return nil, nil
}

func (vtTransform *vulkanTerminator) ClearTransformResources(ctx context.Context) {
	vtTransform.allocations.FreeAllocations()

	for _, f := range vtTransform.cleanupFunctions {
		f()
	}
}

func (vtTransform *vulkanTerminator) TransformCommand(ctx context.Context, id transform.CommandID, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	if vtTransform.terminated {
		return nil, nil
	}

	outputCmds := make([]api.Cmd, 0)
	for _, cmd := range inputCommands {
		if vkQueueSubmitCmd, ok := cmd.(*VkQueueSubmit); ok {
			processedCmds, err := vtTransform.processVkQueueSubmit(ctx, id.GetID(), vkQueueSubmitCmd, inputState)
			if err != nil {
				return nil, err
			}
			outputCmds = append(outputCmds, processedCmds...)
		} else {
			outputCmds = append(outputCmds, cmd)
		}
	}

	if id.GetID() == vtTransform.lastRequest {
		vtTransform.terminated = true
	}

	return outputCmds, nil
}

func (vtTransform *vulkanTerminator) processVkQueueSubmit(ctx context.Context, id api.CmdID, cmd *VkQueueSubmit, inputState *api.GlobalState) ([]api.Cmd, error) {
	doCut := false
	cutIndex := api.SubCmdIdx(nil)
	// If we have been requested to cut at a particular subindex,
	// then do that instead of cutting at the derived cutIndex.
	// It is guaranteed to be safe as long as the requestedSubIndex is
	// less than the calculated one (i.e. we are cutting more)
	if len(vtTransform.requestSubIndex) > 1 && vtTransform.requestSubIndex[0] == uint64(id) && vtTransform.syncData.SubcommandLookup.Value(vtTransform.requestSubIndex) != nil {
		if len(cutIndex) == 0 || !cutIndex.LessThan(vtTransform.requestSubIndex[1:]) {
			cutIndex = vtTransform.requestSubIndex[1:]
			doCut = true
		}
	}

	if !doCut {
		return []api.Cmd{cmd}, nil
	}

	return vtTransform.cutCommandBuffer(ctx, id, cmd, cutIndex, inputState)
}

// cutCommandBuffer rebuilds the given VkQueueSubmit command.
// It will re-write the submission so that it ends at
// idx. It writes any new commands to transform.Writer.
// It will make sure that if the replay were to stop at the given
// index it would remain valid. This means closing any open
// RenderPasses.
func (vtTransform *vulkanTerminator) cutCommandBuffer(ctx context.Context, id api.CmdID, cmd *VkQueueSubmit, idx api.SubCmdIdx, inputState *api.GlobalState) ([]api.Cmd, error) {
	cmd.Extras().Observations().ApplyReads(inputState.Memory.ApplicationPool())

	layout := inputState.MemoryLayout
	submitInfo := cmd.PSubmits().Slice(0, uint64(cmd.SubmitCount()), layout)
	skipAll := len(idx) == 0

	cb := CommandBuilder{Thread: cmd.Thread()}

	// Notes:
	// - We should walk/finish all unfinished render passes
	// idx[0] is the submission index
	// idx[1] is the primary command-buffer index in the submission
	// idx[2] is the command index in the primary command-buffer
	// idx[3] is the secondary command buffer index inside a vkCmdExecuteCommands
	// idx[4] is the secondary command inside the secondary command-buffer
	submitCopy := cb.VkQueueSubmit(cmd.Queue(), cmd.SubmitCount(), cmd.PSubmits(), cmd.Fence(), cmd.Result())
	submitCopy.Extras().MustClone(cmd.Extras().All()...)

	lastSubmit := uint64(0)
	lastCommandBuffer := uint64(0)
	if !skipAll {
		lastSubmit = idx[0]
		if len(idx) > 1 {
			lastCommandBuffer = idx[1]
		}
	}
	submitCopy.SetSubmitCount(uint32(lastSubmit + 1))
	newSubmits, err := submitInfo.Slice(0, lastSubmit+1).Read(ctx, cmd, inputState, nil)
	if err != nil {
		return nil, err
	}
	newSubmits[lastSubmit].SetCommandBufferCount(uint32(lastCommandBuffer + 1))

	newCommandBuffers, err := newSubmits[lastSubmit].PCommandBuffers().Slice(0, lastCommandBuffer+1, layout).Read(ctx, cmd, inputState, nil)
	if err != nil {
		return nil, err
	}

	stateObject := GetState(inputState)

	var lrp RenderPassObjectʳ
	lsp := uint32(0)
	if lastDrawInfo, ok := stateObject.LastDrawInfos().Lookup(cmd.Queue()); ok {
		if lastDrawInfo.InRenderPass() {
			lrp = lastDrawInfo.RenderPass()
			lsp = lastDrawInfo.LastSubpass()
		} else {
			lrp = NilRenderPassObjectʳ
			lsp = 0
		}
	}
	lrp, lsp, err = resolveCurrentRenderPass2(ctx, inputState, cmd, idx, lrp, lsp)
	if err != nil {
		return nil, err
	}

	extraCommands := make([]interface{}, 0)
	if !lrp.IsNil() {
		numSubpasses := uint32(lrp.SubpassDescriptions().Len())
		for i := 0; uint32(i) < numSubpasses-lsp-1; i++ {
			extraCommands = append(extraCommands,
				NewVkCmdNextSubpassXArgsʳ(
					NewSubpassBeginInfoʳ(
						VkSubpassContents_VK_SUBPASS_CONTENTS_INLINE,
					),
					NewSubpassEndInfoʳ(),
					lrp.Version(),
				))
		}
		extraCommands = append(extraCommands, NewVkCmdEndRenderPassXArgsʳ(
			NewSubpassEndInfoʳ(),
			lrp.Version(),
		))
	}
	cmdBuffer := stateObject.CommandBuffers().Get(newCommandBuffers[lastCommandBuffer])
	subIdx := make(api.SubCmdIdx, 0)

	outputCmds := make([]api.Cmd, 0)

	if len(idx) > 1 {
		if !skipAll {
			subIdx = idx[2:]
		}
		var newCmdBuffer VkCommandBuffer
		var newCommands []api.Cmd

		newCmdBuffer, newCommands, vtTransform.cleanupFunctions = rebuildCommandBuffer2(ctx, cb, cmdBuffer, inputState, subIdx, extraCommands)
		newCommandBuffers[lastCommandBuffer] = newCmdBuffer

		bufferMemory := vtTransform.allocations.AllocDataOrPanic(ctx, newCommandBuffers)
		newSubmits[lastSubmit].SetPCommandBuffers(NewVkCommandBufferᶜᵖ(bufferMemory.Ptr()))

		newSubmitData := vtTransform.allocations.AllocDataOrPanic(ctx, newSubmits)
		submitCopy.SetPSubmits(NewVkSubmitInfoᶜᵖ(newSubmitData.Ptr()))
		submitCopy.AddRead(bufferMemory.Data()).AddRead(newSubmitData.Data())

		outputCmds = append(outputCmds, newCommands...)
	} else {
		submitCopy.SetSubmitCount(uint32(lastSubmit + 1))
	}

	outputCmds = append(outputCmds, submitCopy)
	return outputCmds, nil
}

func walkCommands2(s *State, commands U32ːCommandReferenceʳDense_ᵐ, callback func(CommandReferenceʳ)) {
	for _, c := range commands.Keys() {
		callback(commands.Get(c))
		if commands.Get(c).Type() == CommandType_cmd_vkCmdExecuteCommands {
			execSub := s.CommandBuffers().Get(commands.Get(c).Buffer()).BufferCommands().VkCmdExecuteCommands().Get(commands.Get(c).MapIndex())
			for _, k := range execSub.CommandBuffers().Keys() {
				cbc := s.CommandBuffers().Get(execSub.CommandBuffers().Get(k))
				walkCommands2(s, cbc.CommandReferences(), callback)
			}
		}
	}
}

func getExtra2(idx api.SubCmdIdx, loopLevel int) int {
	if len(idx) == loopLevel+1 {
		return 1
	}
	return 0
}

func incrementLoopLevel2(idx api.SubCmdIdx, loopLevel *int) bool {
	if len(idx) == *loopLevel+1 {
		return false
	}
	*loopLevel++
	return true
}

// resolveCurrentRenderPass2 walks all of the current and pending commands
// to determine what renderpass we are in after the idx'th subcommand
func resolveCurrentRenderPass2(ctx context.Context, s *api.GlobalState, submit *VkQueueSubmit,
	idx api.SubCmdIdx, lrp RenderPassObjectʳ, subpass uint32) (RenderPassObjectʳ, uint32, error) {
	if len(idx) == 0 {
		return lrp, subpass, nil
	}
	a := submit
	c := GetState(s)
	l := s.MemoryLayout

	walkCommandsCallback := func(o CommandReferenceʳ) {
		args := GetCommandArgs(ctx, o, GetState(s))
		switch ar := args.(type) {
		case VkCmdBeginRenderPassXArgsʳ:
			lrp = c.RenderPasses().Get(ar.RenderPassBeginInfo().RenderPass())
			subpass = 0
		case VkCmdNextSubpassXArgsʳ:
			subpass++
		case VkCmdEndRenderPassXArgsʳ:
			lrp = NilRenderPassObjectʳ
			subpass = 0
		}
	}

	submitInfo := submit.PSubmits().Slice(0, uint64(submit.SubmitCount()), l)
	loopLevel := 0
	for sub := 0; sub < int(idx[0])+getExtra2(idx, loopLevel); sub++ {

		pInfo, err := submitInfo.Index(uint64(sub)).Read(ctx, a, s, nil)
		if err != nil {
			return NilRenderPassObjectʳ, 0, err
		}
		info := pInfo[0]

		buffers, err := info.PCommandBuffers().Slice(0, uint64(info.CommandBufferCount()), l).Read(ctx, a, s, nil)
		if err != nil {
			return NilRenderPassObjectʳ, 0, err
		}
		for _, buffer := range buffers {
			bufferObject := c.CommandBuffers().Get(buffer)
			walkCommands2(c, bufferObject.CommandReferences(), walkCommandsCallback)
		}
	}
	if !incrementLoopLevel2(idx, &loopLevel) {
		return lrp, subpass, nil
	}
	pLastInfo, err := submitInfo.Index(uint64(idx[0])).Read(ctx, a, s, nil)
	if err != nil {
		return NilRenderPassObjectʳ, 0, err
	}
	lastInfo := pLastInfo[0]
	lastBuffers := lastInfo.PCommandBuffers().Slice(0, uint64(lastInfo.CommandBufferCount()), l)
	for cmdbuffer := 0; cmdbuffer < int(idx[1])+getExtra2(idx, loopLevel); cmdbuffer++ {
		pBuffer, err := lastBuffers.Index(uint64(cmdbuffer)).Read(ctx, a, s, nil)
		if err != nil {
			return NilRenderPassObjectʳ, 0, err
		}
		buffer := pBuffer[0]
		bufferObject := c.CommandBuffers().Get(buffer)
		walkCommands2(c, bufferObject.CommandReferences(), walkCommandsCallback)
	}
	if !incrementLoopLevel2(idx, &loopLevel) {
		return lrp, subpass, nil
	}
	pLastBuffer, err := lastBuffers.Index(uint64(idx[1])).Read(ctx, a, s, nil)
	if err != nil {
		return NilRenderPassObjectʳ, 0, err
	}
	lastBuffer := pLastBuffer[0]
	lastBufferObject := c.CommandBuffers().Get(lastBuffer)
	for cmd := 0; cmd < int(idx[2])+getExtra2(idx, loopLevel); cmd++ {
		walkCommandsCallback(lastBufferObject.CommandReferences().Get(uint32(cmd)))
	}
	if !incrementLoopLevel2(idx, &loopLevel) {
		return lrp, subpass, nil
	}
	lastCommand := lastBufferObject.CommandReferences().Get(uint32(idx[2]))

	if lastCommand.Type() == CommandType_cmd_vkCmdExecuteCommands {
		executeSubcommand := c.CommandBuffers().Get(lastCommand.Buffer()).BufferCommands().VkCmdExecuteCommands().Get(lastCommand.MapIndex())
		for subcmdidx := 0; subcmdidx < int(idx[3])+getExtra2(idx, loopLevel); subcmdidx++ {
			buffer := executeSubcommand.CommandBuffers().Get(uint32(subcmdidx))
			bufferObject := c.CommandBuffers().Get(buffer)
			walkCommands2(c, bufferObject.CommandReferences(), walkCommandsCallback)
		}
		if !incrementLoopLevel2(idx, &loopLevel) {
			return lrp, subpass, nil
		}
		lastsubBuffer := executeSubcommand.CommandBuffers().Get(uint32(idx[3]))
		lastSubBufferObject := c.CommandBuffers().Get(lastsubBuffer)
		for subcmd := 0; subcmd < int(idx[4]); subcmd++ {
			walkCommandsCallback(lastSubBufferObject.CommandReferences().Get(uint32(subcmd)))
		}
	}

	return lrp, subpass, nil
}

// rebuildCommandBuffer2 takes the commands from commandBuffer up to, and
// including idx. It then appends any recreate* arguments to the end
// of the command buffer.
func rebuildCommandBuffer2(ctx context.Context,
	cb CommandBuilder,
	commandBuffer CommandBufferObjectʳ,
	s *api.GlobalState,
	idx api.SubCmdIdx,
	additionalCommands []interface{}) (VkCommandBuffer, []api.Cmd, []func()) {

	// DestroyResourcesAtEndOfFrame will handle this actually removing the
	// command buffer. We have no way to handle WHEN this will be done
	commandBufferID, x, cleanup := allocateNewCmdBufFromExistingOneAndBegin(ctx, cb, commandBuffer.VulkanHandle(), s)

	// If we have ANY data, then we need to copy up to that point
	numCommandsToCopy := uint64(0)
	numSecondaryCmdBuffersToCopy := uint64(0)
	numSecondaryCommandsToCopy := uint64(0)
	if len(idx) > 0 {
		numCommandsToCopy = idx[0]
	}
	// If we only have 1 index, then we have to copy the last command entirely,
	// and not re-write. Otherwise the last command is a vkCmdExecuteCommands
	// and it needs to be modified.
	switch len(idx) {
	case 1:
		// Only primary commands, copies including idx
		numCommandsToCopy++
	case 2:
		// Ends at a secondary command buffer
		numSecondaryCmdBuffersToCopy = idx[1] + 1
	case 3:
		// Ends at a secondary command, copies including idx
		numSecondaryCmdBuffersToCopy = idx[1]
		numSecondaryCommandsToCopy = idx[2] + 1
	}

	for i := uint32(0); i < uint32(numCommandsToCopy); i++ {
		cmd := commandBuffer.CommandReferences().Get(i)
		c, a, _ := AddCommand(ctx, cb, commandBufferID, s, s, GetCommandArgs(ctx, cmd, GetState(s)))
		x = append(x, a)
		cleanup = append(cleanup, c)
	}

	if numSecondaryCommandsToCopy != 0 || numSecondaryCmdBuffersToCopy != 0 {
		newCmdExecuteCommandsData := NewVkCmdExecuteCommandsArgs(
			NewU32ːVkCommandBufferDense_ᵐ(), // CommandBuffers
		)
		pcmd := commandBuffer.CommandReferences().Get(uint32(idx[0]))
		execCmdData, ok := GetCommandArgs(ctx, pcmd, GetState(s)).(VkCmdExecuteCommandsArgsʳ)
		if !ok {
			panic("Rebuild command buffer including secondary commands at a primary " +
				"command other than VkCmdExecuteCommands")
		}
		for scbi := uint32(0); scbi < uint32(numSecondaryCmdBuffersToCopy); scbi++ {
			newCmdExecuteCommandsData.CommandBuffers().Add(scbi, execCmdData.CommandBuffers().Get(scbi))
		}
		if numSecondaryCommandsToCopy != 0 {
			lastSecCmdBuf := execCmdData.CommandBuffers().Get(uint32(idx[1]))
			newSecCmdBuf, extraCmds, extraCleanup := allocateNewCmdBufFromExistingOneAndBegin(ctx, cb, lastSecCmdBuf, s)
			x = append(x, extraCmds...)
			cleanup = append(cleanup, extraCleanup...)
			for sci := uint32(0); sci < uint32(numSecondaryCommandsToCopy); sci++ {
				secCmd := GetState(s).CommandBuffers().Get(lastSecCmdBuf).CommandReferences().Get(sci)
				newCleanups, newSecCmds, _ := AddCommand(ctx, cb, newSecCmdBuf, s, s, GetCommandArgs(ctx, secCmd, GetState(s)))
				x = append(x, newSecCmds)
				cleanup = append(cleanup, newCleanups)
			}
			x = append(x, cb.VkEndCommandBuffer(newSecCmdBuf, VkResult_VK_SUCCESS))
			newCmdExecuteCommandsData.CommandBuffers().Add(uint32(idx[1]), newSecCmdBuf)
		}

		// If we use AddCommand, it will check for the existence of the command buffer,
		// which wont yet exist (because it hasn't been mutated yet)
		commandBufferData, commandBufferCount := unpackDenseMap(ctx, s, newCmdExecuteCommandsData.CommandBuffers())
		newExecSecCmds := cb.VkCmdExecuteCommands(commandBufferID,
			commandBufferCount,
			commandBufferData.Ptr(),
		).AddRead(commandBufferData.Data())

		cleanup = append(cleanup, func() {
			commandBufferData.Free()
		})
		x = append(x, newExecSecCmds)
	}

	for i := range additionalCommands {
		c, a, _ := AddCommand(ctx, cb, commandBufferID, s, s, additionalCommands[i])
		x = append(x, a)
		cleanup = append(cleanup, c)
	}
	x = append(x,
		cb.VkEndCommandBuffer(commandBufferID, VkResult_VK_SUCCESS))
	return VkCommandBuffer(commandBufferID), x, cleanup
}
