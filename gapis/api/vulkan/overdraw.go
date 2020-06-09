// Copyright (C) 2018 Google Inc.
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

	"github.com/google/gapid/core/data/binary"
	"github.com/google/gapid/core/image"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/stream/fmts"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/memory"
	"github.com/google/gapid/gapis/messages"
	"github.com/google/gapid/gapis/replay"
	"github.com/google/gapid/gapis/replay/builder"
	"github.com/google/gapid/gapis/replay/value"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

type stencilOverdraw struct {
	rewrite          map[api.CmdID]replay.Result
	lastSubIdx       map[api.CmdID]api.SubCmdIdx
	cmdBuilder       *CommandBuilder
	allocations      *allocationTracker
	cleanupCmds      []api.Cmd
	cleanupFunctions []func()
}

func NewStencilOverdraw() *stencilOverdraw {
	return &stencilOverdraw{
		rewrite:          map[api.CmdID]replay.Result{},
		lastSubIdx:       map[api.CmdID]api.SubCmdIdx{},
		allocations:      nil,
		cmdBuilder:       nil,
		cleanupCmds:      make([]api.Cmd, 0),
		cleanupFunctions: make([]func(), 0),
	}
}

func (overdrawTransform *stencilOverdraw) add(ctx context.Context, after []uint64, capt *path.Capture, res replay.Result) {
	c, err := capture.ResolveGraphicsFromPath(ctx, capt)
	if err != nil {
		res(nil, err)
		return
	}
	for lastSubmit := int64(after[0]); lastSubmit >= 0; lastSubmit-- {
		switch (c.Commands[lastSubmit]).(type) {
		case *VkQueueSubmit:
			id := api.CmdID(lastSubmit)
			overdrawTransform.rewrite[id] = res
			overdrawTransform.lastSubIdx[id] = api.SubCmdIdx(after[1:])
			log.D(ctx, "Overdraw marked for submit id %v", lastSubmit)
			return
		}
	}
	res(nil, &service.ErrDataUnavailable{
		Reason: messages.ErrMessage("No last queue submission"),
	})
}

func (overdrawTransform *stencilOverdraw) RequiresAccurateState() bool {
	return false
}

func (overdrawTransform *stencilOverdraw) BeginTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	overdrawTransform.allocations = NewAllocationTracker(inputState)
	return inputCommands, nil
}

func (overdrawTransform *stencilOverdraw) EndTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	return inputCommands, nil
}

func (overdrawTransform *stencilOverdraw) ClearTransformResources(ctx context.Context) {
	overdrawTransform.allocations.FreeAllocations()

	for _, f := range overdrawTransform.cleanupFunctions {
		f()
	}
}

func (overdrawTransform *stencilOverdraw) TransformCommand(ctx context.Context, id api.CmdID, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	outputCmds := make([]api.Cmd, 0, len(inputCommands))
	for _, cmd := range inputCommands {
		overdrawTransform.cmdBuilder = &CommandBuilder{Thread: cmd.Thread(), Arena: inputState.Arena}
		modifiedCmds := overdrawTransform.modifyStencilOverdraw(ctx, id, cmd, inputState)
		if modifiedCmds != nil {
			outputCmds = append(outputCmds, modifiedCmds...)
		} else {
			outputCmds = append(outputCmds, cmd)
		}
	}
	return outputCmds, nil
}

func (overdrawTransform *stencilOverdraw) modifyStencilOverdraw(ctx context.Context, id api.CmdID, cmd api.Cmd, inputState *api.GlobalState) []api.Cmd {
	if createImageCmd, ok := cmd.(*VkCreateImage); ok {
		if newCmd := overdrawTransform.rewriteImageCreate(ctx, id, createImageCmd, inputState); newCmd != nil {
			return []api.Cmd{newCmd}
		}

		return nil
	}

	res, ok := overdrawTransform.rewrite[id]
	if !ok {
		return nil
	}

	queueSubmitCmd, ok := cmd.(*VkQueueSubmit)
	if !ok {
		res(nil, &service.ErrDataUnavailable{Reason: messages.ErrMessage("Overdraw change marked for non-VkQueueSubmit")})
		return nil
	}

	lastRenderPassArgs, lastRenderPassIdx, err := getLastRenderPass(ctx, inputState, queueSubmitCmd, overdrawTransform.lastSubIdx[id])
	if err != nil {
		res(nil, &service.ErrDataUnavailable{Reason: messages.ErrMessage(fmt.Sprintf("Could not get overdraw: %v", err))})
		return nil
	}

	if lastRenderPassArgs.IsNil() {
		res(nil, &service.ErrDataUnavailable{Reason: messages.ErrMessage("No render pass in queue submit")})
		return nil
	}

	outputCmds := make([]api.Cmd, 0)
	modifiedCmds, img, err := overdrawTransform.rewriteQueueSubmit(ctx, inputState, queueSubmitCmd, lastRenderPassArgs, lastRenderPassIdx)
	if err != nil {
		res(nil, &service.ErrDataUnavailable{Reason: messages.ErrMessage(fmt.Sprintf("Could not get overdraw: %v", err))})
		return nil
	}
	if modifiedCmds != nil {
		outputCmds = append(outputCmds, modifiedCmds...)
	}

	newCmds := overdrawTransform.postStencilImageData(ctx, inputState, img, res)
	if newCmds != nil {
		outputCmds = append(outputCmds, newCmds...)
	}

	// Melih TODO: This does not matter anymore probably.
	// Don't defer this because we don't want these to execute if something panics
	for i := len(overdrawTransform.cleanupCmds) - 1; i >= 0; i-- {
		outputCmds = append(outputCmds, overdrawTransform.cleanupCmds[i])
	}
	return nil
}

func (overdrawTransform *stencilOverdraw) rewriteImageCreate(ctx context.Context, id api.CmdID, cmd *VkCreateImage, inputState *api.GlobalState) api.Cmd {
	allReads := []api.AllocResult{}
	allocAndRead := func(v ...interface{}) api.AllocResult {
		res := overdrawTransform.allocations.AllocDataOrPanic(ctx, v)
		allReads = append(allReads, res)
		return res
	}
	cmd.Extras().Observations().ApplyReads(inputState.Memory.ApplicationPool())

	createInfo := cmd.PCreateInfo().MustRead(ctx, cmd, inputState, nil)
	mask := VkImageUsageFlags(VkImageUsageFlagBits_VK_IMAGE_USAGE_TRANSFER_SRC_BIT)
	if !isDepthFormat(createInfo.Fmt()) || (createInfo.Usage()&mask == mask) {
		return nil
	}

	newCreateInfo := createInfo.Clone(inputState.Arena, api.CloneContext{})

	if !newCreateInfo.PQueueFamilyIndices().IsNullptr() {
		indices := newCreateInfo.PQueueFamilyIndices().Slice(0,
			uint64(newCreateInfo.QueueFamilyIndexCount()), inputState.MemoryLayout).
			MustRead(ctx, cmd, inputState, nil)
		data := allocAndRead(indices)
		newCreateInfo.SetPQueueFamilyIndices(NewU32ᶜᵖ(data.Ptr()))
	}

	// If the image could be used as a depth buffer, make sure we can transfer from it
	newCreateInfo.SetUsage(newCreateInfo.Usage() | mask)

	newCreateInfoPtr := allocAndRead(newCreateInfo).Ptr()

	allocatorPtr := memory.Nullptr
	if !cmd.PAllocator().IsNullptr() {
		allocatorPtr = allocAndRead(
			cmd.PAllocator().MustRead(ctx, cmd, inputState, nil)).Ptr()
	}

	cmd.Extras().Observations().ApplyWrites(inputState.Memory.ApplicationPool())
	idData := overdrawTransform.allocations.AllocDataOrPanic(ctx, cmd.PImage().MustRead(ctx, cmd, inputState, nil))

	newCmd := overdrawTransform.cmdBuilder.VkCreateImage(cmd.Device(), newCreateInfoPtr,
		allocatorPtr, idData.Ptr(),
		VkResult_VK_SUCCESS).AddWrite(idData.Data())
	for _, read := range allReads {
		newCmd.AddRead(read.Data())
	}

	return newCmd
}

func getLastRenderPass(ctx context.Context,
	inputState *api.GlobalState,
	submit *VkQueueSubmit,
	lastIdx api.SubCmdIdx,
) (VkCmdBeginRenderPassArgsʳ, api.SubCmdIdx, error) {
	lastRenderPassArgs := NilVkCmdBeginRenderPassArgsʳ
	var lastRenderPassIdx api.SubCmdIdx
	submit.Extras().Observations().ApplyReads(inputState.Memory.ApplicationPool())
	submitInfos := submit.PSubmits().Slice(0, uint64(submit.SubmitCount()),
		inputState.MemoryLayout).MustRead(ctx, submit, inputState, nil)
	for i, si := range submitInfos {
		if len(lastIdx) >= 1 && lastIdx[0] < uint64(i) {
			break
		}
		cmdBuffers := si.PCommandBuffers().Slice(0, uint64(si.CommandBufferCount()),
			inputState.MemoryLayout).MustRead(ctx, submit, inputState, nil)
		for j, buf := range cmdBuffers {
			if len(lastIdx) >= 2 && lastIdx[0] == uint64(i) && lastIdx[1] < uint64(j) {
				break
			}
			commandBuffers, ok := GetState(inputState).CommandBuffers().Lookup(buf)
			if !ok {
				return lastRenderPassArgs, lastRenderPassIdx,
					fmt.Errorf("Invalid command buffer %v", buf)
			}
			// vkCmdBeginRenderPass can only be in a primary command buffer,
			// so we don't need to check secondary command buffers
			for k := 0; k < commandBuffers.CommandReferences().Len(); k++ {
				if len(lastIdx) >= 3 && lastIdx[0] == uint64(i) &&
					lastIdx[1] == uint64(j) && lastIdx[2] < uint64(k) {
					break
				}
				cr := commandBuffers.CommandReferences().Get(uint32(k))
				if cr.Type() == CommandType_cmd_vkCmdBeginRenderPass {
					lastRenderPassArgs = commandBuffers.BufferCommands().
						VkCmdBeginRenderPass().
						Get(cr.MapIndex())
					lastRenderPassIdx = api.SubCmdIdx{
						uint64(i), uint64(j), uint64(k)}
				}
			}
		}
	}

	return lastRenderPassArgs, lastRenderPassIdx, nil
}

type stencilImage struct {
	handle VkImage
	format VkFormat
	width  uint32
	height uint32
}

type renderInfo struct {
	renderPass  VkRenderPass
	depthIdx    uint32
	framebuffer VkFramebuffer
	image       stencilImage
	view        VkImageView
}

func (overdrawTransform *stencilOverdraw) createNewRenderPassFramebuffer(ctx context.Context,
	inputState *api.GlobalState,
	oldRenderPass VkRenderPass,
	oldFramebuffer VkFramebuffer) ([]api.Cmd, renderInfo, error) {

	st := GetState(inputState)
	oldRpInfo, ok := st.RenderPasses().Lookup(oldRenderPass)
	if !ok {
		return nil, renderInfo{}, fmt.Errorf("Invalid renderpass %v", oldRenderPass)
	}

	oldFbInfo, ok := st.Framebuffers().Lookup(oldFramebuffer)
	if !ok {
		return nil, renderInfo{}, fmt.Errorf("Invalid framebuffer %v", oldFramebuffer)
	}

	attachDesc, depthIdx, err := overdrawTransform.getStencilAttachmentDescription(ctx, inputState, oldRpInfo)
	if err != nil {
		return nil, renderInfo{}, err
	}

	width, height := oldFbInfo.Width(), oldFbInfo.Height()
	// If we have a pre-existing depth image match our width and height to
	// that for when we render from one to the other.
	if depthIdx != ^uint32(0) {
		depthImage := oldFbInfo.ImageAttachments().Get(depthIdx).Image()
		width = depthImage.Info().Extent().Width()
		height = depthImage.Info().Extent().Height()
	}
	device := oldFbInfo.Device()

	outputCmds := make([]api.Cmd, 0)
	createImageCmds, image, err := overdrawTransform.createImage(ctx, inputState, device, attachDesc.Fmt(), width, height)
	if err != nil {
		return nil, renderInfo{}, err
	}

	if createImageCmds != nil {
		outputCmds = append(outputCmds, createImageCmds...)
	}

	imageViewCmd, imageView := overdrawTransform.createImageView(ctx, inputState, device, image.handle)
	if imageViewCmd != nil {
		outputCmds = append(outputCmds, imageViewCmd)
	}

	renderPassCmd, renderPass := overdrawTransform.createRenderPass(ctx, inputState, device, oldRpInfo, attachDesc)
	if renderPassCmd != nil {
		outputCmds = append(outputCmds, renderPassCmd)
	}

	frameBufferCmd, framebuffer := overdrawTransform.createFramebuffer(ctx, inputState, device, oldFbInfo, renderPass, imageView)
	if frameBufferCmd != nil {
		outputCmds = append(outputCmds, frameBufferCmd)
	}

	return outputCmds, renderInfo{renderPass, depthIdx, framebuffer, image, imageView}, nil
}

func (overdrawTransform *stencilOverdraw) getStencilAttachmentDescription(ctx context.Context,
	inputState *api.GlobalState,
	rpInfo RenderPassObjectʳ,
) (VkAttachmentDescription, uint32, error) {

	depthDesc, idx, err := overdrawTransform.getDepthAttachment(rpInfo)
	if err != nil {
		return NilVkAttachmentDescription, idx, err
	}

	// Clone it, but with a stencil-friendly format
	var stencilDesc VkAttachmentDescription
	var prefFmt VkFormat
	if idx != ^uint32(0) {
		stencilDesc = depthDesc.Clone(inputState.Arena, api.CloneContext{})
		prefFmt, err = depthToStencilFormat(depthDesc.Fmt())
		if err != nil {
			return NilVkAttachmentDescription, idx, err
		}
	} else {
		stencilDesc = MakeVkAttachmentDescription(inputState.Arena)
		prefFmt = 0xFFFFFFFF // defer to preference order
		stencilDesc.SetSamples(VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT)
		stencilDesc.SetLoadOp(VkAttachmentLoadOp_VK_ATTACHMENT_LOAD_OP_DONT_CARE)
		stencilDesc.SetStoreOp(VkAttachmentStoreOp_VK_ATTACHMENT_STORE_OP_DONT_CARE)
	}
	format, err := overdrawTransform.getBestStencilFormat(ctx, GetState(inputState), rpInfo.Device(), prefFmt)
	if err != nil {
		return NilVkAttachmentDescription, idx, err
	}
	stencilDesc.SetFmt(format)
	stencilDesc.SetStencilLoadOp(VkAttachmentLoadOp_VK_ATTACHMENT_LOAD_OP_CLEAR)
	stencilDesc.SetStencilStoreOp(VkAttachmentStoreOp_VK_ATTACHMENT_STORE_OP_STORE)
	stencilDesc.SetInitialLayout(VkImageLayout_VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL)
	stencilDesc.SetFinalLayout(VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL)
	return stencilDesc, idx, nil
}

