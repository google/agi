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

package generic

import (
	"context"
	"errors"

	"github.com/google/gapid/core/os/device/bind"
	"github.com/google/gapid/gapis/perfetto"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/trace/android/validate"
)

type GenericValidator struct {
	counters []validate.GpuCounter
}

func NewGenericValidator(device bind.Device) *GenericValidator {
	specs := device.Instance().GetConfiguration().GetPerfettoCapability().GetGpuProfiling().GetGpuCounterDescriptor().GetSpecs()
	counters := make([]validate.GpuCounter, len(specs))

	for index, spec := range specs {
		counters[index] = validate.GpuCounter{spec.GetCounterId(), spec.GetName(), counterChecker()}
	}
	return &GenericValidator{counters}
}

func counterChecker() validate.Checker {
	return validate.And(validate.IsNumber, validate.CheckLargerThanZero())
}

func (v *GenericValidator) Validate(ctx context.Context, processor *perfetto.Processor) error {
	if len(v.GetCounters()) == 0 {
		return errors.New("Unable to query for GPU counters")
	}

	// Loose check to make sure that at least one counter value is non-zero.
	if err := validate.ValidateGpuCounters(ctx, processor, v.GetCounters() /* passThreshold= */, 1); err != nil {
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

func (v *GenericValidator) GetCounters() []validate.GpuCounter {
	return v.counters
}

func (v *GenericValidator) GetType() service.DeviceValidationResult_ValidatorType {
	return service.DeviceValidationResult_GENERIC
}
