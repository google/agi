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

package transform2

import (
	"context"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/commandGenerator"
	"github.com/google/gapid/gapis/config"
)

// TransformChain is responsible for running all the transforms on
// all the commands produced by command enerator
type TransformChain struct {
	transforms            []Transform
	out                   Writer
	generator             commandGenerator.CommandGenerator
	hasBegun              bool
	hasEnded              bool
	currentCommandID      api.CmdID
	currentTransformIndex int
	mutator               StateMutator
}

// CreateTransformChain creates a transform chain that will run
// the required transforms on every command coming from commandGenerator
func CreateTransformChain(ctx context.Context, generator commandGenerator.CommandGenerator, transforms []Transform, out Writer) *TransformChain {
	chain := TransformChain{
		generator:        generator,
		transforms:       transforms,
		out:              out,
		hasBegun:         false,
		hasEnded:         false,
		currentCommandID: 0,
		mutator:          nil,
	}

	chain.mutator = func(id api.CmdID, cmds []api.Cmd) error {
		return chain.stateMutator(ctx, id, cmds)
	}

	for _, t := range chain.transforms {
		if t.RequiresAccurateState() {
			// Melih TODO: Temporary check until we implement accurate state
			panic("Implement accurate state")
		}

		if t.RequiresInnerStateMutation() {
			t.SetInnerStateMutationFunction(chain.mutator)
		}
	}

	return &chain
}

func (chain *TransformChain) beginChain(ctx context.Context) error {
	var err error
	cmds := make([]api.Cmd, 0)

	for _, transform := range chain.transforms {
		cmds, err = transform.BeginTransform(ctx, cmds, chain.out.State())
		if err != nil {
			log.W(ctx, "Begin Transform Error [%v] : %v", transform, err)
			return err
		}

		if transform.RequiresAccurateState() {
			// Melih TODO: Temporary check until we implement accurate state
			panic("Implement accurate state")
		}
	}

	if err = mutateAndWrite(ctx, 0, cmds, chain.out); err != nil {
		return err
	}

	for _, transform := range chain.transforms {
		transform.ClearTransformResources(ctx)
	}

	return nil
}

func (chain *TransformChain) endChain(ctx context.Context) error {
	var err error
	cmds := make([]api.Cmd, 0)

	for _, transform := range chain.transforms {
		cmds, err = transform.EndTransform(ctx, cmds, chain.out.State())
		if err != nil {
			log.W(ctx, "End Transform Error [%v] : %v", transform, err)
			return err
		}

		if transform.RequiresAccurateState() {
			// Melih TODO: Temporary check until we implement accurate state
			panic("Implement accurate state")
		}
	}

	if err = mutateAndWrite(ctx, 0, cmds, chain.out); err != nil {
		return err
	}

	for _, transform := range chain.transforms {
		transform.ClearTransformResources(ctx)
	}

	return nil
}

func (chain *TransformChain) transformCommand(ctx context.Context, id api.CmdID, inputCmds []api.Cmd, beginTransformIndex int) error {
	for i, transform := range chain.transforms {
		if i < beginTransformIndex {
			continue
		}

		chain.currentTransformIndex = i
		var err error
		inputCmds, err = transform.TransformCommand(ctx, id, inputCmds, chain.out.State())
		if err != nil {
			log.W(ctx, "Error on Transform on cmd [%v:%v] with transform [:v:%v] : %v", id, inputCmds, i, transform, err)
			return err
		}

		if transform.RequiresAccurateState() {
			// Melih TODO: Temporary check until we implement accurate state
			panic("Implement accurate state")
		}

	}

	if err := mutateAndWrite(ctx, id, inputCmds, chain.out); err != nil {
		return err
	}

	if beginTransformIndex == 0 {
		for _, transform := range chain.transforms {
			transform.ClearTransformResources(ctx)
		}
	}

	return nil
}

func (chain *TransformChain) IsEndOfCommands() bool {
	return chain.hasEnded
}

func (chain *TransformChain) GetNextTransformedCommands(ctx context.Context) error {
	if !chain.hasBegun {
		chain.hasBegun = true
		return chain.beginChain(ctx)
	}

	if chain.generator.IsEndOfCommands() {
		if !chain.hasEnded {
			chain.hasEnded = true
			return chain.endChain(ctx)
		}

		return nil
	}

	currentCommand := chain.generator.GetNextCommand(ctx)
	if config.DebugReplay {
		log.I(ctx, "Transforming... (%v:%v)", chain.currentCommandID, currentCommand)
	}

	inputCmds := make([]api.Cmd, 0)
	inputCmds = append(inputCmds, currentCommand)
	err := chain.transformCommand(ctx, chain.currentCommandID, inputCmds, 0)
	if err != nil {
		log.E(ctx, "Replay error (%v:%v): %v", chain.currentCommandID, currentCommand, err)
	}

	chain.currentCommandID++
	return err
}

func (chain *TransformChain) stateMutator(ctx context.Context, id api.CmdID, cmds []api.Cmd) error {
	beginTransformIndex := chain.currentTransformIndex + 1

	if err := chain.transformCommand(ctx, id, cmds, beginTransformIndex); err != nil {
		log.E(ctx, "state mutator error (%v:%v): %v", id, cmds, err)
		return err
	}

	chain.currentTransformIndex = beginTransformIndex - 1
	return nil
}

func mutateAndWrite(ctx context.Context, id api.CmdID, cmds []api.Cmd, out Writer) error {
	for i, cmd := range cmds {
		if err := out.MutateAndWrite(ctx, id, cmd); err != nil {
			log.W(ctx, "State mutation error in command [%v:%v]:%v", id, i, cmd)
			return err
		}
	}

	return nil
}