// TODO: see if we can use the existing depth attachment in place
func (s *stencilOverdraw) getDepthAttachment(rpInfo RenderPassObjectʳ) (VkAttachmentDescription, uint32, error) {
	if rpInfo.SubpassDescriptions().Len() == 0 {
		return NilVkAttachmentDescription, 0,
			fmt.Errorf("RenderPass %v has no subpasses",
				rpInfo.VulkanHandle())
	}
	// depth attachment: don't support them not all using the same one for now
	attachment0 := rpInfo.SubpassDescriptions().Get(0).DepthStencilAttachment()
	for i := uint32(1); i < uint32(rpInfo.SubpassDescriptions().Len()); i++ {
		attachment := rpInfo.SubpassDescriptions().Get(i).DepthStencilAttachment()
		var match bool
		if attachment0.IsNil() {
			match = attachment.IsNil()
		} else {
			match = !attachment.IsNil() &&
				attachment0.Attachment() == attachment.Attachment()
		}
		if !match {
			// TODO: Handle using separate depth attachments (make
			// a separate image for each one and combine them at
			// the end perhaps?)
			return NilVkAttachmentDescription, 0, fmt.Errorf(
				"The subpasses don't have matching depth attachments")
		}
	}
	if attachment0.IsNil() ||
		// VK_ATTACHMENT_UNUSED
		attachment0.Attachment() == ^uint32(0) {
		return NilVkAttachmentDescription, ^uint32(0), nil
	}

	attachmentDesc, ok := rpInfo.AttachmentDescriptions().Lookup(
		attachment0.Attachment(),
	)
	if !ok {
		return NilVkAttachmentDescription, 0,
			fmt.Errorf("Invalid depth attachment")
	}

	return attachmentDesc, attachment0.Attachment(), nil
}

func (s *stencilOverdraw) getBestStencilFormat(ctx context.Context,
	st *State,
	device VkDevice,
	preferred VkFormat,
) (VkFormat, error) {
	deviceInfo := st.Devices().Get(device)
	physicalDeviceInfo := st.PhysicalDevices().Get(deviceInfo.PhysicalDevice())
	formatProps := physicalDeviceInfo.FormatProperties()
	// It should have an entry for every format
	if !formatProps.Contains(VkFormat_VK_FORMAT_UNDEFINED) {
		if preferred == 0xFFFFFFFF {
			// Most likely to be supported
			preferred = VkFormat_VK_FORMAT_D32_SFLOAT_S8_UINT
		}
		log.W(ctx, "Format support information not available, assuming %v is ok.", preferred)
		return preferred, nil
	}

	supported := func(fmt VkFormat) bool {
		return (formatProps.Get(fmt).OptimalTilingFeatures() &
			VkFormatFeatureFlags(VkFormatFeatureFlagBits_VK_FORMAT_FEATURE_DEPTH_STENCIL_ATTACHMENT_BIT)) != 0
	}

	if supported(preferred) {
		return preferred, nil
	}

	var order []VkFormat
	if isDepthFormat(preferred) {
		// Use as many depth bits as we can
		order = []VkFormat{
			VkFormat_VK_FORMAT_D32_SFLOAT_S8_UINT,
			VkFormat_VK_FORMAT_D24_UNORM_S8_UINT,
			VkFormat_VK_FORMAT_D16_UNORM_S8_UINT,
		}
	} else {
		// Use as little space as possible
		order = []VkFormat{
			VkFormat_VK_FORMAT_S8_UINT,
			VkFormat_VK_FORMAT_D16_UNORM_S8_UINT,
			VkFormat_VK_FORMAT_D24_UNORM_S8_UINT,
			VkFormat_VK_FORMAT_D32_SFLOAT_S8_UINT,
		}
	}

	for _, fmt := range order {
		if supported(fmt) {
			if isDepthFormat(preferred) {
				log.W(ctx, "Format %v not supported, using %v instead.  Depth buffer may not act exactly as original.", preferred, fmt)
			}
			return fmt, nil
		}
	}
	return 0, fmt.Errorf("No depth/stencil formats supported")
}

func (overdrawTransform *stencilOverdraw) createImage(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	format VkFormat,
	width uint32,
	height uint32) ([]api.Cmd, stencilImage, error) {

	st := GetState(inputState)

	// The physical device memory properties are used to find the correct
	// memory type index and allocate proper memory for our stencil image.
	deviceInfo, ok := st.Devices().Lookup(device)
	if !ok {
		return nil, stencilImage{}, fmt.Errorf("Invalid device %v", device)
	}
	physicalDeviceInfo, ok := st.PhysicalDevices().Lookup(deviceInfo.PhysicalDevice())
	if !ok {
		return nil, stencilImage{}, fmt.Errorf("Invalid physical device %v", deviceInfo.PhysicalDevice())
	}

	imageCreateInfo := NewVkImageCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO, // sType
		0,                            // pNext
		0,                            // flags
		VkImageType_VK_IMAGE_TYPE_2D, // imageType
		format,                       // format
		NewVkExtent3D(inputState.Arena, width, height, 1), // extent
		1, // mipLevels
		1, // arrayLevels
		VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT, // samples
		VkImageTiling_VK_IMAGE_TILING_OPTIMAL,       // tiling
		VkImageUsageFlags( // usage
			VkImageUsageFlagBits_VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT|
				VkImageUsageFlagBits_VK_IMAGE_USAGE_TRANSFER_SRC_BIT|
				VkImageUsageFlagBits_VK_IMAGE_USAGE_TRANSFER_DST_BIT|
				VkImageUsageFlagBits_VK_IMAGE_USAGE_INPUT_ATTACHMENT_BIT),
		VkSharingMode_VK_SHARING_MODE_EXCLUSIVE, // sharingMode
		0,                                       // queueFamilyIndexCount
		0,                                       // pQueueFamilyIndices
		VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED, // initialLayout
	)

	imageCreateInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, imageCreateInfo)
	image := VkImage(newUnusedID(false, func(id uint64) bool {
		return st.Images().Contains(VkImage(id))
	}))
	imageData := overdrawTransform.allocations.AllocDataOrPanic(ctx, image)

	outputCmds := make([]api.Cmd, 0)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCreateImage(device,
			imageCreateInfoData.Ptr(),
			memory.Nullptr,
			imageData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(imageCreateInfoData.Data()).AddWrite(imageData.Data()))

	imageMemory := VkDeviceMemory(newUnusedID(false, func(id uint64) bool {
		return st.DeviceMemories().Contains(VkDeviceMemory(id))
	}))
	imageMemoryData := overdrawTransform.allocations.AllocDataOrPanic(ctx, imageMemory)

	physicalDeviceMemoryPropertiesData := overdrawTransform.allocations.AllocDataOrPanic(ctx, physicalDeviceInfo.MemoryProperties())

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.ReplayAllocateImageMemory(
			device,
			physicalDeviceMemoryPropertiesData.Ptr(),
			image,
			imageMemoryData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			physicalDeviceMemoryPropertiesData.Data(),
		).AddWrite(
			imageMemoryData.Data(),
		),
		overdrawTransform.cmdBuilder.VkBindImageMemory(
			device, image, imageMemory, VkDeviceSize(0),
			VkResult_VK_SUCCESS))

	overdrawTransform.cleanupCmds = append(overdrawTransform.cleanupCmds,
		overdrawTransform.cmdBuilder.VkDestroyImage(device, image, memory.Nullptr),
		overdrawTransform.cmdBuilder.VkFreeMemory(device, imageMemory, memory.Nullptr))

	return outputCmds, stencilImage{image, format, width, height}, nil
}

func (overdrawTransform *stencilOverdraw) createImageView(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	image VkImage) (api.Cmd, VkImageView) {

	imageObject := GetState(inputState).Images().Get(image)
	createInfo := NewVkImageViewCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO, // sType
		0,                                     // pNext
		0,                                     // flags
		image,                                 // image
		VkImageViewType_VK_IMAGE_VIEW_TYPE_2D, // viewType
		imageObject.Info().Fmt(),              // format
		NewVkComponentMapping(inputState.Arena,
			VkComponentSwizzle_VK_COMPONENT_SWIZZLE_IDENTITY,
			VkComponentSwizzle_VK_COMPONENT_SWIZZLE_IDENTITY,
			VkComponentSwizzle_VK_COMPONENT_SWIZZLE_IDENTITY,
			VkComponentSwizzle_VK_COMPONENT_SWIZZLE_IDENTITY,
		), // components
		NewVkImageSubresourceRange(inputState.Arena,
			imageObject.ImageAspect(), // aspectMask
			0,                         // baseMipLevel
			1,                         // levelCount
			0,                         // baseArrayLayer
			1,                         // layerCount
		), // subresourceRange
	)
	createInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, createInfo)

	imageView := VkImageView(newUnusedID(false, func(id uint64) bool {
		return GetState(inputState).ImageViews().Contains(VkImageView(id))
	}))
	imageViewData := overdrawTransform.allocations.AllocDataOrPanic(ctx, imageView)

	newCmd := overdrawTransform.cmdBuilder.VkCreateImageView(
		device,
		createInfoData.Ptr(),
		memory.Nullptr,
		imageViewData.Ptr(),
		VkResult_VK_SUCCESS,
	).AddRead(
		createInfoData.Data(),
	).AddWrite(
		imageViewData.Data(),
	)

	overdrawTransform.cleanupCmds = append(overdrawTransform.cleanupCmds,
		overdrawTransform.cmdBuilder.VkDestroyImageView(device, imageView, memory.Nullptr))
	return newCmd, imageView
}

func (overdrawTransform *stencilOverdraw) createRenderPass(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	rpInfo RenderPassObjectʳ,
	stencilAttachment VkAttachmentDescription) (api.Cmd, VkRenderPass) {

	allReads := []api.AllocResult{}
	allocAndRead := func(v ...interface{}) api.AllocResult {
		res := overdrawTransform.allocations.AllocDataOrPanic(ctx, v)
		allReads = append(allReads, res)
		return res
	}

	attachments := rpInfo.AttachmentDescriptions().All()
	newAttachments := rpInfo.AttachmentDescriptions().Clone(inputState.Arena, api.CloneContext{})
	newAttachments.Add(uint32(newAttachments.Len()), stencilAttachment)
	newAttachmentsData, newAttachmentsLen := unpackMapWithAllocator(allocAndRead,
		newAttachments)

	stencilAttachmentReference := NewVkAttachmentReference(inputState.Arena,
		uint32(len(attachments)),
		stencilAttachment.InitialLayout(),
	)
	stencilAttachmentReferencePtr := allocAndRead(stencilAttachmentReference).Ptr()

	subpasses := make([]VkSubpassDescription,
		rpInfo.SubpassDescriptions().Len())
	for idx, subpass := range rpInfo.SubpassDescriptions().All() {
		subpasses[idx] = subpassToSubpassDescription(inputState, subpass,
			stencilAttachmentReferencePtr, allocAndRead)
	}
	subpassesData := allocAndRead(subpasses)

	subpassDependenciesData, subpassDependenciesLen := unpackMapWithAllocator(allocAndRead,
		rpInfo.SubpassDependencies())

	renderPassCreateInfo := NewVkRenderPassCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO, // sType
		0,                 // pNext
		0,                 // flags
		newAttachmentsLen, // attachmentCount
		NewVkAttachmentDescriptionᶜᵖ(newAttachmentsData.Ptr()), // pAttachments
		uint32(len(subpasses)),                                  // subpassCount
		NewVkSubpassDescriptionᶜᵖ(subpassesData.Ptr()),          // pSubpasses
		subpassDependenciesLen,                                  // dependencyCount
		NewVkSubpassDependencyᶜᵖ(subpassDependenciesData.Ptr()), // pDependencies
	)
	renderPassCreateInfoData := allocAndRead(renderPassCreateInfo)

	newRenderPass := VkRenderPass(newUnusedID(false, func(id uint64) bool {
		return GetState(inputState).RenderPasses().Contains(VkRenderPass(id))
	}))
	newRenderPassData := overdrawTransform.allocations.AllocDataOrPanic(ctx, newRenderPass)

	newCmd := overdrawTransform.cmdBuilder.VkCreateRenderPass(
		device,
		renderPassCreateInfoData.Ptr(),
		memory.Nullptr,
		newRenderPassData.Ptr(),
		VkResult_VK_SUCCESS,
	).AddWrite(newRenderPassData.Data())

	for _, read := range allReads {
		newCmd.AddRead(read.Data())
	}

	overdrawTransform.cleanupCmds = append(overdrawTransform.cleanupCmds,
		overdrawTransform.cmdBuilder.VkDestroyRenderPass(device, newRenderPass, memory.Nullptr))

	return newCmd, newRenderPass
}

func subpassToSubpassDescription(
	inputState *api.GlobalState,
	subpass SubpassDescription,
	attachRefPtr memory.Pointer,
	allocAndRead func(v ...interface{}) api.AllocResult,
) VkSubpassDescription {
	unpackMapMaybeEmpty := func(m interface{}) (memory.Pointer, uint32) {
		type HasLen interface {
			Len() int
		}
		if m.(HasLen).Len() > 0 {
			allocation, count := unpackMapWithAllocator(allocAndRead, m)
			return allocation.Ptr(), count
		} else {
			return memory.Nullptr, 0
		}
	}

	inputAttachmentsPtr, inputAttachmentsCount :=
		unpackMapMaybeEmpty(subpass.InputAttachments())
	colorAttachmentsPtr, colorAttachmentsCount :=
		unpackMapMaybeEmpty(subpass.ColorAttachments())
	resolveAttachmentsPtr, _ := unpackMapMaybeEmpty(subpass.ResolveAttachments())

	preserveAttachmentsPtr, preserveAttachmentsCount :=
		unpackMapMaybeEmpty(subpass.PreserveAttachments())

	return NewVkSubpassDescription(inputState.Arena,
		subpass.Flags(),                                   // flags
		subpass.PipelineBindPoint(),                       // pipelineBindPoint
		inputAttachmentsCount,                             // inputAttachmentCount
		NewVkAttachmentReferenceᶜᵖ(inputAttachmentsPtr),   // pInputAttachments
		colorAttachmentsCount,                             // colorAttachmentCount
		NewVkAttachmentReferenceᶜᵖ(colorAttachmentsPtr),   // pColorAttachments
		NewVkAttachmentReferenceᶜᵖ(resolveAttachmentsPtr), // pResolveAttachments
		NewVkAttachmentReferenceᶜᵖ(attachRefPtr),          // pDepthStencilAttachment
		preserveAttachmentsCount,                          // preserveAttachmentCount
		NewU32ᶜᵖ(preserveAttachmentsPtr),                  // pPreserveAttachments
	)
}

func (overdrawTransform *stencilOverdraw) createFramebuffer(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	fbInfo FramebufferObjectʳ,
	renderPass VkRenderPass,
	stencilImageView VkImageView) (api.Cmd, VkFramebuffer) {

	attachments := fbInfo.ImageAttachments().All()
	newAttachments := make([]VkImageView, len(attachments)+1)
	for idx, imageView := range attachments {
		newAttachments[idx] = imageView.VulkanHandle()
	}
	newAttachments[len(attachments)] = stencilImageView
	newAttachmentsData := overdrawTransform.allocations.AllocDataOrPanic(ctx, newAttachments)

	createInfo := NewVkFramebufferCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO, // sType
		0,                           // pNext
		0,                           // flags
		renderPass,                  // renderPass
		uint32(len(newAttachments)), // attachmentCount
		NewVkImageViewᶜᵖ(newAttachmentsData.Ptr()), // pAttachments
		fbInfo.Width(),  // width
		fbInfo.Height(), // height
		fbInfo.Layers(), // layers
	)
	createInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, createInfo)

	newFramebuffer := VkFramebuffer(newUnusedID(false, func(id uint64) bool {
		return GetState(inputState).Framebuffers().Contains(VkFramebuffer(id))
	}))
	newFramebufferData := overdrawTransform.allocations.AllocDataOrPanic(ctx, newFramebuffer)

	// CMD or thread

	newCmd := overdrawTransform.cmdBuilder.VkCreateFramebuffer(
		device,
		createInfoData.Ptr(),
		memory.Nullptr,
		newFramebufferData.Ptr(),
		VkResult_VK_SUCCESS,
	).AddRead(
		createInfoData.Data(),
	).AddRead(
		newAttachmentsData.Data(),
	).AddWrite(
		newFramebufferData.Data(),
	)

	overdrawTransform.cleanupCmds = append(overdrawTransform.cleanupCmds,
		overdrawTransform.cmdBuilder.VkDestroyFramebuffer(device, newFramebuffer, memory.Nullptr))

	return newCmd, newFramebuffer
}

