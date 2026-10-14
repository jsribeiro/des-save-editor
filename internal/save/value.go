package save

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// IsEditable reports whether the record holds a scalar or string value.
func (r *Record) IsEditable() bool {
	return r.Children == nil && r.Type != TypeBlob && r.Type != TypeVec4
}

// TypeName is a short label for the record's type.
func (r *Record) TypeName() string {
	switch r.Type {
	case TypeI8:
		return "i8"
	case TypeI16:
		return "i16"
	case TypeI32:
		return "i32"
	case TypeI64:
		return "i64"
	case TypeU8:
		return "u8"
	case TypeU16:
		return "u16"
	case TypeU32:
		return "u32"
	case TypeU64:
		return "u64"
	case TypeF32:
		return "f32"
	case TypeF64:
		return "f64"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeVec4:
		return "vec4"
	case TypeBlob:
		if r.Children != nil {
			return "object"
		}
		return "blob"
	}
	return fmt.Sprintf("0x%02x", r.Type)
}

func (r *Record) uint() uint64 {
	switch len(r.Data) {
	case 1:
		return uint64(r.Data[0])
	case 2:
		return uint64(binary.LittleEndian.Uint16(r.Data))
	case 4:
		return uint64(binary.LittleEndian.Uint32(r.Data))
	default:
		return binary.LittleEndian.Uint64(r.Data)
	}
}

func (r *Record) putUint(v uint64) {
	d := make([]byte, len(r.Data))
	switch len(d) {
	case 1:
		d[0] = byte(v)
	case 2:
		binary.LittleEndian.PutUint16(d, uint16(v))
	case 4:
		binary.LittleEndian.PutUint32(d, uint32(v))
	default:
		binary.LittleEndian.PutUint64(d, v)
	}
	// Replace rather than modify: Data may alias the original file buffer.
	r.Data = d
}

func (r *Record) bits() int { return len(r.Data) * 8 }

// Int returns the value of an integer record (signed types are sign-extended).
func (r *Record) Int() int64 {
	v := r.uint()
	if r.Type >= TypeI8 && r.Type <= TypeI64 {
		shift := 64 - r.bits()
		return int64(v<<shift) >> shift
	}
	return int64(v)
}

// String returns the value formatted for display and editing.
func (r *Record) String() string {
	switch r.Type {
	case TypeI8, TypeI16, TypeI32, TypeI64:
		return strconv.FormatInt(r.Int(), 10)
	case TypeU8, TypeU16, TypeU32, TypeU64:
		return strconv.FormatUint(r.uint(), 10)
	case TypeF32:
		return strconv.FormatFloat(float64(math.Float32frombits(uint32(r.uint()))), 'f', -1, 32)
	case TypeF64:
		return strconv.FormatFloat(math.Float64frombits(r.uint()), 'f', -1, 64)
	case TypeBool:
		return strconv.FormatBool(r.Data[0] != 0)
	case TypeString:
		return string(r.Data)
	case TypeVec4:
		f := make([]string, 4)
		for i := range f {
			f[i] = strconv.FormatFloat(float64(math.Float32frombits(binary.LittleEndian.Uint32(r.Data[i*4:]))), 'f', -1, 32)
		}
		return "(" + strings.Join(f, ", ") + ")"
	case TypeBlob:
		if r.Children != nil {
			return fmt.Sprintf("%d records", len(r.Children))
		}
		return fmt.Sprintf("%d bytes", len(r.Data))
	}
	return ""
}

// Check reports whether s is a valid value for the record without changing it.
func (r *Record) Check(s string) error {
	_, err := r.encode(s)
	return err
}

// Set parses s according to the record's type and stores it.
func (r *Record) Set(s string) error {
	d, err := r.encode(s)
	if err != nil {
		return err
	}
	r.Data = d
	return nil
}

// SetInt stores an integer value, checking it fits the record's type.
func (r *Record) SetInt(v int64) error {
	return r.Set(strconv.FormatInt(v, 10))
}

func (r *Record) encode(s string) ([]byte, error) {
	if !r.IsEditable() {
		return nil, fmt.Errorf("%s values cannot be edited", r.TypeName())
	}
	tmp := &Record{Type: r.Type, Data: make([]byte, len(r.Data))}
	s = strings.TrimSpace(s)
	switch r.Type {
	case TypeI8, TypeI16, TypeI32, TypeI64:
		v, err := strconv.ParseInt(s, 10, r.bits())
		if err != nil {
			return nil, fmt.Errorf("need a whole number that fits %s", r.TypeName())
		}
		tmp.putUint(uint64(v))
	case TypeU8, TypeU16, TypeU32, TypeU64:
		v, err := strconv.ParseUint(s, 10, r.bits())
		if err != nil {
			return nil, fmt.Errorf("need a non-negative whole number that fits %s", r.TypeName())
		}
		tmp.putUint(v)
	case TypeF32:
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return nil, fmt.Errorf("need a number")
		}
		tmp.putUint(uint64(math.Float32bits(float32(v))))
	case TypeF64:
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("need a number")
		}
		tmp.putUint(math.Float64bits(v))
	case TypeBool:
		v, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("need true or false")
		}
		tmp.Data = []byte{0}
		if v {
			tmp.Data[0] = 1
		}
	case TypeString:
		return []byte(s), nil
	}
	return tmp.Data, nil
}
