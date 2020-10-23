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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/gapid/core/app"
	"github.com/google/gapid/core/log"
)

type framegraphVerb struct{ FramegraphFlags }

func init() {
	verb := &framegraphVerb{}
	app.AddVerb(&app.Verb{
		Name:      "framegraph",
		ShortHelp: "Create frame graph from capture",
		Action:    verb,
	})
}

func (verb *framegraphVerb) Run(ctx context.Context, flags flag.FlagSet) error {
	if flags.NArg() != 1 {
		app.Usage(ctx, "Exactly one gfx trace file expected, got %d", flags.NArg())
		return nil
	}

	captureFilename := flags.Arg(0)

	client, capture, err := getGapisAndLoadCapture(ctx, verb.Gapis, GapirFlags{}, captureFilename, CaptureFileFlags{})
	if err != nil {
		return err
	}
	defer client.Close()

	log.I(ctx, "Creating frame graph from capture id: %s", capture.ID)

	framegraph, err := client.GetFramegraph(ctx, capture)
	if err != nil {
		return log.Errf(ctx, err, "GetFramegraph(%v)", capture)
	}

	// Print framegraph in DOT format
	dot := "digraph agiFramegraph {\n"
	dot += "label = \"" + captureFilename + "\";\n"
	dot += "labelloc = \"t\";\n"
	dot += "node [fontname = \"Monospace\"];\n"
	for _, node := range framegraph.Nodes {
		dot += fmt.Sprintf("n%v [label=\"%s\"];\n", node.Id, node.Text)
	}
	dot += "\n"
	for _, edge := range framegraph.Edges {
		dot += fmt.Sprintf("n%v -> n%v;\n", edge.Origin, edge.Destination)
	}
	dot += "}\n"

	filePath := verb.Out
	if filePath == "" {
		filePath = "framegraph.dot"
	}

	file, err := os.Create(filePath)
	if err != nil {
		return log.Errf(ctx, err, "Creating file (%v)", filePath)
	}
	defer file.Close()

	bytesWritten, err := fmt.Fprint(file, dot)
	if err != nil {
		return log.Errf(ctx, err, "Error after writing %d bytes to file", bytesWritten)
	}

	return nil
}
