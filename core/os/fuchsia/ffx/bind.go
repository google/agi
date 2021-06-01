// Copyright (C) 2021 Google Inc.
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

package ffx

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/gapid/core/os/device/bind"
	"github.com/google/gapid/core/os/file"
	"github.com/google/gapid/core/os/fuchsia"
	"github.com/google/gapid/core/os/shell"
)

// binding represents an attached Fuchsia device.
type binding struct {
	bind.Simple
}

// verify that binding implements Device
var _ fuchsia.Device = (*binding)(nil)

// FFX is the path to the ffx executable, or an empty string if the ffx
// executable was not found.
var FFX file.Path

func ffx() (file.Path, error) {
	if !FFX.IsEmpty() {
		return FFX, nil
	}

	exe := "ffx"

	search := []string{exe}

	// If FUCHSIA_DIR is set, build a fully rooted path
	// We still want to call LookPath to pick up the extension and check the binary exists
	if home := os.Getenv("FUCHSIA_DIR"); home != "" {
		search = append(search, filepath.Join(home, "tools", "devshell", "contrib", exe))
	}

	for _, path := range search {
		if p, err := file.FindExecutable(path); err == nil {
			FFX = p
			return FFX, nil
		}
	}

	return file.Path{}, fmt.Errorf("ffx could not be found from FUCHSIA_DIR or PATH\n"+
		"FUCHSIA_DIR: %v\n"+
		"PATH: %v\n"+
		"search: %v",
		os.Getenv("FUCHSIA_DIR"), os.Getenv("PATH"), search)
}

func (b *binding) Command(name string, args ...string) shell.Cmd {
	return shell.Command(name, args...).On(deviceTarget{b})
}

func (b *binding) prepareFFXCommand(cmd shell.Cmd) (shell.Process, error) {
	exe, err := ffx()
	if err != nil {
		return nil, err
	}
	// Adjust the command to: "ffx -t <target> <cmd.Name> <cmd.Args...>"
	old := cmd.Args
	cmd.Args = make([]string, 0, len(old)+4)
	cmd.Args = append(cmd.Args, "-t", b.To.Serial)
	cmd.Args = append(cmd.Args, cmd.Name)
	cmd.Args = append(cmd.Args, old...)
	cmd.Name = exe.System()
	// And delegate to the normal local target
	return shell.LocalTarget.Start(cmd)
}

type deviceTarget struct{ b *binding }

func (t deviceTarget) Start(cmd shell.Cmd) (shell.Process, error) {
	return t.b.prepareFFXCommand(cmd)
}

func (t deviceTarget) String() string {
	return "command:" + t.b.String()
}
