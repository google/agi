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
	transforms       []Transform
	out              Writer
	generator        commandGenerator.CommandGenerator
	hasBegun         bool
	hasEnded         bool
	currentCommandID api.CmdID
}

// CreateTransformChain creates a transform chain that will run
// the required transforms on every command coming from commandGenerator
func CreateTransformChain(generator commandGenerator.CommandGenerator, transforms []Transform, out Writer) *TransformChain {
	return &TransformChain{
		generator:        generator,
		transforms:       transforms,
		out:              out,
		hasBegun:         false,
		hasEnded:         false,
		currentCommandID: 0,
	}
}

func (chain *TransformChain) beginChain(ctx context.Context) error {
	chain.handleInitialState(chain.out.State())

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

func (chain *TransformChain) transformCommand(ctx context.Context, inputCmd api.Cmd, id api.CmdID) error {
	var err error
	cmds := []api.Cmd{inputCmd}

	for _, transform := range chain.transforms {
		cmds, err = transform.TransformCommand(ctx, id, cmds, chain.out.State())
		if err != nil {
			log.W(ctx, "Error on Transform on cmd [%v:%v] with transform [%v] : %v", id, inputCmd, transform, err)
			return err
		}

		if transform.RequiresAccurateState() {
			// Melih TODO: Temporary check until we implement accurate state
			panic("Implement accurate state")
		}
	}

	if err = mutateAndWrite(ctx, id, cmds, chain.out); err != nil {
		return err
	}

	for _, transform := range chain.transforms {
		transform.ClearTransformResources(ctx)
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

	err := chain.transformCommand(ctx, currentCommand, chain.currentCommandID)
	if err != nil {
		log.E(ctx, "Replay error (%v:%v): %v", chain.currentCommandID, currentCommand, err)
	}
	chain.currentCommandID++
	return err
}

func (chain *TransformChain) handleInitialState(state *api.GlobalState) (*api.GlobalState, error) {
	for _, t := range chain.transforms {
		if t.RequiresAccurateState() {
			// Melih TODO: Temporary check until we implement accurate state
			panic("Implement accurate state")
		}
	}

	return state, nil
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
