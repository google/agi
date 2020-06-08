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

package endian

import (
	eb "encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/google/gapid/core/data/binary"
	"github.com/google/gapid/core/math/f16"
	"github.com/google/gapid/core/os/device"
)

func byteOrder(endian device.Endian) eb.ByteOrder {
	switch endian {
	case device.LittleEndian:
		return eb.LittleEndian
	case device.BigEndian:
		return eb.BigEndian
	default:
		return eb.LittleEndian
	}
}

// Reader creates a binary.Reader that reads from the provided io.Reader, with
// the specified byte order.
func Reader(r io.Reader, endian device.Endian) binary.Reader {
	return &reader{reader: r, byteOrder: byteOrder(endian)}
}

// Reader creates a binary.Reader that reads from the provided io.Reader, with
// the specified byte order.
func ReaderForBytes(data []byte, endian device.Endian) binary.Reader {
	return &bytesReader{data: data, byteOrder: byteOrder(endian)}
}

// Writer creates a binary.Writer that writes to the supplied stream, with the
// specified byte order.
func Writer(w io.Writer, endian device.Endian) binary.Writer {
	return &writer{writer: w, byteOrder: byteOrder(endian)}
}

type reader struct {
	reader    io.Reader
	tmp       [8]byte
	byteOrder eb.ByteOrder
	err       error
}

type bytesReader struct {
	data []byte
	head int
	byteOrder eb.ByteOrder
	err       error
}

type writer struct {
	writer    io.Writer
	tmp       [8]byte
	byteOrder eb.ByteOrder
	err       error
}

func (r *reader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	n := len(p)

	if r.head +n > len(r.data) {
		r.err = fmt.Errorf("error after reading %d dynamic bytes", n)
		return 0, r.err
	}

	for i := 0; i < n; i++ {
		p[i] = r.data[r.head +i]
	}

	r.head += n
	return n, nil
}

func (r *reader) Data(p []byte) {
	if r.err != nil {
		return
	}
	n, err := io.ReadFull(r.reader, p)
	if err != nil {
		r.err = err
		err = fmt.Errorf("%v after reading %d dynamic bytes", err, n)
	}
}

func (r *bytesReader) Data(p []byte) {
	r.Read(p)
}

func (w *writer) Data(data []byte) {
	if w.err != nil {
		return
	}
	n, err := w.writer.Write(data)
	if err != nil {
		w.err = err
	} else if n != len(data) {
		w.err = io.ErrShortWrite
	}
}

func (r *reader) Bool() bool {
	return r.Uint8() != 0
}

func (r *bytesReader) Bool() bool {
	return r.Uint8() != 0
}

func (w *writer) Bool(v bool) {
	if v {
		w.Uint8(1)
	} else {
		w.Uint8(0)
	}
}

func (r *reader) Int8() int8 {
	return int8(r.Uint8())
}

func (r *bytesReader) Int8() int8 {
	return int8(r.Uint8())
}

func (w *writer) Int8(v int8) {
	w.Uint8(uint8(v))
}

func (r *reader) Uint8() uint8 {
	if r.err != nil {
		return 0
	}
	b := r.tmp[:1]
	_, r.err = io.ReadFull(r.reader, b[:1])
	return b[0]
}

func (r *bytesReader) Uint8() uint8 {
	if r.err != nil {
		return 0
	}
	if r.head +1 > len(r.data) {
		//panic("AAAA")
		r.err = fmt.Errorf("error after reading 1 byte type")
		return 0
	}
	ret := r.data[r.head]
	r.head += 1
	return ret
}

func (w *writer) Uint8(v uint8) {
	w.tmp[0] = v
	w.Data(w.tmp[:1])
}

func (r *reader) Int16() int16 {
	if r.err != nil {
		return 0
	}
	_, r.err = io.ReadFull(r.reader, r.tmp[:2])
	return int16(r.byteOrder.Uint16(r.tmp[:]))
}

func (r *bytesReader) Int16() int16 {
	return int16(r.Uint16())
}

func (w *writer) Int16(v int16) {
	if w.err != nil {
		return
	}
	w.byteOrder.PutUint16(w.tmp[:], uint16(v))
	_, w.err = w.writer.Write(w.tmp[:2])
}

func (r *reader) Uint16() uint16 {
	if r.err != nil {
		return 0
	}
	_, r.err = io.ReadFull(r.reader, r.tmp[:2]) // ALAN: Handle error if we don't get enough bytes. Do so everywhere in sister functions.
	return r.byteOrder.Uint16(r.tmp[:])
}

func (r *bytesReader) Uint16() uint16 {
	if r.err != nil {
		return 0
	}
	if r.head +2 > len(r.data) {
		r.err = fmt.Errorf("error after reading 2 byte type")
		return 0
	}
	ret := r.byteOrder.Uint16(r.data[r.head : r.head +2])
	r.head += 2
	return ret
}

func (w *writer) Uint16(v uint16) {
	if w.err != nil {
		return
	}
	w.byteOrder.PutUint16(w.tmp[:], v)
	_, w.err = w.writer.Write(w.tmp[:2])
}

