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

package memory

import (
	"context"
	"reflect"

	//"github.com/google/gapid/core/data/endian"
	"github.com/google/gapid/core/data/slice"
	"github.com/google/gapid/core/os/device"
)

// LoadSlice loads the slice elements from s into a go-slice of the slice type.
func LoadSlice(ctx context.Context, s Slice, pools Pools, l *device.MemoryLayout) (interface{}, error) {
	pool := pools.MustGet(s.Pool())
	rng := Range{s.Base(), s.Size()}
	d := pool.Slice(rng).NewDecoder(ctx, l)
	count := int(s.Count())
	sli := slice.New(reflect.SliceOf(s.ElementType()), count, count)
	Read(d, sli.Addr().Interface())
	if err := d.Error(); err != nil {
		return nil, err
	}
	return sli.Interface(), nil
}

// LoadPointer loads the element from p.
func LoadPointer(ctx context.Context, p Pointer, pools Pools, l *device.MemoryLayout) (interface{}, error) {
	d := pools.ApplicationPool().At(p.Address()).NewDecoder(ctx, l)
	elPtr := reflect.New(p.ElementType())
	Read(d, elPtr.Interface())
	if err := d.Error(); err != nil {
		return nil, err
	}
	return elPtr.Elem().Interface(), nil
}
