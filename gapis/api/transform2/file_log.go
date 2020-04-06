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

package transform2

import (
	"context"
	"fmt"
	"os"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/config"
)

type fileLog struct {
	file *os.File
}

// NewFileLog returns a Transformer that will log all commands passed through it
// to the text file at path.
func NewFileLog(ctx context.Context, path string) *fileLog {
	f, err := os.Create(path)
	if err != nil {
		log.E(ctx, "Failed to create replay log file %v: %v", path, err)
		return nil
	}
	return &fileLog{file: f}
}

func (logTransform *fileLog) RequiresAccurateState() bool {
	return false
}

func (logTransform *fileLog) BeginTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	return inputCommands, nil
}

func (logTransform *fileLog) ClearTransformResources(ctx context.Context) {
	// Do nothing
}

func (logTransform *fileLog) EndTransform(ctx context.Context, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	logTransform.file.Close()
	return inputCommands, nil
}

func (logTransform *fileLog) TransformCommand(ctx context.Context, id api.CmdID, inputCommands []api.Cmd, inputState *api.GlobalState) ([]api.Cmd, error) {
	if len(inputCommands) == 0 {
		return inputCommands, nil
	}

	if inputCommands[0].API() != nil {
		logTransform.file.WriteString(fmt.Sprintf("%v: %v\n", id, inputCommands[0]))
	} else {
		logTransform.file.WriteString(fmt.Sprintf("%T\n", inputCommands[0]))
	}

	if config.LogExtrasInTransforms {
		for _, cmd := range inputCommands {
			if extras := cmd.Extras(); extras != nil {
				for _, e := range extras.All() {
					if o, ok := e.(*api.CmdObservations); ok {
						if config.LogMemoryInExtras {
							logTransform.file.WriteString(o.DataString(ctx))
						} else {
							logTransform.file.WriteString(o.String())
						}
					} else {
						logTransform.file.WriteString(fmt.Sprintf("[extra] %T: %v\n", e, e))
					}
				}
			}
		}
	}

	return inputCommands, nil
}
