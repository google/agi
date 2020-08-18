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

package gpu_performance

import (
	"context"
	"sort"

	"github.com/google/gapid/core/data/id"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

func ProcessPerformances(ctx context.Context, slices *service.ProfilingData_GpuSlices, counters []*service.ProfilingData_Counter) (*service.GpuPerformanceMetadata, id.ID, error) {
	// Filter out the slices that are at depth 0 and belong to a command,
	// and then sort them based on the start time.
	groupToCmd := make(map[int32]*path.Command)
	for _, group := range slices.Groups {
		groupToCmd[group.Id] = group.Link
	}
	filteredSlices := make([]*service.ProfilingData_GpuSlices_Slice, 0)
	for i := 0; i < len(slices.Slices); i++ {
		if slices.Slices[i].Depth == 0 && groupToCmd[slices.Slices[i].GroupId] != nil {
			filteredSlices = append(filteredSlices, slices.Slices[i])
		}
	}
	sort.Slice(filteredSlices, func(i, j int) bool {
		return filteredSlices[i].Ts < filteredSlices[j].Ts
	})

	// Group slices based on their group id.
	groupToSlices := make(map[int32][]*service.ProfilingData_GpuSlices_Slice)
	for i := 0; i < len(filteredSlices); i++ {
		groupId := filteredSlices[i].GroupId
		groupToSlices[groupId] = append(groupToSlices[groupId], filteredSlices[i])
	}

	// Initialize the data structure for the returned result.
	metadata := &service.GpuPerformanceMetadata{
		// Allocate space for the returned result.
		Metrics: make([]*service.GpuPerformanceMetadata_Metric, 0),
	}
	groupIds := make([]int32, 0)
	commands := make([]*path.Command, 0)
	perfs := make([]*service.GpuPerformance, 0)
	for groupId := range groupToSlices {
		groupIds = append(groupIds, groupId)
		commands = append(commands, groupToCmd[groupId])
		perfs = append(perfs, &service.GpuPerformance{
			Result: make(map[uint32]float64),
		})
	}

	// Calculate GPU Time Performance and GPU Wall Time Performance.
	setTimeMetrics(metadata, groupIds, perfs, groupToSlices)

	// Calculate GPU Counter Performances.
	setGpuCounterMetrics(ctx, metadata, groupIds, perfs, groupToSlices, counters)

	crudePerf := &service.GpuCrudePerformance{
		Metadata: metadata,
		GroupIds: groupIds,
		Commands: commands,
		Perfs:    perfs,
	}

	crudePerfId, err := database.Store(ctx, crudePerf)
	if err != nil {
		log.Err(ctx, err, "Failed to store crude gpu performance data into database.")
	}

	return metadata, crudePerfId, nil
}

func setTimeMetrics(metadata *service.GpuPerformanceMetadata, groupIds []int32, perfs []*service.GpuPerformance, groupToSlices map[int32][]*service.ProfilingData_GpuSlices_Slice) {
	gpuTimeMetricId, gpuWallTimeMetricId := uint32(0), uint32(1)
	metadata.Metrics = append(metadata.Metrics, &service.GpuPerformanceMetadata_Metric{
		Id:   gpuTimeMetricId,
		Name: "GPU Time",
		Unit: "ns",
		Op:   service.GpuPerformanceMetadata_Sum,
	})
	metadata.Metrics = append(metadata.Metrics, &service.GpuPerformanceMetadata_Metric{
		Id:   gpuWallTimeMetricId,
		Name: "GPU Wall Time",
		Unit: "ns",
		Op:   service.GpuPerformanceMetadata_Sum,
	})
	for i, groupId := range groupIds {
		perf := perfs[i]
		slices := groupToSlices[groupId]
		gpuTime, wallTime := gpuTimeForGroup(slices)
		perf.Result[gpuTimeMetricId] = float64(gpuTime)
		perf.Result[gpuWallTimeMetricId] = float64(wallTime)
	}
}

func gpuTimeForGroup(slices []*service.ProfilingData_GpuSlices_Slice) (uint64, uint64) {
	gpuTime, wallTime := uint64(0), uint64(0)
	lastEnd := uint64(0)
	for _, slice := range slices {
		duration := slice.Dur
		gpuTime += duration
		if slice.Ts < lastEnd {
			if slice.Ts+slice.Dur <= lastEnd {
				continue // completely contained within the other, can ignore it.
			}
			duration -= lastEnd - slice.Ts
		}
		wallTime += duration
		lastEnd = slice.Ts + slice.Dur
	}
	return gpuTime, wallTime
}

func setGpuCounterMetrics(ctx context.Context, metadata *service.GpuPerformanceMetadata, groupIds []int32, perfs []*service.GpuPerformance, groupToSlices map[int32][]*service.ProfilingData_GpuSlices_Slice, counters []*service.ProfilingData_Counter) {
	// The metric ids 0 and 1 are assigned to GPU Time and GPU Wall Time correspondingly,
	// The metric ids for counters start from 2.
	counterMetricIdOffset := 2
	for i, counter := range counters {
		metricId := uint32(counterMetricIdOffset + i)
		op := getCounterAggregationMethod(counter)
		metadata.Metrics = append(metadata.Metrics, &service.GpuPerformanceMetadata_Metric{
			Id:   metricId,
			Name: counter.Name,
			Unit: counter.Unit,
			Op:   op,
		})
		if op != service.GpuPerformanceMetadata_TimeWeightedAvg {
			log.E(ctx, "Counter aggregation method not implemented yet. Operation: %v", op)
			continue
		}
		for i, groupId := range groupIds {
			perf := perfs[i]
			slices := groupToSlices[groupId]
			counterPerf := counterPerfForGroup(slices, counter)
			perf.Result[metricId] = counterPerf
		}
	}
}

func counterPerfForGroup(slices []*service.ProfilingData_GpuSlices_Slice, counter *service.ProfilingData_Counter) float64 {
	// Reduce overlapped counter samples size.
	// Filter out the counter samples whose implicit range collides with `slices`'s gpu time.
	rangeStart, rangeEnd := ^uint64(0), uint64(0)
	ts, vs := make([]uint64, 0), make([]float64, 0)
	for _, slice := range slices {
		rangeStart = min(rangeStart, slice.Ts)
		rangeEnd = max(rangeEnd, slice.Ts+slice.Dur)
	}
	for i := range counter.Timestamps {
		if i > 0 && counter.Timestamps[i-1] > rangeEnd {
			break
		}
		if counter.Timestamps[i] > rangeStart {
			ts = append(ts, counter.Timestamps[i])
			vs = append(vs, counter.Values[i])
		}
	}
	if len(ts) == 0 {
		return float64(-1)
	}
	// Aggregate counter samples.
	// Contribution time is the overlapped time between a counter sample's implicit range and a gpu slice.
	ctSum := uint64(0)             // Accumulation of contribution time.
	weightedValuesum := float64(0) // Accumulation of (counter value * counter's contribution time).
	for _, slice := range slices {
		sStart, sEnd := slice.Ts, slice.Ts+slice.Dur
		if ts[0] > sStart {
			ct := min(ts[0], sEnd) - sStart
			ctSum += ct
			weightedValuesum += float64(ct) * vs[0]
		}
		for i := 1; i < len(ts); i++ {
			cStart, cEnd := ts[i-1], ts[i]
			if cEnd < sStart { // Sample earlier than GPU slice's span.
				continue
			} else if cEnd < sEnd { // Sample inside GPU slice's span.
				ct := cEnd - max(cStart, sStart)
				ctSum += ct
				weightedValuesum += float64(ct) * vs[i]
			} else { // Sample later than GPU slice's span.
				ct := max(0, sEnd-cStart)
				ctSum += ct
				weightedValuesum += float64(ct) * vs[i]
				break
			}
		}
	}
	// Return result.
	if ctSum == 0 {
		return float64(0)
	} else {
		return weightedValuesum / float64(ctSum)
	}
}

func getCounterAggregationMethod(counter *service.ProfilingData_Counter) service.GpuPerformanceMetadata_AggregationOperator {
	// TODO: Use time-weighted average to aggregate all counters for now. May need vendor's support. Bug tracked with b/158057709.
	return service.GpuPerformanceMetadata_TimeWeightedAvg
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	} else {
		return b
	}
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	} else {
		return b
	}
}