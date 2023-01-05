// Copyright (C) 2018 Google Inc.
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

package validate

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/perfetto"
	perfetto_service "github.com/google/gapid/gapis/perfetto/service"
	"github.com/google/gapid/gapis/service"
)

const (
	counterIdQuery     = "select id from gpu_counter_track where name = '%v'"
	counterValuesQuery = "" +
		"select value from counter " +
		"where track_id = %v order by ts " +
		"limit %v offset 10"
	trackIDQuery           = "select id from gpu_track where scope = '%v'"
	renderStageTrackScope  = "gpu_render_stage"
	vulkanEventsTrackScope = "vulkan_events"
	renderStageSlicesQuery = "" +
		"select name, command_buffer, submission_id " +
		"from gpu_slice " +
		"where track_id = %v " +
		"order by id"
	vulkanEventSlicesQuery = "" +
		"select name, submission_id " +
		"from gpu_slice " +
		"where track_id = %v " +
		"order by id"
	sampleCounter = 100
)

type Scope string

// Checker is a function that checks the validity of the values of the given result set column.
type Checker func(column *perfetto_service.QueryResult_ColumnValues, columnType perfetto_service.QueryResult_ColumnDesc_Type) bool

// GpuCounter represents a GPU counter for which the profiling data is validated.
type GpuCounter struct {
	Id    uint32
	Name  string
	Check Checker
}

// Validator is an interface implemented by the various hardware specific validators.
type Validator interface {
	Validate(ctx context.Context, processor *perfetto.Processor) error
	GetCounters() []GpuCounter
	GetType() service.DeviceValidationResult_ValidatorType
}

// And returns a checker that is only valid if all of its arguments are.
func And(cs ...Checker) Checker {
	return func(column *perfetto_service.QueryResult_ColumnValues, columnType perfetto_service.QueryResult_ColumnDesc_Type) bool {
		for _, c := range cs {
			if !c(column, columnType) {
				return false
			}
		}
		return true
	}
}

// Not returns a checker that returns the inverse of the given checker.
func Not(c Checker) Checker {
	return func(column *perfetto_service.QueryResult_ColumnValues, columnType perfetto_service.QueryResult_ColumnDesc_Type) bool {
		return !c(column, columnType)
	}
}

// IsNumber is a checker that checks that the column is a number type.
func IsNumber(column *perfetto_service.QueryResult_ColumnValues, columnType perfetto_service.QueryResult_ColumnDesc_Type) bool {
	if columnType != perfetto_service.QueryResult_ColumnDesc_LONG && columnType != perfetto_service.QueryResult_ColumnDesc_DOUBLE {
		return false
	}
	return true
}

func ForeachValue(check func(float64) bool) Checker {
	return func(column *perfetto_service.QueryResult_ColumnValues, columnType perfetto_service.QueryResult_ColumnDesc_Type) bool {
		longValues := column.GetLongValues()
		doubleValues := column.GetDoubleValues()
		for i := 0; i < sampleCounter; i++ {
			if columnType == perfetto_service.QueryResult_ColumnDesc_LONG {
				if !check(float64(longValues[i])) {
					return false
				}
			} else if columnType == perfetto_service.QueryResult_ColumnDesc_DOUBLE {
				if !check(doubleValues[i]) {
					return false
				}
			}
		}
		return true
	}
}

// Average returns a checker that checks whether the average values meets the condition
func Average(check func(float64) bool) Checker {
	return func(column *perfetto_service.QueryResult_ColumnValues, columnType perfetto_service.QueryResult_ColumnDesc_Type) bool {
		longValues := column.GetLongValues()
		doubleValues := column.GetDoubleValues()
		total := float64(0.0)
		for i := 0; i < sampleCounter; i++ {
			if columnType == perfetto_service.QueryResult_ColumnDesc_LONG {
				total += float64(longValues[i])
			} else if columnType == perfetto_service.QueryResult_ColumnDesc_DOUBLE {
				total += doubleValues[i]
			}
		}
		return check(total / sampleCounter)
	}
}

// CheckLargerThanZero is a checker that checks that the values are all greater than zero.
func CheckLargerThanZero() Checker {
	return ForeachValue(func(v float64) bool {
		return v > 0.0
	})
}

// CheckNonNegative is a checker that checks that no value is less than zero.
func CheckNonNegative() Checker {
	return ForeachValue(func(v float64) bool {
		return v >= 0.0
	})
}

// CheckAllEqualTo returns a checker that checks that all returned value equal the given value.
func CheckAllEqualTo(num float64) Checker {
	return ForeachValue(func(v float64) bool {
		return v == num
	})
}

// CheckAverageApproximateTo checks whether the average of the values is within
// a margin of the given value.
func CheckAverageApproximateTo(num, margin float64) Checker {
	return Average(func(v float64) bool {
		return math.Abs(num-v) <= margin
	})
}

