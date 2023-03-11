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

package fuchsia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/gapid/core/app"
	"github.com/google/gapid/core/event/task"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device/bind"
	"github.com/google/gapid/core/os/file"
	"github.com/google/gapid/core/os/fuchsia"
	"github.com/google/gapid/core/os/fuchsia/ffx"
	"github.com/google/gapid/core/os/shell"
	gapii "github.com/google/gapid/gapii/client"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
	"github.com/google/gapid/gapis/trace/tracer"
)

type traceSession struct {
	device  fuchsia.Device
	options *service.TraceOptions
}

// Capture connects to this trace and waits for a capture to be delivered.
// It copies the capture into the supplied writer.
// If the process was started with the DeferStart flag, then tracing will wait
// until start is fired.
// Capturing will stop when the stop signal is fired (clean stop) or the
// context is cancelled (abort).

func (s *traceSession) Capture(ctx context.Context, start task.Signal, stop task.Signal, ready task.Task, w io.Writer, written *int64) (size int64, err error) {
	// Create trace file.
	traceFile, err := file.Temp()
	if err != nil {
		return 0, log.Err(ctx, err, "Trace Temp file creation")
	}
	defer file.Remove(traceFile)

	// Signal that we are ready to start.
	atomic.StoreInt64(written, 1)

	// Verify defer start option.
	if s.options.DeferStart && !start.Wait(ctx) {
		return 0, log.Err(ctx, nil, "Trace Cancelled")
	}

	// Initiate tracing.
	if err := s.device.StartTrace(ctx, s.options, traceFile, stop, ready); err != nil {
		return 0, err
	}

	// Wait for capture to stop.
	duration := time.Duration(float64(s.options.Duration) * float64(time.Second))
	if duration > 0 {
		stop.TryWait(ctx, duration)
	} else {
		stop.Wait(ctx)
	}

	// Stop tracing.
	if err := s.device.StopTrace(ctx, traceFile); err != nil {
		return 0, err
	}

	// Copy trace file contents to output variables.
	traceFileSize := traceFile.Info().Size()
	atomic.StoreInt64(written, traceFileSize)
	fh, err := os.Open(traceFile.System())
	if err != nil {
		return 0, log.Err(ctx, err, fmt.Sprintf("Failed to open %s", traceFile))
	}

	return io.Copy(w, fh)
}

type fuchsiaTracer struct {
	device fuchsia.Device
}

// TraceConfiguration returns the device's supported trace configuration.
func (t *fuchsiaTracer) TraceConfiguration(ctx context.Context) (*service.DeviceTraceConfiguration, error) {
	return &service.DeviceTraceConfiguration{
		Types:                []*service.TraceTypeCapabilities{tracer.FuchsiaTraceOptions(), tracer.VulkanTraceOptions()},
		ServerLocalPath:      false,
		CanSpecifyCwd:        true,
		CanUploadApplication: false,
		CanSpecifyEnv:        true,
		PreferredRootUri:     "",
		HasCache:             false,
	}, nil
}

type traceableComponent struct {
	URI string
}

func (c *traceableComponent) ApplicationName(ctx context.Context) (string, error) {
	tokens := strings.Split(c.URI, " ")
	if len(tokens) != 3 {
		msg := "Invalid component uri"
		log.E(ctx, msg+". Expecting 3 tokens: <global_id> <koid> <process_name>")
		return "", errors.New(msg)
	}
	return tokens[2], nil
}

// GetTraceTargetNode returns a TraceTargetTreeNode for the given URI
// on the device.
func (t *fuchsiaTracer) GetTraceTargetNode(ctx context.Context, uri string, iconDensity float32) (*tracer.TraceTargetTreeNode, error) {
	if uri == "" {
		exe, err := ffx.Ffx()
		if err != nil {
			return nil, err
		}
		stdout, err := shell.Command(exe.System(), "agis", "vtcs").Call(ctx)
		children, err := ParseComponents(ctx, stdout)
		if err != nil {
			return nil, err
		}
		r := &tracer.TraceTargetTreeNode{}
		for _, child := range children {
			r.Children = append(r.Children, child)
		}
		sort.Strings(r.Children)
		return r, nil
	}
	component := traceableComponent{URI: uri}
	componentName, err := component.ApplicationName(ctx)
	if err != nil {
		return nil, err
	}
	return &tracer.TraceTargetTreeNode{
		Name:            uri,
		Icon:            nil,
		URI:             uri,
		TraceURI:        uri,
		Children:        nil,
		Parent:          "",
		ApplicationName: componentName,
		ExecutableName:  componentName}, nil
}

