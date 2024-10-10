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

package mali

import (
	"context"
	"strconv"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/perfetto"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/trace/android/validate"
)

var (
	// GPU counters that are guaranteed to be found on Mali GPUs.
	counters = []validate.GpuCounter{
		{65536, "GPU utilization", counterChecker()},
	}
)

func GetDriverMajorVersion(driverVersion uint32) uint32 {
	return ((driverVersion >> 22) & 0x7F)
}

func GetDriverMinorVersion(driverVersion uint32) uint32 {
	return ((driverVersion >> 12) & 0x3FF)
}

func counterChecker() validate.Checker {
	return validate.And(validate.IsNumber, validate.CheckNonNegative(), validate.Not(validate.CheckAllEqualTo(0)))
}

type MaliValidator struct {
	gpuName       string
	driverVersion uint32
}

func NewMaliValidator(gpuName string, driverVersion uint32) *MaliValidator {
	return &MaliValidator{gpuName, driverVersion}
}

func (v *MaliValidator) Validate(ctx context.Context, processor *perfetto.Processor) error {
	driverType := "prod"
	if (v.driverVersion & 0xFFF) != 0 {
		driverType = "dev"
	}
	log.I(ctx, "Validating (%v) with %v driver %v.%v(0x%v)",
		v.gpuName,
		driverType,
		GetDriverMajorVersion(v.driverVersion),
		GetDriverMinorVersion(v.driverVersion),
		strconv.FormatInt(int64(v.driverVersion), 16))
	if err := validate.ValidateGpuCounters(ctx, processor, v.GetCounters(), len(v.GetCounters())); err != nil {
		return err
	}
	if err := validate.ValidateGpuSlices(ctx, processor); err != nil {
		return err
	}

	return nil
}

func (v *MaliValidator) GetCounters() []validate.GpuCounter {
	return counters
}

func (v *MaliValidator) GetType() service.DeviceValidationResult_ValidatorType {
	return service.DeviceValidationResult_MALI
}