// ValidateGpuCounters queries for the GPU counter from the trace and
// validates the value based on the associated check,
// up to the amount specified in passThreshold.
func ValidateGpuCounters(ctx context.Context, processor *perfetto.Processor, counters []GpuCounter, passThreshold int) error {
	passCount := 0
	var failedCounters []GpuCounter
	var missingCounters []GpuCounter
	counters = trimDuplicates(counters)
	if passThreshold > len(counters) {
		log.E(ctx, "Received passThreshold (%v) higher than amount of counters (%v)", passThreshold, len(counters))
		passThreshold = len(counters)
	}

	for _, counter := range counters {
		// Converts counter name to track table ID.
		counterIdResult, err := processor.Query(fmt.Sprintf(counterIdQuery, counter.Name))
		if err != nil {
			return log.Errf(ctx, err, "Failed to query with %v", fmt.Sprintf(counterIdQuery, counter.Name))
		}

		// Ensure that there's one track table ID for the current counter.
		counterIdValues := counterIdResult.GetColumns()[0].GetLongValues()
		if len(counterIdValues) == 0 {
			log.E(ctx, "Trace does not contain expected counter %v", counter)
			missingCounters = append(missingCounters, counter)
			continue
		} else if len(counterIdValues) != 1 {
			// We should never run into situation where there are more than 1 results.
			return log.Errf(ctx, nil, "Found unexpected %v results for counter %v", len(counterIdValues), counter)
		}
		trackTableId := counterIdValues[0]

		valueQueryResult, err := processor.Query(fmt.Sprintf(counterValuesQuery, trackTableId, sampleCounter))
		if err != nil {
			return log.Errf(ctx, err, "Failed to query with %v for counter %v", fmt.Sprintf(counterValuesQuery, trackTableId), counter)
		}

		// Query exactly #sampleCounter samples, fail if not enough samples
		if valueQueryResult.GetNumRecords() != sampleCounter {
			log.E(ctx, "Expected %v number of samples for counter %v but got: %v", sampleCounter, counter, valueQueryResult.GetNumRecords())
			failedCounters = append(failedCounters, counter)
			continue
		}

		if counter.Check(valueQueryResult.GetColumns()[0], valueQueryResult.GetColumnDescriptors()[0].GetType()) {
			passCount++
			if passCount >= passThreshold {
				return nil
			}
		} else {
			failedCounters = append(failedCounters, counter)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Passed %v checks out of %v, expected to pass %v check(s). ", passCount, len(counters), passThreshold))

	if len(missingCounters) > 0 {
		sb.WriteString(fmt.Sprintf("Missing %v counter(s): %v\n", len(missingCounters), missingCounters))
	}
	if len(failedCounters) > 0 {
		sb.WriteString(fmt.Sprintf("Failed check for %v counter(s): %v", len(failedCounters), failedCounters))
	}

	return log.Errf(ctx, nil, sb.String())
}

// trimDuplicates removes duplicates from the GPU counter array.
func trimDuplicates(counters []GpuCounter) []GpuCounter {
	duplicateMap := make(map[int]GpuCounter)
	res := make([]GpuCounter, 0)
	for _, counter := range counters {
		duplicateMap[int(counter.Id)] = counter
	}

	// Sort the ids to retrieve from the map.
	ids := make([]int, 0, len(res))
	for id := range duplicateMap {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		res = append(res, duplicateMap[id])
	}

	return res
}

// GetTrackIDs returns all track ids from gpu_track with the given scope.
func GetTrackIDs(ctx context.Context, s Scope, processor *perfetto.Processor) ([]int64, error) {
	queryResult, err := processor.Query(fmt.Sprintf(trackIDQuery, s))
	if err != nil || queryResult.GetNumRecords() <= 0 {
		return []int64{}, log.Errf(ctx, err, "Failed to query track ids with scope: %v", s)
	}
	result := make([]int64, queryResult.GetNumRecords())
	for i, v := range queryResult.GetColumns()[0].GetLongValues() {
		result[i] = v
	}
	return result, nil
}

// ValidateGpuSlices validates gpu slices, returns nil if all validation passes.
func ValidateGpuSlices(ctx context.Context, processor *perfetto.Processor) error {
	tIds, err := GetTrackIDs(ctx, renderStageTrackScope, processor)
	if err != nil {
		return err
	}
	for _, tId := range tIds {
		queryResult, err := processor.Query(fmt.Sprintf(renderStageSlicesQuery, tId))
		if err != nil {
			return log.Errf(ctx, err, "Failed to query with %v", fmt.Sprintf(renderStageSlicesQuery, tId))
		}
		numRecords := queryResult.GetNumRecords()
		if numRecords == 0 {
			log.W(ctx, "No GPU activity slices found in GPU track: %v", tId)
			continue
		}
		columns := queryResult.GetColumns()
		names := columns[0].GetStringValues()
		commandBuffers := columns[1].GetLongValues()
		submissionIds := columns[2].GetLongValues()
		for i := uint64(0); i < numRecords; i++ {
			if commandBuffers[i] == 0 {
				return log.Errf(ctx, nil, "GPU activity slice %v has null command buffer", names[i])
			}
			if submissionIds[i] == 0 {
				return log.Errf(ctx, nil, "GPU activity slice %v has null submission id", names[i])
			}
		}
	}
	return nil
}

// ValidateVulkanEvents validates vulkan events slices, returns nil if all validation passes.
func ValidateVulkanEvents(ctx context.Context, processor *perfetto.Processor) error {
	tIds, err := GetTrackIDs(ctx, vulkanEventsTrackScope, processor)
	if err != nil {
		return err
	}
	for _, tId := range tIds {
		queryResult, err := processor.Query(fmt.Sprintf(vulkanEventSlicesQuery, tId))
		if err != nil || queryResult.GetNumRecords() <= 0 {
			return log.Errf(ctx, err, "Failed to query with %v", fmt.Sprintf(vulkanEventSlicesQuery, tId))
		}
		columns := queryResult.GetColumns()
		numRecords := queryResult.GetNumRecords()
		names := columns[0].GetStringValues()
		submissionIds := columns[1].GetLongValues()
		if numRecords == 0 {
			return log.Err(ctx, nil, "No GPU activity slices")
		}
		for i := uint64(0); i < numRecords; i++ {
			if names[i] == "vkQueueSubmit" && submissionIds[i] == 0 {
				return log.Errf(ctx, nil, "Vulkan event slice %v has null submission id", names[i])
			}
		}
	}
	return nil
}
