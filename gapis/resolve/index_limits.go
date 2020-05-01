// Copyright (C) 2017 Google Inc.
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
	"bytes"
	"context"
	"fmt"

	"github.com/google/gapid/core/data/binary"
	"github.com/google/gapid/core/data/id"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/service/path"
)

// IndexRange represents the range of indices which were referenced by index buffer.
type IndexRange struct {
	First uint32
	Count uint32
}

// IndexLimits returns the range of indices which were referenced by index
// buffer with identifier id. The buffer holds count elements, each of size
// bytes.
func IndexLimits(ctx context.Context, data id.ID, count int, size int, littleEndian bool) (*IndexRange, error) {
	obj, err := database.Build(ctx, &IndexLimitsResolvable{
		IndexSize:    uint64(size),
		Count:        uint64(count),
		LittleEndian: littleEndian,
		Data:         path.NewBlob(data),
	})
	if err != nil {
		return nil, err
	}
	return obj.(*IndexRange), nil
}

// Resolve implements the database.Resolver interface.
func (c *IndexLimitsResolvable) Resolve(ctx context.Context) (interface{}, error) {
	if c.Count == 0 {
		return &IndexRange{First: 0, Count: 0}, nil
	}
	min, max := ^uint32(0), uint32(0)
	data, err := database.Resolve(ctx, c.Data.ID.ID())
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(data.([]byte))

	var decode func() uint32
	switch c.IndexSize {
	case 1:
		val, _ := binary.ReadUint8(r)
		decode = func() uint32 { return uint32(val) }
	case 2:
		val, _ := binary.ReadUint16(r)
		decode = func() uint32 { return uint32(val) }
	case 4:
		val, _ := binary.ReadUint32(r)
		decode = func() uint32 { return uint32(val) }
	default:
		return nil, fmt.Errorf("Unsupported index size %v", c.IndexSize)
	}

	for i := uint64(0); i < c.Count; i++ {
		v := decode()
		if min > v {
			min = v
		}
		if max < v {
			max = v
		}
	}

	return &IndexRange{First: min, Count: max + 1 - min}, nil
}
