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
	"fmt"
	"strings"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/commandGenerator"
	"github.com/google/gapid/gapis/api/controlFlowGenerator"
	"github.com/google/gapid/gapis/api/transform2"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/config"
	"github.com/google/gapid/gapis/replay"
	"github.com/google/gapid/gapis/resolve/initialcmds"
	"github.com/google/gapid/gapis/service/path"
)

// GetInitialPayload creates a replay that emits instructions for
// state priming of a capture.
func (a API) GetInitialPayload(ctx context.Context,
	capture *path.Capture,
	device *device.Instance,
	out transform2.Writer) error {

	initialCmds, im, _ := initialcmds.InitialCommands(ctx, capture)
	out.State().Allocator.ReserveRanges(im)
	cmdGenerator := commandGenerator.NewLinearCommandGenerator(initialCmds, nil)

	transforms := make([]transform2.Transform, 0)
	transforms = append(transforms, newMakeAttachmentReadable(false))
	transforms = append(transforms, newDropInvalidDestroy("GetInitialPayload"))

	chain := transform2.CreateTransformChain(ctx, cmdGenerator, transforms, out)
	controlFlow := controlFlowGenerator.NewLinearControlFlowGenerator(chain)
	if err := controlFlow.TransformAll(ctx); err != nil {
		log.E(ctx, "[GetInitialPayload] Error: %v", err)
		return err
	}

	return nil
}

// CleanupResources creates a replay that emits instructions for
// destroying resources at a given stateg
func (a API) CleanupResources(ctx context.Context, device *device.Instance, out transform2.Writer) error {
	cmdGenerator := commandGenerator.NewLinearCommandGenerator(nil, nil)
	transforms := []transform2.Transform{
		newDestroyResourcesAtEOS(),
	}
	chain := transform2.CreateTransformChain(ctx, cmdGenerator, transforms, out)
	controlFlow := controlFlowGenerator.NewLinearControlFlowGenerator(chain)
	if err := controlFlow.TransformAll(ctx); err != nil {
		log.E(ctx, "[CleanupResources] Error: %v", err)
		return err
	}

	return nil
}

func (a API) Replay(
	ctx context.Context,
	intent replay.Intent,
	cfg replay.Config,
	dependentPayload string,
	rrs []replay.RequestAndResult,
	device *device.Instance,
	c *capture.GraphicsCapture,
	out transform2.Writer) error {
	if a.GetReplayPriority(ctx, device, c.Header) == 0 {
		return log.Errf(ctx, nil, "Cannot replay Vulkan commands on device '%v'", device.Name)
	}

	if len(rrs) == 0 {
		return log.Errf(ctx, nil, "No request has been found for the replay")
	}

	firstRequest := rrs[0].Request
	replayType := getReplayTypeName(firstRequest)

	if len(rrs) > 1 {
		if _, ok := firstRequest.(framebufferRequest); !ok {
			// Only the framebuffer requests are batched
			panic(fmt.Sprintf("Batched request is not supported for %v", replayType))
		}
	}

	transforms := make([]transform2.Transform, 0)

	_, isProfileRequest := firstRequest.(profileRequest)
	transforms = append(transforms, newMakeAttachmentReadable(isProfileRequest))

	transforms = append(transforms, newDropInvalidDestroy(replayType))

	// Melih TODO: DCE probably should be here
	initialCmds := getInitialCmds(ctx, dependentPayload, intent, out)
	numOfInitialCmds := api.CmdID(len(initialCmds))

	// Due to how replay system works, different types of replays cannot be batched
	switch firstRequest.(type) {
	case framebufferRequest:
		framebufferTransforms, err := getFramebufferTransforms(ctx, numOfInitialCmds, intent, cfg, rrs)
		if err != nil {
			log.E(ctx, "%v Error: %v", replayType, err)
			return err
		}
		transforms = append(transforms, framebufferTransforms...)
	case issuesRequest:
		transforms = append(transforms, getIssuesTransforms(ctx, c, numOfInitialCmds, &rrs[0])...)
	case profileRequest:
		transforms = append(transforms, getProfileTransforms(ctx, numOfInitialCmds, device, &rrs[0])...)
	case timestampsRequest:
		transforms = append(transforms, getTimestampTransforms(ctx, &rrs[0])...)
	default:
		panic("Unknown request type")
	}

	transforms = append(transforms, newDestroyResourcesAtEOS())
	transforms = appendLogTransforms(ctx, replayType, c, transforms)

	cmdGenerator := commandGenerator.NewLinearCommandGenerator(initialCmds, c.Commands)
	chain := transform2.CreateTransformChain(ctx, cmdGenerator, transforms, out)
	controlFlow := controlFlowGenerator.NewLinearControlFlowGenerator(chain)
	if err := controlFlow.TransformAll(ctx); err != nil {
		log.E(ctx, "%v Error: %v", replayType, err)
		return err
	}

	return nil
}