// FindTraceTargets finds TraceTargetTreeNodes for a given search string on
// the device.
func (t *fuchsiaTracer) FindTraceTargets(ctx context.Context, uri string) ([]*tracer.TraceTargetTreeNode, error) {
	log.E(ctx, "FindTraceTargets is returning nil")
	return nil, nil
}

// SetupTrace starts the application on the device, and causes it to wait
// for the trace to be started. It returns the process that was created, as
// well as a function that can be used to clean up the device.
func (t *fuchsiaTracer) SetupTrace(ctx context.Context, o *service.TraceOptions) (tracer.Process, app.Cleanup, error) {
	session := &traceSession{
		device:  t.device,
		options: o,
	}
	switch traceType := o.GetType(); traceType {
	case service.TraceType_Graphics:
		fuchsiaTraceConfig := o.GetFuchsiaTraceConfig()
		globalId := strconv.FormatInt(int64(fuchsiaTraceConfig.GetGlobalId()), 10)
		path := "/tmp/agis" + globalId
		log.I(ctx, "SetupTrace: Going to Listen on Path: "+path)

		listener, err := net.Listen("unix", path)

		if err != nil {
			log.E(ctx, "SetupTrace: failed to create unix pipe for ffx")
			return nil, nil, err
		} else {
			log.I(ctx, "SetupTrace: Listening on /tmp/agis"+globalId)
		}
		cmd := t.device.Command("agis", "listen", globalId)
		log.I(ctx, "SetupTrace: Command: "+cmd.Name)
		for _, arg := range cmd.Args {
			log.I(ctx, "\targ: "+arg)
		}

		result, err := cmd.Call(ctx)

		if err != nil {
			log.E(ctx, "SetupTrace: ffx agis listen command failed.  Result: "+result)
			log.E(ctx, "\tError: "+err.Error())
			return nil, nil, err
		}
		log.I(ctx, "SetupTrace: FFX command result: "+result)

		conn, err := listener.Accept()

		if err != nil {
			log.E(ctx, "SetupTrace: Accept failed")
		} else {
			log.I(ctx, "SetupTrace: Accept succeeded")
		}
		var cleanup app.Cleanup
		process := &gapii.Process{Conn: conn, Device: t.device, Options: tracer.GapiiOptions(o)}
		return process, cleanup, nil
	case service.TraceType_Fuchsia:
		log.I(ctx, "SetupTrace, TRACE TYPE: Fuchsia")
	default:
		log.E(ctx, "SetupTrace, TRACE TYPE: UNKNOWN")
		return nil, nil, errors.New("Unrecognized Fuchsia trace type")
	}
	return session, nil, nil
}

// GetDevice returns the device associated with this tracer.
func (t *fuchsiaTracer) GetDevice() bind.Device {
	return t.device
}

// ProcessProfilingData takes a buffer for a Perfetto trace and translates it into
// a ProfilingData.
func (t *fuchsiaTracer) ProcessProfilingData(ctx context.Context, buffer *bytes.Buffer,
	capture *path.Capture, staticAnalysisResult chan *api.StaticAnalysisProfileData,
	handleMapping map[uint64][]service.VulkanHandleMappingItem, syncData *sync.Data) (*service.ProfilingData, error) {

	<-staticAnalysisResult // Ignore the static analysis result.
	log.E(ctx, "ProcessProfilingData is returning nil")
	return nil, nil
}

// Validate validates the GPU profiling capabilities of the given device and returns
// an error if validation failed or the GPU profiling data is invalid.
func (t *fuchsiaTracer) Validate(ctx context.Context, enableLocalFiles bool) (*service.DeviceValidationResult, error) {
	return &service.DeviceValidationResult{
		ErrorCode: service.DeviceValidationResult_OK,
	}, nil
}

func NewTracer(d bind.Device) tracer.Tracer {
	return &fuchsiaTracer{device: d.(fuchsia.Device)}
}

type VtcJSON struct {
	GlobalID    int    `json:"global_id"`
	ProcessKoid int64  `json:"process_koid"`
	ProcessName string `json:"process_name"`
}

// ParseComponents parses the vulkan traceable component list returned from `ffx agis vtcs“.
func ParseComponents(ctx context.Context, stdout string) ([]string, error) {
	var vtcs []VtcJSON
	if err := json.Unmarshal([]byte(stdout), &vtcs); err != nil {
		if stdout == "{}" || stdout == "[]" {
			msg := "No Vulkan traceable components found"
			log.E(ctx, msg+" in \""+stdout+"\"")
			return nil, errors.New(msg)
		}
		return nil, err
	}
	components := []string{}
	for _, vtc := range vtcs {
		components = append(components, strconv.Itoa(vtc.GlobalID)+" "+
			strconv.FormatInt(int64(vtc.ProcessKoid), 10)+" "+vtc.ProcessName)
	}

	return components, nil
}
