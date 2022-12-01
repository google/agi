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

package powervr

import (
	"context"
	"strings"

	"github.com/google/gapid/core/os/device/bind"
	"github.com/google/gapid/gapis/perfetto"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/trace/android/validate"
)

var (
	// Counters available on GPUs based on the PowerVR Rogue architecture.
	rogueCounters = []validate.GpuCounter{
		{0, "Triangles In", counterCheckerHaveNonZero()},
		{1, "Triangles In per Draw", counterCheckerHaveNonZero()},
		{2, "Triangles Out", counterCheckerHaveNonZero()},
		{3, "Triangle Ratio", counterCheckerHaveNonZero()},
		{4, "Vertex Sharing", counterCheckerHaveNonZero()},
		{5, "Tiles per Triangle", counterCheckerHaveNonZero()},
		{6, "Geometry Load", counterCheckerHaveNonZero()},
		{7, "HSR Efficiency", counterCheckerNonNegative()},
		{8, "Overdraw", counterCheckerNonNegative()},
		{9, "ISP Pixel Load", counterCheckerHaveNonZero()},
		{10, "Z Load/Store", counterCheckerNonNegative()},
		{11, "ISP Throughput", counterCheckerHaveNonZero()},
		{12, "Pixels Out", counterCheckerHaveNonZero()},
		{13, "PBE Load", counterCheckerHaveNonZero()},
		{14, "Vertex Shader Load", counterCheckerHaveNonZero()},
		{15, "Fragment Shader Load", counterCheckerHaveNonZero()},
		{16, "Compute Shader Load", counterCheckerNonNegative()},
		{17, "Shaded Vertices", counterCheckerHaveNonZero()},
		{18, "Shaded Fragments", counterCheckerHaveNonZero()},
		{19, "Processed Kernels", counterCheckerNonNegative()},
		{20, "Cycles per Vertex", counterCheckerHaveNonZero()},
		{21, "Cycles per Fragment", counterCheckerHaveNonZero()},
		{22, "Cycles per Kernel", counterCheckerNonNegative()},
		{23, "Register Overload", counterCheckerNonNegative()},
		{24, "Texture Unit Active", counterCheckerNonNegative()},
		{25, "Texture Unit Overload", counterCheckerNonNegative()},
		{26, "GPU Memory Interface Load", counterCheckerHaveNonZero()},
		{27, "GPU Clock Speed", counterCheckerHaveNonZero()},
	}

	// Counters available on GPUs based on the PowerVR Series A/B/C & Furian architectures.
	seriesCounters = []validate.GpuCounter{
		{0, "Triangles In", counterCheckerHaveNonZero()},
		{1, "Triangles Out", counterCheckerHaveNonZero()},
		{2, "Triangle Ratio", counterCheckerHaveNonZero()},
		{3, "Vertex Sharing", counterCheckerHaveNonZero()},
		{4, "HSR Efficiency", counterCheckerNonNegative()},
		{5, "ISP Pixel Load", counterCheckerHaveNonZero()},
		{6, "ISP Throughput", counterCheckerHaveNonZero()},
		{7, "Vertex Shader Load", counterCheckerHaveNonZero()},
		{8, "Fragment Shader Load", counterCheckerHaveNonZero()},
		{9, "Compute Shader Load", counterCheckerNonNegative()},
		{10, "Shaded Vertices", counterCheckerHaveNonZero()},
		{11, "Shaded Fragments", counterCheckerHaveNonZero()},
		{12, "Processed Kernels", counterCheckerNonNegative()},
		{13, "Cycles per Vertex", counterCheckerHaveNonZero()},
		{14, "Cycles per Fragment", counterCheckerHaveNonZero()},
		{15, "Cycles per Kernel", counterCheckerNonNegative()},
		{16, "Register Overload: Vertex", counterCheckerNonNegative()},
		{17, "Register Overload: Fragment", counterCheckerNonNegative()},
		{18, "Register Overload: Compute", counterCheckerNonNegative()},
		{19, "Texture Unit Active", counterCheckerNonNegative()},
		{20, "Texture Samples per Fragment", counterCheckerNonNegative()},
		{21, "Cycles per Texture Sample", counterCheckerNonNegative()},
		{22, "Texture Read Stall", counterCheckerNonNegative()},
		{23, "GPU Memory Interface Load", counterCheckerHaveNonZero()},
		{24, "GPU Clock Speed", counterCheckerHaveNonZero()},
	}
)

func counterCheckerNonNegative() validate.Checker {
	return validate.And(validate.IsNumber, validate.CheckNonNegative())
}

func counterCheckerHaveNonZero() validate.Checker {
	return validate.And(validate.IsNumber, validate.Not(validate.CheckAllEqualTo(0)))
} 

type PowerVRValidator struct {
	counters []validate.GpuCounter
	countersPassThreshold int
}

func NewPowerVRValidator(device bind.Device) *PowerVRValidator {
	devConf := device.Instance().GetConfiguration()
	gpuName := devConf.GetHardware().GetGPU().GetName()

	if strings.Contains(gpuName, "GX6250") ||
		strings.Contains(gpuName, "GX6450") ||
		strings.Contains(gpuName, "GX6650") ||
		strings.Contains(gpuName, "GE8300") ||
		strings.Contains(gpuName, "GE8310") ||
		strings.Contains(gpuName, "GE8320") ||
		strings.Contains(gpuName, "GE8322") ||
		strings.Contains(gpuName, "GE8430") ||
		strings.Contains(gpuName, "GM9445") ||
		strings.Contains(gpuName, "GM9446") ||
		strings.Contains(gpuName, "G6110") ||
		strings.Contains(gpuName, "G6200") ||
		strings.Contains(gpuName, "G6400") {
		return &PowerVRValidator{rogueCounters, len(rogueCounters)}
	} else if strings.Contains(gpuName, "GT9524") ||
		strings.Contains(gpuName, "BXM-8-256") {
		return &PowerVRValidator{seriesCounters, len(seriesCounters)}
	} else {
		// Fallback in case we encounter a device that the checks above don't yet account for.
		specs := devConf.GetPerfettoCapability().GetGpuProfiling().GetGpuCounterDescriptor().GetSpecs()
		counters := make([]validate.GpuCounter, len(specs))

		for index, spec := range specs {
			counters[index] = validate.GpuCounter{spec.GetCounterId(), spec.GetName(), counterCheckerHaveNonZero()}
		}

		// Loose check to make sure that at least one counter value is non-zero.
		return &PowerVRValidator{counters, 1}
	}
}

func (v *PowerVRValidator) Validate(ctx context.Context, processor *perfetto.Processor) error {
	if err := validate.ValidateGpuCounters(ctx, processor, v.GetCounters(), v.countersPassThreshold); err != nil {
		return err
	}
	if err := validate.ValidateGpuSlices(ctx, processor); err != nil {
		return err
	}
	if err := validate.ValidateVulkanEvents(ctx, processor); err != nil {
		return err
	}

	return nil
}

func (v *PowerVRValidator) GetCounters() []validate.GpuCounter {
	return v.counters
}

func (v *PowerVRValidator) GetType() service.DeviceValidationResult_ValidatorType {
	return service.DeviceValidationResult_POWERVR
}