func getFramebufferTransforms(ctx context.Context,
	numOfInitialCmds api.CmdID,
	intent replay.Intent,
	cfg replay.Config,
	rrs []replay.RequestAndResult) ([]transform2.Transform, error) {

	shouldRenderWired := false
	doDisplayToSurface := false
	shouldOverDraw := false

	vulkanTerminator, err := newVulkanTerminator2(ctx, intent.Capture, numOfInitialCmds)
	if err != nil {
		log.E(ctx, "Vulkan terminator failed: %v", err)
		return nil, err
	}

	splitterTransform := NewCommandSplitter(ctx)
	readFramebufferTransform := newReadFramebuffer(ctx)
	overdrawTransform := NewStencilOverdraw()

	for _, rr := range rrs {
		request := rr.Request.(framebufferRequest)

		cfg := cfg.(drawConfig)
		cmdID := request.after[0]

		if cfg.drawMode == path.DrawMode_OVERDRAW {
			// TODO(subcommands): Add subcommand support here
			if err := vulkanTerminator.Add(ctx, api.CmdID(cmdID), request.after[1:]); err != nil {
				log.E(ctx, "Vulkan terminator error on Cmd(%v) : %v", cmdID, err)
				return nil, err
			}
			shouldOverDraw = true
			overdrawTransform.add(ctx, request.after, intent.Capture, rr.Result)
			break
		}

		if err := vulkanTerminator.Add(ctx, api.CmdID(cmdID), api.SubCmdIdx{}); err != nil {
			log.E(ctx, "Vulkan terminator error on Cmd(%v) : %v", cmdID, err)
			return nil, err
		}

		subIdx := append(api.SubCmdIdx{}, request.after...)
		splitterTransform.Split(ctx, subIdx)

		switch cfg.drawMode {
		case path.DrawMode_WIREFRAME_ALL:
			shouldRenderWired = true
		case path.DrawMode_WIREFRAME_OVERLAY:
			return nil, fmt.Errorf("Overlay wireframe view is not currently supported")
			// Overdraw is handled above, since it breaks out of the normal read flow.
		}

		switch request.attachment {
		case api.FramebufferAttachmentType_OutputDepth, api.FramebufferAttachmentType_InputDepth:
			readFramebufferTransform.Depth(ctx, subIdx, request.width, request.height, request.framebufferIndex, rr.Result)
		case api.FramebufferAttachmentType_OutputColor, api.FramebufferAttachmentType_InputColor:
			readFramebufferTransform.Color(ctx, subIdx, request.width, request.height, request.framebufferIndex, rr.Result)
		default:
			return nil, fmt.Errorf("Stencil attachments are not currently supported")
		}

		if request.displayToSurface {
			doDisplayToSurface = true
		}
	}

	transforms := make([]transform2.Transform, 0)

	if shouldRenderWired {
		transforms = append(transforms, newWireframeTransform())
	}

	if doDisplayToSurface {
		transforms = append(transforms, newDisplayToSurface())
	}

	transforms = append(transforms, vulkanTerminator)

	if shouldOverDraw {
		transforms = append(transforms, overdrawTransform)
	}

	transforms = append(transforms, splitterTransform)
	transforms = append(transforms, readFramebufferTransform)
	return transforms, nil
}

func getIssuesTransforms(ctx context.Context,
	c *capture.GraphicsCapture,
	numOfInitialCmds api.CmdID,
	requestAndResult *replay.RequestAndResult) []transform2.Transform {
	transforms := make([]transform2.Transform, 0)

	issuesTransform := newFindIssues(ctx, c, numOfInitialCmds)
	issuesTransform.AddResult(requestAndResult.Result)
	transforms = append(transforms, issuesTransform)

	request := requestAndResult.Request.(issuesRequest)
	if request.displayToSurface {
		transforms = append(transforms, newDisplayToSurface())
	}

	return transforms
}

func getProfileTransforms(ctx context.Context,
	numOfInitialCmds api.CmdID,
	device *device.Instance,
	requestAndResult *replay.RequestAndResult) []transform2.Transform {
	var layerName string
	if device.GetConfiguration().GetPerfettoCapability().GetGpuProfiling().GetHasRenderStageProducerLayer() {
		layerName = "VkRenderStagesProducer"
	}

	request := requestAndResult.Request.(profileRequest)

	profileTransform := newEndOfReplay()
	profileTransform.AddResult(requestAndResult.Result)

	transforms := make([]transform2.Transform, 0)
	transforms = append(transforms, newWaitForPerfetto(request.traceOptions, request.handler, request.buffer, numOfInitialCmds))
	transforms = append(transforms, newProfilingLayers(layerName))
	transforms = append(transforms, newMappingExporter(ctx, request.handleMappings))
	transforms = append(transforms, profileTransform)
	return transforms
}

