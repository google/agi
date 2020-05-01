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

package binary

import (
	"io"
	"fmt"
	gobinary "encoding/binary"

	"github.com/google/gapid/core/math/f16"
)

// Reader provides methods for decoding values.
type Reader interface {
	io.Reader
	// Data reads the data bytes in their entirety.
	Data([]byte)
	// Bool decodes and returns a boolean value from the Reader.
	Bool() bool
	// Int8 decodes and returns a signed, 8 bit integer value from the Reader.
	Int8() int8
	// Uint8 decodes and returns an unsigned, 8 bit integer value from the Reader.
	Uint8() uint8
	// Int16 decodes and returns a signed, 16 bit integer value from the Reader.
	Int16() int16
	// Uint16 decodes and returns an unsigned, 16 bit integer value from the Reader.
	Uint16() uint16
	// Int32 decodes and returns a signed, 32 bit integer value from the Reader.
	Int32() int32
	// Uint32 decodes and returns an unsigned, 32 bit integer value from the Reader.
	Uint32() uint32
	// Float16 decodes and returns a 16 bit floating-point value from the Reader.
	Float16() f16.Number
	// Float32 decodes and returns a 32 bit floating-point value from the Reader.
	Float32() float32
	// Int64 decodes and returns a signed, 64 bit integer value from the Reader.
	Int64() int64
	// Uint64 decodes and returns an unsigned, 64 bit integer value from the Reader.
	Uint64() uint64
	// Float64 decodes and returns a 64 bit floating-point value from the Reader.
	Float64() float64
	// String decodes and returns a string from the Reader.
	String() string
	// Decode a collection count from the stream.
	Count() uint32
	// If there is an error reading any input, all further reading returns the
	// zero value of the type read. Error() returns the error which stopped
	// reading from the stream. If reading has not stopped it returns nil.
	Error() error
	// Set the error state and stop reading from the stream.
	SetError(error)
}

