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
	"io"

	"github.com/google/gapid/core/data/binary"
	"github.com/google/gapid/core/math/u64"
	"github.com/google/gapid/core/os/device"
)

// Decoder provides methods to read primitives from a binary.Reader, respecting
// a given MemoryLayout.
// Decoder will automatically handle alignment and types sizes.
type Decoder struct {
	r io.Reader
	m *device.MemoryLayout
	o uint64
	err error
}

// NewDecoder constructs and returns a new Decoder that reads from r using
// the memory layout m.
func NewDecoder(r io.Reader, m *device.MemoryLayout) *Decoder {
	return &Decoder{r, m, 0, nil}
}

func (d *Decoder) alignAndOffset(l *device.DataTypeLayout) {
	d.Align(uint64(l.Alignment))
	d.o += uint64(l.Size)
}

// MemoryLayout returns the MemoryLayout used by the decoder.
func (d *Decoder) MemoryLayout() *device.MemoryLayout {
	return d.m
}

// Offset returns the byte offset of the reader from the initial Decoder
// creation.
func (d *Decoder) Offset() uint64 {
	return d.o
}

// Align skips bytes until the read position is a multiple of to.
func (d *Decoder) Align(to uint64) {
	alignment := u64.AlignUp(d.o, uint64(to))
	if pad := alignment - d.o; pad != 0 {
		d.Skip(pad)
	}
}

// Skip skips n bytes from the reader.
func (d *Decoder) Skip(n uint64) {
	_, err := binary.ConsumeBytes(d.r, n)
	if err != nil {
		d.err = err
	}
	d.o += n
}

// Pointer loads and returns a pointer address.
// ALAN: Error
func (d *Decoder) Pointer() uint64 {
	d.alignAndOffset(d.m.Pointer)
	ret, err := binary.ReadUint(d.r, 8*d.m.Pointer.Size)
	if err != nil {
		d.err = err
	}
	return ret
}

// F32 loads and returns a float32.
func (d *Decoder) F32() float32 {
	d.alignAndOffset(d.m.F32)
	ret, err := binary.ReadFloat32(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// F64 loads and returns a float64.
func (d *Decoder) F64() float64 {
	d.alignAndOffset(d.m.F64)
	ret, err := binary.ReadFloat64(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// I8 loads and returns a int8.
func (d *Decoder) I8() int8 {
	d.alignAndOffset(d.m.I8)
	ret, err := binary.ReadInt8(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// I16 loads and returns a int16.
func (d *Decoder) I16() int16 {
	d.alignAndOffset(d.m.I16)
	ret, err := binary.ReadInt16(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// I32 loads and returns a int32.
func (d *Decoder) I32() int32 {
	d.alignAndOffset(d.m.I32)
	ret, err := binary.ReadInt32(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// I64 loads and returns a int64.
func (d *Decoder) I64() int64 {
	d.alignAndOffset(d.m.I64)
	ret, err := binary.ReadInt64(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// U8 loads and returns a uint8.
func (d *Decoder) U8() uint8 {
	d.alignAndOffset(d.m.I8)
	ret, err := binary.ReadUint8(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// U16 loads and returns a uint16.
func (d *Decoder) U16() uint16 {
	d.alignAndOffset(d.m.I16)
	ret, err := binary.ReadUint16(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// U32 loads and returns a uint32.
func (d *Decoder) U32() uint32 {
	d.alignAndOffset(d.m.I32)
	ret, err := binary.ReadUint32(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// U64 loads and returns a uint64.
func (d *Decoder) U64() uint64 {
	d.alignAndOffset(d.m.I64)
	ret, err := binary.ReadUint64(d.r)
	if err != nil {
		d.err = err
	}
	return ret
}

// Char loads and returns an char.
func (d *Decoder) Char() Char {
	d.alignAndOffset(d.m.Char)
	val, err := binary.ReadInt(d.r, 8*d.m.Char.Size)
	if err != nil {
		d.err = err
	}
	return Char(val)
}

// Int loads and returns an int.
func (d *Decoder) Int() Int {
	d.alignAndOffset(d.m.Integer)
	val, err := binary.ReadInt(d.r, 8*d.m.Integer.Size)
	if err != nil {
		d.err = err
	}
	return Int(val)
}

// Uint loads and returns a uint.
func (d *Decoder) Uint() Uint {
	d.alignAndOffset(d.m.Integer)
	val, err := binary.ReadUint(d.r, 8*d.m.Integer.Size)
	if err != nil {
		d.err = err
	}
	return Uint(val)
}

// Size loads and returns a size_t.
func (d *Decoder) Size() Size {
	d.alignAndOffset(d.m.Size)
	val, err := binary.ReadUint(d.r, 8*d.m.Size.Size)
	if err != nil {
		d.err = err
	}
	return Size(val)
}

// String loads and returns a null-terminated string.
func (d *Decoder) String() string {
	out, err := binary.ReadString(d.r)
	if err != nil {
		d.err = err
	}
	d.o += uint64(len(out) + 1)
	return out
}

// Bool loads and returns a boolean value.
func (d *Decoder) Bool() bool {
	d.o++
	return d.U8() != 0
}

// Data reads raw bytes into buf.
func (d *Decoder) Data(buf []byte) {
	err := binary.ReadData(d.r, buf)
	if err != nil {
		d.err = err
	}
	d.o += uint64(len(buf))
}

// Error returns the error state of the underlying reader.
func (d *Decoder) Error() error {
	return d.err
}