func (r *reader) Int32() int32 {
	if r.err != nil {
		return 0
	}
	_, r.err = io.ReadFull(r.reader, r.tmp[:4])
	return int32(r.byteOrder.Uint32(r.tmp[:]))
}

func (r *bytesReader) Int32() int32 {
	return int32(r.Uint32())
}

func (w *writer) Int32(v int32) {
	if w.err != nil {
		return
	}
	w.byteOrder.PutUint32(w.tmp[:], uint32(v))
	_, w.err = w.writer.Write(w.tmp[:4])
}

func (r *reader) Uint32() uint32 {
	if r.err != nil {
		return 0
	}
	_, r.err = io.ReadFull(r.reader, r.tmp[:4])
	return r.byteOrder.Uint32(r.tmp[:])
}

func (r *bytesReader) Uint32() uint32 {
	if r.err != nil {
		return 0
	}
	if r.head +4 > len(r.data) {
		r.err = fmt.Errorf("error after reading 4 byte type")
		return 0
	}
	ret := r.byteOrder.Uint32(r.data[r.head : r.head +4])
	r.head += 4
	return ret
}

func (w *writer) Uint32(v uint32) {
	if w.err != nil {
		return
	}
	w.byteOrder.PutUint32(w.tmp[:], v)
	_, w.err = w.writer.Write(w.tmp[:4])
}

func (r *reader) Int64() int64 {
	if r.err != nil {
		return 0
	}
	_, r.err = io.ReadFull(r.reader, r.tmp[:8])
	return int64(r.byteOrder.Uint64(r.tmp[:]))
}

func (r *bytesReader) Int64() int64 {
	return int64(r.Uint64())
}

func (w *writer) Int64(v int64) {
	if w.err != nil {
		return
	}
	w.byteOrder.PutUint64(w.tmp[:], uint64(v))
	_, w.err = w.writer.Write(w.tmp[:8])
}

func (r *reader) Uint64() uint64 {
	if r.err != nil {
		return 0
	}
	_, r.err = io.ReadFull(r.reader, r.tmp[:8])
	return r.byteOrder.Uint64(r.tmp[:])
}

func (r *bytesReader) Uint64() uint64 {
	if r.err != nil {
		return 0
	}
	if r.head +8 > len(r.data) {
		r.err = fmt.Errorf("error after reading 8 byte type")
		return 0
	}
	ret := r.byteOrder.Uint64(r.data[r.head : r.head +8])
	r.head += 8
	return ret
}

func (w *writer) Uint64(v uint64) {
	if w.err != nil {
		return
	}
	w.byteOrder.PutUint64(w.tmp[:], v)
	_, w.err = w.writer.Write(w.tmp[:8])
}

func (r *reader) Float16() f16.Number {
	return f16.Number(r.Uint16())
}

func (r *bytesReader) Float16() f16.Number {
	return f16.Number(r.Uint16())
}

func (w *writer) Float16(v f16.Number) {
	w.Uint16(uint16(v))
}

func (r *reader) Float32() float32 {
	if r.err != nil {
		return 0
	}
	_, r.err = io.ReadFull(r.reader, r.tmp[:4])
	return math.Float32frombits(r.byteOrder.Uint32(r.tmp[:]))
}

func (r *bytesReader) Float32() float32 {
	return math.Float32frombits(r.Uint32())
}

func (w *writer) Float32(v float32) {
	if w.err != nil {
		return
	}
	w.byteOrder.PutUint32(w.tmp[:], math.Float32bits(v))
	_, w.err = w.writer.Write(w.tmp[:4])
}

func (r *reader) Float64() float64 {
	if r.err != nil {
		return 0
	}
	_, r.err = io.ReadFull(r.reader, r.tmp[:8])
	return math.Float64frombits(r.byteOrder.Uint64(r.tmp[:]))
}

func (r *bytesReader) Float64() float64 {
	return math.Float64frombits(r.Uint64())
}

func (w *writer) Float64(v float64) {
	if w.err != nil {
		return
	}
	w.byteOrder.PutUint64(w.tmp[:], math.Float64bits(v))
	_, w.err = w.writer.Write(w.tmp[:8])
}

func (r *reader) String() string {
	s := []byte{}
	for {
		c := r.Uint8()
		if c == 0 {
			break
		}
		s = append(s, c)
	}
	return string(s)
}

func (r *bytesReader) String() string {
	s := []byte{}
	for {
		c := r.Uint8()
		if c == 0 {
			break
		}
		s = append(s, c)
	}
	return string(s)
}

func (w *writer) String(v string) {
	if w.err != nil {
		return
	}
	w.writer.Write([]byte(v))
	w.Uint8(0)
}

func (r *reader) Count() uint32 {
	return r.Uint32()
}

func (r *bytesReader) Count() uint32 {
	return r.Uint32()
}

func (r *reader) Error() error {
	return r.err
}

func (w *bytesReader) Error() error {
	return w.err
}

func (r *writer) Error() error {
	return r.err
}

func (r *reader) SetError(err error) {
	if r.err != nil {
		return
	}
	r.err = err
}

func (r *bytesReader) SetError(err error) {
	if r.err != nil {
		return
	}
	r.err = err
}

func (w *writer) SetError(err error) {
	if w.err != nil {
		return
	}
	w.err = err
}
