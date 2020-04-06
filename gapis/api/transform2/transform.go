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

	"github.com/google/gapid/gapis/api"
)

// Transform is the interface that wraps the basic Transform functionality.
type Transform interface {
	// BeginTransform is called before transforming any command.
	BeginTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error)

	// EndTransform is called after all commands are transformed.
	EndTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error)

	// TransformCommand takes a given command list and a state.
	// It outputs a new set of commands after running the transformation.
	// Transform must not modify cmd(s) in any way.
	TransformCommand(ctx context.Context, id api.CmdID, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error)

	// Melih Note: This will increase our active memory usage.
	// Previously after every transform mutated, we were releasing memory.
	// With the current change, it may run a few transforms until a command mutation
	// So that memory can be released
	ClearTransformResources(ctx context.Context)

	// Returns true if transform needs the observe the accurate state
	RequiresAccurateState() bool
}

// Writer is the interface which consumes the output of an Transformer.
// It also keeps track of state changes caused by all commands written to it.
// Conceptually, each Writer object contains its own separate State object,
// which is modified when MutateAndWrite is called.
// This allows the transform to access the state both before and after the
// mutation of state happens. It is also possible to omit/insert commands.
// In practice, single state object can be shared by all transforms for
// performance (i.e. the mutation is done only once at the very end).
// This potentially allows state changes to leak upstream so care is needed.
// There is a configuration flag to switch between the shared/separate modes.
type Writer interface {
	// State returns the state object associated with this writer.
	State() *api.GlobalState
	// MutateAndWrite mutates the state object associated with this writer,
	// and it passes the command to further consumers.
	MutateAndWrite(ctx context.Context, id api.CmdID, cmd api.Cmd) error
}
