package test

import (
	"context"
	"encoding/binary"
	"encoding/csv"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"radar/lib/radar"
)

func validFrame() *radar.Frame {
	f := &radar.Frame{Raw: make([]byte, radar.FrameBytes)}
	for i := 0; i < radar.SamplesPerChannel; i++ {
		for ch := 0; ch < radar.Channels; ch++ {
			v := uint16((i+ch)%16384) << 2
			if ch >= 6 && i%radar.SamplesPerPeriod <= 1753 {
				v |= 2
			}
			binary.LittleEndian.PutUint16(f.Raw[(i*radar.Channels+ch)*2:], v)
		}
	}
	return f
}

func packet(f *radar.Frame, offset int) []byte {
	p := make([]byte, radar.PacketBytes)
	binary.LittleEndian.PutUint32(p, uint32(offset))
	copy(p[4:], f.Raw[offset:offset+radar.PayloadBytes])
	return p
}

func TestAssemblerRecoveryAndDecode(t *testing.T) {
	f := validFrame()
	var a radar.Assembler
	if got, err := a.Add(packet(f, 0)); got != nil || err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := a.Add(packet(f, 2*radar.PayloadBytes)); err == nil {
		t.Fatal("missing packet accepted")
	}
	if got, _ := a.Add(packet(f, 3*radar.PayloadBytes)); got != nil {
		t.Fatal("partial frame emitted")
	}
	// A new frame start replaces even an in-progress frame immediately.
	_, _ = a.Add(packet(f, 0))
	for off := 0; off < radar.FrameBytes; off += radar.PayloadBytes {
		got, err := a.Add(packet(f, off))
		if err != nil {
			t.Fatal(err)
		}
		if off+radar.PayloadBytes < radar.FrameBytes {
			if got != nil {
				t.Fatal("early frame")
			}
			continue
		}
		if got == nil {
			t.Fatal("no complete frame")
		}
		for _, i := range []int{0, 2124, 2125, 8191, 16383, radar.SamplesPerChannel - 1} {
			for ch := 0; ch < radar.Channels; ch++ {
				if v := got.Sample(i, ch); v != int16((i+ch)%16384) {
					t.Fatalf("sample %d channel %d: %d", i, ch, v)
				}
			}
		}
	}
}

func TestMalformedAndSync(t *testing.T) {
	for _, n := range []int{0, 3, radar.PacketBytes - 1, radar.PacketBytes + 1} {
		if _, err := new(radar.Assembler).Add(make([]byte, n)); err == nil {
			t.Fatalf("accepted length %d", n)
		}
	}
	for _, off := range []uint32{1, radar.FrameBytes, 0xffffffff} {
		p := make([]byte, radar.PacketBytes)
		binary.LittleEndian.PutUint32(p, off)
		if _, err := radar.PacketOffset(p); err == nil {
			t.Fatalf("accepted offset %d", off)
		}
	}
	f := validFrame()
	if err := f.Verify(); err != nil {
		t.Fatal(err)
	}
	f.Raw[(100*radar.Channels+6)*2] &^= 2
	if err := f.Verify(); err == nil {
		t.Fatal("bad sync accepted")
	}
	var a radar.Assembler
	for off := 0; off < radar.FrameBytes; off += radar.PayloadBytes {
		got, err := a.Add(packet(f, off))
		if got != nil {
			t.Fatal("invalid frame emitted")
		}
		if off+radar.PayloadBytes == radar.FrameBytes && err == nil {
			t.Fatal("invalid frame not rejected")
		}
	}
}

func TestCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frame.csv")
	frame := validFrame()
	if err := radar.SaveCSV(path, frame); err != nil {
		t.Fatal(err)
	}
	if err := radar.SaveCSV(path, frame); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing file not protected: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil || len(header) != 4 || header[0] != "period_index" || header[1] != "sample_in_period" || header[2] != "ch6" || header[3] != "ch7" {
		t.Fatalf("header: %v %v", header, err)
	}
	for i := 0; i < radar.SamplesPerChannel; i++ {
		row, err := r.Read()
		if err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
		if len(row) != 4 {
			t.Fatalf("row %d: expected two indices and ch6/ch7, got %v", i, row)
		}
		if row[0] != strconv.Itoa(i/radar.SamplesPerPeriod) || row[1] != strconv.Itoa(i%radar.SamplesPerPeriod) {
			t.Fatalf("row %d: incorrect period indices: %v", i, row)
		}
		for col, ch := range []int{6, 7} {
			if row[col+2] != strconv.Itoa((i+ch)%16384) {
				t.Fatalf("CSV mismatch at %d channel %d", i, ch)
			}
		}
	}
	if _, err := r.Read(); err != io.EOF {
		t.Fatalf("unexpected trailing row: %v", err)
	}
	badPath := filepath.Join(t.TempDir(), "invalid.csv")
	if err := radar.SaveCSV(badPath, &radar.Frame{}); err == nil {
		t.Fatal("invalid frame saved")
	}
	if _, err := os.Stat(badPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed export left a file")
	}
}

func TestUDPReceiveAndTimeout(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadBuffer(16 * 1024 * 1024); err != nil {
		t.Fatal(err)
	}
	sender, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		f, err := radar.ReceiveFrame(ctx, conn, net.IPv4(127, 0, 0, 1))
		if err == nil && f.Sample(radar.SamplesPerChannel-1, 7) != 6 {
			err = errors.New("last sample mismatch")
		}
		result <- err
	}()
	f := validFrame()
	for off := 0; off < radar.FrameBytes; off += radar.PayloadBytes {
		if _, err := sender.Write(packet(f, off)); err != nil {
			t.Fatal(err)
		}
		if off%(32*radar.PayloadBytes) == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	short, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if _, err := radar.ReceiveFrame(short, conn, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
}
