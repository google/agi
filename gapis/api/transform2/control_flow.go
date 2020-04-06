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
	"fmt"
	"strings"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/command_generator"
	"github.com/google/gapid/gapis/config"
)

type ControlFlow struct {
	transforms       []Transform
	commandGenerator command_generator.CommandGenerator
	tag              string
	out              Writer
}

func NewControlFlow(tag string, commandGenerator command_generator.CommandGenerator, out Writer) *ControlFlow {
	return &ControlFlow{
		transforms:       make([]Transform, 0),
		commandGenerator: commandGenerator,
		out:              out,
		tag:              tag,
	}
}

func (cf *ControlFlow) AddTransform(transforms ...Transform) {
	cf.transforms = append(cf.transforms, transforms...)
}

func (cf *ControlFlow) TransformAll(ctx context.Context) error {
	if config.LogTransformsToFile {
		newTransforms := make([]Transform, 0)
		newTransforms = append(newTransforms, NewFileLog(ctx, "0_original_cmds"))
		for i, t := range cf.transforms {
			var name string
			if n, ok := t.(interface {
				Name() string
			}); ok {
				name = n.Name()
			} else {
				name = strings.Replace(fmt.Sprintf("%T", t), "*", "", -1)
			}
			newTransforms = append(newTransforms, t, NewFileLog(ctx, fmt.Sprintf("%v_cmds_after_%v", i+1, name)))
		}
		cf.transforms = newTransforms
	}

	chain := CreateTransformChain(cf.out, cf.transforms)

	err := chain.BeginChain(ctx)
	if err != nil {
		log.E(ctx, "[%v] Error on beginning transformations: %v", cf.tag, err)
	}

	currentCommandID := api.CmdID(0)
	currentCommand := cf.commandGenerator.GetNextCommand(ctx)

	for currentCommand != nil {
		if config.DebugReplay {
			log.I(ctx, "[%v] Transforming... (%v:%v)", cf.tag, currentCommandID, currentCommand)
		}

		err := chain.TransformCommand(ctx, currentCommand, currentCommandID)
		if err != nil {
			log.E(ctx, "[%v] Replay error (%v:%v): %v", cf.tag, currentCommandID, currentCommand, err)
		}

		currentCommand = cf.commandGenerator.GetNextCommand(ctx)
		currentCommandID++
	}

	err = chain.EndChain(ctx)
	if err != nil {
		log.E(ctx, "[%v] Error on beginning transformations: %v", cf.tag, err)
	}

	return nil
}