func (overdrawTransform *stencilOverdraw) createGraphicsPipeline(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	pipeline VkPipeline,
	renderPass VkRenderPass) ([]api.Cmd, VkPipeline, error) {

	reads := []api.AllocResult{}
	allocAndRead := func(v ...interface{}) api.AllocResult {
		res := overdrawTransform.allocations.AllocDataOrPanic(ctx, v)
		reads = append(reads, res)
		return res
	}

	outputCmds := make([]api.Cmd, 0)

	createInfoCmds, createInfo, err := overdrawTransform.createGraphicsPipelineCreateInfo(
		ctx, inputState, pipeline, renderPass, allocAndRead)
	if err != nil {
		return nil, VkPipeline(0), err
	}
	createInfoData := allocAndRead(createInfo)

	if createInfoCmds != nil {
		outputCmds = append(outputCmds, createInfoCmds...)
	}

	newPipeline := VkPipeline(newUnusedID(false, func(id uint64) bool {
		return GetState(inputState).GraphicsPipelines().Contains(VkPipeline(id))
	}))
	newPipelineData := allocAndRead(newPipeline)

	newCmd := overdrawTransform.cmdBuilder.VkCreateGraphicsPipelines(
		device,                // device
		0,                     // pipelineCache: VK_NULL_HANDLE
		1,                     // createInfoCount
		createInfoData.Ptr(),  // pCreateInfos
		memory.Nullptr,        // pAllocator
		newPipelineData.Ptr(), // pPipelines
		VkResult_VK_SUCCESS,   // result
	).AddRead(
		createInfoData.Data(),
	).AddWrite(
		newPipelineData.Data(),
	)

	for _, read := range reads {
		newCmd.AddRead(read.Data())
	}

	outputCmds = append(outputCmds, newCmd)

	overdrawTransform.cleanupCmds = append(overdrawTransform.cleanupCmds,
		overdrawTransform.cmdBuilder.VkDestroyPipeline(device, newPipeline, memory.Nullptr))

	return outputCmds, newPipeline, nil
}

func (overdrawTransform *stencilOverdraw) createGraphicsPipelineCreateInfo(ctx context.Context,
	inputState *api.GlobalState,
	pipeline VkPipeline,
	renderPass VkRenderPass,
	allocAndRead func(v ...interface{}) api.AllocResult) ([]api.Cmd, VkGraphicsPipelineCreateInfo, error) {

	unpackMapMaybeEmpty := func(m interface{}) (memory.Pointer, uint32) {
		type HasLen interface {
			Len() int
		}
		if m.(HasLen).Len() > 0 {
			allocation, count := unpackMapWithAllocator(allocAndRead, m)
			return allocation.Ptr(), count
		} else {
			return memory.Nullptr, 0
		}
	}

	// TODO: Recreating a lot of work from state_rebuilder, look into merging with that
	pInfo, ok := GetState(inputState).GraphicsPipelines().Lookup(pipeline)
	if !ok {
		return nil, NilVkGraphicsPipelineCreateInfo, fmt.Errorf("Invalid graphics pipeline %v", pipeline)
	}

	outputCmds := make([]api.Cmd, 0)

	shaderStagesPtr := memory.Nullptr
	shaderStagesCount := uint32(0)
	if pInfo.Stages().Len() > 0 {
		stages := pInfo.Stages().All()
		data := make([]VkPipelineShaderStageCreateInfo, len(stages))
		for idx, stage := range stages {
			module := stage.Module().VulkanHandle()
			if !GetState(inputState).ShaderModules().Contains(module) {
				shaderModuleCmds, m := overdrawTransform.createShaderModule(ctx, inputState, stage.Module())
				if shaderModuleCmds != nil {
					outputCmds = append(outputCmds, shaderModuleCmds)
				}
				module = m
			}
			data[idx] = NewVkPipelineShaderStageCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO, // sType
				0,             // pNext
				0,             // flags
				stage.Stage(), // stage
				module,        // module
				NewCharᶜᵖ(allocAndRead(stage.EntryPoint()).Ptr()),                               // pName
				createSpecializationInfo(ctx, inputState, stage.Specialization(), allocAndRead), // pSpecializationInfo
			)
		}
		allocation := allocAndRead(data)
		shaderStagesPtr = allocation.Ptr()
		shaderStagesCount = uint32(len(data))
	}

	vertexInputPtr := memory.Nullptr
	{
		bindingPtr, bindingCount := unpackMapMaybeEmpty(
			pInfo.VertexInputState().BindingDescriptions())
		attributePtr, attributeCount := unpackMapMaybeEmpty(
			pInfo.VertexInputState().AttributeDescriptions())
		vertexInputPtr = allocAndRead(
			NewVkPipelineVertexInputStateCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO, // sType
				0,            // pNext
				0,            // flags
				bindingCount, // vertexBindingDescriptionCount
				NewVkVertexInputBindingDescriptionᶜᵖ(bindingPtr), // pVertexBindingDescriptions
				attributeCount, // vertexAttributeDescriptionCount
				NewVkVertexInputAttributeDescriptionᶜᵖ(attributePtr), // pVertexAttributeDescriptions
			)).Ptr()
	}

	inputAssemblyPtr := memory.Nullptr
	{
		info := pInfo.InputAssemblyState()
		inputAssemblyPtr = allocAndRead(
			NewVkPipelineInputAssemblyStateCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO, // sType
				0,                             // pNext
				0,                             // flags
				info.Topology(),               // topology
				info.PrimitiveRestartEnable(), // primitiveRestartEnable
			)).Ptr()
	}

	tessellationPtr := memory.Nullptr
	if !pInfo.TessellationState().IsNil() {
		info := pInfo.TessellationState()
		tessellationPtr = allocAndRead(
			NewVkPipelineTessellationStateCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_TESSELLATION_STATE_CREATE_INFO, // sType
				0,                         // pNext
				0,                         // flags
				info.PatchControlPoints(), // patchControlPoints
			)).Ptr()
	}

	viewportPtr := memory.Nullptr
	if !pInfo.ViewportState().IsNil() {
		info := pInfo.ViewportState()
		viewPtr, _ := unpackMapMaybeEmpty(info.Viewports())
		scissorPtr, _ := unpackMapMaybeEmpty(info.Scissors())
		viewportPtr = allocAndRead(
			NewVkPipelineViewportStateCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO, // sType
				0,                         // pNext
				0,                         // flags
				info.ViewportCount(),      // viewportCount
				NewVkViewportᶜᵖ(viewPtr),  // pViewports
				info.ScissorCount(),       // scissorCount
				NewVkRect2Dᶜᵖ(scissorPtr), // pScissors
			)).Ptr()
	}

	rasterizationPtr := memory.Nullptr
	{
		info := pInfo.RasterizationState()
		rasterizationPtr = allocAndRead(
			NewVkPipelineRasterizationStateCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO, // sType
				0, // pNext
				0, // flags
				info.DepthClampEnable(),
				info.RasterizerDiscardEnable(),
				info.PolygonMode(),
				info.CullMode(),
				info.FrontFace(),
				info.DepthBiasEnable(),
				info.DepthBiasConstantFactor(),
				info.DepthBiasClamp(),
				info.DepthBiasSlopeFactor(),
				info.LineWidth(),
			)).Ptr()
	}

	multisamplePtr := memory.Nullptr
	if !pInfo.MultisampleState().IsNil() {
		info := pInfo.MultisampleState()
		sampleMaskPtr, _ := unpackMapMaybeEmpty(info.SampleMask())
		multisamplePtr = allocAndRead(
			NewVkPipelineMultisampleStateCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO, // sType
				0, // pNext
				0, // flags
				info.RasterizationSamples(),
				info.SampleShadingEnable(),
				info.MinSampleShading(),
				NewVkSampleMaskᶜᵖ(sampleMaskPtr), // pSampleMask
				info.AlphaToCoverageEnable(),
				info.AlphaToOneEnable(),
			)).Ptr()
	}

	var depthStencilPtr memory.Pointer
	{
		// FIXME: work with existing depth buffer
		stencilOp := NewVkStencilOpState(inputState.Arena,
			0, // failOp
			VkStencilOp_VK_STENCIL_OP_INCREMENT_AND_CLAMP, // passOp
			0,                                // depthFailOp
			VkCompareOp_VK_COMPARE_OP_ALWAYS, // compareOp
			255,                              // compareMask
			255,                              // writeMask
			0,                                // reference
		)
		state := MakeVkPipelineDepthStencilStateCreateInfo(inputState.Arena)
		state.SetSType(
			VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_DEPTH_STENCIL_STATE_CREATE_INFO)
		state.SetStencilTestEnable(1)
		state.SetFront(stencilOp)
		state.SetBack(stencilOp)
		if !pInfo.DepthState().IsNil() {
			info := pInfo.DepthState()
			if info.StencilTestEnable() != 0 {
				return nil, NilVkGraphicsPipelineCreateInfo, fmt.Errorf("The stencil buffer is already in use")
			}

			state.SetDepthTestEnable(info.DepthTestEnable())
			state.SetDepthWriteEnable(info.DepthWriteEnable())
			state.SetDepthCompareOp(info.DepthCompareOp())
			state.SetDepthBoundsTestEnable(info.DepthBoundsTestEnable())
			state.SetMinDepthBounds(info.MinDepthBounds())
			state.SetMaxDepthBounds(info.MaxDepthBounds())
		}
		depthStencilPtr = allocAndRead(state).Ptr()
	}

	colorBlendPtr := memory.Nullptr
	if !pInfo.ColorBlendState().IsNil() {
		info := pInfo.ColorBlendState()
		attachmentPtr, attachmentCount := unpackMapMaybeEmpty(info.Attachments())
		colorBlendPtr = allocAndRead(
			NewVkPipelineColorBlendStateCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO, // sType
				0, // pNext
				0, // flags
				info.LogicOpEnable(),
				info.LogicOp(),
				attachmentCount, // attachmentCount
				NewVkPipelineColorBlendAttachmentStateᶜᵖ(
					attachmentPtr), // pAttachments
				info.BlendConstants(),
			)).Ptr()
	}

	dynamicPtr := memory.Nullptr
	if !pInfo.DynamicState().IsNil() {
		info := pInfo.DynamicState()
		statesPtr, statesCount := unpackMapMaybeEmpty(info.DynamicStates())
		dynamicPtr = allocAndRead(
			NewVkPipelineDynamicStateCreateInfo(inputState.Arena,
				VkStructureType_VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO, // sType
				0,                              // pNext
				0,                              // flags
				statesCount,                    // dynamicStateCount
				NewVkDynamicStateᶜᵖ(statesPtr), // pDynamicStates
			)).Ptr()
	}

	flags := pInfo.Flags()
	basePipelineHandle := VkPipeline(0)
	if flags&VkPipelineCreateFlags(
		VkPipelineCreateFlagBits_VK_PIPELINE_CREATE_ALLOW_DERIVATIVES_BIT) != 0 {

		flags |= VkPipelineCreateFlags(
			VkPipelineCreateFlagBits_VK_PIPELINE_CREATE_DERIVATIVE_BIT)
		basePipelineHandle = pipeline
	}

	return outputCmds, NewVkGraphicsPipelineCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO, // sType
		0,                 // pNext
		0,                 // flags
		shaderStagesCount, // stageCount
		NewVkPipelineShaderStageCreateInfoᶜᵖ(shaderStagesPtr),         // pStages
		NewVkPipelineVertexInputStateCreateInfoᶜᵖ(vertexInputPtr),     // pVertexInputState
		NewVkPipelineInputAssemblyStateCreateInfoᶜᵖ(inputAssemblyPtr), // pInputAssemblyState
		NewVkPipelineTessellationStateCreateInfoᶜᵖ(tessellationPtr),   // pTessellationState
		NewVkPipelineViewportStateCreateInfoᶜᵖ(viewportPtr),           // pViewportState
		NewVkPipelineRasterizationStateCreateInfoᶜᵖ(rasterizationPtr), // pRasterizationState
		NewVkPipelineMultisampleStateCreateInfoᶜᵖ(multisamplePtr),     // pMultisampleState
		NewVkPipelineDepthStencilStateCreateInfoᶜᵖ(depthStencilPtr),   // pDepthStencilState
		NewVkPipelineColorBlendStateCreateInfoᶜᵖ(colorBlendPtr),       // pColorBlendState
		NewVkPipelineDynamicStateCreateInfoᶜᵖ(dynamicPtr),             // pDynamicState
		pInfo.Layout().VulkanHandle(),                                 // layout
		renderPass,                                                    // renderPass
		pInfo.Subpass(),                                               // subpass
		basePipelineHandle,                                            // basePipelineHandle
		-1,                                                            // basePipelineIndex
	), nil
}

func (overdrawTransform *stencilOverdraw) createShaderModule(ctx context.Context,
	inputState *api.GlobalState,
	info ShaderModuleObjectʳ) (api.Cmd, VkShaderModule) {

	module := VkShaderModule(newUnusedID(false, func(id uint64) bool {
		return GetState(inputState).ShaderModules().Contains(VkShaderModule(id))
	}))
	moduleData := overdrawTransform.allocations.AllocDataOrPanic(ctx, module)

	words := info.Words().MustRead(ctx, nil, inputState, nil)
	wordsData := overdrawTransform.allocations.AllocDataOrPanic(ctx, words)
	createInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx,
		NewVkShaderModuleCreateInfo(inputState.Arena,
			VkStructureType_VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO, // sType
			0, // pNext
			0, // flags
			memory.Size(len(words)*4),
			NewU32ᶜᵖ(wordsData.Ptr()),
		))

	newCmd := overdrawTransform.cmdBuilder.VkCreateShaderModule(
		info.Device(),
		createInfoData.Ptr(),
		memory.Nullptr,
		moduleData.Ptr(),
		VkResult_VK_SUCCESS,
	).AddRead(
		createInfoData.Data(),
	).AddRead(
		wordsData.Data(),
	).AddWrite(
		moduleData.Data(),
	)

	overdrawTransform.cleanupCmds = append(overdrawTransform.cleanupCmds,
		overdrawTransform.cmdBuilder.VkDestroyShaderModule(
			info.Device(),
			module,
			memory.Nullptr,
		))

	return newCmd, module
}

