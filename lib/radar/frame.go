// Package radar receives and decodes the radar protocol in exampleCode/merger.go.
package radar

import (
	"encoding/binary"
	"fmt"
)

const (
	DefaultPort       = 1995
	Channels          = 8
	FrameBytes        = 8 * 1024 * 1024
	BytesPerSample    = 2
	SamplesPerChannel = FrameBytes / BytesPerSample / Channels
	SamplesPerPeriod  = 2125 // 31,250,000 Hz * 68 microseconds
	PayloadBytes      = 8192
	PacketBytes       = 4 + PayloadBytes
)

// Frame contains interleaved, little-endian samples, in acquisition order.
type Frame struct {
	Raw []byte
}

// Verify checks the same SYNC bit and tolerance windows as the reference.
// Only complete periods are checked; the final partial period is retained.
func (f *Frame) Verify() error {
	if f == nil || len(f.Raw) != FrameBytes {
		return fmt.Errorf("frame must contain %d bytes", FrameBytes)
	}
	for period := 0; period < SamplesPerChannel/SamplesPerPeriod; period++ {
		for ch := 6; ch < Channels; ch++ {
			for sample := 0; sample < SamplesPerPeriod; sample++ {
				offset := ((period*SamplesPerPeriod+sample)*Channels + ch) * BytesPerSample
				high := f.Raw[offset]&2 != 0
				if (sample > 1 && sample < 1752 && !high) || (sample > 1754 && sample < 2124 && high) {
					return fmt.Errorf("bad sync: period=%d channel=%d sample=%d", period, ch, sample)
				}
			}
		}
	}
	return nil
}

// Sample follows the reference's unsigned shift, with no sign extension.
// Callers must use a complete frame and indices within their respective bounds.
func (f *Frame) Sample(index, channel int) int16 {
	offset := (index*Channels + channel) * BytesPerSample
	return int16(binary.LittleEndian.Uint16(f.Raw[offset:offset+BytesPerSample]) >> 2)
}

// PacketOffset validates a UDP datagram before any slicing or copying.
func PacketOffset(packet []byte) (int, error) {
	if len(packet) != PacketBytes {
		return 0, fmt.Errorf("packet length %d, want %d", len(packet), PacketBytes)
	}
	offset := binary.LittleEndian.Uint32(packet[:4])
	if offset >= FrameBytes || offset%PayloadBytes != 0 {
		return 0, fmt.Errorf("invalid packet offset %d", offset)
	}
	return int(offset), nil
}

// Assembler requires in-order packets from a single source. A discontinuity
// discards the partial frame; a new offset-zero packet immediately restarts it.
// The wire protocol has no frame ID, so reordered packets are not reassembled.
type Assembler struct {
	raw      []byte
	expected int
}

func (a *Assembler) Add(packet []byte) (*Frame, error) {
	offset, err := PacketOffset(packet)
	if err != nil {
		a.expected = 0
		return nil, err
	}
	if offset == 0 {
		a.expected = 0
	}
	if offset != a.expected {
		want := a.expected
		a.expected = 0
		return nil, fmt.Errorf("packet offset %d, expected %d; discarded partial frame", offset, want)
	}
	if a.raw == nil {
		a.raw = make([]byte, FrameBytes)
	}
	copy(a.raw[offset:offset+PayloadBytes], packet[4:])
	a.expected += PayloadBytes
	if a.expected != FrameBytes {
		return nil, nil
	}
	f := &Frame{Raw: a.raw}
	a.raw, a.expected = nil, 0
	if err := f.Verify(); err != nil {
		return nil, err
	}
	return f, nil
}
