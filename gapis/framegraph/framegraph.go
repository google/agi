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

// map[memory.PoolID]map[memory.Range]ImageObject^r

// stateResourceMapping correlates VkImage handles with their memory accesses.
// It is created once per vkQueueSubmit, and then used as a cache to avoid
// scanning the state data-structures everytime we try to find an image from
// a memory observation. There is room for improvement in this caching approach,
// e.g. index the outermost map by memory pool IDs.
type stateResourceMapping struct {
	images map[vulkan.VkImage]map[memory.PoolID][]memory.Range
}

func createStateResourceMapping(s *vulkan.State) stateResourceMapping {
	srm := stateResourceMapping{
		images: make(map[vulkan.VkImage]map[memory.PoolID][]memory.Range),
	}

	for handle, image := range s.Images().All() {
		if _, ok := srm.images[handle]; !ok {
			srm.images[handle] = make(map[memory.PoolID][]memory.Range)
		}
		for _, aspect := range image.Aspects().All() {
			for _, layer := range aspect.Layers().All() {
				for _, level := range layer.Levels().All() {
					data := level.Data()
					pool := data.Pool()
					if _, ok := srm.images[handle][pool]; !ok {
						srm.images[handle][pool] = []memory.Range{}
					}
					srm.images[handle][pool] = append(srm.images[handle][pool], data.Range())
				}
			}
		}
		planeMemInfos := image.PlaneMemoryInfo().All()
		for _, memInfo := range planeMemInfos {
			data := memInfo.BoundMemory().Data()
			pool := data.Pool()
			if _, ok := srm.images[handle][pool]; !ok {
				srm.images[handle][pool] = []memory.Range{}
			}
			srm.images[handle][pool] = append(srm.images[handle][pool], data.Range())
		}
	}

	// TODO: It is probaly not necessary to scan device memories,
	// as it will just alias to the info gathered by scanning images.
	devMems := s.DeviceMemories().All()
	for _, devMem := range devMems {
		data := devMem.Data()
		pool := data.Pool()
		boundObj := devMem.BoundObjects().All()
		// TODO: deal with memory offset from boundObj
		found := false
		for objHandle := range boundObj {
			imgHandle := vulkan.VkImage(objHandle)
			if _, ok := srm.images[imgHandle]; ok {
				if found {
					fmt.Printf("\nHUGUES double handle: %v", imgHandle)
				}
				found = true
				if _, ok := srm.images[imgHandle][pool]; !ok {
					srm.images[imgHandle][pool] = []memory.Range{}
				}
				srm.images[imgHandle][pool] = append(srm.images[imgHandle][pool], data.Range())
			}
		}
	}

	return srm
}

func (s stateResourceMapping) imageLookup(poolID memory.PoolID, rang memory.Range) (uint64, bool) {
	for img, mem := range s.images {
		if ranges, ok := mem[poolID]; ok {
			for _, r := range ranges {
				if r.First() <= rang.First() && rang.Last() <= r.Last() {
					return uint64(img), true
				}
			}
		}
	}
	return uint64(0), false
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

		srm := createStateResourceMapping(vkState)

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
				switch memAccess.Mode {
				case d2.ACCESS_READ:
					rpi.totalRead += count
					if _, ok := rpi.read[memAccess.Pool]; ok {
						rpi.read[memAccess.Pool] += count
					} else {
						rpi.read[memAccess.Pool] = count
					}
					if imgHandle, ok := srm.imageLookup(memAccess.Pool, memRange); ok {
						if _, ok := rpi.imgRead[imgHandle]; ok {
							rpi.imgRead[imgHandle] += count
						} else {
							rpi.imgRead[imgHandle] = count
						}
					}
					//else {
					// 	log.W(ctx, "HUGUES FAIL lookup (read) pool:%v span:%v", memAccess.Pool, memRange)
					// }
				case d2.ACCESS_WRITE:
					rpi.totalWrite += count
					if _, ok := rpi.write[memAccess.Pool]; ok {
						rpi.write[memAccess.Pool] += count
					} else {
						rpi.write[memAccess.Pool] = count
					}

					if imgHandle, ok := srm.imageLookup(memAccess.Pool, memRange); ok {
						//log.W(ctx, "HUGUES hit write resource: %v", res)
						if _, ok := rpi.imgWrite[imgHandle]; ok {
							rpi.imgWrite[imgHandle] += count
						} else {
							rpi.imgWrite[imgHandle] = count
						}
					}
					// else {
					// 	log.W(ctx, "HUGUES FAIL lookup (write) pool:%v span:%v", memAccess.Pool, memRange)
					// }
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
