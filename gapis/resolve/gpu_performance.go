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

package resolve

import (
	"context"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

func GpuPerformance(ctx context.Context, p *path.GpuPerformance, rc *path.ResolveConfig) (*service.GpuPerformance, error) {
	// Retrive the crude gpu performance data from database.
	obj, err := database.Resolve(ctx, p.ID.ID())
	if err != nil {
		return nil, log.Err(ctx, err, "Error resolving Command GPU Performance.")
	}
	crudePerf, ok := obj.(*service.GpuCrudePerformance)
	if !ok {
		return nil, log.Err(ctx, err, "Error resolving Command GPU Performance.")
	}

	perfForCmds := &service.GpuPerformance{
		Result: make(map[uint32]float64),
	}

	cmdsRange := p.Range
	metricIdToOp := make(map[uint32]service.GpuPerformanceMetadata_AggregationOperator)
	for _, metric := range crudePerf.Metadata.Metrics {
		metricIdToOp[metric.Id] = metric.Op
	}

	for metricId, op := range metricIdToOp {
		if op == service.GpuPerformanceMetadata_Sum {
			perfForCmds.Result[metricId] = aggregateBySum(crudePerf, cmdsRange, metricId)
		} else if op == service.GpuPerformanceMetadata_TimeWeightedAvg {
			perfForCmds.Result[metricId] = aggregateByTimeWeightedAvg(crudePerf, cmdsRange, metricId)
		}
	}

	return perfForCmds, nil
}

// Calculate the specified metric's performance for a command range, the
// aggregation method is summation.
func aggregateBySum(crudePerf *service.GpuCrudePerformance, cmdsRange *path.Commands, metricId uint32) float64 {
	result := float64(0)
	for i := range crudePerf.Perfs {
		if contains(cmdsRange, crudePerf.Commands[i]) {
			result += crudePerf.Perfs[i].Result[metricId]
		}
	}
	return result
}

// Calculate the specified metric's performance for a command range, the
// aggregation method is time-weighted average.
func aggregateByTimeWeightedAvg(crudePerf *service.GpuCrudePerformance, cmdsRange *path.Commands, metricId uint32) float64 {
	accumulatedTime := float64(0)
	accumulatedWeightedValue := float64(0)
	gpuTimeMetricId := uint32(0)
	for i := range crudePerf.Perfs {
		if contains(cmdsRange, crudePerf.Commands[i]) {
			accumulatedTime += crudePerf.Perfs[i].Result[gpuTimeMetricId]
			accumulatedWeightedValue += crudePerf.Perfs[i].Result[gpuTimeMetricId] * crudePerf.Perfs[i].Result[metricId]
		}
	}
	if accumulatedTime == 0 {
		return float64(-1)
	} else {
		return accumulatedWeightedValue / accumulatedTime
	}
}

// Return true if command range `rng` contains `command`.
func contains(rng *path.Commands, command *path.Command) bool {
	return lessOrEqual(rng.From, command.Indices, false) && greaterOrEqual(rng.To, command.Indices, true)
}

// Return true if command index `a` is considered less than or equal to
// command index `b`.
func lessOrEqual(a, b []uint64, open bool) bool {
	for i := 0; i < len(a); i++ {
		if i > len(b) {
			return false
		} else if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return true
}

// Return true if command index `a` is considered greater than or equal to
// command index `b`.
func greaterOrEqual(a, b []uint64, open bool) bool {
	for i := 0; i < len(a); i++ {
		if i > len(b) {
			return true
		} else if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return open || len(a) == len(b)
}
