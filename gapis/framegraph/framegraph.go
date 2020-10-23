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

package framegraph

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/api/vulkan"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/memory"
	d2 "github.com/google/gapid/gapis/resolve/dependencygraph2"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

func lookupImage(state *vulkan.State, pool memory.PoolID, memRange memory.Range) *vulkan.ImageObjectʳ {
	for _, image := range state.Images().All() {
		for _, aspect := range image.Aspects().All() {
			for _, layer := range aspect.Layers().All() {
				for _, level := range layer.Levels().All() {
					data := level.Data()
					if data.Pool() == pool && data.Range().First() <= memRange.First() && memRange.Last() <= data.Range().Last() {
						return &image
					}
				}
			}
		}
	}
	return nil
}

// rpInfo stores information for a given renderpass
type rpInfo struct {
	beginCmdIdx api.SubCmdIdx
	numCmds     uint64
	dpNodes     map[d2.NodeID]bool
	imgRead     map[vulkan.VkImage]bool
	imgWrite    map[vulkan.VkImage]bool
	imgInfos    map[vulkan.VkImage]vulkan.ImageInfo
}

// GetFramegraph creates and returns the framegraph of a capture.
func GetFramegraph(ctx context.Context, p *path.Capture) (*service.Framegraph, error) {

	c, err := capture.ResolveGraphicsFromPath(ctx, p)
	if err != nil {
		return nil, err
	}

	// get dependency graph
	config := d2.DependencyGraphConfig{
		SaveNodeAccesses:    true,
		ReverseDependencies: true, // TODO: maybe not needed?
	}
	dependencyGraph, err := d2.GetDependencyGraph(ctx, p, config)
	if err != nil {
		return nil, err
	}

	rpInfos := []*rpInfo{}
	var rpi *rpInfo

	postCmdCb := func(*api.GlobalState, api.SubCmdIdx, api.Cmd) {}

	postSubCmdCb := func(state *api.GlobalState, subCmdIdx api.SubCmdIdx, cmd api.Cmd, i interface{}) {
		vkState := vulkan.GetState(state)

		cmdRef := i.(vulkan.CommandReferenceʳ)
		cmdArgs := vulkan.GetCommandArgs(ctx, cmdRef, vkState)
		log.W(ctx, "HUGUES subcmdix:%v cmdArgs %v", subCmdIdx, cmdArgs)

		// Beginning of RP
		if _, ok := cmdArgs.(vulkan.VkCmdBeginRenderPassArgsʳ); ok {
			if rpi != nil {
				panic("Nested renderpasses?")
			}
			rpi = &rpInfo{
				beginCmdIdx: append(subCmdIdx),
				dpNodes:     make(map[d2.NodeID]bool),
				imgRead:     make(map[vulkan.VkImage]bool),
				imgWrite:    make(map[vulkan.VkImage]bool),
				imgInfos:    make(map[vulkan.VkImage]vulkan.ImageInfo),
			}
		}

		// Store info for subcommands that are inside a RP
		if rpi != nil {
			// Collect dependencygraph nodes from this RP
			rpi.numCmds++
			// TODO: maybe there's a better way to find dependencies between RPs?
			nodeID := dependencyGraph.GetCmdNodeID(api.CmdID(subCmdIdx[0]), subCmdIdx[1:])
			rpi.dpNodes[nodeID] = true

			// Analyze memory accesses
			for _, memAccess := range dependencyGraph.GetNodeAccesses(nodeID).MemoryAccesses {
				// TODO: refactor rpInfo fields to make the following code smoother to write
				count := memAccess.Span.End - memAccess.Span.Start
				memRange := memory.Range{
					Base: memAccess.Span.Start,
					Size: count,
				}
				image := lookupImage(vkState, memAccess.Pool, memRange)
				if image != nil {
					rpi.imgInfos[image.VulkanHandle()] = image.Info()
				}
				switch memAccess.Mode {
				case d2.ACCESS_READ:
					if image != nil {
						rpi.imgRead[image.VulkanHandle()] = true
					}
				case d2.ACCESS_WRITE:
					if image != nil {
						rpi.imgWrite[image.VulkanHandle()] = true
					}
				}
			}
		}

		// Ending of RP
		if _, ok := cmdArgs.(vulkan.VkCmdEndRenderPassArgsʳ); ok {
			rpInfos = append(rpInfos, rpi)
			rpi = nil
		}
	}

	if err := sync.MutateWithSubcommands(ctx, p, c.Commands, postCmdCb, nil, postSubCmdCb); err != nil {
		return nil, err
	}

	// Better use MutateWithSubCommands than syncData.
	// executing queuesubmit may NOT always execute the subcommands, e.g.
	// it may need a setEvent from host side to be able to execute.
	// Basically, you cannot assume that all subcommands are executed
	// upon the queuesubmit.
	// MutateWithSubCommand handles all this properly. Also secondary
	// command buffers.

	// Create framegraph contents based on rpInfo and dependency graph
	nodes := []*api.FramegraphNode{}
	for i, rpi := range rpInfos {

		// Use "\l" for newlines as this produce left-align lines in graphviz DOT labels
		text := fmt.Sprintf("%v Cmds, startIdx:%v\\l\\l", rpi.numCmds, rpi.beginCmdIdx)
		for img, info := range rpi.imgInfos {
			// Represent read/write with 2 characters as in file accesss bits, e.g. -- / r- / -w / rw
			r := "-"
			if _, ok := rpi.imgRead[img]; ok {
				r = "r"
			}
			w := "-"
			if _, ok := rpi.imgWrite[img]; ok {
				w = "w"
			}

			extent := info.Extent()
			dimensions := fmt.Sprintf("[%vx%vx%v]", extent.Width(), extent.Height(), extent.Depth())
			imgType := strings.TrimPrefix(fmt.Sprintf("%v", info.ImageType()), "VK_IMAGE_TYPE_")
			imgFmt := strings.TrimPrefix(fmt.Sprintf("%v", info.Fmt()), "VK_FORMAT_")
			text += fmt.Sprintf("Img 0x%X %v%v %v %v %v\\l", img, r, w, dimensions, imgType, imgFmt)
		}

		nodes = append(nodes, &api.FramegraphNode{
			Id:   uint64(i),
			Type: api.FramegraphNodeType_RENDERPASS,
			Text: text,
		})
	}

	edges := []*api.FramegraphEdge{}
	for i, srcRpi := range rpInfos {
		dependsOn := map[int]bool{}
		for src := range srcRpi.dpNodes {
			dependencyGraph.ForeachDependencyFrom(src, func(nodeID d2.NodeID) error {
				for j, dstRpi := range rpInfos {
					if dstRpi == srcRpi {
						continue
					}
					if _, ok := dstRpi.dpNodes[nodeID]; ok {
						dependsOn[j] = true
					}
				}
				return nil
			})
		}
		for dep := range dependsOn {
			edges = append(edges, &api.FramegraphEdge{
				Origin:      uint64(i),
				Destination: uint64(dep),
			})
		}
	}

	return &service.Framegraph{Nodes: nodes, Edges: edges}, nil
}

// TODO: I guess there's already a helper function somewhere
// to do this properly.
func memFmt(bytes uint64) string {
	kb := bytes / 1000
	mb := kb / 1000
	if mb > 0 {
		return fmt.Sprintf("%vMb", mb)
	}
	if kb > 0 {
		return fmt.Sprintf("%vKb", kb)
	}
	return fmt.Sprintf("%vb", bytes)
}
