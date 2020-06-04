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
	"github.com/google/gapid/core/data/binary"
	"github.com/google/gapid/core/math/u64"
	"github.com/google/gapid/core/os/device"
)

type Decoder interface {
	alignAndOffset(l *device.DataTypeLayout)
	MemoryLayout() *device.MemoryLayout
	Offset() uint64 
	Align(to uint64)
	Skip(n uint64)
	Pointer() uint64
	F32() float32
	F64() float64
	I8() int8
	I16() int16
	I32() int32
	I64() int64
	U8() uint8
	U16() uint16
	U32() uint32
	U64() uint64
	Char() Char
	Int() Int
	Uint() Uint
	Size() Size
	String() string
	Bool() bool
	Data(buf []byte)
	Error() error
	SetError(err error)
}

// SimpleDecoder provides methods to read primitives from a binary.Reader, respecting
// a given MemoryLayout.
// SimpleDecoder will automatically handle alignment and types sizes.
type SimpleDecoder struct {
	reader binary.Reader
	memLayout *device.MemoryLayout
	offset uint64
}

// NewDecoder constructs and returns a new SimpleDecoder that reads from reader using
// the memory layout memLayout.
func NewDecoder(reader binary.Reader, memLayout *device.MemoryLayout) Decoder {
	return SimpleDecoder{reader, memLayout, 0}
}

func (d SimpleDecoder) alignAndOffset(l *device.DataTypeLayout) {
	d.Align(uint64(l.Alignment))
	d.offset += uint64(l.Size)
}

// MemoryLayout returns the MemoryLayout used by the decoder.
func (d SimpleDecoder) MemoryLayout() *device.MemoryLayout {
	return d.memLayout
}

// Offset returns the byte offset of the reader from the initial SimpleDecoder
// creation.
func (d SimpleDecoder) Offset() uint64 {
	return d.offset
}

// Align skips bytes until the read position is a multiple of to.
func (d SimpleDecoder) Align(to uint64) {
	alignment := u64.AlignUp(d.offset, uint64(to))
	if pad := alignment - d.offset; pad != 0 {
		d.Skip(pad)
	}
}

// Skip skips n bytes from the reader.
func (d SimpleDecoder) Skip(n uint64) {
	binary.ConsumeBytes(d.reader, n)
	d.offset += n
}

// Pointer loads and returns a pointer address.
func (d SimpleDecoder) Pointer() uint64 {
	d.alignAndOffset(d.memLayout.Pointer)
	return binary.ReadUint(d.reader, 8*d.memLayout.Pointer.Size)
}

// F32 loads and returns a float32.
func (d SimpleDecoder) F32() float32 {
	d.alignAndOffset(d.memLayout.F32)
	return d.reader.Float32()
}

// F64 loads and returns a float64.
func (d SimpleDecoder) F64() float64 {
	d.alignAndOffset(d.memLayout.F64)
	return d.reader.Float64()
}

// I8 loads and returns a int8.
func (d SimpleDecoder) I8() int8 {
	d.alignAndOffset(d.memLayout.I8)
	return d.reader.Int8()
}

// I16 loads and returns a int16.
func (d SimpleDecoder) I16() int16 {
	d.alignAndOffset(d.memLayout.I16)
	return d.reader.Int16()
}

// I32 loads and returns a int32.
func (d SimpleDecoder) I32() int32 {
	d.alignAndOffset(d.memLayout.I32)
	return d.reader.Int32()
}

// I64 loads and returns a int64.
func (d SimpleDecoder) I64() int64 {
	d.alignAndOffset(d.memLayout.I64)
	return d.reader.Int64()
}

// U8 loads and returns a uint8.
func (d SimpleDecoder) U8() uint8 {
	d.alignAndOffset(d.memLayout.I8)
	return d.reader.Uint8()
}

// U16 loads and returns a uint16.
func (d SimpleDecoder) U16() uint16 {
	d.alignAndOffset(d.memLayout.I16)
	return d.reader.Uint16()
}

// U32 loads and returns a uint32.
func (d SimpleDecoder) U32() uint32 {
	d.alignAndOffset(d.memLayout.I32)
	return d.reader.Uint32()
}

// U64 loads and returns a uint64.
func (d SimpleDecoder) U64() uint64 {
	d.alignAndOffset(d.memLayout.I64)
	return d.reader.Uint64()
}

// Char loads and returns an char.
func (d SimpleDecoder) Char() Char {
	d.alignAndOffset(d.memLayout.Char)
	return Char(binary.ReadInt(d.reader, 8*d.memLayout.Char.Size))
}

// Int loads and returns an int.
func (d SimpleDecoder) Int() Int {
	d.alignAndOffset(d.memLayout.Integer)
	return Int(binary.ReadInt(d.reader, 8*d.memLayout.Integer.Size))
}

// Uint loads and returns a uint.
func (d SimpleDecoder) Uint() Uint {
	d.alignAndOffset(d.memLayout.Integer)
	return Uint(binary.ReadUint(d.reader, 8*d.memLayout.Integer.Size))
}

// Size loads and returns a size_t.
func (d SimpleDecoder) Size() Size {
	d.alignAndOffset(d.memLayout.Size)
	return Size(binary.ReadUint(d.reader, 8*d.memLayout.Size.Size))
}

// String loads and returns a null-terminated string.
func (d SimpleDecoder) String() string {
	out := d.reader.String()
	d.offset += uint64(len(out) + 1)
	return out
}

// Bool loads and returns a boolean value.
func (d SimpleDecoder) Bool() bool {
	d.offset++
	return d.reader.Uint8() != 0
}

// Data reads raw bytes into buf.
func (d SimpleDecoder) Data(buf []byte) {
	d.reader.Data(buf)
	d.offset += uint64(len(buf))
}

// Error returns the error state of the underlying reader.
func (d SimpleDecoder) Error() error {
	return d.reader.Error()
}

func (d SimpleDecoder) SetError(err error) {
	return d.reader.SetError(err)
}