func (overdrawTransform *stencilOverdraw) createDepthCopyBuffer(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	format VkFormat,
	width uint32,
	height uint32) ([]api.Cmd, VkBuffer) {

	elsize := VkDeviceSize(4)
	if format == VkFormat_VK_FORMAT_D16_UNORM ||
		format == VkFormat_VK_FORMAT_D16_UNORM_S8_UINT {

		elsize = VkDeviceSize(2)
	}

	bufferSize := elsize * VkDeviceSize(width*height)

	buffer := VkBuffer(newUnusedID(false, func(id uint64) bool {
		return GetState(inputState).Buffers().Contains(VkBuffer(id))
	}))
	bufferData := overdrawTransform.allocations.AllocDataOrPanic(ctx, buffer)

	bufferInfo := NewVkBufferCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO, // sType
		0,          // pNext
		0,          // flags
		bufferSize, // size
		VkBufferUsageFlags(
			VkBufferUsageFlagBits_VK_BUFFER_USAGE_TRANSFER_SRC_BIT|
				VkBufferUsageFlagBits_VK_BUFFER_USAGE_TRANSFER_DST_BIT), // usage
		VkSharingMode_VK_SHARING_MODE_EXCLUSIVE, // sharingMode
		0,                                       // queueFamilyIndexCount
		0,                                       // pQueueFamilyIndices
	)
	bufferInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, bufferInfo)

	bufferMemoryTypeIndex := uint32(0)
	physicalDevice := GetState(inputState).PhysicalDevices().Get(
		GetState(inputState).Devices().Get(device).PhysicalDevice())

	for i := uint32(0); i < physicalDevice.MemoryProperties().MemoryTypeCount(); i++ {
		t := physicalDevice.MemoryProperties().MemoryTypes().Get(int(i))
		if 0 != (t.PropertyFlags() & VkMemoryPropertyFlags(
			VkMemoryPropertyFlagBits_VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT)) {
			bufferMemoryTypeIndex = i
			break
		}
	}

	deviceMemory := VkDeviceMemory(newUnusedID(false, func(id uint64) bool {
		return GetState(inputState).DeviceMemories().Contains(VkDeviceMemory(id))
	}))
	deviceMemoryData := overdrawTransform.allocations.AllocDataOrPanic(ctx, deviceMemory)
	memoryAllocateInfo := NewVkMemoryAllocateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO, // sType
		0,                     // pNext
		bufferSize*2,          // allocationSize
		bufferMemoryTypeIndex, // memoryTypeIndex
	)
	memoryAllocateInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, memoryAllocateInfo)

	outputCmds := make([]api.Cmd, 0)
	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCreateBuffer(
			device,
			bufferInfoData.Ptr(),
			memory.Nullptr,
			bufferData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			bufferInfoData.Data(),
		).AddWrite(
			bufferData.Data(),
		),
		overdrawTransform.cmdBuilder.VkAllocateMemory(
			device,
			memoryAllocateInfoData.Ptr(),
			memory.Nullptr,
			deviceMemoryData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			memoryAllocateInfoData.Data(),
		).AddWrite(
			deviceMemoryData.Data(),
		),
		overdrawTransform.cmdBuilder.VkBindBufferMemory(
			device,
			buffer,
			deviceMemory,
			0,
			VkResult_VK_SUCCESS,
		),
	)

	overdrawTransform.cleanupCmds = append(overdrawTransform.cleanupCmds,
		overdrawTransform.cmdBuilder.VkDestroyBuffer(
			device,
			buffer,
			memory.Nullptr,
		),
		overdrawTransform.cmdBuilder.VkFreeMemory(
			device,
			deviceMemory,
			memory.Nullptr,
		),
	)

	return outputCmds, buffer
}

type imageDesc struct {
	image       ImageObjectʳ
	subresource VkImageSubresourceRange
	layout      VkImageLayout
	// This might not be the same as subresource.aspectMask if we only want
	// to copy one aspect
	aspect VkImageAspectFlagBits
}

// Facilitate copying the depth aspect of an image from one image to another,
// either for going from the original depth buffer to our depth buffer,
// or copying back the new depth buffer to the original depth buffer.
func (overdrawTransform *stencilOverdraw) copyImageAspect(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	cmdBuffer VkCommandBuffer,
	srcImgDesc imageDesc,
	dstImgDesc imageDesc,
	extent VkExtent3D) []api.Cmd {

	srcImg := srcImgDesc.image
	dstImg := dstImgDesc.image

	outputCmds := make([]api.Cmd, 0)
	copyCmds, copyBuffer := overdrawTransform.createDepthCopyBuffer(ctx, inputState, device,
		srcImg.Info().Fmt(), extent.Width(), extent.Height())

	if copyCmds != nil {
		outputCmds = append(outputCmds, copyCmds...)
	}

	allCommandsStage := VkPipelineStageFlags(
		VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT)
	allMemoryAccess := VkAccessFlags(
		VkAccessFlagBits_VK_ACCESS_MEMORY_WRITE_BIT |
			VkAccessFlagBits_VK_ACCESS_MEMORY_READ_BIT)

	imgBarriers0 := make([]VkImageMemoryBarrier, 2)
	imgBarriers1 := make([]VkImageMemoryBarrier, 2)
	// Transition the src image in and out of the required layouts
	imgBarriers0[0] = NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0,               // pNext
		allMemoryAccess, // srcAccessMask
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT), // dstAccessMask
		srcImgDesc.layout, // oldLayout
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, // newLayout
		^uint32(0),             // srcQueueFamilyIndex: VK_QUEUE_FAMILY_IGNORED
		^uint32(0),             // dstQueueFamilyIndex
		srcImg.VulkanHandle(),  // image
		srcImgDesc.subresource, // subresourceRange
	)
	srcFinalLayout := srcImgDesc.layout
	if srcFinalLayout == VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED ||
		srcFinalLayout == VkImageLayout_VK_IMAGE_LAYOUT_PREINITIALIZED {
		srcFinalLayout = VkImageLayout_VK_IMAGE_LAYOUT_GENERAL
	}
	imgBarriers1[0] = NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT), // srcAccessMask
		allMemoryAccess, // dstAccessMask
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, // oldLayout
		srcFinalLayout,         // newLayout
		^uint32(0),             // srcQueueFamilyIndex: VK_QUEUE_FAMILY_IGNORED
		^uint32(0),             // dstQueueFamilyIndex
		srcImg.VulkanHandle(),  // image
		srcImgDesc.subresource, // subresourceRange
	)

	// Transition the new image in and out of its required layouts
	imgBarriers0[1] = NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0,               // pNext
		allMemoryAccess, // srcAccessMask
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // dstAccessMask
		dstImgDesc.layout, // oldLayout
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, // newLayout
		^uint32(0),             // srcQueueFamilyIndex: VK_QUEUE_FAMILY_IGNORED
		^uint32(0),             // dstQueueFamilyIndex
		dstImg.VulkanHandle(),  // image
		dstImgDesc.subresource, // subresourceRange
	)

	dstFinalLayout := dstImgDesc.layout
	if dstFinalLayout == VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED ||
		dstFinalLayout == VkImageLayout_VK_IMAGE_LAYOUT_PREINITIALIZED {
		dstFinalLayout = VkImageLayout_VK_IMAGE_LAYOUT_GENERAL
	}
	imgBarriers1[1] = NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // srcAccessMask
		allMemoryAccess, // dstAccessMask
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, // oldLayout
		dstFinalLayout,         // newLayout
		^uint32(0),             // srcQueueFamilyIndex: VK_QUEUE_FAMILY_IGNORED
		^uint32(0),             // dstQueueFamilyIndex
		dstImg.VulkanHandle(),  // image
		dstImgDesc.subresource, // subresourceRange
	)

	bufBarrier := NewVkBufferMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // srcAccessMask
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT),  // dstAccessMask
		^uint32(0),       // srcQueueFamilyIndex: VK_QUEUE_FAMILY_IGNORED
		^uint32(0),       // dstQueueFamilyIndex
		copyBuffer,       // buffer
		0,                // offset
		^VkDeviceSize(0), // size: VK_WHOLE_SIZE
	)

	ibCopy := NewVkBufferImageCopy(inputState.Arena,
		0, // bufferOffset
		0, // bufferRowLength
		0, // bufferImageHeight
		NewVkImageSubresourceLayers(inputState.Arena,
			VkImageAspectFlags(srcImgDesc.aspect),   // aspectMask
			srcImgDesc.subresource.BaseMipLevel(),   // mipLevel
			srcImgDesc.subresource.BaseArrayLayer(), // baseArrayLayer
			1,                                       // layerCount
		), // srcSubresource
		NewVkOffset3D(inputState.Arena, 0, 0, 0),                            // offset
		NewVkExtent3D(inputState.Arena, extent.Width(), extent.Height(), 1), // extent
	)

	biCopy := NewVkBufferImageCopy(inputState.Arena,
		0, // bufferOffset
		0, // bufferRowLength
		0, // bufferImageHeight
		NewVkImageSubresourceLayers(inputState.Arena,
			VkImageAspectFlags(dstImgDesc.aspect),   // aspectMask
			dstImgDesc.subresource.BaseMipLevel(),   // mipLevel
			dstImgDesc.subresource.BaseArrayLayer(), // baseArrayLayer
			1,                                       // layerCount
		), // srcSubresource
		NewVkOffset3D(inputState.Arena, 0, 0, 0),                            // offset
		NewVkExtent3D(inputState.Arena, extent.Width(), extent.Height(), 1), // extent
	)

	imgBarriers0Data := overdrawTransform.allocations.AllocDataOrPanic(ctx, imgBarriers0)
	ibCopyData := overdrawTransform.allocations.AllocDataOrPanic(ctx, ibCopy)
	bufBarrierData := overdrawTransform.allocations.AllocDataOrPanic(ctx, bufBarrier)
	biCopyData := overdrawTransform.allocations.AllocDataOrPanic(ctx, biCopy)
	imgBarriers1Data := overdrawTransform.allocations.AllocDataOrPanic(ctx, imgBarriers1)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(cmdBuffer,
			allCommandsStage, // srcStageMask
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_TRANSFER_BIT), // dstStageMask
			0,                      // dependencyFlags
			0,                      // memoryBarrierCount
			memory.Nullptr,         // pMemoryBarriers
			0,                      // bufferMemoryBarrierCount
			memory.Nullptr,         // pBufferMemoryBarriers
			2,                      // imageMemoryBarrierCount
			imgBarriers0Data.Ptr(), // pImageMemoryBarriers
		).AddRead(imgBarriers0Data.Data()),
		overdrawTransform.cmdBuilder.VkCmdCopyImageToBuffer(cmdBuffer,
			srcImg.VulkanHandle(),                              // srcImage
			VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, // srcImageLayout
			copyBuffer,       // dstBuffer
			1,                // regionCount
			ibCopyData.Ptr(), // pRegions
		).AddRead(ibCopyData.Data()),
		overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(cmdBuffer,
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_TRANSFER_BIT), // srcStageMask
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_TRANSFER_BIT), // dstStageMask
			0,                    // dependencyFlags
			0,                    // memoryBarrierCount
			memory.Nullptr,       // pMemoryBarriers
			1,                    // bufferMemoryBarrierCount
			bufBarrierData.Ptr(), // pBufferMemoryBarriers
			0,                    // imageMemoryBarrierCount
			memory.Nullptr,       // pImageMemoryBarriers
		).AddRead(bufBarrierData.Data()),
		overdrawTransform.cmdBuilder.VkCmdCopyBufferToImage(cmdBuffer,
			copyBuffer,            // srcBuffer
			dstImg.VulkanHandle(), // dstImage
			VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, // dstImageLayout
			1,                // regionCount
			biCopyData.Ptr(), // pRegions
		).AddRead(biCopyData.Data()),
		overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(cmdBuffer,
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_TRANSFER_BIT), // srcStageMask
			allCommandsStage,       // dstStageMask
			0,                      // dependencyFlags
			0,                      // memoryBarrierCount
			memory.Nullptr,         // pMemoryBarriers
			0,                      // bufferMemoryBarrierCount
			memory.Nullptr,         // pBufferMemoryBarriers
			2,                      // imageMemoryBarrierCount
			imgBarriers1Data.Ptr(), // pImageMemoryBarriers
		).AddRead(imgBarriers1Data.Data()),
	)

	return outputCmds
}

type overdrawTransformWriter struct {
	state            *api.GlobalState
	statebuilderCmds []api.Cmd
}

func newOverdrawTransformWriter(state *api.GlobalState) *overdrawTransformWriter {
	return &overdrawTransformWriter{
		state:            state,
		statebuilderCmds: make([]api.Cmd, 0),
	}
}

func (writer *overdrawTransformWriter) State() *api.GlobalState {
	return writer.state
}

func (writer *overdrawTransformWriter) MutateAndWrite(ctx context.Context, id api.CmdID, cmd api.Cmd) error {
	writer.statebuilderCmds = append(writer.statebuilderCmds, cmd)
	return nil
}

func (overdrawTransform *stencilOverdraw) renderAspect(
	ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	queue VkQueue,
	cmdBuffer VkCommandBuffer,
	srcImg imageDesc,
	dstImg imageDesc,
	inputFormat VkFormat) ([]api.Cmd, error) {

	// Melih TODO: Should this clone a state?
	tempTransformWriter := newOverdrawTransformWriter(inputState)
	sb := GetState(inputState).newStateBuilder(ctx, newTransformerOutput(tempTransformWriter))
	queueHandler, err := newQueueCommandHandler(sb, queue, cmdBuffer)
	if err != nil {
		return nil, log.Errf(sb.ctx, err, "failed at creating queue command handler")
	}
	dstLayer := dstImg.subresource.BaseArrayLayer()
	if srcImg.subresource.BaseArrayLayer() != dstLayer {
		return nil, fmt.Errorf("input attachment and render target layer do not match")
	}
	dstLevel := dstImg.subresource.BaseMipLevel()
	if srcImg.subresource.BaseMipLevel() != dstLevel {
		return nil, fmt.Errorf("input attachment and render target mip level do not match")
	}
	sizes := sb.levelSize(dstImg.image.Info().Extent(),
		dstImg.image.Info().Fmt(), dstLevel, dstImg.aspect)
	recipe := ipRenderRecipe{
		inputAttachmentImage:  srcImg.image.VulkanHandle(),
		inputAttachmentAspect: srcImg.aspect,
		renderImage:           dstImg.image.VulkanHandle(),
		renderAspect:          dstImg.aspect,
		layer:                 dstLayer,
		level:                 dstLevel,
		renderRectX:           int32(0),
		renderRectY:           int32(0),
		renderRectWidth:       uint32(sizes.width),
		renderRectHeight:      uint32(sizes.height),
		wordIndex:             uint32(0),
		framebufferWidth:      uint32(sizes.width),
		framebufferHeight:     uint32(sizes.height),
	}
	ip := newImagePrimer(sb)
	queueHandler.RecordPostExecuted(func() { ip.Free() })
	renderKitBuilder := ip.GetRenderKitBuilder(device)
	kits, err := renderKitBuilder.BuildRenderKits(sb, recipe)
	if err != nil {
		return nil, log.Errf(sb.ctx, err, "failed at building render kits")
	}
	if len(kits) != 1 {
		return nil, fmt.Errorf("unexpected length of render kits, actual: %v, expected: 1", len(kits))
	}
	renderingLayout := ipRenderColorOutputLayout
	if (dstImg.image.Info().Usage() & VkImageUsageFlags(VkImageUsageFlagBits_VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT)) != 0 {
		renderingLayout = ipRenderDepthStencilOutputLayout
	}

	inputPreBarrier := ipImageSubresourceLayoutTransitionBarrier(sb,
		srcImg.image,
		srcImg.aspect,
		srcImg.subresource.BaseArrayLayer(),
		srcImg.subresource.BaseMipLevel(),
		srcImg.layout,
		ipRenderInputAttachmentLayout,
	)
	inputPostBarrier := ipImageSubresourceLayoutTransitionBarrier(sb,
		srcImg.image,
		srcImg.aspect,
		srcImg.subresource.BaseArrayLayer(),
		srcImg.subresource.BaseMipLevel(),
		ipRenderInputAttachmentLayout,
		srcImg.layout,
	)
	outputPreBarrier := ipImageSubresourceLayoutTransitionBarrier(sb,
		dstImg.image,
		dstImg.aspect,
		dstImg.subresource.BaseArrayLayer(),
		dstImg.subresource.BaseMipLevel(),
		dstImg.layout,
		renderingLayout,
	)
	outputPostBarrier := ipImageSubresourceLayoutTransitionBarrier(sb,
		dstImg.image,
		dstImg.aspect,
		dstImg.subresource.BaseArrayLayer(),
		dstImg.subresource.BaseMipLevel(),
		renderingLayout,
		dstImg.layout,
	)

	err = ipRecordImageMemoryBarriers(sb, queueHandler, inputPreBarrier, outputPreBarrier)
	if err != nil {
		return nil, log.Errf(sb.ctx, err, "failed at recording pre rendering image layout transition")
	}
	renderCmds := kits[0].BuildRenderCommands(sb)
	err = renderCmds.Commit(sb, queueHandler)
	if err != nil {
		return nil, log.Errf(sb.ctx, err, "failed at commiting rendering commands")
	}
	err = ipRecordImageMemoryBarriers(sb, queueHandler, inputPostBarrier, outputPostBarrier)
	if err != nil {
		return nil, log.Errf(sb.ctx, err, "failed at recording post rendering image layout transition")
	}

	// Make sure it doesn't use temporary memory as that would cause a flush of the scratch resources
	// queueScratch.memorySize = scratchTask.totalAllocationSize

	cleanupFunc := func() {
		queueHandler.Submit(sb)
		queueHandler.WaitUntilFinish(sb)
	}

	overdrawTransform.cleanupFunctions = append(overdrawTransform.cleanupFunctions, cleanupFunc)
	return tempTransformWriter.statebuilderCmds, nil
}

