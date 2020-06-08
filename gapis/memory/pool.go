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
	"fmt"
	"io"
	"strings"

	"github.com/google/gapid/core/data/id"
	// "github.com/google/gapid/core/data/endian"
	"github.com/google/gapid/core/math/interval"
	"github.com/google/gapid/core/math/u64"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/core/os/device"
	// "github.com/pkg/errors"
)

// Pool represents an unbounded and isolated memory space. Pool can be used
// to represent the application address space, or hidden GPU Pool.
//
// Pool can be sliced into smaller regions which can be read or written to.
// All writes to Pool or its slices do not actually perform binary data
// copies, but instead all writes are stored as lightweight records. Only when a
// Pool slice has Get called will any resolving, loading or copying of binary
// data occur.
//
// OnRead and OnWrite functions take 3 parameters. The first is
// the range of memory. The second is the root of the read.
//   that is, the pointer from which the range was derived. For
//   example:
//      uint32_t* foo;
//      foo[2]; Range(Base: foo+2, Size 4), Root foo
//  Last is the service.type ID of the observations. This will
//      always be a slice at this point.
type Pool struct {
	writes  poolWriteList
	OnRead  func(rng Range, root uint64, t uint64, api id.ID)
	OnWrite func(rng Range, root uint64, t uint64, api id.ID)
}

// PoolID is an identifier of a Pool.
type PoolID uint32

// Pools contains a collection of Pools identified by PoolIDs.
type Pools struct {
	pools      map[PoolID]*Pool
	nextPoolID PoolID
	OnCreate   func(PoolID, *Pool)
}

func (p *Pools) Clone() Pools {
	x := NewPools()
	for k, v := range p.pools {
		var np *Pool
		if k != ApplicationPool {
			np = x.NewAt(k)
		} else {
			np = x.pools[ApplicationPool]
		}
		np.writes = append(poolWriteList{}, v.writes[:]...)
	}
	return x
}

type poolSlice struct {
	rng    Range         // The memory range of the slice.
	writes poolWriteList // The list of writes to the pool when this slice was created.
}

const (
	// ApplicationPool is the PoolID of Pool representing the application's memory
	// address space.
	ApplicationPool = PoolID(PoolNames_Application)
)

// NewPools creates and returns a new Pools instance.
func NewPools() Pools {
	return Pools{
		pools:      map[PoolID]*Pool{ApplicationPool: {}},
		nextPoolID: ApplicationPool + 1,
	}
}

// Slice returns a Data referencing the subset of the Pool range.
func (m *Pool) Slice(rng Range) Data {
	i, c := interval.Intersect(&m.writes, rng.Span())
	if c == 1 {
		w := m.writes[i]
		if rng == w.dst {
			// Exact hit
			//return w.src
			return poolSlice{rng: rng, writes: m.writes[i : i+1]}
		}
		if rng.First() >= w.dst.First() && rng.Last() <= w.dst.Last() {
			// Subset of a write.
			rng.Base -= w.dst.First()
			//return w.src.Slice(rng)
			return poolSlice{rng: rng, writes: m.writes[i : i+1]}
		}
	}
	writes := make(poolWriteList, c)
	copy(writes, m.writes[i:i+c])
	return poolSlice{rng: rng, writes: writes}
}

// TempSlice returns a slice that is only valid until the next
// Read/Write operation on this pool.
// This puts much less pressure on the garbage collector since we don't
// have to make a copy of the range.
func (m *Pool) TempSlice(rng Range) Data {
	i, c := interval.Intersect(&m.writes, rng.Span())
	if c == 1 {
		w := m.writes[i]
		if rng == w.dst {
			// Exact hit
			return w.src
		}
		if rng.First() >= w.dst.First() && rng.Last() <= w.dst.Last() {
			// Subset of a write.
			rng.Base -= w.dst.First()
			return w.src.Slice(rng)
		}
	}
	return poolSlice{rng: rng, writes: m.writes[i : i+c]}
}