// Data reads the data bytes in their entirety.
func ReadData(r io.Reader, data []byte) error {
	_, err := io.ReadFull(r, data)
	return err
}
// Bool decodes and returns a boolean value from the Reader.
func ReadBool(r io.Reader) (bool, error) {
	var val bool
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Int8 decodes and returns a signed, 8 bit integer value from the Reader.
func ReadInt8(r io.Reader) (int8, error) {
	var val int8
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Uint8 decodes and returns an unsigned, 8 bit integer value from the Reader.
func ReadUint8(r io.Reader) (uint8, error) {
	var val uint8
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Int16 decodes and returns a signed, 16 bit integer value from the Reader.
func ReadInt16(r io.Reader) (int16, error) {
	var val int16
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Uint16 decodes and returns an unsigned, 16 bit integer value from the Reader.
func ReadUint16(r io.Reader) (uint16, error) {
	var val uint16
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Int32 decodes and returns a signed, 32 bit integer value from the Reader.
func ReadInt32(r io.Reader) (int32, error) {
	var val int32
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Uint32 decodes and returns an unsigned, 32 bit integer value from the Reader.
func ReadUint32(r io.Reader) (uint32, error) {
	var val uint32
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Float16 decodes and returns a 16 bit floating-point value from the Reader.
func ReadFloat16(r io.Reader) (f16.Number, error) {
	var val f16.Number
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Float32 decodes and returns a 32 bit floating-point value from the Reader.
func ReadFloat32(r io.Reader) (float32, error) {
	var val float32
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Int64 decodes and returns a signed, 64 bit integer value from the Reader.
func ReadInt64(r io.Reader) (int64, error) {
	var val int64
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Uint64 decodes and returns an unsigned, 64 bit integer value from the Reader.
func ReadUint64(r io.Reader) (uint64, error) {
	var val uint64
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Float64 decodes and returns a 64 bit floating-point value from the Reader.
func ReadFloat64(r io.Reader) (float64, error) {
	var val float64
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// String decodes and returns a string from the Reader.
func ReadString(r io.Reader) (string, error) {
	var val string
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}
// Decode a collection count from the stream.
func ReadCount(r io.Reader) (uint32, error) {
	var val uint32
	err := gobinary.Read(r, gobinary.LittleEndian, &val)
	return val, err
}


// Bool decodes and returns a boolean value from the Reader.
func ReadBEBool(r io.Reader) (bool, error) {
	var val bool
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Int8 decodes and returns a signed, 8 bit integer value from the Reader.
func ReadBEInt8(r io.Reader) (int8, error) {
	var val int8
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Uint8 decodes and returns an unsigned, 8 bit integer value from the Reader.
func ReadBEUint8(r io.Reader) (uint8, error) {
	var val uint8
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Int16 decodes and returns a signed, 16 bit integer value from the Reader.
func ReadBEInt16(r io.Reader) (int16, error) {
	var val int16
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Uint16 decodes and returns an unsigned, 16 bit integer value from the Reader.
func ReadBEUint16(r io.Reader) (uint16, error) {
	var val uint16
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Int32 decodes and returns a signed, 32 bit integer value from the Reader.
func ReadBEInt32(r io.Reader) (int32, error) {
	var val int32
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Uint32 decodes and returns an unsigned, 32 bit integer value from the Reader.
func ReadBEUint32(r io.Reader) (uint32, error) {
	var val uint32
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Float16 decodes and returns a 16 bit floating-point value from the Reader.
func ReadBEFloat16(r io.Reader) (f16.Number, error) {
	var val f16.Number
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Float32 decodes and returns a 32 bit floating-point value from the Reader.
func ReadBEFloat32(r io.Reader) (float32, error) {
	var val float32
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Int64 decodes and returns a signed, 64 bit integer value from the Reader.
func ReadBEInt64(r io.Reader) (int64, error) {
	var val int64
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Uint64 decodes and returns an unsigned, 64 bit integer value from the Reader.
func ReadBEUint64(r io.Reader) (uint64, error) {
	var val uint64
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Float64 decodes and returns a 64 bit floating-point value from the Reader.
func ReadBEFloat64(r io.Reader) (float64, error) {
	var val float64
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// String decodes and returns a string from the Reader.
func ReadBEString(r io.Reader) (string, error) {
	var val string
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}
// Decode a collection count from the stream.
func ReadBECount(r io.Reader) (uint32, error) {
	var val uint32
	err := gobinary.Read(r, gobinary.BigEndian, &val)
	return val, err
}

// ReadUint reads an unsigned integer of either 8, 16, 32 or 64 bits from r,
// returning the result as a uint64.
// ALAN: We don't handle errors here.
func ReadUint(r io.Reader, bits int32) (uint64, error) {
	switch bits {
	case 8:
		val, err := ReadUint8(r)
		return uint64(val), err
	case 16:
		val, err := ReadUint16(r)
		return uint64(val), err
	case 32:
		val, err := ReadUint32(r)
		return uint64(val), err
	case 64:
		return ReadUint64(r)
	default:
		return 0, fmt.Errorf("Unsupported integer bit count")
	}
}

// ReadInt reads a signed integer of either 8, 16, 32 or 64 bits from r,
// returning the result as a int64.
// ALAN: We don't handle errors here.
func ReadInt(r io.Reader, bits int32) (int64, error) {
	switch bits {
	case 8:
		val, err := ReadInt8(r)
		return int64(val), err
	case 16:
		val, err := ReadInt16(r)
		return int64(val), err
	case 32:
		val, err := ReadInt32(r)
		return int64(val), err
	case 64:
		return ReadInt64(r)
	default:
		return 0, fmt.Errorf("Unsupported integer bit count")
	}
}

// ConsumeBytes reads and throws away a number of bytes from r, returning the
// number of bytes it consumed.
// ALAN: We don't handle errors here. What's the point of the return value?
func ConsumeBytes(r io.Reader, bytes uint64) (uint64, error) {
	for i := uint64(0); i < bytes; i++ {
		_, err := ReadUint8(r)
		if err != nil {
			return i, fmt.Errorf("Error reading bytes")
		}
	}
	return bytes, nil
}
