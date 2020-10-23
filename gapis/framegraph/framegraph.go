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
	// do we get a copy of the image object here?
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
	dpNodes     map[d2.NodeID]bool
	totalRead   uint64
	totalWrite  uint64
	read        map[memory.PoolID]uint64
	write       map[memory.PoolID]uint64
	imgRead     map[uint64]uint64
	imgWrite    map[uint64]uint64
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
				read:        make(map[memory.PoolID]uint64),
				write:       make(map[memory.PoolID]uint64),
				dpNodes:     make(map[d2.NodeID]bool),
				imgRead:     make(map[uint64]uint64),
				imgWrite:    make(map[uint64]uint64),
			}
		}

		// Store info for subcommands that are inside a RP
		if rpi != nil {
			// Collect dependencygraph nodes from this RP
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
				switch memAccess.Mode {
				case d2.ACCESS_READ:
					rpi.totalRead += count
					if _, ok := rpi.read[memAccess.Pool]; ok {
						rpi.read[memAccess.Pool] += count
					} else {
						rpi.read[memAccess.Pool] = count
					}
					if image != nil {
						if _, ok := rpi.imgRead[uint64(image.VulkanHandle())]; ok {
							rpi.imgRead[uint64(image.VulkanHandle())] += count
						} else {
							rpi.imgRead[uint64(image.VulkanHandle())] = count
						}
					}
				case d2.ACCESS_WRITE:
					rpi.totalWrite += count
					if _, ok := rpi.write[memAccess.Pool]; ok {
						rpi.write[memAccess.Pool] += count
					} else {
						rpi.write[memAccess.Pool] = count
					}
					if image != nil {
						if _, ok := rpi.imgWrite[uint64(image.VulkanHandle())]; ok {
							rpi.imgWrite[uint64(image.VulkanHandle())] += count
						} else {
							rpi.imgWrite[uint64(image.VulkanHandle())] = count
						}
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
		text := fmt.Sprintf("RP %v\\lTotal: read(%v) write(%v)\\l", rpi.beginCmdIdx, memFmt(rpi.totalRead), memFmt(rpi.totalWrite))
		for img, bytes := range rpi.imgRead {
			text += fmt.Sprintf("Img 0x%x read(%v)\\l", img, memFmt(bytes))
		}
		for img, bytes := range rpi.imgWrite {
			text += fmt.Sprintf("Img 0x%x  write(%v)\\l", img, memFmt(bytes))
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
