// Copyright (C) 2022 Google Inc.
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

package profile

import (
	"sort"

	"github.com/google/gapid/core/data/slice"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

type groupTreeNode struct {
	id   int32
	name string
	link sync.SubCmdRange

	children []groupTreeNode // sorted by child.link
}

type GroupTree struct {
	nextID int32 // The first unused ID in this tree.
	groupTreeNode
}

func NewGroupTree() *GroupTree {
	return &GroupTree{
		nextID: 1,
		groupTreeNode: groupTreeNode{
			id:   0,
			name: "root",
		},
	}
}

// GetOrCreateGroup finds or creates the group for the given command range and returns its id.
// TODO: this function makes some assumptions about command/sub command IDs:
// 1. we only get groups for command buffers, renderpasses and draw calls.
// 2. no overlaps.
// 3. the sub command ids are [cmdId, submission, cmdbuff, cmd].
// All these assumptions currently hold and are also made in other parts of the
// code in some way. The assumptions will need to be codified as part of the
// command/sub-command refactor that is already planned.
func (t *GroupTree) GetOrCreateGroup(name string, link sync.SubCmdRange) int32 {
	submit, ok := t.findOrInsert(t.nextID, "submit", sync.SubCmdRange{From: link.From[:1], To: link.From[:1]})
	if !ok {
		t.nextID++
	}

	cmdBuf, ok := submit.findOrInsert(t.nextID, "cmdbuf", sync.SubCmdRange{From: link.From[:3], To: link.From[:3]})
	if !ok {
		t.nextID++
	}

	if len(link.From) == 3 {
		// We've found our command buffer. Let's update the name in case we created it with "cmdbuf".
		cmdBuf.name = name
		return cmdBuf.id
	}

	rp, ok := cmdBuf.findOrInsert(t.nextID, name, link)
	if !ok {
		t.nextID++
	}

	return rp.id
}

// GetDrawCallGroup finds or creates the group for the given draw call command and returns its id.
// TODO: the same assumptions apply here as do for GetOrCreateGroup.
func (t *GroupTree) GetOrCreateDrawCallGroup(name string, link api.SubCmdIdx) int32 {
	submit := t.find(link[:1])
	if submit == nil {
		return -1
	}

	cmdBuf := submit.find(link[:3])
	if cmdBuf == nil {
		return -1
	}

	draw, ok := cmdBuf.findOrInsertDrawCall(t.nextID, name, link)
	if !ok {
		t.nextID++
	}

	if draw == nil {
		return -1
	}
	return draw.id
}

// Visit visits each node in the tree invoking the given callback for each node.
func (t *GroupTree) Visit(callback func(parent int32, node *groupTreeNode)) {
	for i := range t.children {
		t.children[i].visit(0, callback)
	}
}

// Flatten flattens this tree into a list of group protos.
func (t *GroupTree) Flatten(capture *path.Capture) []*service.ProfilingData_Group {
	list := []*service.ProfilingData_Group{}
	t.Visit(func(parent int32, n *groupTreeNode) {
		list = append(list, &service.ProfilingData_Group{
			Id:       n.id,
			Name:     n.name,
			ParentId: parent,
			Link:     &path.Commands{Capture: capture, From: n.link.From, To: n.link.To},
		})
	})
	return list
}

func (n *groupTreeNode) visit(parent int32, callback func(parent int32, node *groupTreeNode)) {
	callback(parent, n)
	for i := range n.children {
		n.children[i].visit(n.id, callback)
	}
}

// find returns the child node that matches the given SubCmdIdx or nil if none matches.
func (n *groupTreeNode) find(link api.SubCmdIdx) *groupTreeNode {
	idx := sort.Search(len(n.children), func(i int) bool {
		return link.LEQ(n.children[i].link.From)
	})
	if idx < len(n.children) && n.children[idx].link.From.Equals(link) {
		return &n.children[idx]
	}
	return nil
}

func (n *groupTreeNode) findOrInsert(id int32, name string, link sync.SubCmdRange) (*groupTreeNode, bool) {
	idx := sort.Search(len(n.children), func(i int) bool {
		return link.From.LEQ(n.children[i].link.From)
	})
	if idx < len(n.children) && n.children[idx].link.From.Equals(link.From) {
		return &n.children[idx], true
	}
	slice.InsertBefore(&n.children, idx, groupTreeNode{id, name, link, nil})
	return &n.children[idx], false
}

func (n *groupTreeNode) findOrInsertDrawCall(id int32, name string, link api.SubCmdIdx) (*groupTreeNode, bool) {
	// First find the render pass child this draw call belongs to.
	rpIdx := sort.Search(len(n.children), func(i int) bool {
		return link.LEQ(n.children[i].link.To)
	})
	if rpIdx >= len(n.children) || !n.children[rpIdx].link.From.LEQ(link) {
		// No render pass was found for this draw call.
		return nil, true
	}
	rp := &n.children[rpIdx]

	// Find the correct location within this renderpass for this draw call.
	drawIdx := sort.Search(len(rp.children), func(i int) bool {
		return link.LEQ(rp.children[i].link.From)
	})
	if drawIdx < len(rp.children) && rp.children[drawIdx].link.From.Equals(link) {
		return &rp.children[drawIdx], true
	}
	slice.InsertBefore(&rp.children, drawIdx, groupTreeNode{
		id, name, sync.SubCmdRange{From: link, To: link}, nil,
	})
	return &rp.children[drawIdx], false
}
