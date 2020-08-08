// Copyright (C) 2019 Google Inc.
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

package adreno

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/gapid/core/data/id"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/perfetto"
	perfetto_service "github.com/google/gapid/gapis/perfetto/service"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

var (
	slicesQuery = "" +
		"SELECT s.context_id, s.render_target, s.frame_id, s.submission_id, s.hw_queue_id, s.command_buffer, s.render_pass, s.ts, s.dur, s.id, s.name, depth, arg_set_id, track_id, t.name " +
		"FROM gpu_track t LEFT JOIN gpu_slice s " +
		"ON s.track_id = t.id WHERE t.scope = 'gpu_render_stage' ORDER BY s.ts"
	argsQueryFmt = "" +
		"SELECT key, string_value FROM args WHERE args.arg_set_id = %d"
	queueSubmitQuery = "" +
		"SELECT submission_id FROM gpu_slice s JOIN track t ON s.track_id = t.id WHERE s.name = 'vkQueueSubmit' AND t.name = 'Vulkan Events' ORDER BY submission_id"
	counterTracksQuery = "" +
		"SELECT id, name, unit, description FROM gpu_counter_track ORDER BY id"
	countersQueryFmt = "" +
		"SELECT ts, value FROM counter c WHERE c.track_id = %d ORDER BY ts"
	renderPassSliceName = "Surface"
)

func ProcessProfilingData(ctx context.Context, processor *perfetto.Processor, capture *path.Capture, desc *device.GpuCounterDescriptor, handleMapping *map[uint64][]service.VulkanHandleMappingItem, syncData *sync.Data) (*service.ProfilingData, error) {
	slices, err := processGpuSlices(ctx, processor, capture, handleMapping, syncData)
	if err != nil {
		log.Err(ctx, err, "Failed to get GPU slices")
	}
	counters, err := processCounters(ctx, processor, desc)
	if err != nil {
		log.Err(ctx, err, "Failed to get GPU counters")
	}
	perfMetadata, crudePerfId, err := processPerformances(ctx, slices, counters)
	if err != nil {
		log.Err(ctx, err, "Failed to calculate performance data based on GPU slices and counters")
	}

	return &service.ProfilingData{
		Slices:          slices,
		Counters:        counters,
		GpuPerfMetadata: perfMetadata,
		GpuCrudePerfId:  path.NewID(crudePerfId),
	}, nil
}

func extractTraceHandles(ctx context.Context, replayHandles *[]int64, replayHandleType string, handleMapping *map[uint64][]service.VulkanHandleMappingItem) {
	for i, v := range *replayHandles {
		handles, ok := (*handleMapping)[uint64(v)]
		if !ok {
			log.E(ctx, "%v not found in replay: %v", replayHandleType, v)
			continue
		}

		found := false
		for _, handle := range handles {
			if handle.HandleType == replayHandleType {
				(*replayHandles)[i] = int64(handle.TraceValue)
				found = true
				break
			}
		}

		if !found {
			log.E(ctx, "Incorrect Handle type for %v: %v", replayHandleType, v)
		}
	}
}

