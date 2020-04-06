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
	"bytes"
	"context"
	"fmt"

	"github.com/google/gapid/core/image"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/command_generator"
	"github.com/google/gapid/gapis/api/transform2"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/config"
	"github.com/google/gapid/gapis/replay"
	"github.com/google/gapid/gapis/resolve"
	"github.com/google/gapid/gapis/resolve/initialcmds"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
	"github.com/google/gapid/gapis/trace"
)

var (
	// Interface compliance tests
	_ = replay.QueryIssues(API{})
	_ = replay.QueryFramebufferAttachment(API{})
	_ = replay.Support(API{})
	_ = replay.QueryTimestamps(API{})
	_ = replay.Profiler(API{})
)

// GetReplayPriority returns a uint32 representing the preference for
// replaying this trace on the given device.
// A lower number representgis a higher priority, and Zero represents
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
		// Requires same vendor, device and version of API.
		for _, devPhyInfo := range devVkDriver.GetPhysicalDevices() {
			for _, tracePhyInfo := range traceVkDriver.GetPhysicalDevices() {
				// TODO: More sophisticated rules
				if devPhyInfo.GetVendorId() != tracePhyInfo.GetVendorId() {
					continue
				}
				if devPhyInfo.GetDeviceId() != tracePhyInfo.GetDeviceId() {
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

// drawConfig is a replay.Config used by colorBufferRequest and
// depthBufferRequests.
type drawConfig struct {
	startScope                api.CmdID
	endScope                  api.CmdID
	subindices                string // drawConfig needs to be comparable, so we cannot use a slice
	drawMode                  path.DrawMode
	disableReplayOptimization bool
}

type imgRes struct {
	img *image.Data // The image data.
	err error       // The error that occurred generating the image.
}

// framebufferRequest requests a postback of a framebuffer's attachment.
type framebufferRequest struct {
	after            []uint64
	width, height    uint32
	attachment       api.FramebufferAttachmentType
	framebufferIndex uint32
	out              chan imgRes
	wireframeOverlay bool
	displayToSurface bool
}

// issuesConfig is a replay.Config used by issuesRequests.
type issuesConfig struct {
}

// issuesRequest requests all issues found during replay to be reported to out.
type issuesRequest struct {
	out              chan<- replay.Issue
	displayToSurface bool
	loopCount        int32
}

type timestampsConfig struct {
}

type timestampsRequest struct {
	handler   service.TimeStampsHandler
	loopCount int32
}

// uniqueConfig returns a replay.Config that is guaranteed to be unique.
// Any requests made with a Config returned from uniqueConfig will not be
// batched with any other request.
func uniqueConfig() replay.Config {
	return &struct{}{}
}

type profileRequest struct {
	traceOptions   *service.TraceOptions
	handler        *replay.SignalHandler
	buffer         *bytes.Buffer
	handleMappings *map[uint64][]service.VulkanHandleMappingItem
}

func (a API) GetInitialPayload(ctx context.Context,
	capture *path.Capture,
	device *device.Instance,
	out transform2.Writer) error {

	initialCmds, im, _ := initialcmds.InitialCommands(ctx, capture)
	out.State().Allocator.ReserveRanges(im)
	initialCommandGenerator := command_generator.NewLinearCommandGenerator(initialCmds)

	controlFlow := transform2.NewControlFlow("initial Payload", initialCommandGenerator, out)
	controlFlow.AddTransform(NewMakeAttachmentReadable(false))
	controlFlow.AddTransform(NewDropInvalidDestroy("GetInitialPayload"))

	return controlFlow.TransformAll(ctx)
}

func (a API) CleanupResources(ctx context.Context, device *device.Instance, out transform2.Writer) error {
	commandGenerator := command_generator.NewLinearCommandGenerator([]api.Cmd{})
	controlFlow := transform2.NewControlFlow("Cleanup Resources", commandGenerator, out)
	controlFlow.AddTransform(NewDestroyResourcesAtEOS())
	return controlFlow.TransformAll(ctx)
}

func (a API) QueryFramebufferAttachment(
	ctx context.Context,
	intent replay.Intent,
	mgr replay.Manager,
	after []uint64,
	width, height uint32,
	attachment api.FramebufferAttachmentType,
	framebufferIndex uint32,
	drawMode path.DrawMode,
	disableReplayOptimization bool,
	displayToSurface bool,
	hints *path.UsageHints) (*image.Data, error) {

	beginIndex := api.CmdID(0)
	endIndex := api.CmdID(0)
	subcommand := ""
	// We cant break up overdraw right now, but we can break up
	// everything else.
	if drawMode == path.DrawMode_OVERDRAW {
		if len(after) > 1 { // If we are replaying subcommands, then we can't batch at all
			beginIndex = api.CmdID(after[0])
			endIndex = api.CmdID(after[0])
			for i, j := range after[1:] {
				if i != 0 {
					subcommand += ":"
				}
				subcommand += fmt.Sprintf("%d", j)
			}
		}
	}

	c := drawConfig{beginIndex, endIndex, subcommand, drawMode, disableReplayOptimization}
	out := make(chan imgRes, 1)
	r := framebufferRequest{after: after, width: width, height: height, framebufferIndex: framebufferIndex, attachment: attachment, out: out, displayToSurface: displayToSurface}
	res, err := mgr.Replay(ctx, intent, c, r, a, hints, false)
	if err != nil {
		return nil, err
	}
	if _, ok := mgr.(replay.Exporter); ok {
		return nil, nil
	}
	return res.(*image.Data), nil
}

func (a API) QueryIssues(
	ctx context.Context,
	intent replay.Intent,
	mgr replay.Manager,
	loopCount int32,
	displayToSurface bool,
	hints *path.UsageHints) ([]replay.Issue, error) {

	c, r := issuesConfig{}, issuesRequest{displayToSurface: displayToSurface, loopCount: loopCount}
	res, err := mgr.Replay(ctx, intent, c, r, a, hints, true)

	if err != nil {
		return nil, err
	}
	if _, ok := mgr.(replay.Exporter); ok {
		return nil, nil
	}
	return res.([]replay.Issue), nil
}

func (a API) QueryTimestamps(
	ctx context.Context,
	intent replay.Intent,
	mgr replay.Manager,
	loopCount int32,
	handler service.TimeStampsHandler,
	hints *path.UsageHints) error {

	c, r := timestampsConfig{}, timestampsRequest{
		handler:   handler,
		loopCount: loopCount}
	_, err := mgr.Replay(ctx, intent, c, r, a, hints, false)
	if err != nil {
		return err
	}
	if _, ok := mgr.(replay.Exporter); ok {
		return nil
	}
	return nil
}

func (a API) Profile(
	ctx context.Context,
	intent replay.Intent,
	mgr replay.Manager,
	hints *path.UsageHints,
	traceOptions *service.TraceOptions) (*service.ProfilingData, error) {

	c := uniqueConfig()
	handler := replay.NewSignalHandler()
	var buffer bytes.Buffer
	handleMappings := make(map[uint64][]service.VulkanHandleMappingItem)
	r := profileRequest{traceOptions, handler, &buffer, &handleMappings}
	_, err := mgr.Replay(ctx, intent, c, r, a, hints, true)
	if err != nil {
		return nil, err
	}
	handler.DoneSignal.Wait(ctx)

	s, err := resolve.SyncData(ctx, intent.Capture)
	if err != nil {
		return nil, err
	}

	d, err := trace.ProcessProfilingData(ctx, intent.Device, intent.Capture, &buffer, &handleMappings, s)
	return d, err
}

func getCommonInitializationTransforms(tag string, readOnlyImageAttachments bool) []transform2.Transform {
	return []transform2.Transform{
		NewMakeAttachmentReadable(readOnlyImageAttachments),
		NewDropInvalidDestroy(tag),
	}
}

func getLogTransforms(ctx context.Context, tag string, capture *capture.GraphicsCapture) []transform2.Transform {
	transforms := make([]transform2.Transform, 0)

	if config.LogTransformsToCapture {
		transforms = append(transforms, transform2.NewCaptureLog(ctx, capture, tag+"replay_log.gfxtrace"))
	}

	if config.LogMappingsToFile {
		transforms = append(transforms, replay.NewMappingExporterWithPrint(ctx, tag+"mappings.txt"))
	}

	if len(transforms) == 0 {
		return nil
	}

	return transforms
}

func getInitialControlFlow(ctx context.Context, dependentPayload string, intent replay.Intent, out transform2.Writer) *transform2.ControlFlow {
	initialCmds := make([]api.Cmd, 0)

	// Melih TODO: Do we really need this(dependentPayload) for the particular replay type?
	if dependentPayload == "" {
		cmds, im, _ := initialcmds.InitialCommands(ctx, intent.Capture)
		out.State().Allocator.ReserveRanges(im)
		initialCmds = append(initialCmds, cmds...)
	}

	if config.DebugReplay {
		log.I(ctx, "Replaying %d initial commands using transform chain:", len(initialCmds))
	}

	initialCommandGenerator := command_generator.NewLinearCommandGenerator(initialCmds)
	initialControlFlow := transform2.NewControlFlow("initial_commands", initialCommandGenerator, out)
	return initialControlFlow
}

func replayIssues(ctx context.Context,
	intent replay.Intent,
	dependentPayload string,
	rrs []replay.RequestAndResult,
	c *capture.GraphicsCapture,
	out transform2.Writer) error {

	initialControlFlow := getInitialControlFlow(ctx, dependentPayload, intent, out)
	initialControlFlow.AddTransform(getCommonInitializationTransforms("IssuesReplay_Initial", false)...)

	if config.DebugReplay {
		log.I(ctx, "Replaying %d real commands using transform chain:", len(c.Commands))
	}

	realCommandGenerator := command_generator.NewLinearCommandGenerator(c.Commands)
	realControlFlow := transform2.NewControlFlow("RealCommands", realCommandGenerator, out)
	realControlFlow.AddTransform(getCommonInitializationTransforms("IssuesReplay_Real", false)...)

	issuesTransform := NewFindIssues(ctx, c)
	doDisplayToSurface := false

	// Melih TODO: Can we get rid of this loop and typecast?
	for _, rr := range rrs {
		issuesTransform.AddResult(rr.Result)

		if req, ok := rr.Request.(issuesRequest); ok {
			// Melih TODO: Do we need this for issues?
			if req.displayToSurface {
				doDisplayToSurface = true
			}
		}
	}

	// We are only interested in issues in real commands
	realControlFlow.AddTransform(issuesTransform)

	if doDisplayToSurface {
		// Melih TODO: Check if doDisplayToSurface affects the initial cmds
		realControlFlow.AddTransform(NewDisplayToSurface())
	}

	// Add destroy only to end of whole replay
	realControlFlow.AddTransform(NewDestroyResourcesAtEOS())

	logTransforms := getLogTransforms(ctx, "IssuesReplay", c)
	if logTransforms != nil {
		// Melih TODO: adding only to real commands is enough?
		realControlFlow.AddTransform(logTransforms...)
	}

	err := initialControlFlow.TransformAll(ctx)
	if err != nil {
		log.E(ctx, "[Issues Replay] Error on initial cmds: %v", err)
		return err
	}

	err = realControlFlow.TransformAll(ctx)
	if err != nil {
		log.E(ctx, "[Issues Replay] Error on real cmds: %v", err)
		return err
	}

	return nil
}

func replayTimestamps(ctx context.Context,
	intent replay.Intent,
	dependentPayload string,
	rrs []replay.RequestAndResult,
	c *capture.GraphicsCapture,
	out transform2.Writer) error {

	initialControlFlow := getInitialControlFlow(ctx, dependentPayload, intent, out)
	initialControlFlow.AddTransform(getCommonInitializationTransforms("TimestampReplay_Initial", false)...)

	realCommandGenerator := command_generator.NewLinearCommandGenerator(c.Commands)
	realControlFlow := transform2.NewControlFlow("RealCommands", realCommandGenerator, out)
	realControlFlow.AddTransform(getCommonInitializationTransforms("TimestampReplay_Real", false)...)

	request := rrs[0].Request.(timestampsRequest)
	timestampsTransform := NewQueryTimestamps(ctx, request.handler)
	for _, rr := range rrs {
		timestampsTransform.AddResult(rr.Result)
	}

	// We are interested with only real commands
	realControlFlow.AddTransform(timestampsTransform)

	// Add destroy only to end of whole replay
	realControlFlow.AddTransform(NewDestroyResourcesAtEOS())

	logTransforms := getLogTransforms(ctx, "TimestampsReplay", c)
	if logTransforms != nil {
		// Melih TODO: adding only to real commands is enough?
		realControlFlow.AddTransform(logTransforms...)
	}

	err := initialControlFlow.TransformAll(ctx)
	if err != nil {
		log.E(ctx, "[Timestamps Replay] Error on initial cmds: %v", err)
		return err
	}

	err = realControlFlow.TransformAll(ctx)
	if err != nil {
		log.E(ctx, "[Timestamps Replay] Error on real cmds: %v", err)
		return err
	}

	return nil
}
func replayProfile(ctx context.Context,
	intent replay.Intent,
	dependentPayload string,
	rrs []replay.RequestAndResult,
	c *capture.GraphicsCapture,
	device *device.Instance,
	out transform2.Writer) error {

	initialControlFlow := getInitialControlFlow(ctx, dependentPayload, intent, out)
	initialControlFlow.AddTransform(getCommonInitializationTransforms("ProfileReplay_Initial", true)...)

	realCommandGenerator := command_generator.NewLinearCommandGenerator(c.Commands)
	realControlFlow := transform2.NewControlFlow("RealCommands", realCommandGenerator, out)
	realControlFlow.AddTransform(getCommonInitializationTransforms("ProfileReplay_Real", true)...)

	profileTransform := replay.NewEndOfReplay()
	for _, rr := range rrs {
		profileTransform.AddResult(rr.Result)
	}

	// Melih TODO: I guess there is actually one request at this point
	request := rrs[0].Request.(profileRequest)
	realControlFlow.AddTransform(NewWaitForPerfetto(request.traceOptions, request.handler, request.buffer))

	var layerName string
	if device.GetConfiguration().GetPerfettoCapability().GetGpuProfiling().GetHasRenderStageProducerLayer() {
		layerName = "VkRenderStagesProducer"
	}
	realControlFlow.AddTransform(NewProfilingLayers(layerName))
	realControlFlow.AddTransform(replay.NewMappingExporter(ctx, request.handleMappings))

	// We are only measuring the real commands
	realControlFlow.AddTransform(profileTransform)

	// Add destroy only to end of whole replay
	realControlFlow.AddTransform(NewDestroyResourcesAtEOS())

	logTransforms := getLogTransforms(ctx, "ProfileReplay", c)
	if logTransforms != nil {
		// Melih TODO: adding only to real commands is enough?
		realControlFlow.AddTransform(logTransforms...)
	}

	err := initialControlFlow.TransformAll(ctx)
	if err != nil {
		log.E(ctx, "[Profile Replay] Error on initial cmds: %v", err)
		return err
	}

	err = realControlFlow.TransformAll(ctx)
	if err != nil {
		log.E(ctx, "[Profile Replay] Error on real cmds: %v", err)
		return err
	}

	return nil
}

func replayFramebuffer(ctx context.Context,
	intent replay.Intent,
	cfg replay.Config,
	dependentPayload string,
	rrs []replay.RequestAndResult,
	c *capture.GraphicsCapture,
	out transform2.Writer) error {

	// Melih TODO : Special case with DCE
	initialControlFlow := getInitialControlFlow(ctx, dependentPayload, intent, out)
	initialControlFlow.AddTransform(getCommonInitializationTransforms("ProfileReplay_Initial", false)...)

	realCommandGenerator := command_generator.NewLinearCommandGenerator(c.Commands)
	realControlFlow := transform2.NewControlFlow("RealCommands", realCommandGenerator, out)
	realControlFlow.AddTransform(getCommonInitializationTransforms("ProfileReplay_Real", false)...)

	shouldRenderWired := false
	doDisplayToSurface := false
	shouldOverDraw := false

	vulkanTerminator, err := NewVulkanTerminator(ctx, intent.Capture)
	if err != nil {
		log.E(ctx, "Vulkan terminator failed: %v", err)
		return err
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
				return err
			}
			shouldOverDraw = true
			overdrawTransform.add(ctx, request.after, intent.Capture, rr.Result)
			break
		}

		if err := vulkanTerminator.Add(ctx, api.CmdID(cmdID), api.SubCmdIdx{}); err != nil {
			log.E(ctx, "Vulkan terminator error on Cmd(%v) : %v", cmdID, err)
			return err
		}

		subIdx := append(api.SubCmdIdx{}, request.after...)
		splitterTransform.Split(ctx, subIdx)

		switch cfg.drawMode {
		case path.DrawMode_WIREFRAME_ALL:
			shouldRenderWired = true
		case path.DrawMode_WIREFRAME_OVERLAY:
			return fmt.Errorf("Overlay wireframe view is not currently supported")
			// Overdraw is handled above, since it breaks out of the normal read flow.
		}

		switch request.attachment {
		case api.FramebufferAttachmentType_OutputDepth, api.FramebufferAttachmentType_InputDepth:
			readFramebufferTransform.Depth(ctx, subIdx, request.framebufferIndex, rr.Result)
		case api.FramebufferAttachmentType_OutputColor, api.FramebufferAttachmentType_InputColor:
			readFramebufferTransform.Color(ctx, subIdx, request.width, request.height, request.framebufferIndex, rr.Result)
		default:
			return fmt.Errorf("Stencil attachments are not currently supported")
		}

		if request.displayToSurface {
			doDisplayToSurface = true
		}
	}

	// Melih TODO: Any transform after this required for initialCmds? Yes we need
	if doDisplayToSurface {
		realControlFlow.AddTransform(NewDisplayToSurface())
	}

	if shouldRenderWired {
		realControlFlow.AddTransform(NewWireframeTransform())
	}

	realControlFlow.AddTransform(vulkanTerminator)

	if shouldOverDraw {
		realControlFlow.AddTransform(overdrawTransform)
	}

	realControlFlow.AddTransform(splitterTransform)
	realControlFlow.AddTransform(readFramebufferTransform)

	// Add destroy only to end of whole replay
	realControlFlow.AddTransform(NewDestroyResourcesAtEOS())

	logTransforms := getLogTransforms(ctx, "FramebufferReplay", c)
	if logTransforms != nil {
		// Melih TODO: adding only to real commands is enough?
		realControlFlow.AddTransform(logTransforms...)
	}

	err = initialControlFlow.TransformAll(ctx)
	if err != nil {
		log.E(ctx, "[Framebuffer Replay] Error on initial cmds: %v", err)
		return err
	}

	err = realControlFlow.TransformAll(ctx)
	if err != nil {
		log.E(ctx, "[Framebuffer Replay] Error on real cmds: %v", err)
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

	for _, rr := range rrs {
		switch rr.Request.(type) {
		case issuesRequest:
			return replayIssues(ctx, intent, dependentPayload, rrs, c, out)
		case timestampsRequest:
			return replayTimestamps(ctx, intent, dependentPayload, rrs, c, out)
		case profileRequest:
			return replayProfile(ctx, intent, dependentPayload, rrs, c, device, out)
		case framebufferRequest:
			return replayFramebuffer(ctx, intent, cfg, dependentPayload, rrs, c, out)
		default:
			panic("This should never happen")
		}
	}

	log.W(ctx, "No request has found")
	return nil
}

func shouldOptimise(rrs []replay.RequestAndResult) bool {
	return false

	if config.DisableDeadCodeElimination {
		return false
	}

	for _, rr := range rrs {
		if _, ok := rr.Request.(framebufferRequest); ok {
			return true
		}
	}

	return false
}
