// Package protowire provides minimal protobuf wire-format encoding/decoding.
//
// This is an internal package implementing just enough protobuf encoding to
// communicate with Steam's IAuthenticationService endpoints, without pulling in
// the full google.golang.org/protobuf dependency.
//
// We only need varint, string, bytes, and fixed-width encodings — which is
// straightforward to implement by hand.
package protowire

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Wire types
const (
	WireVarint  = 0
	Wire64Bit   = 1
	WireBytes   = 2
	Wire32Bit   = 5
)

// Encoder builds a protobuf message by appending fields.
type Encoder struct {
	buf []byte
}

// NewEncoder creates a new protobuf encoder.
func NewEncoder() *Encoder {
	return &Encoder{}
}

// Bytes returns the encoded protobuf message.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// Reset clears the encoder for reuse.
func (e *Encoder) Reset() {
	e.buf = e.buf[:0]
}

// tag encodes a field tag (field_number << 3 | wire_type).
func (e *Encoder) tag(fieldNumber int, wireType int) {
	e.appendVarint(uint64(fieldNumber<<3 | wireType))
}

// appendVarint appends a varint-encoded uint64.
func (e *Encoder) appendVarint(v uint64) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	e.buf = append(e.buf, buf[:n]...)
}

// --- Field encoders ---

// EncodeString encodes a string field (wire type 2).
func (e *Encoder) EncodeString(fieldNumber int, value string) {
	if value == "" {
		return
	}
	e.tag(fieldNumber, WireBytes)
	e.appendVarint(uint64(len(value)))
	e.buf = append(e.buf, value...)
}

// EncodeBytes encodes a bytes field (wire type 2).
func (e *Encoder) EncodeBytes(fieldNumber int, value []byte) {
	if len(value) == 0 {
		return
	}
	e.tag(fieldNumber, WireBytes)
	e.appendVarint(uint64(len(value)))
	e.buf = append(e.buf, value...)
}

// EncodeUint64 encodes a uint64 varint field (wire type 0).
func (e *Encoder) EncodeUint64(fieldNumber int, value uint64) {
	if value == 0 {
		return
	}
	e.tag(fieldNumber, WireVarint)
	e.appendVarint(value)
}

// EncodeInt32 encodes an int32 varint field (wire type 0).
// Handles negative values using two's complement (standard protobuf signed encoding).
func (e *Encoder) EncodeInt32(fieldNumber int, value int32) {
	if value == 0 {
		return
	}
	e.tag(fieldNumber, WireVarint)
	e.appendVarint(uint64(value))
}

// EncodeUint32 encodes a uint32 varint field (wire type 0).
func (e *Encoder) EncodeUint32(fieldNumber int, value uint32) {
	if value == 0 {
		return
	}
	e.tag(fieldNumber, WireVarint)
	e.appendVarint(uint64(value))
}

// EncodeBool encodes a bool field (wire type 0, varint 0 or 1).
func (e *Encoder) EncodeBool(fieldNumber int, value bool) {
	if !value {
		return
	}
	e.tag(fieldNumber, WireVarint)
	e.appendVarint(1)
}

// EncodeFixed64 encodes a fixed64 field (wire type 1).
func (e *Encoder) EncodeFixed64(fieldNumber int, value uint64) {
	if value == 0 {
		return
	}
	e.tag(fieldNumber, Wire64Bit)
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], value)
	e.buf = append(e.buf, buf[:]...)
}

// EncodeFloat encodes a float field (wire type 5, fixed32).
func (e *Encoder) EncodeFloat(fieldNumber int, value float32) {
	if value == 0 {
		return
	}
	e.tag(fieldNumber, Wire32Bit)
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(value))
	e.buf = append(e.buf, buf[:]...)
}

// EncodeEnum encodes an enum field (same as int32 varint).
func (e *Encoder) EncodeEnum(fieldNumber int, value int) {
	e.EncodeInt32(fieldNumber, int32(value))
}

