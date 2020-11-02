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
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/resolve/dependencygraph2"
	"github.com/google/gapid/gapis/service/path"
)

// renderpassInfo stores a renderpass' info relevant for the framegraph.
type renderpassInfo struct {
	id       uint64
	beginIdx api.SubCmdIdx
	endIdx   api.SubCmdIdx
	nodes    []dependencygraph2.NodeID
	deps     map[uint64]struct{} // set of renderpasses this renderpass depends on
}

// GetFramegraph creates the framegraph of the given capture.
func (API) GetFramegraph(ctx context.Context, p *path.Capture) (*api.Framegraph, error) {
	config := dependencygraph2.DependencyGraphConfig{
		SaveNodeAccesses:    true,
		ReverseDependencies: true,
	}
	dependencyGraph, err := dependencygraph2.GetDependencyGraph(ctx, p, config)
	if err != nil {
		return nil, err
	}

	rpInfos := []*renderpassInfo{}
	var rpInfo *renderpassInfo
	currRpId := uint64(0)

	// postSubCmdCb effectively processes each subcommand to extract renderpass info.
	postSubCmdCb := func(state *api.GlobalState, subCmdIdx api.SubCmdIdx, cmd api.Cmd, i interface{}) {
		vkState := GetState(state)
		cmdRef := i.(CommandReferenceʳ)
		cmdArgs := GetCommandArgs(ctx, cmdRef, vkState)

		// Beginning of renderpass
		if _, ok := cmdArgs.(VkCmdBeginRenderPassArgsʳ); ok {
			if rpInfo != nil {
				panic("Renderpass starts without having ended")
			}
			rpInfo = &renderpassInfo{
				id:       currRpId,
				beginIdx: subCmdIdx,
				nodes:    []dependencygraph2.NodeID{},
				deps:     make(map[uint64]struct{}),
			}
			currRpId++
		}

		// Process commands that are inside a renderpass
		if rpInfo != nil {
			nodeID := dependencyGraph.GetCmdNodeID(api.CmdID(subCmdIdx[0]), subCmdIdx[1:])
			rpInfo.nodes = append(rpInfo.nodes, nodeID)
		}

		// Ending of renderpass
		if _, ok := cmdArgs.(VkCmdEndRenderPassArgsʳ); ok {
			if rpInfo == nil {
				panic("Renderpass ends without having started")
			}
			rpInfo.endIdx = subCmdIdx
			rpInfos = append(rpInfos, rpInfo)
			rpInfo = nil
		}
	}

	// Iterate on the capture commands to gather information
	c, err := capture.ResolveGraphicsFromPath(ctx, p)
	if err != nil {
		return nil, err
	}
	if err := sync.MutateWithSubcommands(ctx, p, c.Commands, nil, nil, postSubCmdCb); err != nil {
		return nil, err
	}

	updateDependencies(rpInfos, dependencyGraph)

	// Build the framegraph nodes and edges from collected data.
	nodes := make([]*api.FramegraphNode, len(rpInfos))
	for i, rpInfo := range rpInfos {
		// Graphviz DOT: use "\l" as a newline to obtain left-aligned text.
		text := fmt.Sprintf("Renderpass %v\\lbegin:%v\\lend:%v\\l", rpInfo.id, rpInfo.beginIdx, rpInfo.endIdx)
		nodes[i] = &api.FramegraphNode{
			Id:   rpInfo.id,
			Text: text,
		}
	}

	edges := []*api.FramegraphEdge{}
	for _, rpInfo = range rpInfos {
		for deps := range rpInfo.deps {
			edges = append(edges, &api.FramegraphEdge{
				// We want the graph to show the flow of how the frame is
				// created (rather than the flow of dependencies), so use the
				// dependency as the edge origin and rpInfo as the destination.
				Origin:      deps,
				Destination: rpInfo.id,
			})
		}
	}

	return &api.Framegraph{Nodes: nodes, Edges: edges}, nil
}

// updateDependencies establishes dependencies between renderpasses.
func updateDependencies(rpInfos []*renderpassInfo, dependencyGraph dependencygraph2.DependencyGraph) {
	// isInsideRenderpass: node -> renderpass it belongs to.
	isInsideRenderpass := map[dependencygraph2.NodeID]uint64{}
	for _, rpInfo := range rpInfos {
		for _, n := range rpInfo.nodes {
			isInsideRenderpass[n] = rpInfo.id
		}
	}
	// node2renderpasses: node -> set of renderpasses it depends on.
	node2renderpasses := map[dependencygraph2.NodeID]map[uint64]struct{}{}

	// For a given renderpass RP, for each of its node, explore the dependency
	// graph in reverse order to mark all the nodes dependending on RP until we
	// hit the node of another renderpass, which then depends on RP.
	for _, rpInfo := range rpInfos {
		// markNode is recursive, so declare it before initializing it.
		var markNode func(dependencygraph2.NodeID) error
		markNode = func(node dependencygraph2.NodeID) error {
			if id, ok := isInsideRenderpass[node]; ok {
				if id != rpInfo.id {
					// Reached a node that is inside another renderpass, so this
					// renderpass depends on rpInfo.
					rpInfos[id].deps[rpInfo.id] = struct{}{}
				}
				return nil
			}
			if _, ok := node2renderpasses[node]; !ok {
				node2renderpasses[node] = map[uint64]struct{}{}
			}
			if _, ok := node2renderpasses[node][rpInfo.id]; ok {
				// Node already visited, stop recursion
				return nil
			}
			node2renderpasses[node][rpInfo.id] = struct{}{}
			return dependencyGraph.ForeachDependencyTo(node, markNode)
		}
		for _, node := range rpInfo.nodes {
			dependencyGraph.ForeachDependencyTo(node, markNode)
		}
	}
}