func processGpuSlices(ctx context.Context, processor *perfetto.Processor, capture *path.Capture, handleMapping *map[uint64][]service.VulkanHandleMappingItem, syncData *sync.Data) (*service.ProfilingData_GpuSlices, error) {
	slicesQueryResult, err := processor.Query(slicesQuery)
	if err != nil {
		return nil, log.Errf(ctx, err, "SQL query failed: %v", slicesQuery)
	}

	queueSubmitQueryResult, err := processor.Query(queueSubmitQuery)
	if err != nil {
		return nil, log.Errf(ctx, err, "SQL query failed: %v", queueSubmitQuery)
	}
	queueSubmitColumns := queueSubmitQueryResult.GetColumns()
	queueSubmitIds := queueSubmitColumns[0].GetLongValues()
	submissionOrdering := make(map[int64]uint64)

	for i, v := range queueSubmitIds {
		submissionOrdering[v] = uint64(i)
	}

	trackIdCache := make(map[int64]bool)
	argsQueryCache := make(map[int64]*perfetto_service.QueryResult)
	slicesColumns := slicesQueryResult.GetColumns()
	numSliceRows := slicesQueryResult.GetNumRecords()
	slices := make([]*service.ProfilingData_GpuSlices_Slice, numSliceRows)
	groups := make([]*service.ProfilingData_GpuSlices_Group, 0)
	groupIds := make([]int32, numSliceRows)
	var tracks []*service.ProfilingData_GpuSlices_Track
	// Grab all the column values. Depends on the order of columns selected in slicesQuery

	contextIds := slicesColumns[0].GetLongValues()
	extractTraceHandles(ctx, &contextIds, "VkDevice", handleMapping)

	renderTargets := slicesColumns[1].GetLongValues()
	extractTraceHandles(ctx, &renderTargets, "VkFramebuffer", handleMapping)

	commandBuffers := slicesColumns[5].GetLongValues()
	extractTraceHandles(ctx, &commandBuffers, "VkCommandBuffer", handleMapping)

	renderPasses := slicesColumns[6].GetLongValues()
	extractTraceHandles(ctx, &renderPasses, "VkRenderPass", handleMapping)

	frameIds := slicesColumns[2].GetLongValues()
	submissionIds := slicesColumns[3].GetLongValues()
	hwQueueIds := slicesColumns[4].GetLongValues()
	timestamps := slicesColumns[7].GetLongValues()
	durations := slicesColumns[8].GetLongValues()
	ids := slicesColumns[9].GetLongValues()
	names := slicesColumns[10].GetStringValues()
	depths := slicesColumns[11].GetLongValues()
	argSetIds := slicesColumns[12].GetLongValues()
	trackIds := slicesColumns[13].GetLongValues()
	trackNames := slicesColumns[14].GetStringValues()

	subCommandGroupMap := make(map[api.CmdSubmissionKey]int)
	for i, v := range submissionIds {
		subOrder, ok := submissionOrdering[v]
		if ok {
			cb := uint64(commandBuffers[i])
			key := api.CmdSubmissionKey{subOrder, cb, uint64(renderPasses[i]), uint64(renderTargets[i])}
			if indices, ok := syncData.SubmissionIndices[key]; ok {
				if names[i] == renderPassSliceName {
					var idx []uint64
					if c, ok := subCommandGroupMap[key]; ok {
						idx = indices[c]
					} else {
						idx = indices[0]
						subCommandGroupMap[key] = 0
					}

					group := &service.ProfilingData_GpuSlices_Group{
						Id:   int32(len(groups)),
						Link: &path.Command{Capture: capture, Indices: idx},
					}
					groups = append(groups, group)
					subCommandGroupMap[key]++
				}
			}
		} else {
			log.W(ctx, "Encountered submission ID mismatch %v", v)
		}

		groupIds[i] = int32(len(groups)) - 1
	}

	for i := uint64(0); i < numSliceRows; i++ {
		var argsQueryResult *perfetto_service.QueryResult
		var ok bool
		if argsQueryResult, ok = argsQueryCache[argSetIds[i]]; !ok {
			argsQuery := fmt.Sprintf(argsQueryFmt, argSetIds[i])
			argsQueryResult, err = processor.Query(argsQuery)
			if err != nil {
				log.W(ctx, "SQL query failed: %v", argsQuery)
			}
			argsQueryCache[argSetIds[i]] = argsQueryResult
		}
		argsColumns := argsQueryResult.GetColumns()
		numArgsRows := argsQueryResult.GetNumRecords()
		var extras []*service.ProfilingData_GpuSlices_Slice_Extra
		for j := uint64(0); j < numArgsRows; j++ {
			keys := argsColumns[0].GetStringValues()
			values := argsColumns[1].GetStringValues()
			extras = append(extras, &service.ProfilingData_GpuSlices_Slice_Extra{
				Name:  keys[j],
				Value: &service.ProfilingData_GpuSlices_Slice_Extra_StringValue{StringValue: values[j]},
			})
		}
		extras = append(extras, &service.ProfilingData_GpuSlices_Slice_Extra{
			Name:  "contextId",
			Value: &service.ProfilingData_GpuSlices_Slice_Extra_IntValue{IntValue: uint64(contextIds[i])},
		})
		extras = append(extras, &service.ProfilingData_GpuSlices_Slice_Extra{
			Name:  "renderTarget",
			Value: &service.ProfilingData_GpuSlices_Slice_Extra_IntValue{IntValue: uint64(renderTargets[i])},
		})
		extras = append(extras, &service.ProfilingData_GpuSlices_Slice_Extra{
			Name:  "commandBuffer",
			Value: &service.ProfilingData_GpuSlices_Slice_Extra_IntValue{IntValue: uint64(commandBuffers[i])},
		})
		extras = append(extras, &service.ProfilingData_GpuSlices_Slice_Extra{
			Name:  "renderPass",
			Value: &service.ProfilingData_GpuSlices_Slice_Extra_IntValue{IntValue: uint64(renderPasses[i])},
		})
		extras = append(extras, &service.ProfilingData_GpuSlices_Slice_Extra{
			Name:  "frameId",
			Value: &service.ProfilingData_GpuSlices_Slice_Extra_IntValue{IntValue: uint64(frameIds[i])},
		})
		extras = append(extras, &service.ProfilingData_GpuSlices_Slice_Extra{
			Name:  "submissionId",
			Value: &service.ProfilingData_GpuSlices_Slice_Extra_IntValue{IntValue: uint64(submissionIds[i])},
		})
		extras = append(extras, &service.ProfilingData_GpuSlices_Slice_Extra{
			Name:  "hwQueueId",
			Value: &service.ProfilingData_GpuSlices_Slice_Extra_IntValue{IntValue: uint64(hwQueueIds[i])},
		})

		if names[i] == renderPassSliceName && groupIds[i] != -1 {
			names[i] = fmt.Sprintf("%v", groups[groupIds[i]].Link.Indices)
		}

		slices[i] = &service.ProfilingData_GpuSlices_Slice{
			Ts:      uint64(timestamps[i]),
			Dur:     uint64(durations[i]),
			Id:      uint64(ids[i]),
			Label:   names[i],
			Depth:   int32(depths[i]),
			Extras:  extras,
			TrackId: int32(trackIds[i]),
			GroupId: groupIds[i],
		}

		if _, ok := trackIdCache[trackIds[i]]; !ok {
			trackIdCache[trackIds[i]] = true
			tracks = append(tracks, &service.ProfilingData_GpuSlices_Track{
				Id:   int32(trackIds[i]),
				Name: trackNames[i],
			})
		}
	}

	return &service.ProfilingData_GpuSlices{
		Slices: slices,
		Tracks: tracks,
		Groups: groups,
	}, nil
}