// EncodeMessage encodes a sub-message field (wire type 2).
func (e *Encoder) EncodeMessage(fieldNumber int, subEncoder *Encoder) {
	data := subEncoder.Bytes()
	if len(data) == 0 {
		return
	}
	e.tag(fieldNumber, WireBytes)
	e.appendVarint(uint64(len(data)))
	e.buf = append(e.buf, data...)
}

// --- Decoder ---

// Decoder reads protobuf fields from a byte slice.
type Decoder struct {
	buf []byte
	pos int
}

// NewDecoder creates a new protobuf decoder.
func NewDecoder(data []byte) *Decoder {
	return &Decoder{buf: data}
}

// Done returns true when all data has been consumed.
func (d *Decoder) Done() bool {
	return d.pos >= len(d.buf)
}

// Field reads the next field tag and returns the field number, wire type, and raw value.
func (d *Decoder) Field() (fieldNumber int, wireType int, err error) {
	if d.Done() {
		return 0, 0, fmt.Errorf("protowire: unexpected end of data")
	}

	tag, err := d.readVarint()
	if err != nil {
		return 0, 0, err
	}

	fieldNumber = int(tag >> 3)
	wireType = int(tag & 0x7)
	return fieldNumber, wireType, nil
}

// ReadVarint reads a varint value (for WireVarint fields).
func (d *Decoder) ReadVarint() (uint64, error) {
	return d.readVarint()
}

// ReadFixed64 reads a fixed64 value (for Wire64Bit fields).
func (d *Decoder) ReadFixed64() (uint64, error) {
	if d.pos+8 > len(d.buf) {
		return 0, fmt.Errorf("protowire: not enough data for fixed64")
	}
	v := binary.LittleEndian.Uint64(d.buf[d.pos : d.pos+8])
	d.pos += 8
	return v, nil
}

// ReadFixed32 reads a fixed32 value (for Wire32Bit fields).
func (d *Decoder) ReadFixed32() (uint32, error) {
	if d.pos+4 > len(d.buf) {
		return 0, fmt.Errorf("protowire: not enough data for fixed32")
	}
	v := binary.LittleEndian.Uint32(d.buf[d.pos : d.pos+4])
	d.pos += 4
	return v, nil
}

// ReadBytes reads a length-delimited field (for WireBytes fields — strings, bytes, sub-messages).
func (d *Decoder) ReadBytes() ([]byte, error) {
	length, err := d.readVarint()
	if err != nil {
		return nil, err
	}
	end := d.pos + int(length)
	if end > len(d.buf) {
		return nil, fmt.Errorf("protowire: not enough data for bytes field (need %d, have %d)", length, len(d.buf)-d.pos)
	}
	data := d.buf[d.pos:end]
	d.pos = end
	return data, nil
}

// ReadString reads a string field.
func (d *Decoder) ReadString() (string, error) {
	b, err := d.ReadBytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Skip skips over a field value based on its wire type.
func (d *Decoder) Skip(wireType int) error {
	switch wireType {
	case WireVarint:
		_, err := d.readVarint()
		return err
	case Wire64Bit:
		if d.pos+8 > len(d.buf) {
			return fmt.Errorf("protowire: not enough data to skip fixed64")
		}
		d.pos += 8
	case WireBytes:
		_, err := d.ReadBytes()
		return err
	case Wire32Bit:
		if d.pos+4 > len(d.buf) {
			return fmt.Errorf("protowire: not enough data to skip fixed32")
		}
		d.pos += 4
	default:
		return fmt.Errorf("protowire: unknown wire type %d", wireType)
	}
	return nil
}

func (d *Decoder) readVarint() (uint64, error) {
	var result uint64
	var shift uint
	for {
		if d.pos >= len(d.buf) {
			return 0, fmt.Errorf("protowire: unexpected end of data reading varint")
		}
		b := d.buf[d.pos]
		d.pos++
		result |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return result, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("protowire: varint too large")
		}
	}
}