func (overdrawTransform *stencilOverdraw) transferDepthValues(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	queue VkQueue,
	cmdBuffer VkCommandBuffer,
	width uint32,
	height uint32,
	srcImgDesc imageDesc,
	dstImgDesc imageDesc) ([]api.Cmd, error) {

	srcFmt := srcImgDesc.image.Info().Fmt()
	dstFmt := dstImgDesc.image.Info().Fmt()
	if depthBits(srcFmt) == depthBits(dstFmt) {
		newCmds := overdrawTransform.copyImageAspect(ctx, inputState, device, cmdBuffer,
			srcImgDesc, dstImgDesc, NewVkExtent3D(inputState.Arena, width, height, 1))

		return newCmds, nil
	}

	outputCmds := make([]api.Cmd, 0)

	// This would have errored previously if it was going to error now
	depthFmt, _ := depthStencilToDepthFormat(srcFmt)
	stageFmt, _ := depthToStageFormat(depthFmt)
	imageCmds, stageImgInfo, err := overdrawTransform.createImage(ctx, inputState, device, stageFmt, width, height)
	if err != nil {
		return nil, err
	}

	if imageCmds != nil {
		outputCmds = append(outputCmds, imageCmds...)
	}

	stageImg := GetState(inputState).Images().Get(stageImgInfo.handle)
	stageImgDesc := imageDesc{
		stageImg,
		NewVkImageSubresourceRange(inputState.Arena,
			VkImageAspectFlags(VkImageAspectFlagBits_VK_IMAGE_ASPECT_COLOR_BIT),
			0,
			1,
			0,
			1,
		),
		VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED, // this will be transitioned to general
		VkImageAspectFlagBits_VK_IMAGE_ASPECT_COLOR_BIT,
	}
	copyCmds := overdrawTransform.copyImageAspect(ctx,
		inputState, device, cmdBuffer, srcImgDesc, stageImgDesc,
		NewVkExtent3D(inputState.Arena, width, height, 1))

	if copyCmds != nil {
		outputCmds = append(outputCmds, copyCmds...)
	}

	stageImgDesc.layout = VkImageLayout_VK_IMAGE_LAYOUT_GENERAL

	renderCmds, err := overdrawTransform.renderAspect(ctx, inputState, device, queue,
		cmdBuffer, stageImgDesc, dstImgDesc, srcFmt)
	if err != nil {
		return nil, err
	}

	if renderCmds != nil {
		outputCmds = append(outputCmds, renderCmds...)
	}

	return outputCmds, nil
}

// If the depth attachment is in "load" mode we need to copy the depth values
// over to the depth aspect of our new depth/stencil buffer.
func (overdrawTransform *stencilOverdraw) loadExistingDepthValues(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	queue VkQueue,
	cmdBuffer VkCommandBuffer,
	renderInfo renderInfo) ([]api.Cmd, error) {
	if renderInfo.depthIdx == ^uint32(0) {
		return nil, nil
	}
	rpInfo := GetState(inputState).RenderPasses().Get(renderInfo.renderPass)
	daInfo := rpInfo.AttachmentDescriptions().Get(renderInfo.depthIdx)

	if daInfo.LoadOp() != VkAttachmentLoadOp_VK_ATTACHMENT_LOAD_OP_LOAD {
		return nil, nil
	}

	fbInfo := GetState(inputState).Framebuffers().Get(renderInfo.framebuffer)

	oldImageView := fbInfo.ImageAttachments().Get(renderInfo.depthIdx)
	newImageView := fbInfo.ImageAttachments().Get(uint32(fbInfo.ImageAttachments().Len() - 1))

	oldImageDesc := imageDesc{
		oldImageView.Image(),
		oldImageView.SubresourceRange(),
		daInfo.InitialLayout(),
		VkImageAspectFlagBits_VK_IMAGE_ASPECT_DEPTH_BIT,
	}
	newImageDesc := imageDesc{
		newImageView.Image(),
		newImageView.SubresourceRange(),
		VkImageLayout_VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
		VkImageAspectFlagBits_VK_IMAGE_ASPECT_DEPTH_BIT,
	}

	return overdrawTransform.transferDepthValues(ctx, inputState, device, queue,
		cmdBuffer, fbInfo.Width(), fbInfo.Height(), oldImageDesc, newImageDesc)
}

// If the depth attachment is in "store" mode we need to copy the depth values
// over from the depth aspect of our new depth/stencil buffer.
func (overdrawTransform *stencilOverdraw) storeNewDepthValues(ctx context.Context,
	inputState *api.GlobalState,
	device VkDevice,
	queue VkQueue,
	cmdBuffer VkCommandBuffer,
	renderInfo renderInfo) ([]api.Cmd, error) {

	if renderInfo.depthIdx == ^uint32(0) {
		return nil, nil
	}
	rpInfo := GetState(inputState).RenderPasses().Get(renderInfo.renderPass)
	daInfo := rpInfo.AttachmentDescriptions().Get(renderInfo.depthIdx)

	if daInfo.StoreOp() != VkAttachmentStoreOp_VK_ATTACHMENT_STORE_OP_STORE {
		return nil, nil
	}

	fbInfo := GetState(inputState).Framebuffers().Get(renderInfo.framebuffer)

	oldImageView := fbInfo.ImageAttachments().Get(uint32(fbInfo.ImageAttachments().Len() - 1))
	newImageView := fbInfo.ImageAttachments().Get(renderInfo.depthIdx)

	oldImageDesc := imageDesc{
		oldImageView.Image(),
		oldImageView.SubresourceRange(),
		rpInfo.AttachmentDescriptions().Get(uint32(fbInfo.ImageAttachments().Len() - 1)).FinalLayout(),
		VkImageAspectFlagBits_VK_IMAGE_ASPECT_DEPTH_BIT,
	}
	newImageDesc := imageDesc{
		newImageView.Image(),
		newImageView.SubresourceRange(),
		daInfo.FinalLayout(),
		VkImageAspectFlagBits_VK_IMAGE_ASPECT_DEPTH_BIT,
	}
	return overdrawTransform.transferDepthValues(ctx, inputState, device, queue,
		cmdBuffer, fbInfo.Width(), fbInfo.Height(), oldImageDesc, newImageDesc)
}

func (overdrawTransform *stencilOverdraw) transitionStencilImage(ctx context.Context,
	inputState *api.GlobalState,
	cmdBuffer VkCommandBuffer,
	renderInfo renderInfo) api.Cmd {

	imageView := GetState(inputState).ImageViews().Get(renderInfo.view)
	imgBarrier := NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		0, // srcAccessMask
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT|
			VkAccessFlagBits_VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT|
			VkAccessFlagBits_VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT), // dstAccessMask
		VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED,                        // oldLayout
		VkImageLayout_VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL, // newLayout
		^uint32(0),                       // srcQueueFamilyIndex: VK_QUEUE_FAMILY_IGNORED
		^uint32(0),                       // dstQueueFamilyIndex
		imageView.Image().VulkanHandle(), // image
		imageView.SubresourceRange(),     // subresourceRange
	)

	imgBarrierData := overdrawTransform.allocations.AllocDataOrPanic(ctx, imgBarrier)

	return overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(cmdBuffer,
		VkPipelineStageFlags(
			VkPipelineStageFlagBits_VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT), // srcStageMask
		VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_TRANSFER_BIT|
			VkPipelineStageFlagBits_VK_PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT|
			VkPipelineStageFlagBits_VK_PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT), // dstStageMask
		0,                    // dependencyFlags
		0,                    // memoryBarrierCount
		memory.Nullptr,       // pMemoryBarriers
		0,                    // bufferMemoryBarrierCount
		memory.Nullptr,       // pBufferMemoryBarriers
		1,                    // imageMemoryBarrierCount
		imgBarrierData.Ptr(), // pImageMemoryBarriers
	).AddRead(imgBarrierData.Data())
}

func (overdrawTransform *stencilOverdraw) createCommandBuffer(ctx context.Context,
	inputState *api.GlobalState,
	queue VkQueue,
	cmdBuffer VkCommandBuffer,
	renderInfo renderInfo,
	rpStartIdx uint64) ([]api.Cmd, VkCommandBuffer, error) {
	bInfo, ok := GetState(inputState).CommandBuffers().Lookup(cmdBuffer)
	if !ok {
		return nil, VkCommandBuffer(0), fmt.Errorf("Invalid command buffer %v", cmdBuffer)
	}
	device := bInfo.Device()

	outputCmds := make([]api.Cmd, 0)
	newCmdBuffer, cmdBufferCmds, cleanup := allocateNewCmdBufFromExistingOneAndBegin(ctx,
		*overdrawTransform.cmdBuilder, cmdBuffer, inputState)

	if cmdBufferCmds != nil {
		outputCmds = append(outputCmds, cmdBufferCmds...)
	}
	overdrawTransform.cleanupFunctions = append(overdrawTransform.cleanupFunctions, cleanup...)

	pipelines := map[VkPipeline]VkPipeline{}
	secCmdBuffers := map[VkCommandBuffer]VkCommandBuffer{}

	rpEnded := false
	for i := 0; i < bInfo.CommandReferences().Len(); i++ {
		cr := bInfo.CommandReferences().Get(uint32(i))
		args := GetCommandArgs(ctx, cr, GetState(inputState))
		if uint64(i) >= rpStartIdx && !rpEnded {
			switch ar := args.(type) {
			case VkCmdBeginRenderPassArgsʳ:
				// Transition the stencil image to the right layout
				newCmd := overdrawTransform.transitionStencilImage(ctx, inputState, newCmdBuffer, renderInfo)
				if newCmd != nil {
					outputCmds = append(outputCmds, newCmd)
				}
				// Add commands to handle copying the old depth
				// values if necessary
				loadCmds, err := overdrawTransform.loadExistingDepthValues(
					ctx, inputState, device, queue, newCmdBuffer, renderInfo)
				if err != nil {
					return nil, VkCommandBuffer(0), err
				}

				if loadCmds != nil {
					outputCmds = append(outputCmds, loadCmds...)
				}

				newArgs := ar.Clone(inputState.Arena, api.CloneContext{})
				newArgs.SetRenderPass(renderInfo.renderPass)
				newArgs.SetFramebuffer(renderInfo.framebuffer)

				rpInfo := GetState(inputState).RenderPasses().Get(renderInfo.renderPass)
				attachmentIdx := uint32(rpInfo.AttachmentDescriptions().Len()) - 1
				newClear := NewU32ː4ᵃ(inputState.Arena)

				if renderInfo.depthIdx != ^uint32(0) &&
					rpInfo.AttachmentDescriptions().Get(renderInfo.depthIdx).LoadOp() ==
						VkAttachmentLoadOp_VK_ATTACHMENT_LOAD_OP_CLEAR {
					newClear.Set(0, newArgs.
						ClearValues().
						Get(renderInfo.depthIdx).
						Color().
						Uint32().
						Get(0))
				}
				for j := uint32(0); j < attachmentIdx; j++ {
					if !newArgs.ClearValues().Contains(j) {
						newArgs.ClearValues().Add(j, NilVkClearValue)
					}
				}
				// 0 initialize the stencil buffer
				// Use VkClearColorValue instead of
				// VkClearDepthValue because it doesn't
				// seem like the union is set up in the
				// API DSL
				newArgs.ClearValues().Add(attachmentIdx, NewVkClearValue(inputState.Arena,
					NewVkClearColorValue(inputState.Arena, newClear)))
				args = newArgs
			case VkCmdEndRenderPassArgsʳ:
				rpEnded = true
			case VkCmdBindPipelineArgsʳ:
				newArgs := ar
				if ar.PipelineBindPoint() == VkPipelineBindPoint_VK_PIPELINE_BIND_POINT_GRAPHICS {
					newArgs = ar.Clone(inputState.Arena, api.CloneContext{})
					pipe := ar.Pipeline()
					newPipe, ok := pipelines[pipe]
					if !ok {
						createPipelineCmds, createdPipe, err := overdrawTransform.createGraphicsPipeline(
							ctx, inputState, device, pipe, renderInfo.renderPass)
						if err != nil {
							return nil, VkCommandBuffer(0), err
						}
						if createPipelineCmds != nil {
							outputCmds = append(outputCmds, createPipelineCmds...)
						}
						newPipe = createdPipe
						pipelines[pipe] = newPipe
					}
					newArgs.SetPipeline(newPipe)
				}
				args = newArgs
			case VkCmdExecuteCommandsArgsʳ:
				newArgs := ar
				for i := uint32(0); i < uint32(ar.CommandBuffers().Len()); i++ {
					cmdbuf := ar.CommandBuffers().Get(i)
					newCmdbuf, ok := secCmdBuffers[cmdbuf]
					if !ok {
						createCmdBufferCmds, createdCmdbuf, err := overdrawTransform.createCommandBuffer(
							ctx, inputState, queue, cmdbuf, renderInfo, 0)
						if err != nil {
							return nil, VkCommandBuffer(0), err
						}
						if createCmdBufferCmds != nil {
							outputCmds = append(outputCmds, createCmdBufferCmds...)
						}
						newCmdBuffer = createdCmdbuf
						secCmdBuffers[cmdbuf] = newCmdbuf
					}
					newArgs.CommandBuffers().Add(i, newCmdbuf)
				}
				args = newArgs
			}
		}
		cleanup, cmd, _ := AddCommand(ctx, *overdrawTransform.cmdBuilder, newCmdBuffer, inputState, inputState, args)
		if cmd != nil {
			outputCmds = append(outputCmds, cmd)
		}

		overdrawTransform.cleanupFunctions = append(overdrawTransform.cleanupFunctions, cleanup)

		if _, ok := args.(VkCmdEndRenderPassArgsʳ); ok {
			// Add commands to handle storing the new depth values if necessary
			storeCmds, err := overdrawTransform.storeNewDepthValues(ctx,
				inputState, device, queue, newCmdBuffer, renderInfo)
			if err != nil {
				return nil, VkCommandBuffer(0), err
			}
			if storeCmds != nil {
				outputCmds = append(outputCmds, storeCmds...)
			}
		}
	}
	outputCmds = append(outputCmds, overdrawTransform.cmdBuilder.VkEndCommandBuffer(
		newCmdBuffer, VkResult_VK_SUCCESS))

	return outputCmds, newCmdBuffer, nil
}

