// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/rap

package rap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

var (
	rapSignature = [4]byte{0x00, 'r', 'a', 'P'}
)

// binaryReader provides bounds-checked little-endian reads.
type binaryReader struct {
	data []byte
	off  int
}

// newBinaryReader creates reader over source bytes.
func newBinaryReader(data []byte) *binaryReader {
	return &binaryReader{
		data: data,
	}
}

// pos returns current absolute byte offset.
func (r *binaryReader) pos() int {
	return r.off
}

// seekAbsolute moves reader to absolute offset.
func (r *binaryReader) seekAbsolute(off int) error {
	if off < 0 || off > len(r.data) {
		return fmt.Errorf("%w: seek out of bounds offset=%d size=%d", ErrInvalidRAP, off, len(r.data))
	}

	r.off = off

	return nil
}

// readByte reads one byte.
func (r *binaryReader) readByte() (byte, error) {
	if r.off >= len(r.data) {
		return 0, fmt.Errorf("%w: read byte out of bounds at %d", ErrInvalidRAP, r.off)
	}

	value := r.data[r.off]
	r.off++

	return value, nil
}

// readU32 reads little-endian uint32.
func (r *binaryReader) readU32() (uint32, error) {
	if r.off+4 > len(r.data) {
		return 0, fmt.Errorf("%w: read u32 out of bounds at %d", ErrInvalidRAP, r.off)
	}

	value := binary.LittleEndian.Uint32(r.data[r.off : r.off+4])
	r.off += 4

	return value, nil
}

// readI32 reads little-endian int32.
func (r *binaryReader) readI32() (int32, error) {
	if r.off+4 > len(r.data) {
		return 0, fmt.Errorf("%w: read i32 out of bounds at %d", ErrInvalidRAP, r.off)
	}

	var value int32
	value |= int32(r.data[r.off])
	value |= int32(r.data[r.off+1]) << 8
	value |= int32(r.data[r.off+2]) << 16
	value |= int32(r.data[r.off+3]) << 24
	r.off += 4

	return value, nil
}

// readI64 reads little-endian int64.
func (r *binaryReader) readI64() (int64, error) {
	if r.off+8 > len(r.data) {
		return 0, fmt.Errorf("%w: read i64 out of bounds at %d", ErrInvalidRAP, r.off)
	}

	//nolint:gosec // preserve signed two's-complement bit pattern from RAP payload
	value := int64(binary.LittleEndian.Uint64(r.data[r.off : r.off+8]))
	r.off += 8

	return value, nil
}

// readF32 reads little-endian float32.
func (r *binaryReader) readF32() (float32, error) {
	bits, err := r.readU32()
	if err != nil {
		return 0, err
	}

	return math.Float32frombits(bits), nil
}

// readCString reads zero-terminated string.
func (r *binaryReader) readCString() (string, error) {
	start := r.off
	endRel := bytes.IndexByte(r.data[start:], 0)
	if endRel < 0 {
		return "", fmt.Errorf("%w: unterminated asciiz at %d", ErrInvalidRAP, start)
	}

	end := start + endRel
	value := string(r.data[start:end])
	r.off = end + 1

	return value, nil
}

// readCompressedInt reads BIS compressed integer.
func (r *binaryReader) readCompressedInt() (int, error) {
	first, err := r.readByte()
	if err != nil {
		return 0, err
	}

	value := int(first)
	if first&0x80 == 0 {
		return value, nil
	}

	shift := 7
	current := first
	for current&0x80 != 0 {
		next, nextErr := r.readByte()
		if nextErr != nil {
			return 0, nextErr
		}

		value += (int(next) - 1) << shift
		current = next
		shift += 7

		if shift > 28 {
			return 0, fmt.Errorf("%w: compressed int overflow", ErrInvalidRAP)
		}
	}

	return value, nil
}

// binaryWriter appends little-endian primitives.
type binaryWriter struct {
	buf []byte
}

// newBinaryWriterWithCapacity creates empty writer with requested capacity.
func newBinaryWriterWithCapacity(capacity int) *binaryWriter {
	if capacity < 1024 {
		capacity = 1024
	}

	return &binaryWriter{
		buf: make([]byte, 0, capacity),
	}
}

// bytes returns internal output bytes.
func (w *binaryWriter) bytes() []byte {
	return w.buf
}

// pos returns current absolute byte offset.
func (w *binaryWriter) pos() int {
	return len(w.buf)
}

// writeByte appends one byte.
func (w *binaryWriter) writeByte(value byte) {
	w.buf = append(w.buf, value)
}

// writeU32 appends little-endian uint32.
func (w *binaryWriter) writeU32(value uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], value)
	w.buf = append(w.buf, tmp[:]...)
}

// writeI32 appends little-endian int32.
func (w *binaryWriter) writeI32(value int32) {
	var tmp [4]byte
	//nolint:gosec // preserve signed two's-complement bit pattern into RAP payload
	binary.LittleEndian.PutUint32(tmp[:], uint32(value))
	w.buf = append(w.buf, tmp[:]...)
}

// writeI64 appends little-endian int64.
func (w *binaryWriter) writeI64(value int64) {
	var tmp [8]byte
	//nolint:gosec // preserve signed two's-complement bit pattern into RAP payload
	binary.LittleEndian.PutUint64(tmp[:], uint64(value))
	w.buf = append(w.buf, tmp[:]...)
}

// writeF32 appends little-endian float32.
func (w *binaryWriter) writeF32(value float32) {
	w.writeU32(math.Float32bits(value))
}

// writeCString appends zero-terminated string.
func (w *binaryWriter) writeCString(value string) {
	w.buf = append(w.buf, value...)
	w.buf = append(w.buf, 0)
}

// patchU32Int updates little-endian uint32 at absolute byte offset from int value.
func (w *binaryWriter) patchU32Int(at int, value int) error {
	if at < 0 || at+4 > len(w.buf) {
		return fmt.Errorf("%w: patch u32 out of bounds at %d", ErrInvalidRAP, at)
	}

	if value < 0 || uint64(value) > uint64(math.MaxUint32) {
		return fmt.Errorf("%w: patch u32 value out of range=%d", ErrInvalidRAP, value)
	}

	var tmp [4]byte
	//nolint:gosec // value range is validated against uint32 bounds above
	binary.LittleEndian.PutUint32(tmp[:], uint32(value))
	copy(w.buf[at:at+4], tmp[:])

	return nil
}

// writeCompressedInt writes BIS compressed integer.
func (w *binaryWriter) writeCompressedInt(value int) error {
	if value < 0 {
		return fmt.Errorf("%w: negative compressed int %d", ErrInvalidRAP, value)
	}

	if value < 0x80 {
		w.writeByte(byte(value))

		return nil
	}

	first := 0x80 + (value % 128)
	w.writeByte(byte(first))

	// Tail bytes are encoded as compressed integer of shifted remainder + 1.
	// This matches decode equation:
	// value = first + ((tailDecoded - 1) << 7)
	tailValue := ((value - first) >> 7) + 1

	return w.writeCompressedInt(tailValue)
}
