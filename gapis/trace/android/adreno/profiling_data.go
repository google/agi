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
	"strings"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/perfetto"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/trace/android/profile"
)

var (
	queueSubmitQuery = "" +
		"SELECT submission_id FROM gpu_slice s JOIN track t ON s.track_id = t.id WHERE s.name = 'vkQueueSubmit' AND t.name = 'Vulkan Events' ORDER BY submission_id"
	counterTracksQuery = "" +
		"SELECT id, name, unit, description FROM gpu_counter_track ORDER BY id"
	countersQueryFmt = "" +
		"SELECT ts, value FROM counter c WHERE c.track_id = %d ORDER BY ts"
	renderPassSliceName = "Surface"
)

func ProcessProfilingData(ctx context.Context, processor *perfetto.Processor,
	desc *device.GpuCounterDescriptor, handleMapping map[uint64][]service.VulkanHandleMappingItem,
	syncData *sync.Data, data *profile.ProfilingData) error {

	err := processGpuSlices(ctx, processor, handleMapping, syncData, data)
	if err != nil {
		log.Err(ctx, err, "Failed to get GPU slices")
	}
	data.Counters, err = processCounters(ctx, processor, desc)
	if err != nil {
		log.Err(ctx, err, "Failed to get GPU counters")
	}
	data.ComputeCounters(ctx)
	updateCounterGroups(ctx, data)
	return nil
}

func fixContextIDs(data profile.SliceData) {
	// This is a workaround a QC bug(b/192546534)
	// that causes first deviceID to be zero after a
	// renderpass change in the same queue submit.
	// So, we fill the zero devices with the existing
	// device id, where there is only one device id.

	zeroIndices := make([]int, 0)
	contextID := int64(0)

	for i := range data {
		if data[i].Context == 0 {
			zeroIndices = append(zeroIndices, i)
			continue
		}

		if contextID == 0 {
			contextID = data[i].Context
			continue
		}

		if contextID != data[i].Context {
			// There are multiple devices
			// We cannot know which one to fill
			return
		}
	}

	for _, v := range zeroIndices {
		// If there is only one device in entire trace
		// We can assume that we possibly have only one device
		data[v].Context = contextID
	}
}

func processGpuSlices(ctx context.Context, processor *perfetto.Processor,
	handleMapping map[uint64][]service.VulkanHandleMappingItem, syncData *sync.Data,
	data *profile.ProfilingData) (err error) {

	data.Slices, err = profile.ExtractSliceData(ctx, processor)
	if err != nil {
		return log.Errf(ctx, err, "Extracting slice data failed")
	}

	queueSubmitQueryResult, err := processor.Query(queueSubmitQuery)
	if err != nil {
		return log.Errf(ctx, err, "SQL query failed: %v", queueSubmitQuery)
	}
	queueSubmitColumns := queueSubmitQueryResult.GetColumns()
	queueSubmitIds := queueSubmitColumns[0].GetLongValues()
	submissionOrdering := make(map[int64]int)

	for i, v := range queueSubmitIds {
		submissionOrdering[v] = i
	}

	fixContextIDs(data.Slices)
	data.Slices.MapIdentifiers(ctx, handleMapping)

	groupID := int32(-1)
	for i := range data.Slices {
		slice := &data.Slices[i]
		subOrder, ok := submissionOrdering[slice.Submission]
		if ok {
			cb := uint64(slice.CommandBuffer)
			key := sync.RenderPassKey{
				subOrder, cb, uint64(slice.Renderpass), uint64(slice.RenderTarget),
			}
			// Create a new group for each main renderPass slice.
			idx := syncData.RenderPassLookup.Lookup(ctx, key)
			if !idx.IsNil() && slice.Name == renderPassSliceName {
				slice.Name = fmt.Sprintf("%v-%v", idx.From, idx.To)
				groupID = data.Groups.GetOrCreateGroup(
					fmt.Sprintf("RenderPass %v, RenderTarget %v", uint64(slice.Renderpass), uint64(slice.RenderTarget)),
					idx,
				)
			}
		} else {
			log.W(ctx, "Encountered submission ID mismatch %v", slice.Submission)
		}

		if groupID < 0 {
			log.W(ctx, "Group missing for slice %v at submission %v, commandBuffer %v, renderPass %v, renderTarget %v",
				slice.Name, slice.Submission, slice.CommandBuffer, slice.Renderpass, slice.RenderTarget)
		}
		slice.GroupID = groupID
	}

	return nil
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

	nameToSpec := map[string]*device.GpuCounterDescriptor_GpuCounterSpec{}
	if desc != nil {
		for _, spec := range desc.Specs {
			nameToSpec[spec.Name] = spec
		}
	}

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

		spec, _ := nameToSpec[names[i]]
		// TODO(apbodnar) Populate the `default` field once the trace processor supports it (b/147432390)
		counters[i] = &service.ProfilingData_Counter{
			Id:          uint32(trackIds[i]),
			Name:        names[i],
			Unit:        units[i],
			Description: descriptions[i],
			Spec:        spec,
			Timestamps:  timestamps,
			Values:      values,
		}
	}
	return counters, nil
}

const (
	defaultCounterGroup  uint32 = 1
	veretexCounterGroup  uint32 = 2
	fragmentCounterGroup uint32 = 3
	textureCounterGroup  uint32 = 4
)

func updateCounterGroups(ctx context.Context, data *profile.ProfilingData) {
	data.CounterGroups = append(data.CounterGroups,
		&service.ProfilingData_CounterGroup{
			Id:    defaultCounterGroup,
			Label: "Performance",
		},
		&service.ProfilingData_CounterGroup{
			Id:    veretexCounterGroup,
			Label: "Vertex",
		},
		&service.ProfilingData_CounterGroup{
			Id:    fragmentCounterGroup,
			Label: "Fragment",
		},
		&service.ProfilingData_CounterGroup{
			Id:    textureCounterGroup,
			Label: "Texture",
		},
	)

	for _, counter := range data.GpuCounters.Metrics {
		name := strings.ToLower(counter.Name)
		if counter.SelectByDefault {
			counter.CounterGroupIds = append(counter.CounterGroupIds, defaultCounterGroup)
		}
		if strings.Index(name, "vertex") >= 0 {
			counter.CounterGroupIds = append(counter.CounterGroupIds, veretexCounterGroup)
		}
		if strings.Index(name, "fragment") >= 0 {
			counter.CounterGroupIds = append(counter.CounterGroupIds, fragmentCounterGroup)
		}
		if strings.Index(name, "texture") >= 0 {
			counter.CounterGroupIds = append(counter.CounterGroupIds, textureCounterGroup)
		}
	}
}
