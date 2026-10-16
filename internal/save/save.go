// Package save reads and writes Demon's Souls (2020 remake) USR-DATA save files.
//
// File layout (all little-endian):
//
//	0x00 u32  version (7)
//	0x04 u32  checksum: FNV-1a 32 of every byte from 0x10 to EOF
//	0x08 u32  length of the data from 0x10 to EOF
//	0x0C u32  unknown (1)
//	0x10 ...  records
//
// Each record is [u32 key][u8 type][value]. The key is the FNV-1a 32 hash of an
// UPPER_SNAKE_CASE field name. Strings and blobs carry a LEB128 length prefix;
// blobs often contain a nested record list.
package save

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
)

const headerSize = 0x10

// Record types. Widths are certain; signedness of the integer types is a guess.
const (
	TypeI8     byte = 0x02
	TypeI16    byte = 0x03
	TypeI32    byte = 0x04
	TypeI64    byte = 0x05
	TypeU8     byte = 0x06
	TypeU16    byte = 0x07
	TypeU32    byte = 0x08
	TypeU64    byte = 0x09
	TypeF32    byte = 0x0a
	TypeF64    byte = 0x0b
	TypeBool   byte = 0x0c
	TypeString byte = 0x0d
	TypeBlob   byte = 0x0e
	TypeVec4   byte = 0x0f
)

var fixedSize = map[byte]int{
	TypeI8: 1, TypeI16: 2, TypeI32: 4, TypeI64: 8,
	TypeU8: 1, TypeU16: 2, TypeU32: 4, TypeU64: 8,
	TypeF32: 4, TypeF64: 8, TypeBool: 1, TypeVec4: 16,
}

// Record is one key/type/value entry.
type Record struct {
	Key  uint32
	Type byte
	// Offset of the record in the file it was parsed from.
	Offset int
	// Data holds the raw value bytes: the fixed-width value, the string bytes,
	// or the blob bytes when the blob is not a nested record list.
	Data []byte
	// Children is non-nil when a blob was parsed as a nested record list.
	Children []*Record
}

// Save is a parsed USR-DATA file.
type Save struct {
	Version   uint32
	Unknown0C uint32
	Records   []*Record
}

var ErrChecksum = errors.New("checksum mismatch")

// Checksum computes the FNV-1a 32 hash used for both keys and the file checksum.
func Checksum(b []byte) uint32 {
	h := fnv.New32a()
	h.Write(b)
	return h.Sum32()
}

// Parse decodes a save file and verifies its length and checksum.
func Parse(b []byte) (*Save, error) {
	if len(b) < headerSize {
		return nil, fmt.Errorf("file too small (%d bytes)", len(b))
	}
	s := &Save{
		Version:   binary.LittleEndian.Uint32(b[0:]),
		Unknown0C: binary.LittleEndian.Uint32(b[0x0c:]),
	}
	stored := binary.LittleEndian.Uint32(b[4:])
	length := binary.LittleEndian.Uint32(b[8:])
	if int(length) != len(b)-headerSize {
		return nil, fmt.Errorf("header length %d does not match file size %d", length, len(b))
	}
	if sum := Checksum(b[headerSize:]); sum != stored {
		return nil, fmt.Errorf("%w: stored %08x, computed %08x", ErrChecksum, stored, sum)
	}
	recs, err := parseRecords(b, headerSize, len(b))
	if err != nil {
		return nil, err
	}
	s.Records = recs
	return s, nil
}

func readVarint(b []byte, off, end int) (uint64, int, error) {
	var v uint64
	for shift := 0; off < end && shift < 64; shift += 7 {
		c := b[off]
		off++
		v |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return v, off, nil
		}
	}
	return 0, off, errors.New("bad varint")
}

func appendVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

func parseRecords(b []byte, off, end int) ([]*Record, error) {
	recs := []*Record{}
	for off < end {
		if off+5 > end {
			return nil, fmt.Errorf("truncated record at 0x%x", off)
		}
		r := &Record{Key: binary.LittleEndian.Uint32(b[off:]), Type: b[off+4], Offset: off}
		p := off + 5
		if n, ok := fixedSize[r.Type]; ok {
			if p+n > end {
				return nil, fmt.Errorf("truncated value at 0x%x", off)
			}
			r.Data = b[p : p+n]
			off = p + n
		} else if r.Type == TypeString || r.Type == TypeBlob {
			n, q, err := readVarint(b, p, end)
			if err != nil || uint64(end-q) < n {
				return nil, fmt.Errorf("bad length at 0x%x", off)
			}
			r.Data = b[q : q+int(n)]
			off = q + int(n)
			if r.Type == TypeBlob && n > 0 {
				// Not every blob is a record list (e.g. raw vectors); keep it raw if it doesn't parse.
				if kids, err := parseRecords(b, q, off); err == nil {
					r.Children = kids
					r.Data = nil
				}
			}
		} else {
			return nil, fmt.Errorf("unknown type 0x%02x at 0x%x", r.Type, off)
		}
		recs = append(recs, r)
	}
	return recs, nil
}

func appendRecords(dst []byte, recs []*Record) []byte {
	for _, r := range recs {
		dst = binary.LittleEndian.AppendUint32(dst, r.Key)
		dst = append(dst, r.Type)
		switch {
		case r.Children != nil:
			body := appendRecords(nil, r.Children)
			dst = appendVarint(dst, uint64(len(body)))
			dst = append(dst, body...)
		case r.Type == TypeString || r.Type == TypeBlob:
			dst = appendVarint(dst, uint64(len(r.Data)))
			dst = append(dst, r.Data...)
		default:
			dst = append(dst, r.Data...)
		}
	}
	return dst
}

// Bytes serializes the save, recomputing nested lengths, the header length and the checksum.
func (s *Save) Bytes() []byte {
	body := appendRecords(nil, s.Records)
	out := make([]byte, headerSize, headerSize+len(body))
	binary.LittleEndian.PutUint32(out[0:], s.Version)
	binary.LittleEndian.PutUint32(out[4:], Checksum(body))
	binary.LittleEndian.PutUint32(out[8:], uint32(len(body)))
	binary.LittleEndian.PutUint32(out[0x0c:], s.Unknown0C)
	return append(out, body...)
}

// Find returns the first top-level record with the given key, or nil.
func (s *Save) Find(key uint32) *Record {
	for _, r := range s.Records {
		if r.Key == key {
			return r
		}
	}
	return nil
}

// FindName returns the first top-level record whose key is the hash of name, or nil.
func (s *Save) FindName(name string) *Record {
	return s.Find(Hash(name))
}

// FindPath follows keys through nested objects, starting at the top level, and returns the record or nil.
func (s *Save) FindPath(keys ...uint32) *Record {
	recs := s.Records
	var found *Record
	for _, k := range keys {
		found = nil
		for _, r := range recs {
			if r.Key == k {
				found = r
				break
			}
		}
		if found == nil {
			return nil
		}
		recs = found.Children
	}
	return found
}

// Walk visits every record depth-first.
func (s *Save) Walk(fn func(r *Record, depth int)) {
	var walk func([]*Record, int)
	walk = func(recs []*Record, depth int) {
		for _, r := range recs {
			fn(r, depth)
			if r.Children != nil {
				walk(r.Children, depth+1)
			}
		}
	}
	walk(s.Records, 0)
}