func getTimestampTransforms(ctx context.Context,
	requestAndResult *replay.RequestAndResult) []transform2.Transform {
	request := requestAndResult.Request.(timestampsRequest)
	timestampTransform := newQueryTimestamps(ctx, request.handler)
	timestampTransform.AddResult(requestAndResult.Result)
	return []transform2.Transform{timestampTransform}
}

func appendLogTransforms(ctx context.Context, tag string, capture *capture.GraphicsCapture, transforms []transform2.Transform) []transform2.Transform {
	if config.LogTransformsToFile {
		newTransforms := make([]transform2.Transform, 0)
		newTransforms = append(newTransforms, newFileLog(ctx, "0_original_cmds"))
		for i, t := range transforms {
			var name string
			if n, ok := t.(interface {
				Name() string
			}); ok {
				name = n.Name()
			} else {
				name = strings.Replace(fmt.Sprintf("%T", t), "*", "", -1)
			}
			newTransforms = append(newTransforms, t, newFileLog(ctx, fmt.Sprintf("%v_cmds_after_%v", i+1, name)))
		}
		transforms = newTransforms
	}

	if config.LogTransformsToCapture {
		transforms = append(transforms, newCaptureLog(ctx, capture, tag+"_replay_log.gfxtrace"))
	}

	if config.LogMappingsToFile {
		transforms = append(transforms, newMappingExporterWithPrint(ctx, tag+"_mappings.txt"))
	}

	return transforms
}

func getInitialCmds(ctx context.Context,
	dependentPayload string,
	intent replay.Intent,
	out transform2.Writer) []api.Cmd {

	if dependentPayload == "" {
		cmds, im, _ := initialcmds.InitialCommands(ctx, intent.Capture)
		out.State().Allocator.ReserveRanges(im)
		return cmds
	}

	return []api.Cmd{}
}

func getReplayTypeName(request replay.Request) string {
	switch request.(type) {
	case framebufferRequest:
		return "Framebuffer Replay"
	case issuesRequest:
		return "Issues Replay"
	case profileRequest:
		return "Profile Replay"
	case timestampsRequest:
		return "Timestamp Replay"
	}

	panic("Unknown replay type")
	return "Unknown Replay Type"
}

// GetReplayPriority returns a uint32 representing the preference for
// replaying this trace on the given device.
// A lower number represents a higher priority, and Zero represents
// an inability for the trace to be replayed on the given device.
func (a API) GetReplayPriority(ctx context.Context, i *device.Instance, h *capture.Header) uint32 {
	devConf := i.GetConfiguration()
	devAbis := devConf.GetABIs()
	devVkDriver := devConf.GetDrivers().GetVulkan()
	traceVkDriver := h.GetDevice().GetConfiguration().GetDrivers().GetVulkan()

	if traceVkDriver == nil {
		log.E(ctx, "Vulkan trace does not contain VulkanDriver info.")
		return 0
	}

	// The device does not support Vulkan
	if devVkDriver == nil {
		return 0
	}

	for _, abi := range devAbis {
		// Memory layout must match.
		if !abi.GetMemoryLayout().SameAs(h.GetABI().GetMemoryLayout()) {
			continue
		}
		// If there is no physical devices, the trace must not contain
		// vkCreateInstance, any ABI compatible Vulkan device should be able to
		// replay.
		if len(traceVkDriver.GetPhysicalDevices()) == 0 {
			return 1
		}
		// Requires same vendor, device, driver version and API version.
		for _, devPhyInfo := range devVkDriver.GetPhysicalDevices() {
			for _, tracePhyInfo := range traceVkDriver.GetPhysicalDevices() {
				// TODO: More sophisticated rules
				if devPhyInfo.GetVendorId() != tracePhyInfo.GetVendorId() {
					continue
				}
				if devPhyInfo.GetDeviceId() != tracePhyInfo.GetDeviceId() {
					continue
				}
				if devPhyInfo.GetDriverVersion() != tracePhyInfo.GetDriverVersion() {
					continue
				}
				// Ignore the API patch level (bottom 12 bits) when comparing the API version.
				if (devPhyInfo.GetApiVersion() & ^uint32(0xfff)) != (tracePhyInfo.GetApiVersion() & ^uint32(0xfff)) {
					continue
				}
				return 1
			}
		}
	}
	return 0
}
