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
)

// TransformChain is the wrapper to run transforms
type transformChain struct {
	transforms []Transform
	out        Writer
}

func CreateTransformChain(out Writer, transforms []Transform) *transformChain {
	return &transformChain{
		transforms: transforms,
		out:        out,
	}
}

func (chain *transformChain) BeginChain(ctx context.Context) error {
	handleInitialState(chain.out.State())

	var err error
	cmds := make([]api.Cmd, 0)

	for _, transform := range chain.transforms {
		cmds, err = transform.BeginTransform(ctx, cmds, chain.out.State())
		if err != nil {
			log.W(ctx, "Begin Transform Error [%v] : %v", transform, err)
		}

		if transform.RequiresAccurateState() {
			err = mutateAndWrite(ctx, 0, cmds, chain.out)
			if err != nil {
				return err
			}

			transform.ClearTransformResources(ctx)
		}
	}

	err = mutateAndWrite(ctx, 0, cmds, chain.out)
	if err != nil {
		return err
	}

	for _, transform := range chain.transforms {
		transform.ClearTransformResources(ctx)
	}

	return nil
}

func (chain *transformChain) EndChain(ctx context.Context) error {
	var err error
	cmds := make([]api.Cmd, 0)

	for _, transform := range chain.transforms {
		cmds, err = transform.EndTransform(ctx, cmds, chain.out.State())
		if err != nil {
			log.W(ctx, "End Transform Error [%v] : %v", transform, err)
		}

		if transform.RequiresAccurateState() {
			err = mutateAndWrite(ctx, 0, cmds, chain.out)
			if err != nil {
				return err
			}

			transform.ClearTransformResources(ctx)
		}
	}

	err = mutateAndWrite(ctx, 0, cmds, chain.out)
	if err != nil {
		return err
	}

	for _, transform := range chain.transforms {
		transform.ClearTransformResources(ctx)
	}

	return nil
}

func (chain *transformChain) TransformCommand(ctx context.Context, inputCmd api.Cmd, id api.CmdID) error {
	var err error
	cmds := []api.Cmd{inputCmd}

	for _, transform := range chain.transforms {
		cmds, err = transform.TransformCommand(ctx, id, cmds, chain.out.State())
		if err != nil {
			log.W(ctx, "Error on Transform on cmd [%v:%v] with transform [%v] : %v", id, inputCmd, transform, err)
		}

		if transform.RequiresAccurateState() {
			err = mutateAndWrite(ctx, id, cmds, chain.out)
			if err != nil {
				return err
			}

			transform.ClearTransformResources(ctx)
		}
	}

	err = mutateAndWrite(ctx, id, cmds, chain.out)
	if err != nil {
		return err
	}

	for _, transform := range chain.transforms {
		transform.ClearTransformResources(ctx)
	}
	return nil
}

func handleInitialState(state *api.GlobalState) (*api.GlobalState, error) {
	// Melih TODO: Currently this is not in use as we do not need the feature yet
	// It will likely to be added in next a few PR.
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