func (overdrawTransform *stencilOverdraw) rewriteQueueSubmit(ctx context.Context,
	inputState *api.GlobalState,
	queueSubmitCmd *VkQueueSubmit,
	rpBeginArgs VkCmdBeginRenderPassArgsʳ,
	rpBeginIdx api.SubCmdIdx) ([]api.Cmd, stencilImage, error) {

	// Need to deep clone all of the submit info so we can mark it as
	// reads.  TODO: We could possibly optimize this by copying the
	// pointers and using the fact that we know what size it should be to
	// create the observations.
	reads := []api.AllocResult{}
	allocAndRead := func(v ...interface{}) api.AllocResult {
		res := overdrawTransform.allocations.AllocDataOrPanic(ctx, v)
		reads = append(reads, res)
		return res
	}

	outputCmds := make([]api.Cmd, 0)
	newCmds, renderInfo, err := overdrawTransform.createNewRenderPassFramebuffer(
		ctx, inputState, rpBeginArgs.RenderPass(), rpBeginArgs.Framebuffer())
	if err != nil {
		return nil, stencilImage{}, err
	}
	if newCmds != nil {
		outputCmds = append(outputCmds, newCmds...)
	}

	layout := inputState.MemoryLayout
	queueSubmitCmd.Extras().Observations().ApplyReads(inputState.Memory.ApplicationPool())
	submitCount := queueSubmitCmd.SubmitCount()
	submitInfos := queueSubmitCmd.PSubmits().Slice(0, uint64(submitCount), layout).MustRead(
		ctx, queueSubmitCmd, inputState, nil)

	newSubmitInfos := make([]VkSubmitInfo, submitCount)
	for i := uint32(0); i < submitCount; i++ {
		si := submitInfos[i]

		waitSemPtr := memory.Nullptr
		waitDstStagePtr := memory.Nullptr
		if count := uint64(si.WaitSemaphoreCount()); count > 0 {
			waitSemPtr = allocAndRead(si.PWaitSemaphores().
				Slice(0, count, layout).
				MustRead(ctx, queueSubmitCmd, inputState, nil)).Ptr()
			waitDstStagePtr = allocAndRead(si.PWaitDstStageMask().
				Slice(0, count, layout).
				MustRead(ctx, queueSubmitCmd, inputState, nil)).Ptr()
		}

		signalSemPtr := memory.Nullptr
		if count := uint64(si.SignalSemaphoreCount()); count > 0 {
			signalSemPtr = allocAndRead(si.PSignalSemaphores().
				Slice(0, count, layout).
				MustRead(ctx, queueSubmitCmd, inputState, nil)).Ptr()
		}

		cmdBufferPtr := memory.Nullptr
		if count := uint64(si.CommandBufferCount()); count > 0 {
			cmdBuffers := si.PCommandBuffers().
				Slice(0, count, layout).
				MustRead(ctx, queueSubmitCmd, inputState, nil)
			if uint64(i) == rpBeginIdx[0] {
				createCmdBuffCmds, newCommandBuffer, err := overdrawTransform.createCommandBuffer(ctx, inputState,
					queueSubmitCmd.Queue(), cmdBuffers[rpBeginIdx[1]], renderInfo, rpBeginIdx[2])
				if err != nil {
					return nil, stencilImage{}, err
				}
				if createCmdBuffCmds != nil {
					outputCmds = append(outputCmds, createCmdBuffCmds...)
				}

				cmdBuffers[rpBeginIdx[1]] = newCommandBuffer
			}
			cmdBufferPtr = allocAndRead(cmdBuffers).Ptr()
		}

		newSubmitInfos[i] = NewVkSubmitInfo(inputState.Arena,
			VkStructureType_VK_STRUCTURE_TYPE_SUBMIT_INFO,
			0,                            // pNext
			si.WaitSemaphoreCount(),      // waitSemaphoreCount
			NewVkSemaphoreᶜᵖ(waitSemPtr), // pWaitSemaphores
			NewVkPipelineStageFlagsᶜᵖ(waitDstStagePtr), // pWaitDstStageMask
			si.CommandBufferCount(),                    // commandBufferCount
			NewVkCommandBufferᶜᵖ(cmdBufferPtr),         // pCommandBuffers
			si.SignalSemaphoreCount(),                  // signalSemaphoreCount
			NewVkSemaphoreᶜᵖ(signalSemPtr),             // pSignalSemaphores
		)
	}
	submitInfoPtr := allocAndRead(newSubmitInfos).Ptr()

	cmd := overdrawTransform.cmdBuilder.VkQueueSubmit(
		queueSubmitCmd.Queue(),
		queueSubmitCmd.SubmitCount(),
		submitInfoPtr,
		queueSubmitCmd.Fence(),
		VkResult_VK_SUCCESS,
	)
	for _, read := range reads {
		cmd.AddRead(read.Data())
	}

	outputCmds = append(outputCmds, cmd)

	return outputCmds, renderInfo.image, nil
}