// At returns an unbounded Data starting at p.
func (m *Pool) At(addr uint64) Data {
	return m.Slice(Range{Base: addr, Size: ^uint64(0) - addr})
}

// Write copies the data src to the address dst.
func (m *Pool) Write(dst uint64, src Data) {
	rng := Range{Base: dst, Size: src.Size()}
	i := interval.Replace(&m.writes, rng.Span())
	m.writes[i].src = src
}

// Strlen returns the run length of bytes starting from ptr before a 0 byte is
// reached.
func (m *Pool) Strlen(ctx context.Context, ptr uint64) (uint64, error) {
	first := interval.IndexOf(&m.writes, ptr)
	if first < 0 {
		return 0, nil
	}
	count := uint64(0)
	for i, w := range m.writes[first:] {
		if i > 0 && m.writes[i-1].dst.End() != w.dst.Base {
			return count, nil // Gap between writes holds 0
		}
		v, err := w.src.Strlen(ctx)
		if err != nil {
			return 0, err
		}
		if v >= 0 {
			return count + uint64(v), nil
		}
		count += w.dst.Size
	}
	return count, nil
}

// String returns the full history of writes performed to this pool.
func (m *Pool) String() string {
	l := make([]string, len(m.writes)+1)
	l[0] = fmt.Sprintf("Pool(%p):", m)
	for i, w := range m.writes {
		l[i+1] = fmt.Sprintf("(%d) %v <- %v", i, w.dst, w.src)
	}
	return strings.Join(l, "\n")
}

// NextPoolID returns the next free pool ID (but does not assign it).
// All existing pools in the set have pool ID which is less then this value.
func (m *Pools) NextPoolID() PoolID {
	return m.nextPoolID
}

// New creates and returns a new Pool and its id.
func (m *Pools) New() (id PoolID, p *Pool) {
	id, p = m.nextPoolID, &Pool{}
	m.pools[id] = p
	m.nextPoolID++

	if m.OnCreate != nil {
		m.OnCreate(id, p)
	}
	return
}

// NewAt creates and returns a new Pool with a specific ID, fails if it cannot
func (m *Pools) NewAt(id PoolID) *Pool {
	if _, ok := m.pools[id]; ok {
		panic("Could not create given pool")
	}
	p := &Pool{}
	m.pools[id] = p
	if id >= m.nextPoolID {
		m.nextPoolID = id + 1
	}

	if m.OnCreate != nil {
		m.OnCreate(id, p)
	}
	return p
}

