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
	"context"
	"sync"
	"time"

	"github.com/google/gapid/core/event/task"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/core/os/device/bind"
	"github.com/google/gapid/core/os/fuchsia"
	"github.com/google/gapid/core/os/shell"
)

const (
	// Frequency at which to print scan errors.
	printScanErrorsEveryNSeconds = 120
)

var (
	// cache is a map of device serials to fully resolved bindings.
	cache      = map[string]*binding{}
	cacheMutex sync.Mutex // Guards cache.

	// Registry of all the discovered devices.
	registry = bind.NewRegistry()
)

// DeviceList is a list of devices.
type DeviceList []fuchsia.Device

// Devices returns the list of attached Android devices.
func Devices(ctx context.Context) (DeviceList, error) {
	if err := scanDevices(ctx); err != nil {
		return nil, err
	}
	devs := registry.Devices()
	out := make(DeviceList, len(devs))
	for i, d := range devs {
		out[i] = d.(fuchsia.Device)
	}
	return out, nil
}

// Monitor updates the registry with devices that are added and removed at the
// specified interval. Monitor returns once the context is cancelled.
func Monitor(ctx context.Context, r *bind.Registry, interval time.Duration) error {
	unlisten := registry.Listen(bind.NewDeviceListener(r.AddDevice, r.RemoveDevice))
	defer unlisten()

	for _, d := range registry.Devices() {
		r.AddDevice(ctx, d)
	}

	var lastErrorPrinted time.Time
	for {
		if err := scanDevices(ctx); err != nil {
			if time.Since(lastErrorPrinted).Seconds() > printScanErrorsEveryNSeconds {
				log.E(ctx, "Couldn't scan devices: %v", err)
				lastErrorPrinted = time.Now()
			}
		} else {
			lastErrorPrinted = time.Time{}
		}

		select {
		case <-task.ShouldStop(ctx):
			return nil
		case <-time.After(interval):
		}
	}
}

func newDevice(ctx context.Context, serial string) (*binding, error) {
	d := &binding{
		Simple: bind.Simple{
			To: &device.Instance{
				Serial:        serial,
				Configuration: &device.Configuration{},
			},
			LastStatus: bind.Status_Online,
		},
	}

	// TODO: fill in d.To.Name and d.To.Configuration
	// defined in device.proto

	return d, nil
}

// scanDevices returns the list of attached Fuchsia devices.
func scanDevices(ctx context.Context) error {
	exe, err := ffx()
	if err != nil {
		return log.Err(ctx, err, "")
	}
	stdout, err := shell.Command(exe.System(), "target", "list", "-f", "simple").Call(ctx)
	if err != nil {
		return err
	}
	parsed, err := parseDevices(ctx, stdout)
	if err != nil {
		return err
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for serial, _ := range parsed {
		if _, ok := cache[serial]; !ok {
			device, err := newDevice(ctx, serial)
			if err != nil {
				return err
			}
			cache[serial] = device
			registry.AddDevice(ctx, device)
		}
	}

	// Remove cached results for removed devices.
	for serial, cached := range cache {
		if _, found := parsed[serial]; !found {
			delete(cache, serial)
			registry.RemoveDevice(ctx, cached)
		}
	}

	return nil
}

func parseDevices(ctx context.Context, out string) (map[string]struct{}, error) {
	// TODO: implement.
	devices := map[string]struct{}{
		"fuchsia-b02a-4370-c245": struct{}{},
	}
	return devices, nil
}