func (overdrawTransform *stencilOverdraw) postStencilImageData(ctx context.Context, inputState *api.GlobalState, img stencilImage, res replay.Result) []api.Cmd {
	imageObject := GetState(inputState).Images().Get(img.handle)
	vkFormat := img.format
	layer := uint32(0)
	level := uint32(0)
	imgWidth := img.width
	imgHeight := img.height
	requestWidth := img.width
	requestHeight := img.height
	aspectFlagBit := VkImageAspectFlagBits_VK_IMAGE_ASPECT_STENCIL_BIT

	// This is the format used for building the final image resource and
	// calculating the data size for the final resource. Note that the staging
	// image is not created with this format.
	var formatOfImgRes *image.Format
	var err error
	if aspectFlagBit == VkImageAspectFlagBits_VK_IMAGE_ASPECT_COLOR_BIT {
		formatOfImgRes, err = getImageFormatFromVulkanFormat(vkFormat)
	} else if aspectFlagBit == VkImageAspectFlagBits_VK_IMAGE_ASPECT_DEPTH_BIT {
		// When depth image is requested, the format, which is used for
		// resolving/bliting/copying attachment image data to the mapped buffer
		// might be different with the format used in image resource. This is
		// because we need to strip the stencil data if the source attachment image
		// contains both depth and stencil data.
		formatOfImgRes, err = getDepthImageFormatFromVulkanFormat(vkFormat)
	} else if aspectFlagBit == VkImageAspectFlagBits_VK_IMAGE_ASPECT_STENCIL_BIT {
		// Similarly to above, we may need to strip the depth data if the
		// source attachment image contains both depth and stencil data.
		formatOfImgRes, err = getStencilImageFormatFromVulkanFormat(vkFormat)
	} else {
		res(nil, &service.ErrDataUnavailable{Reason: messages.ErrFramebufferUnavailable()})
		return nil
	}
	if err != nil {
		res(nil, &service.ErrDataUnavailable{Reason: messages.ErrFramebufferUnavailable()})
		return nil
	}

	resolveSrcDepth := int32(0)
	blitSrcDepth := int32(0)
	copySrcDepth := int32(0)

	if imageObject.Info().ImageType() == VkImageType_VK_IMAGE_TYPE_3D {
		resolveSrcDepth = int32(layer)
		blitSrcDepth = int32(layer)
		copySrcDepth = int32(layer)
		layer = 0
	}
	resolveSrcLayer := layer
	blitSrcLayer := layer
	copySrcLayer := layer
	if imageObject.Info().Samples() != VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT {
		resolveSrcLayer = layer
		blitSrcDepth = 0
		blitSrcLayer = 0
		copySrcDepth = 0
		copySrcLayer = 0
	}
	doBlit := !(requestWidth == imgWidth && requestHeight == imgHeight)
	if doBlit {
		copySrcDepth = 0
		copySrcLayer = 0
	}

	origLayout := imageObject.Aspects().Get(aspectFlagBit).Layers().Get(layer).Levels().Get(level).Layout()

	queue := imageObject.Aspects().Get(aspectFlagBit).Layers().Get(layer).Levels().Get(level).LastBoundQueue()
	if queue.IsNil() {
		queue = imageObject.LastBoundQueue()
		if queue.IsNil() {
			res(nil, &service.ErrDataUnavailable{Reason: messages.ErrMessage("The target image object has not been bound with a vkQueue")})
			return nil
		}
	}

	vkQueue := queue.VulkanHandle()
	vkDevice := queue.Device()
	device := GetState(inputState).Devices().Get(vkDevice)
	vkPhysicalDevice := device.PhysicalDevice()
	physicalDevice := GetState(inputState).PhysicalDevices().Get(vkPhysicalDevice)

	if properties, ok := physicalDevice.QueueFamilyProperties().Lookup(queue.Family()); ok {
		if properties.QueueFlags()&VkQueueFlags(VkQueueFlagBits_VK_QUEUE_GRAPHICS_BIT) == 0 {
			if imageObject.Info().Samples() == VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT &&
				aspectFlagBit == VkImageAspectFlagBits_VK_IMAGE_ASPECT_COLOR_BIT {
				// If this is on the compute queue, we cannot do a blit operation,
				// We can however do it on the CPU afterwards, or let the
				// client deal with it
				requestWidth = imgWidth
				requestHeight = imgHeight
			} else {
				res(nil, &service.ErrDataUnavailable{Reason: messages.ErrMessage("Unhandled: Reading a multisampled or depth image on the compute queue")})
				return nil
			}
		}
	} else {
		res(nil, &service.ErrDataUnavailable{Reason: messages.ErrMessage("Not found the properties information of the bound vkQueue")})
		return nil
	}

	fenceID := VkFence(newUnusedID(false, func(x uint64) bool { return GetState(inputState).Fences().Contains(VkFence(x)) }))

	fenceCreateInfo := NewVkFenceCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_FENCE_CREATE_INFO, // sType
		NewVoidᶜᵖ(memory.Nullptr),                           // pNext
		VkFenceCreateFlags(0),                               // flags
	)

	fenceCreateData := overdrawTransform.allocations.AllocDataOrPanic(ctx, fenceCreateInfo)
	fenceData := overdrawTransform.allocations.AllocDataOrPanic(ctx, fenceID)

	// The physical device memory properties are used for
	// replayAllocateImageMemory to find the correct memory type index and
	// allocate proper memory for our staging and resolving image.
	physicalDeviceMemoryPropertiesData := overdrawTransform.allocations.AllocDataOrPanic(ctx, physicalDevice.MemoryProperties())
	bufferMemoryTypeIndex := uint32(0)
	for i := uint32(0); i < physicalDevice.MemoryProperties().MemoryTypeCount(); i++ {
		t := physicalDevice.MemoryProperties().MemoryTypes().Get(int(i))
		if 0 != (t.PropertyFlags() & VkMemoryPropertyFlags(
			VkMemoryPropertyFlagBits_VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT|
				VkMemoryPropertyFlagBits_VK_MEMORY_PROPERTY_HOST_COHERENT_BIT)) {
			bufferMemoryTypeIndex = i
			break
		}
	}

	bufferSize := uint64(formatOfImgRes.Size(int(requestWidth), int(requestHeight), 1))
	// For the depth aspect of VK_FORMAT_X8_D24_UNORM_PACK32 and
	// VK_FORMAT_D24_UNORM_S8_UINT format, each depth element requires 4 bytes in
	// the buffer when used in buffer image copy.
	if aspectFlagBit == VkImageAspectFlagBits_VK_IMAGE_ASPECT_DEPTH_BIT && (vkFormat == VkFormat_VK_FORMAT_X8_D24_UNORM_PACK32 || vkFormat == VkFormat_VK_FORMAT_D24_UNORM_S8_UINT) {
		r32Fmt, _ := getImageFormatFromVulkanFormat(VkFormat_VK_FORMAT_R32_UINT)
		bufferSize = uint64(r32Fmt.Size(int(requestWidth), int(requestHeight), 1))
	}

	// Data and info for destination buffer creation
	bufferID := VkBuffer(newUnusedID(false, func(x uint64) bool { ok := GetState(inputState).Buffers().Contains(VkBuffer(x)); return ok }))
	bufferMemoryID := VkDeviceMemory(newUnusedID(false, func(x uint64) bool {
		ok := GetState(inputState).DeviceMemories().Contains(VkDeviceMemory(x))
		return ok
	}))
	bufferMemoryAllocInfo := NewVkMemoryAllocateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO, // sType
		0,                          // pNext
		VkDeviceSize(bufferSize*2), // allocationSize
		bufferMemoryTypeIndex,      // memoryTypeIndex
	)
	bufferMemoryAllocateInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, bufferMemoryAllocInfo)
	bufferMemoryData := overdrawTransform.allocations.AllocDataOrPanic(ctx, bufferMemoryID)
	bufferCreateInfo := NewVkBufferCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO,                       // sType
		NewVoidᶜᵖ(memory.Nullptr),                                                  // pNext
		VkBufferCreateFlags(0),                                                     // flags
		VkDeviceSize(bufferSize),                                                   // size
		VkBufferUsageFlags(VkBufferUsageFlagBits_VK_BUFFER_USAGE_TRANSFER_DST_BIT), // usage
		VkSharingMode_VK_SHARING_MODE_EXCLUSIVE,                                    // sharingMode
		0,                                                                          // queueFamilyIndexCount
		NewU32ᶜᵖ(memory.Nullptr),                                                   // pQueueFamilyIndices
	)
	bufferCreateInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, bufferCreateInfo)
	bufferData := overdrawTransform.allocations.AllocDataOrPanic(ctx, bufferID)

	// Data and info for staging image creation
	stagingImageID := VkImage(newUnusedID(false, func(x uint64) bool { ok := GetState(inputState).Images().Contains(VkImage(x)); return ok }))
	stagingImageCreateInfo := NewVkImageCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO, // sType
		0,                            // pNext
		0,                            // flags
		VkImageType_VK_IMAGE_TYPE_2D, // imageType
		vkFormat,                     // format
		NewVkExtent3D(inputState.Arena, // extent
			requestWidth,
			requestHeight,
			1,
		),
		1, // mipLevels
		1, // arrayLayers
		VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT, // samples
		VkImageTiling_VK_IMAGE_TILING_OPTIMAL,       // tiling
		VkImageUsageFlags(VkImageUsageFlagBits_VK_IMAGE_USAGE_TRANSFER_SRC_BIT|
			VkImageUsageFlagBits_VK_IMAGE_USAGE_TRANSFER_DST_BIT), // usage
		VkSharingMode_VK_SHARING_MODE_EXCLUSIVE, // sharingMode
		0,                                       // queueFamilyIndexCount
		0,                                       // pQueueFamilyIndices
		VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED, // initialLayout
	)
	stagingImageCreateInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, stagingImageCreateInfo)
	stagingImageData := overdrawTransform.allocations.AllocDataOrPanic(ctx, stagingImageID)
	stagingImageMemoryID := VkDeviceMemory(newUnusedID(false, func(x uint64) bool {
		ok := GetState(inputState).DeviceMemories().Contains(VkDeviceMemory(x))
		ok = ok || VkDeviceMemory(x) == bufferMemoryID
		return ok
	}))
	stagingImageMemoryData := overdrawTransform.allocations.AllocDataOrPanic(ctx, stagingImageMemoryID)

	// Data and info for resolve image creation. Resolve image is used when the attachment image is multi-sampled
	resolveImageID := VkImage(newUnusedID(false, func(x uint64) bool { ok := GetState(inputState).Images().Contains(VkImage(x)); return ok }))
	resolveImageCreateInfo := NewVkImageCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO, // sType
		0,                            // pNext
		0,                            // flags
		VkImageType_VK_IMAGE_TYPE_2D, // imageType
		vkFormat,                     // format
		NewVkExtent3D(inputState.Arena, // extent
			imgWidth,  // same width as the attachment image, not the request
			imgHeight, // same height as the attachment image, not the request
			1),
		1, // mipLevels
		1, // arrayLayers
		VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT, // samples
		VkImageTiling_VK_IMAGE_TILING_OPTIMAL,       // tiling
		VkImageUsageFlags(VkImageUsageFlagBits_VK_IMAGE_USAGE_TRANSFER_SRC_BIT|
			VkImageUsageFlagBits_VK_IMAGE_USAGE_TRANSFER_DST_BIT), // usage
		VkSharingMode_VK_SHARING_MODE_EXCLUSIVE, // sharingMode
		0,                                       // queueFamilyIndexCount
		0,                                       // pQueueFamilyIndices
		VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED, // initialLayout
	)
	resolveImageCreateInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, resolveImageCreateInfo)
	resolveImageData := overdrawTransform.allocations.AllocDataOrPanic(ctx, resolveImageID)
	resolveImageMemoryID := VkDeviceMemory(newUnusedID(false, func(x uint64) bool {
		ok := GetState(inputState).DeviceMemories().Contains(VkDeviceMemory(x))
		ok = ok || VkDeviceMemory(x) == bufferMemoryID || VkDeviceMemory(x) == stagingImageMemoryID
		return ok
	}))
	resolveImageMemoryData := overdrawTransform.allocations.AllocDataOrPanic(ctx, resolveImageMemoryID)

	// Command pool and command buffer
	commandPoolID := VkCommandPool(newUnusedID(false, func(x uint64) bool { ok := GetState(inputState).CommandPools().Contains(VkCommandPool(x)); return ok }))
	commandPoolCreateInfo := NewVkCommandPoolCreateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,                                 // sType
		NewVoidᶜᵖ(memory.Nullptr),                                                                  // pNext
		VkCommandPoolCreateFlags(VkCommandPoolCreateFlagBits_VK_COMMAND_POOL_CREATE_TRANSIENT_BIT), // flags
		queue.Family(), // queueFamilyIndex
	)
	commandPoolCreateInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, commandPoolCreateInfo)
	commandPoolData := overdrawTransform.allocations.AllocDataOrPanic(ctx, commandPoolID)
	commandBufferAllocateInfo := NewVkCommandBufferAllocateInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO, // sType
		NewVoidᶜᵖ(memory.Nullptr),                                      // pNext
		commandPoolID,                                                  // commandPool
		VkCommandBufferLevel_VK_COMMAND_BUFFER_LEVEL_PRIMARY,           // level
		1, // commandBufferCount
	)
	commandBufferAllocateInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, commandBufferAllocateInfo)
	commandBufferID := VkCommandBuffer(newUnusedID(true, func(x uint64) bool {
		ok := GetState(inputState).CommandBuffers().Contains(VkCommandBuffer(x))
		return ok
	}))
	commandBufferData := overdrawTransform.allocations.AllocDataOrPanic(ctx, commandBufferID)

	// Data and info for Vulkan commands in command buffers
	beginCommandBufferInfo := NewVkCommandBufferBeginInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO, // sType
		0, // pNext
		VkCommandBufferUsageFlags(VkCommandBufferUsageFlagBits_VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT), // flags
		0, // pInheritanceInfo
	)
	beginCommandBufferInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, beginCommandBufferInfo)

	bufferImageCopy := NewVkBufferImageCopy(inputState.Arena,
		0, // bufferOffset
		0, // bufferRowLength
		0, // bufferImageHeight
		NewVkImageSubresourceLayers(inputState.Arena, // imageSubresource
			VkImageAspectFlags(aspectFlagBit), // aspectMask
			level,                             // mipLevel
			copySrcLayer,                      // baseArrayLayer
			1,                                 // layerCount
		),
		NewVkOffset3D(inputState.Arena, int32(0), int32(0), copySrcDepth), // imageOffset
		NewVkExtent3D(inputState.Arena, requestWidth, requestHeight, 1),   // imageExtent
	)
	bufferImageCopyData := overdrawTransform.allocations.AllocDataOrPanic(ctx, bufferImageCopy)

	commandBuffers := overdrawTransform.allocations.AllocDataOrPanic(ctx, commandBufferID)
	submitInfo := NewVkSubmitInfo(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_SUBMIT_INFO, // sType
		0, // pNext
		0, // waitSemaphoreCount
		0, // pWaitSemaphores
		0, // pWaitDstStageMask
		1, // commandBufferCount
		NewVkCommandBufferᶜᵖ(commandBuffers.Ptr()), // pCommandBuffers
		0, // signalSemaphoreCount
		0, // pSignalSemaphores
	)
	submitInfoData := overdrawTransform.allocations.AllocDataOrPanic(ctx, submitInfo)

	mappedMemoryRange := NewVkMappedMemoryRange(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_MAPPED_MEMORY_RANGE, // sType
		0,                                // pNext
		bufferMemoryID,                   // memory
		VkDeviceSize(0),                  // offset
		VkDeviceSize(0xFFFFFFFFFFFFFFFF), // size
	)
	mappedMemoryRangeData := overdrawTransform.allocations.AllocDataOrPanic(ctx, mappedMemoryRange)

	at, err := overdrawTransform.allocations.Alloc(ctx, bufferSize)
	if err != nil {
		res(nil, &service.ErrDataUnavailable{Reason: messages.ErrMessage("Device Memory -> Host mapping failed")})
	}
	mappedPointer := overdrawTransform.allocations.AllocDataOrPanic(ctx, at.Address())

	barrierAspectMask := VkImageAspectFlags(aspectFlagBit)
	depthStencilMask := VkImageAspectFlagBits_VK_IMAGE_ASPECT_DEPTH_BIT |
		VkImageAspectFlagBits_VK_IMAGE_ASPECT_STENCIL_BIT
	if VkImageAspectFlagBits(imageObject.ImageAspect())&depthStencilMask == depthStencilMask {
		barrierAspectMask |= VkImageAspectFlags(depthStencilMask)
	}
	// Barrier data for layout transitions of staging image
	stagingImageToDstBarrier := NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT|
			VkAccessFlagBits_VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT|
			VkAccessFlagBits_VK_ACCESS_SHADER_WRITE_BIT|
			VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // srcAccessMask
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // dstAccessMask
		VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED,                      // oldLayout
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,           // newLayout
		0xFFFFFFFF,     // srcQueueFamilyIndex
		0xFFFFFFFF,     // dstQueueFamilyIndex
		stagingImageID, // image
		NewVkImageSubresourceRange(inputState.Arena, // subresourceRange
			barrierAspectMask, // aspectMask
			0,                 // baseMipLevel
			1,                 // levelCount
			0,                 // baseArrayLayer
			1,                 // layerCount
		),
	)
	stagingImageToDstBarrierData := overdrawTransform.allocations.AllocDataOrPanic(ctx, stagingImageToDstBarrier)

	stagingImageToSrcBarrier := NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // srcAccessMask
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT),  // dstAccessMask
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,           // oldLayout
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,           // newLayout
		0xFFFFFFFF,     // srcQueueFamilyIndex
		0xFFFFFFFF,     // dstQueueFamilyIndex
		stagingImageID, // image
		NewVkImageSubresourceRange(inputState.Arena, // subresourceRange
			barrierAspectMask, // aspectMask
			0,                 // baseMipLevel
			1,                 // levelCount
			0,                 // baseArrayLayer
			1,                 // layerCount
		),
	)
	stagingImageToSrcBarrierData := overdrawTransform.allocations.AllocDataOrPanic(ctx, stagingImageToSrcBarrier)

	// Barrier data for layout transitions of resolve image. This only used when the attachment image is
	// multi-sampled.
	resolveImageToDstBarrier := NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT|
			VkAccessFlagBits_VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT|
			VkAccessFlagBits_VK_ACCESS_SHADER_WRITE_BIT|
			VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // srcAccessMask
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // dstAccessMask
		VkImageLayout_VK_IMAGE_LAYOUT_UNDEFINED,                      // oldLayout
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,           // newLayout
		0xFFFFFFFF,     // srcQueueFamilyIndex
		0xFFFFFFFF,     // dstQueueFamilyIndex
		resolveImageID, // image
		NewVkImageSubresourceRange(inputState.Arena, // subresourceRange
			barrierAspectMask, // aspectMask
			0,                 // baseMipLevel
			1,                 // levelCount
			0,                 // baseArrayLayer
			1,                 // layerCount
		),
	)
	resolveImageToDstBarrierData := overdrawTransform.allocations.AllocDataOrPanic(ctx, resolveImageToDstBarrier)

	resolveImageToSrcBarrier := NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT), // srcAccessMask
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT),  // dstAccessMask
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,           // oldLayout
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,           // newLayout
		0xFFFFFFFF,     // srcQueueFamilyIndex
		0xFFFFFFFF,     // dstQueueFamilyIndex
		resolveImageID, // image
		NewVkImageSubresourceRange(inputState.Arena, // subresourceRange
			barrierAspectMask, // aspectMask
			0,                 // baseMipLevel
			1,                 // levelCount
			0,                 // baseArrayLayer
			1,                 // layerCount
		),
	)
	resolveImageToSrcBarrierData := overdrawTransform.allocations.AllocDataOrPanic(ctx, resolveImageToSrcBarrier)

	// Barrier data for layout transitions of attachment image
	attachmentImageToSrcBarrier := NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags( // srcAccessMask
			VkAccessFlagBits_VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT|
				VkAccessFlagBits_VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT|
				VkAccessFlagBits_VK_ACCESS_SHADER_WRITE_BIT|
				VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT|
				VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT,
		),
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT), // dstAccessMask
		origLayout, // oldLayout
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, // newLayout
		0xFFFFFFFF,                 // srcQueueFamilyIndex
		0xFFFFFFFF,                 // dstQueueFamilyIndex
		imageObject.VulkanHandle(), // image
		NewVkImageSubresourceRange(inputState.Arena, // subresourceRange
			barrierAspectMask, // aspectMask
			0,                 // baseMipLevel
			1,                 // levelCount
			0,                 // baseArrayLayer
			1,                 // layerCount
		),
	)
	attachmentImageToSrcBarrierData := overdrawTransform.allocations.AllocDataOrPanic(ctx, attachmentImageToSrcBarrier)

	attachmentImageResetLayoutBarrier := NewVkImageMemoryBarrier(inputState.Arena,
		VkStructureType_VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, // sType
		0, // pNext
		VkAccessFlags(VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT), // srcAccessMask
		VkAccessFlags( // dstAccessMask
			VkAccessFlagBits_VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT|
				VkAccessFlagBits_VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT|
				VkAccessFlagBits_VK_ACCESS_SHADER_WRITE_BIT|
				VkAccessFlagBits_VK_ACCESS_TRANSFER_WRITE_BIT|
				VkAccessFlagBits_VK_ACCESS_TRANSFER_READ_BIT),
		VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, // oldLayout
		origLayout,                 // newLayout
		0xFFFFFFFF,                 // srcQueueFamilyIndex
		0xFFFFFFFF,                 // dstQueueFamilyIndex
		imageObject.VulkanHandle(), // image
		NewVkImageSubresourceRange(inputState.Arena, // subresourceRange
			barrierAspectMask, // aspectMask
			0,                 // baseMipLevel
			1,                 // levelCount
			0,                 // baseArrayLayer
			1,                 // layerCount
		),
	)
	attachmentImageResetLayoutBarrierData := overdrawTransform.allocations.AllocDataOrPanic(ctx, attachmentImageResetLayoutBarrier)

	// Observation data for vkCmdBlitImage
	imageBlit := NewVkImageBlit(inputState.Arena,
		NewVkImageSubresourceLayers(inputState.Arena, // srcSubresource
			VkImageAspectFlags(aspectFlagBit), // aspectMask
			0,                                 // mipLevel
			blitSrcLayer,                      // baseArrayLayer
			1,                                 // layerCount
		),
		NewVkOffset3Dː2ᵃ(inputState.Arena, // srcOffsets
			NewVkOffset3D(inputState.Arena, int32(0), int32(0), blitSrcDepth),
			NewVkOffset3D(inputState.Arena, int32(imgWidth), int32(imgHeight), blitSrcDepth+int32(1)),
		),
		NewVkImageSubresourceLayers(inputState.Arena, // dstSubresource
			VkImageAspectFlags(aspectFlagBit), // aspectMask
			0,                                 // mipLevel
			0,                                 // baseArrayLayer
			1,                                 // layerCount
		),
		NewVkOffset3Dː2ᵃ(inputState.Arena, // dstOffsets
			MakeVkOffset3D(inputState.Arena),
			NewVkOffset3D(inputState.Arena, int32(requestWidth), int32(requestHeight), 1),
		),
	)
	imageBlitData := overdrawTransform.allocations.AllocDataOrPanic(ctx, imageBlit)

	// Observation data for vkCmdResolveImage
	imageResolve := NewVkImageResolve(inputState.Arena,
		NewVkImageSubresourceLayers(inputState.Arena, // srcSubresource
			VkImageAspectFlags(aspectFlagBit), // aspectMask
			0,                                 // mipLevel
			resolveSrcLayer,                   // baseArrayLayer
			1,                                 // layerCount
		),
		NewVkOffset3D(inputState.Arena, int32(0), int32(0), resolveSrcDepth), // srcOffset
		NewVkImageSubresourceLayers(inputState.Arena, // dstSubresource
			VkImageAspectFlags(aspectFlagBit), // aspectMask
			0,                                 // mipLevel
			0,                                 // baseArrayLayer
			1,                                 // layerCount
		),
		MakeVkOffset3D(inputState.Arena),                                        // dstOffset
		NewVkExtent3D(inputState.Arena, uint32(imgWidth), uint32(imgHeight), 1), // extent
	)
	imageResolveData := overdrawTransform.allocations.AllocDataOrPanic(ctx, imageResolve)

	outputCmds := make([]api.Cmd, 0)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCreateImage(
			vkDevice,
			stagingImageCreateInfoData.Ptr(),
			memory.Nullptr,
			stagingImageData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			stagingImageCreateInfoData.Data(),
		).AddWrite(
			stagingImageData.Data(),
		),
		overdrawTransform.cmdBuilder.ReplayAllocateImageMemory(
			vkDevice,
			physicalDeviceMemoryPropertiesData.Ptr(),
			stagingImageID,
			stagingImageMemoryData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			physicalDeviceMemoryPropertiesData.Data(),
		).AddWrite(
			stagingImageMemoryData.Data(),
		),
		overdrawTransform.cmdBuilder.VkBindImageMemory(
			vkDevice,
			stagingImageID,
			stagingImageMemoryID,
			VkDeviceSize(0),
			VkResult_VK_SUCCESS,
		),
	)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCreateBuffer(
			vkDevice,
			bufferCreateInfoData.Ptr(),
			memory.Nullptr,
			bufferData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			bufferCreateInfoData.Data(),
		).AddWrite(
			bufferData.Data(),
		),
		overdrawTransform.cmdBuilder.VkAllocateMemory(
			vkDevice,
			bufferMemoryAllocateInfoData.Ptr(),
			memory.Nullptr,
			bufferMemoryData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			bufferMemoryAllocateInfoData.Data(),
		).AddWrite(
			bufferMemoryData.Data(),
		),
		overdrawTransform.cmdBuilder.VkBindBufferMemory(
			vkDevice,
			bufferID,
			bufferMemoryID,
			VkDeviceSize(0),
			VkResult_VK_SUCCESS,
		),
	)

	// If the attachment image is multi-sampled, an resolve image is required
	if imageObject.Info().Samples() != VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT {
		outputCmds = append(outputCmds,
			overdrawTransform.cmdBuilder.VkCreateImage(
				vkDevice,
				resolveImageCreateInfoData.Ptr(),
				memory.Nullptr,
				resolveImageData.Ptr(),
				VkResult_VK_SUCCESS,
			).AddRead(
				resolveImageCreateInfoData.Data(),
			).AddWrite(
				resolveImageData.Data(),
			),
			overdrawTransform.cmdBuilder.ReplayAllocateImageMemory(
				vkDevice,
				physicalDeviceMemoryPropertiesData.Ptr(),
				resolveImageID,
				resolveImageMemoryData.Ptr(),
				VkResult_VK_SUCCESS,
			).AddRead(
				physicalDeviceMemoryPropertiesData.Data(),
			).AddWrite(
				resolveImageMemoryData.Data(),
			),
			overdrawTransform.cmdBuilder.VkBindImageMemory(
				vkDevice,
				resolveImageID,
				resolveImageMemoryID,
				VkDeviceSize(0),
				VkResult_VK_SUCCESS,
			),
		)
	}

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCreateCommandPool(
			vkDevice,
			commandPoolCreateInfoData.Ptr(),
			memory.Nullptr,
			commandPoolData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			commandPoolCreateInfoData.Data(),
		).AddWrite(
			commandPoolData.Data(),
		),
		overdrawTransform.cmdBuilder.VkAllocateCommandBuffers(
			vkDevice,
			commandBufferAllocateInfoData.Ptr(),
			commandBufferData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			commandBufferAllocateInfoData.Data(),
		).AddWrite(
			commandBufferData.Data(),
		),
	)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCreateFence(
			vkDevice,
			fenceCreateData.Ptr(),
			memory.Nullptr,
			fenceData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			fenceCreateData.Data(),
		).AddWrite(
			fenceData.Data(),
		),
	)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkBeginCommandBuffer(
			commandBufferID,
			beginCommandBufferInfoData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(
			beginCommandBufferInfoData.Data(),
		),
		overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(
			commandBufferID,
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
			VkDependencyFlags(0),
			0,
			memory.Nullptr,
			0,
			memory.Nullptr,
			1,
			attachmentImageToSrcBarrierData.Ptr(),
		).AddRead(
			attachmentImageToSrcBarrierData.Data(),
		),
		overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(
			commandBufferID,
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
			VkDependencyFlags(0),
			0,
			memory.Nullptr,
			0,
			memory.Nullptr,
			1,
			stagingImageToDstBarrierData.Ptr(),
		).AddRead(
			stagingImageToDstBarrierData.Data(),
		),
	)

	// If the attachment image is multi-sampled, resolve the attchment image to resolve image before
	// blit the image.
	if imageObject.Info().Samples() != VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT {
		outputCmds = append(outputCmds,
			overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(
				commandBufferID,
				VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
				VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
				VkDependencyFlags(0),
				0,
				memory.Nullptr,
				0,
				memory.Nullptr,
				1,
				resolveImageToDstBarrierData.Ptr(),
			).AddRead(
				resolveImageToDstBarrierData.Data(),
			),
			overdrawTransform.cmdBuilder.VkCmdResolveImage(
				commandBufferID,
				imageObject.VulkanHandle(),
				VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,
				resolveImageID,
				VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
				1,
				imageResolveData.Ptr(),
			).AddRead(imageResolveData.Data()),
			overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(
				commandBufferID,
				VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
				VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
				VkDependencyFlags(0),
				0,
				memory.Nullptr,
				0,
				memory.Nullptr,
				1,
				resolveImageToSrcBarrierData.Ptr(),
			).AddRead(
				resolveImageToSrcBarrierData.Data(),
			),
		)
	}

	// Blit image, if the attachment image is multi-sampled, use resolve image as the source image, otherwise
	// use the attachment image as the source image directly
	blitSrcImage := imageObject.VulkanHandle()
	if imageObject.Info().Samples() != VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT {
		blitSrcImage = resolveImageID
	}
	// If the src image is a depth/stencil image, the filter must be NEAREST
	filter := VkFilter_VK_FILTER_LINEAR
	if aspectFlagBit != VkImageAspectFlagBits_VK_IMAGE_ASPECT_COLOR_BIT {
		filter = VkFilter_VK_FILTER_NEAREST
	}

	copySrc := blitSrcImage

	if doBlit {
		copySrc = stagingImageID
		outputCmds = append(outputCmds,
			overdrawTransform.cmdBuilder.VkCmdBlitImage(
				commandBufferID,
				blitSrcImage,
				VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,
				stagingImageID,
				VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
				1,
				imageBlitData.Ptr(),
				filter,
			).AddRead(imageBlitData.Data()),
			overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(
				commandBufferID,
				VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
				VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
				VkDependencyFlags(0),
				0,
				memory.Nullptr,
				0,
				memory.Nullptr,
				1,
				stagingImageToSrcBarrierData.Ptr(),
			).AddRead(
				stagingImageToSrcBarrierData.Data(),
			),
		)
	}

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCmdCopyImageToBuffer(
			commandBufferID,
			copySrc,
			VkImageLayout_VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,
			bufferID,
			1,
			bufferImageCopyData.Ptr(),
		).AddRead(
			bufferImageCopyData.Data(),
		),
	)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkCmdPipelineBarrier(
			commandBufferID,
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
			VkPipelineStageFlags(VkPipelineStageFlagBits_VK_PIPELINE_STAGE_ALL_COMMANDS_BIT),
			VkDependencyFlags(0),
			0,
			memory.Nullptr,
			0,
			memory.Nullptr,
			1,
			attachmentImageResetLayoutBarrierData.Ptr(),
		).AddRead(
			attachmentImageResetLayoutBarrierData.Data(),
		),
		overdrawTransform.cmdBuilder.VkEndCommandBuffer(
			commandBufferID,
			VkResult_VK_SUCCESS,
		),
	)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkDeviceWaitIdle(vkDevice, VkResult_VK_SUCCESS),
		overdrawTransform.cmdBuilder.VkQueueSubmit(
			vkQueue,
			1,
			submitInfoData.Ptr(),
			fenceID,
			VkResult_VK_SUCCESS,
		).AddRead(
			submitInfoData.Data(),
		).AddRead(
			commandBuffers.Data(),
		),
		overdrawTransform.cmdBuilder.VkWaitForFences(
			vkDevice,
			1,
			fenceData.Ptr(),
			1,
			0xFFFFFFFFFFFFFFFF,
			VkResult_VK_SUCCESS,
		).AddRead(
			fenceData.Data(),
		),
		overdrawTransform.cmdBuilder.VkDeviceWaitIdle(vkDevice, VkResult_VK_SUCCESS),
	)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkMapMemory(
			vkDevice,
			bufferMemoryID,
			VkDeviceSize(0),
			VkDeviceSize(bufferSize),
			VkMemoryMapFlags(0),
			mappedPointer.Ptr(),
			VkResult_VK_SUCCESS,
		).AddWrite(mappedPointer.Data()),
		overdrawTransform.cmdBuilder.VkInvalidateMappedMemoryRanges(
			vkDevice,
			1,
			mappedMemoryRangeData.Ptr(),
			VkResult_VK_SUCCESS,
		).AddRead(mappedMemoryRangeData.Data()),
	)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.Custom(func(ctx context.Context, s *api.GlobalState, b *builder.Builder) error {
			b.Post(value.ObservedPointer(at.Address()), uint64(bufferSize), func(r binary.Reader, err error) {
				var bytes []byte
				if err == nil {
					bytes = make([]byte, bufferSize)
					r.Data(bytes)
					r.Error()

					// For the depth aspect of VK_FORMAT_X8_D24_UNORM_PACK32 and
					// VK_FORMAT_D24_UNORM_S8_UINT format, we need to strip the
					// undefined value in the MSB byte.
					if aspectFlagBit == VkImageAspectFlagBits_VK_IMAGE_ASPECT_DEPTH_BIT && (vkFormat == VkFormat_VK_FORMAT_X8_D24_UNORM_PACK32 || vkFormat == VkFormat_VK_FORMAT_D24_UNORM_S8_UINT) {
						inBufSize := 4
						inImgSize := 3
						count := len(bytes) / inBufSize
						for i := 0; i < count; i++ {
							copy(bytes[i*inImgSize:(i+1)*inImgSize], bytes[i*inBufSize:(i+1)*inBufSize])
						}
						bufferSize = uint64(count * inImgSize)
						bytes = bytes[0:bufferSize]
					}

					// Flip the image in Y axis
					rowSizeInBytes := uint64(formatOfImgRes.Size(int(requestWidth), 1, 1))
					top := uint64(0)
					bottom := bufferSize - rowSizeInBytes
					var temp = make([]byte, rowSizeInBytes)
					for top < bottom {
						copy(temp, bytes[top:top+rowSizeInBytes])
						copy(bytes[top:top+rowSizeInBytes], bytes[bottom:bottom+rowSizeInBytes])
						copy(bytes[bottom:bottom+rowSizeInBytes], temp)
						top += rowSizeInBytes
						bottom -= rowSizeInBytes
					}
				}
				if err != nil {
					err = fmt.Errorf("Could not read framebuffer data (expected length %d bytes): %v", bufferSize, err)
					bytes = nil
				}

				img := &image.Data{
					Bytes:  bytes,
					Width:  uint32(requestWidth),
					Height: uint32(requestHeight),
					Depth:  1,
					Format: formatOfImgRes,
				}

				if err == nil {
					err = checkImage(ctx, img)
				}

				res(img, err)
			})
			return nil
		}),
	)

	outputCmds = append(outputCmds,
		overdrawTransform.cmdBuilder.VkUnmapMemory(vkDevice, bufferMemoryID),
		overdrawTransform.cmdBuilder.VkDestroyBuffer(vkDevice, bufferID, memory.Nullptr),
		overdrawTransform.cmdBuilder.VkDestroyCommandPool(vkDevice, commandPoolID, memory.Nullptr),
		overdrawTransform.cmdBuilder.VkDestroyImage(vkDevice, stagingImageID, memory.Nullptr),
		overdrawTransform.cmdBuilder.VkFreeMemory(vkDevice, stagingImageMemoryID, memory.Nullptr),
		overdrawTransform.cmdBuilder.VkFreeMemory(vkDevice, bufferMemoryID, memory.Nullptr))
	if imageObject.Info().Samples() != VkSampleCountFlagBits_VK_SAMPLE_COUNT_1_BIT {
		outputCmds = append(outputCmds,
			overdrawTransform.cmdBuilder.VkDestroyImage(vkDevice, resolveImageID, memory.Nullptr),
			overdrawTransform.cmdBuilder.VkFreeMemory(vkDevice, resolveImageMemoryID, memory.Nullptr))
	}
	outputCmds = append(outputCmds, overdrawTransform.cmdBuilder.VkDestroyFence(vkDevice, fenceID, memory.Nullptr))
	return outputCmds
}

