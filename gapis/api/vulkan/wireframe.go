// Copyright (C) 2017 Google Inc.
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

	"github.com/google/gapid/gapis/api"
)

type wireframe struct {
	allocations *allocationTracker
}

func NewWireframeTransform() *wireframe {
	return &wireframe{
		allocations: nil,
	}
}

func (t *wireframe) RequiresAccurateState() bool {
	return false
}

func (wireframeTransform *wireframe) BeginTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	wireframeTransform.allocations = NewAllocationTracker(inputState)
	return inputCommands, nil
}

func (wireframeTransform *wireframe) EndTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	return inputCommands, nil
}

func (wireframeTransform *wireframe) ClearTransformResources(ctx context.Context) {
	// Melih TODO: This memory never been released.
	// Check if it's intentional
	// wireframeTransform.allocations.FreeAllocations()
}

func (wireframeTransform *wireframe) TransformCommand(ctx context.Context, id api.CmdID, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	for i, cmd := range inputCommands {
		if createGraphicsPipelinesCmd, ok := cmd.(*VkCreateGraphicsPipelines); ok {
			modifiedCmd := wireframeTransform.updateGraphicsPipelines(ctx, createGraphicsPipelinesCmd, inputState)
			if modifiedCmd != nil {
				inputCommands[i] = modifiedCmd
			}
		}
	}

	return inputCommands, nil
}

func (wireframeTransform *wireframe) updateGraphicsPipelines(ctx context.Context, cmd *VkCreateGraphicsPipelines, inputState *api.GlobalState) api.Cmd {
	cmd.Extras().Observations().ApplyReads(inputState.Memory.ApplicationPool())

	count := uint64(cmd.CreateInfoCount())
	infos := cmd.PCreateInfos().Slice(0, count, inputState.MemoryLayout)
	newInfos := make([]VkGraphicsPipelineCreateInfo, count)

	newRasterStateDatas := make([]api.AllocResult, count)
	for i := uint64(0); i < count; i++ {
		info := infos.Index(i).MustRead(ctx, cmd, inputState, nil)[0]
		rasterState := info.PRasterizationState().MustRead(ctx, cmd, inputState, nil)
		rasterState.SetPolygonMode(VkPolygonMode_VK_POLYGON_MODE_LINE)
		newRasterStateDatas[i] = wireframeTransform.allocations.AllocDataOrPanic(ctx, rasterState)
		info.SetPRasterizationState(NewVkPipelineRasterizationStateCreateInfoᶜᵖ(newRasterStateDatas[i].Ptr()))
		newInfos[i] = info
	}
	newInfosData := wireframeTransform.allocations.AllocDataOrPanic(ctx, newInfos)

	cb := CommandBuilder{Thread: cmd.Thread(), Arena: inputState.Arena}
	newCmd := cb.VkCreateGraphicsPipelines(cmd.Device(),
		cmd.PipelineCache(), cmd.CreateInfoCount(), newInfosData.Ptr(),
		cmd.PAllocator(), cmd.PPipelines(), cmd.Result()).AddRead(newInfosData.Data())

	for _, r := range newRasterStateDatas {
		newCmd.AddRead(r.Data())
	}

	for _, w := range cmd.Extras().Observations().Writes {
		newCmd.AddWrite(w.Range, w.ID)
	}
	return newCmd
}
