// Package beatnik defines data objects and a text language for encoding midi
// drum tracks.
package beatnik

// Type definitions.

import (
	"bytes"
	"encoding/binary"
)

// TODO(amit): Add velocity (volume) to individual notes.

// A Track is an entire drum track, with its drum data and metadata.
type Track struct {
	Hits []*Hit // Order of hits in this track.
	BPM  uint   // Track tempo.
}

// MarshalBinary returns a binary encoding of the track as a complete midi file.
func (t *Track) MarshalBinary() []byte {
	buf := bytes.NewBuffer(nil)
	buf.Write(t.encodeHeaderChunk())
	buf.Write(t.encodeMetaChunk())
	buf.Write(t.encodeHits())
	return buf.Bytes()
}

// encodeHeaderChunk returns a binary encoding of the midi header track.
func (*Track) encodeHeaderChunk() []byte {
	buf := bytes.NewBuffer(nil)
	buf.Write([]byte("MThd"))
	binary.Write(buf, binary.BigEndian, uint32(6))
	binary.Write(buf, binary.BigEndian, uint16(1)) // File format (0/1/2).
	binary.Write(buf, binary.BigEndian, uint16(2)) // Number of tracks.
	binary.Write(buf, binary.BigEndian, uint16(96))

	return buf.Bytes()
}

// encodeMetaChunk returns a binary encoding of the midi first (metadata)
// track.
func (t *Track) encodeMetaChunk() []byte {
	// Extract us per beat from bpm.
	mpb := 1 / float64(t.BPM)
	uspb := uint32(mpb * 60 * 1000000)

	// Encode track.
	buf := bytes.NewBuffer(nil)
	buf.Write([]byte("MTrk"))

	buf2 := bytes.NewBuffer(nil)
	// TODO(amit): Extract meta events to functions.
	buf2.Write([]byte{0, 0xFF, 0x58, 4, 4, 2, 24, 8})
	buf2.Write([]byte{0, 0xFF, 0x51, 3})
	buf2.Write(bin(uspb)[1:])
	buf2.Write([]byte{0, 0xFF, 0x2F, 0})

	buf.Write(bin(uint32(buf2.Len())))
	return append(buf.Bytes(), buf2.Bytes()...)
}

// encodeHits returns a binary encoding of the drum hits in this track as a
// single midi track.
func (t *Track) encodeHits() []byte {
	buf := bytes.NewBuffer([]byte("MTrk"))
	buf2 := bytes.NewBuffer(nil)
	for _, h := range t.Hits {
		buf2.Write(h.encode())
	}
	buf2.Write([]byte{0, 0xFF, 0x2F, 0})
	buf.Write(bin(uint32(buf2.Len())))

	return append(buf.Bytes(), buf2.Bytes()...)
}

// A Hit is a set of drums being hit at the same time.
type Hit struct {
	Notes map[byte]struct{} // Set of notes to strike.
	T     uint              // Number of ticks this hit lasts (96 is a quarter bar).
	V     Velocity          // Velocity (volume) of the hit.
}

// NewHit returns a new hit instance.
func NewHit(ticks uint, v Velocity, notes ...byte) *Hit {
	h := &Hit{map[byte]struct{}{}, ticks, v}
	for _, n := range notes {
		h.Notes[n] = struct{}{}
	}
	return h
}

// encode returns a binary encoding of the hit as midi events.
func (h *Hit) encode() []byte {
	buf := bytes.NewBuffer(nil)
	for n := range h.Notes {
		buf.Write([]byte{0, 0x99, n, byte(h.V)})
	}
	first := true
	for n := range h.Notes {
		if first {
			buf.Write(uvarint(h.T))
			first = false
		} else {
			buf.Write(uvarint(0))
		}
		buf.Write([]byte{0x89, n, 64})
	}
	return buf.Bytes()
}

// Velocity is a drum hit's volume.
type Velocity byte

// Predefined velocities.
const (
	PPP Velocity = 16  // Pianississimo
	PP  Velocity = 32  // Pianissimo
	P   Velocity = 48  // Piano
	MP  Velocity = 64  // Mezzo-piano
	MF  Velocity = 80  // Mezzo-forte
	F   Velocity = 96  // Forte
	FF  Velocity = 112 // Fortissimo
	FFF Velocity = 127 // Fortississimo
)