func depthToStencilFormat(depthFormat VkFormat) (VkFormat, error) {
	switch depthFormat {
	case VkFormat_VK_FORMAT_D16_UNORM:
		return VkFormat_VK_FORMAT_D16_UNORM_S8_UINT, nil
	case VkFormat_VK_FORMAT_X8_D24_UNORM_PACK32:
		return VkFormat_VK_FORMAT_D24_UNORM_S8_UINT, nil
	case VkFormat_VK_FORMAT_D32_SFLOAT:
		return VkFormat_VK_FORMAT_D32_SFLOAT_S8_UINT, nil

	case VkFormat_VK_FORMAT_D16_UNORM_S8_UINT,
		VkFormat_VK_FORMAT_D24_UNORM_S8_UINT,
		VkFormat_VK_FORMAT_D32_SFLOAT_S8_UINT:
		return depthFormat, nil
	default:
		return 0, fmt.Errorf("Unrecognized depth format %v",
			depthFormat)
	}
}

func createSpecializationInfo(ctx context.Context,
	inputState *api.GlobalState,
	info SpecializationInfoʳ,
	allocAndRead func(v ...interface{}) api.AllocResult,
) VkSpecializationInfoᶜᵖ {
	if info.IsNil() {
		return 0
	}
	mapEntries, mapEntryCount := unpackMapWithAllocator(allocAndRead, info.Specializations().All())
	data := info.Data().MustRead(ctx, nil, inputState, nil)
	return NewVkSpecializationInfoᶜᵖ(allocAndRead(
		NewVkSpecializationInfo(inputState.Arena,
			mapEntryCount, // mapEntryCount
			NewVkSpecializationMapEntryᶜᵖ(mapEntries.Ptr()), // pMapEntries
			memory.Size(len(data)),                          // dataSize,
			NewVoidᶜᵖ(allocAndRead(data).Ptr()),             // pData
		)).Ptr())
}

func depthStencilToDepthFormat(depthStencilFormat VkFormat) (VkFormat, error) {
	switch depthStencilFormat {
	case VkFormat_VK_FORMAT_D16_UNORM,
		VkFormat_VK_FORMAT_D16_UNORM_S8_UINT:
		return VkFormat_VK_FORMAT_D16_UNORM, nil
	case VkFormat_VK_FORMAT_X8_D24_UNORM_PACK32,
		VkFormat_VK_FORMAT_D24_UNORM_S8_UINT:
		return VkFormat_VK_FORMAT_X8_D24_UNORM_PACK32, nil
	case VkFormat_VK_FORMAT_D32_SFLOAT,
		VkFormat_VK_FORMAT_D32_SFLOAT_S8_UINT:
		return VkFormat_VK_FORMAT_D32_SFLOAT, nil
	default:
		return 0, fmt.Errorf("Unrecognized depth/stencil format %v",
			depthStencilFormat)
	}
}

func depthToStageFormat(depthFormat VkFormat) (VkFormat, error) {
	switch depthFormat {
	case VkFormat_VK_FORMAT_D16_UNORM:
		return VkFormat_VK_FORMAT_R16_UINT, nil
	case VkFormat_VK_FORMAT_X8_D24_UNORM_PACK32,
		VkFormat_VK_FORMAT_D32_SFLOAT:
		return VkFormat_VK_FORMAT_R32_UINT, nil
	default:
		return 0, fmt.Errorf("Unrecognized depth format %v",
			depthFormat)
	}
}

func isDepthFormat(depthFormat VkFormat) bool {
	return depthBits(depthFormat) != 0
}

func depthBits(depthFormat VkFormat) int {
	switch depthFormat {
	case VkFormat_VK_FORMAT_D16_UNORM,
		VkFormat_VK_FORMAT_D16_UNORM_S8_UINT:
		return 16
	case VkFormat_VK_FORMAT_X8_D24_UNORM_PACK32,
		VkFormat_VK_FORMAT_D24_UNORM_S8_UINT:
		return 24
	case VkFormat_VK_FORMAT_D32_SFLOAT,
		VkFormat_VK_FORMAT_D32_SFLOAT_S8_UINT:
		return 32
	default:
		return 0
	}
}

func checkImage(ctx context.Context, img *image.Data) error {
	// Check if any bytes are 255, which indicates potential saturation
	for _, byt := range img.Bytes {
		if byt == 255 {
			log.W(ctx, "Overdraw hit limit of 255, further overdraw cannot be measured")
			break
		}
	}
	// Even though the image comes from a stencil, content-wise
	// it's a gray image.
	img.Format = image.NewUncompressed("Count_U8", fmts.Count_U8)
	return nil
}