func processCounters(ctx context.Context, processor *perfetto.Processor, desc *device.GpuCounterDescriptor) ([]*service.ProfilingData_Counter, error) {
	counterTracksQueryResult, err := processor.Query(counterTracksQuery)
	if err != nil {
		return nil, log.Errf(ctx, err, "SQL query failed: %v", counterTracksQuery)
	}
	// t.id, name, unit, description, ts, value
	tracksColumns := counterTracksQueryResult.GetColumns()
	numTracksRows := counterTracksQueryResult.GetNumRecords()
	counters := make([]*service.ProfilingData_Counter, numTracksRows)
	// Grab all the column values. Depends on the order of columns selected in countersQuery
	trackIds := tracksColumns[0].GetLongValues()
	names := tracksColumns[1].GetStringValues()
	units := tracksColumns[2].GetStringValues()
	descriptions := tracksColumns[3].GetStringValues()

	for i := uint64(0); i < numTracksRows; i++ {
		countersQuery := fmt.Sprintf(countersQueryFmt, trackIds[i])
		countersQueryResult, err := processor.Query(countersQuery)
		countersColumns := countersQueryResult.GetColumns()
		if err != nil {
			return nil, log.Errf(ctx, err, "SQL query failed: %v", counterTracksQuery)
		}
		timestampsLong := countersColumns[0].GetLongValues()
		timestamps := make([]uint64, len(timestampsLong))
		for i, t := range timestampsLong {
			timestamps[i] = uint64(t)
		}
		values := countersColumns[1].GetDoubleValues()
		// TODO(apbodnar) Populate the `default` field once the trace processor supports it (b/147432390)
		counters[i] = &service.ProfilingData_Counter{
			Id:          uint32(trackIds[i]),
			Name:        names[i],
			Unit:        units[i],
			Description: descriptions[i],
			Timestamps:  timestamps,
			Values:      values,
		}
	}
	return counters, nil
}

func processPerformances(ctx context.Context, slices *service.ProfilingData_GpuSlices, counters []*service.ProfilingData_Counter) (*service.GpuPerformanceMetadata, id.ID, error) {
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