// Get returns the Pool with the given id.
func (m *Pools) Get(id PoolID) (*Pool, error) {
	if p, ok := m.pools[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("Pool %v not found", id)
}

// MustGet returns the Pool with the given id, or panics if it's not found.
func (m *Pools) MustGet(id PoolID) *Pool {
	if p, ok := m.pools[id]; ok {
		return p
	}
	panic(fmt.Errorf("Pool %v not found", id))
}

// ApplicationPool returns the application memory pool.
func (m *Pools) ApplicationPool() *Pool {
	return m.pools[ApplicationPool]
}

// Count returns the number of pools.
func (m *Pools) Count() int {
	return len(m.pools)
}

// SetOnCreate sets the OnCreate callback and invokes it for every pool already created.
func (m *Pools) SetOnCreate(onCreate func(PoolID, *Pool)) {
	m.OnCreate = onCreate
	for i, p := range m.pools {
		onCreate(i, p)
	}
}

// String returns a string representation of all pools.
func (m *Pools) String() string {
	mem := make([]string, 0, len(m.pools))
	for i, p := range m.pools {
		mem[i] = fmt.Sprintf("    %d: %v", i, strings.Replace(p.String(), "\n", "\n      ", -1))
	}
	return strings.Join(mem, "\n")
}

func (m poolSlice) Get(ctx context.Context, offset uint64, dst []byte) error {
	orng := Range{Base: m.rng.Base + offset, Size: m.rng.Size - offset}
	i, c := interval.Intersect(&m.writes, orng.Span())
	for _, w := range m.writes[i : i+c] {
		if w.dst.First() > orng.First() {
			if err := w.src.Get(ctx, 0, dst[w.dst.First()-orng.First():]); err != nil {
				return err
			}
		} else {
			if err := w.src.Get(ctx, orng.First()-w.dst.First(), dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m poolSlice) ResourceID(ctx context.Context) (id.ID, error) {
	getBytes := func() ([]byte, error) {
		bytes := make([]byte, m.Size())
		if err := m.Get(ctx, 0, bytes); err != nil {
			return []byte{}, err
		}
		return bytes, nil
	}
	return database.Store(ctx, getBytes)
}

func (m poolSlice) Slice(rng Range) Data {
	if uint64(rng.Last()) > m.rng.Size {
		panic(fmt.Errorf("%v.Slice(%v) - out of bounds", m.String(), rng))
	}
	rng.Base += m.rng.Base
	i, c := interval.Intersect(&m.writes, rng.Span())
	return poolSlice{rng, m.writes[i : i+c]}
}

func (m poolSlice) ValidRanges() RangeList {
	valid := make(RangeList, len(m.writes))
	for i, w := range m.writes {
		s := u64.Max(w.dst.Base, m.rng.Base)
		e := u64.Min(w.dst.Base+w.dst.Size, m.rng.Base+m.rng.Size)
		valid[i] = Range{Base: s - m.rng.Base, Size: e - s}
	}
	return valid
}

func (m poolSlice) Strlen(ctx context.Context) (int, error) {
	count := 0
	for i, w := range m.writes {
		if i > 0 && m.writes[i-1].dst.End() != w.dst.Base {
			return count, nil // Gap between writes holds 0
		}
		v, err := w.src.Strlen(ctx)
		if err != nil {
			return 0, err
		}
		if v >= 0 {
			return count + v, nil
		}
		count += int(w.dst.Size)
	}
	return -1, nil
}

func (m poolSlice) Size() uint64 {
	return m.rng.Size
}

func (m poolSlice) String() string {
	return fmt.Sprintf("Slice(%v)", m.rng)
}

func (m poolSlice) NewDecoder(ctx context.Context, memLayout *device.MemoryLayout) Decoder {
	decode := &poolSliceDecoder{ctx: ctx, writes: m.writes, rng: m.rng}
	return decode
}

type poolSliceDecoder struct {
	ctx      context.Context
	writes   poolWriteList
	rng      Range
	simpleDecoder Decoder
}

func (r *poolSliceDecoder) alignAndOffset(l *device.DataTypeLayout) {
	r.ensureDecoder()
	r.simpleDecoder.alignAndOffset(l)
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
}

func (r *poolSliceDecoder) MemoryLayout() *device.MemoryLayout {
	r.ensureDecoder()
	ret := r.simpleDecoder.MemoryLayout()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) Offset() uint64 {
	r.ensureDecoder()
	ret := r.simpleDecoder.Offset()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) Align(to uint64) {
	r.ensureDecoder()
	r.simpleDecoder.Align(to)
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
}

func (r *poolSliceDecoder) Skip(n uint64) {
	r.ensureDecoder()
	r.simpleDecoder.Skip(n)
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
}

func (r *poolSliceDecoder) Pointer() uint64 {
	r.ensureDecoder()
	ret := r.simpleDecoder.Pointer()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) F32() float32 {
	r.ensureDecoder()
	ret := r.simpleDecoder.F32()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) F64() float64 {
	r.ensureDecoder()
	ret := r.simpleDecoder.F64()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) I8() int8 {
	r.ensureDecoder()
	ret := r.simpleDecoder.I8()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) I16() int16 {
	r.ensureDecoder()
	ret := r.simpleDecoder.I16()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) I32() int32 {
	r.ensureDecoder()
	ret := r.simpleDecoder.I32()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) I64() int64 {
	r.ensureDecoder()
	ret := r.simpleDecoder.I64()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) U8() uint8 {
	r.ensureDecoder()
	ret := r.simpleDecoder.U8()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) U16() uint16 {
	r.ensureDecoder()
	ret := r.simpleDecoder.U16()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) U32() uint32 {
	r.ensureDecoder()
	ret := r.simpleDecoder.U32()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) U64() uint64 {
	r.ensureDecoder()
	ret := r.simpleDecoder.U64()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) Char() Char {
	r.ensureDecoder()
	ret := r.simpleDecoder.Char()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) Int() Int {
	r.ensureDecoder()
	ret := r.simpleDecoder.Int()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) Uint() Uint {
	r.ensureDecoder()
	ret := r.simpleDecoder.Uint()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) Size() Size {
	r.ensureDecoder()
	ret := r.simpleDecoder.Size()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) String() string {
	r.ensureDecoder()
	ret := r.simpleDecoder.String()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) Bool() bool {
	r.ensureDecoder()
	ret := r.simpleDecoder.Bool()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	return ret
}

func (r *poolSliceDecoder) Data(buf []byte) {
	r.ensureDecoder()
	if r.simpleDecoder.Error() != nil {
		panic("ALAN") //r.handleError()
	}
	r.simpleDecoder.Data(buf)
}

func (r *poolSliceDecoder) Error() error {
	r.ensureDecoder()
	return r.simpleDecoder.Error()
}

func (r *poolSliceDecoder) ensureDecoder() {
	if r.simpleDecoder == nil {
		r.handleError() // If we don't have an initalised simple decoder, then this is the first read.
		// We can treat this the same as an error, so call handleError to go get a new simpleDecoder.
	}
}

func (r *poolSliceDecoder) handleError() {
	if r.rng.Size <= 0 {
		r.SetError(io.EOF)
	}

	if len(r.writes) > 0 {
		w := r.writes[0]
		intersection := w.dst.Intersect(r.rng)

		if intersection.First() > r.rng.First() {
			zeroSize := intersection.First() - r.rng.First()
			r.simpleDecoder = &zeroReadDecoder{size: zeroSize}
		} else {
			r.writes = r.writes[1:]
			slice := w.src
			if intersection != w.dst {
				slice = w.src.Slice(Range{
					Base: intersection.First() - w.dst.First(),
					Size: intersection.Size,
				})
			}
			//panic(fmt.Errorf("XXXXXXX %v", r))
			r.simpleDecoder = slice.NewDecoder(r.ctx, r.MemoryLayout()) // intersection.Size
		}
	} else {
		r.simpleDecoder = &zeroReadDecoder{size: r.rng.Size}
	}
}

// prepareAndRead determines whether we are about to read from an area covered
// by a write. If so, it obtains an io.Reader for the write and starts reading
// from it (additionally, subsequent Read() calls will skip this function, and
// go through a fast path that delegates to the Reader, until we reach the end
// of the write). If, instead, the area we're looking at is unwritten, we will
// have a fast path until the end of the unwritten area which simply fills the
// output buffers with zeros. At the end of each contiguous region (one single
// write or unwritten area), we go through this again.
// func (r *poolSliceDecoder) prepareAndRead(p []byte) (n int, err error) {
// 	if r.rng.Size <= 0 {
// 		return r.setError(0, io.EOF)
// 	}
// 	if len(p) == 0 {
// 		return 0, nil
// 	}

// 	if len(r.writes) > 0 {
// 		w := r.writes[0]
// 		intersection := w.dst.Intersect(r.rng)

// 		if intersection.First() > r.rng.First() {
// 			r.readImpl = r.zeroReadFunc(intersection.First() - r.rng.First())
// 		} else {
// 			r.writes = r.writes[1:]
// 			slice := w.src
// 			if intersection != w.dst {
// 				slice = w.src.Slice(Range{
// 					Base: intersection.First() - w.dst.First(),
// 					Size: intersection.Size,
// 				})
// 			}
// 			//var sliceReader io.Reader
// 			//_ = slice
// 			//sliceReader := slice.NewReader(r.ctx)
// 			sliceDecoder := slice.NewDecoder(r.ctx, r.MemoryLayout())
// 			r.readImpl = r.readerReadFunc(sliceReader, intersection.Size)
// 		}
// 	} else {
// 		r.readImpl = r.zeroReadFunc(r.rng.Size)
// 	}

// 	return r.readImpl(p)
// }

// zeroReadFunc returns a read function that fills up to bytesLeft bytes
// in the buffer with zeros, after which it switches to the slow path.
// func (r *poolSliceDecoder) zeroReadFunc(bytesLeft uint64) readFunction {
// 	r.rng = r.rng.TrimLeft(bytesLeft)
// 	return func(p []byte) (n int, err error) {
// 		zeroCount := min(bytesLeft, uint64(len(p)))
// 		for i := uint64(0); i < zeroCount; i++ {
// 			p[i] = 0
// 		}

// 		bytesLeft -= zeroCount
// 		if bytesLeft == 0 {
// 			r.readImpl = r.prepareAndRead
// 		}

// 		return int(zeroCount), nil
// 	}
// }

// readerReadFunc returns a read function that reads up to bytesLeft
// bytes from srcReader after which it switches to the slow path.
// func (r *poolSliceDecoder) readerReadFunc(srcReader io.Reader, bytesLeft uint64) readFunction {
// 	r.rng = r.rng.TrimLeft(bytesLeft)
// 	return func(p []byte) (n int, err error) {
// 		bytesToRead := min(bytesLeft, uint64(len(p)))

// 		bytesRead, err := srcReader.Read(p[:bytesToRead])
// 		if bytesRead == 0 && errors.Cause(err) == io.EOF {
// 			return r.setError(0, fmt.Errorf("Premature EOF from underlying reader"))
// 		}
// 		if err != nil && err != io.EOF {
// 			return r.setError(bytesRead, err)
// 		}

// 		bytesLeft -= uint64(bytesRead)
// 		if bytesLeft == 0 {
// 			r.readImpl = r.prepareAndRead
// 		}
// 		return bytesRead, nil
// 	}
// }

// setError returns its arguments and makes subsequent Read()s return (0, err).
func (r *poolSliceDecoder) SetError(err error) {
	panic("qwerty")
	r.simpleDecoder.SetError(err) // ALAN: Is this right? Because we're using the error to advance across pool slices?
}

type zeroReadDecoder struct {
	size uint64
}

func (r *zeroReadDecoder) alignAndOffset(l *device.DataTypeLayout) {}
func (r *zeroReadDecoder) MemoryLayout() *device.MemoryLayout { return nil }
func (r *zeroReadDecoder) Offset() uint64  { return 0 }
func (r *zeroReadDecoder) Align(to uint64) {}
func (r *zeroReadDecoder) Skip(n uint64) {}
func (r *zeroReadDecoder) Pointer() uint64 { return 0 }
func (r *zeroReadDecoder) F32() float32 { return 0 }
func (r *zeroReadDecoder) F64() float64 { return 0 }
func (r *zeroReadDecoder) I8() int8 { return 0 }
func (r *zeroReadDecoder) I16() int16 { return 0 }
func (r *zeroReadDecoder) I32() int32 { return 0 }
func (r *zeroReadDecoder) I64() int64 { return 0 }
func (r *zeroReadDecoder) U8() uint8 { return 0 }
func (r *zeroReadDecoder) U16() uint16 { return 0 }
func (r *zeroReadDecoder) U32() uint32 { return 0 }
func (r *zeroReadDecoder) U64() uint64 { return 0 }
func (r *zeroReadDecoder) Char() Char { return 0 }
func (r *zeroReadDecoder) Int() Int { return 0 }
func (r *zeroReadDecoder) Uint() Uint { return 0 }
func (r *zeroReadDecoder) Size() Size { return 0 }
func (r *zeroReadDecoder) String() string { return "" }
func (r *zeroReadDecoder) Bool() bool { return false } 
func (r *zeroReadDecoder) Data(buf []byte) {}
func (r *zeroReadDecoder) Error() error { return nil }
func (r *zeroReadDecoder) SetError(err error) {}