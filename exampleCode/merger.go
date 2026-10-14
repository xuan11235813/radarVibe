package core

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const NBuffer = 4

const (
	NRxChannel        = 8
	LenFrame          = 8 * 1024 * 1024 // 8MiB
	NBytePerSample    = 2
	NSamplePerChannel = LenFrame / NBytePerSample / NRxChannel
	AdcFreq           = 31_250_000
	NSamplePerPeriod  = int(AdcFreq * 68e-6)                   // 68μs
	NPeriod           = 1 + NSamplePerChannel/NSamplePerPeriod // in case overflow
)

type Raw [LenFrame]byte
type Parsed [NPeriod][NRxChannel][NSamplePerPeriod]int16

type Merged struct {
	Raw
	Parsed
	Timestamp int64
}

type merger struct {
	buffer    [NBuffer]Merged
	ChInput   chan []byte
	ChOutput  chan *Merged
	ChOutput1 chan *Merged
	filling   int
}

func NewMerger() *merger {
	m := &merger{
		ChInput:   make(chan []byte, 4096),
		ChOutput:  make(chan *Merged, 16),
		ChOutput1: make(chan *Merged, 16),
	}
	go m.worker()
	return m
}

func (m *merger) worker() {
	once := false
	expected := uint32(0)
	for {
		pkt := <-m.ChInput
		cur := &m.buffer[m.filling]

		if len(pkt) != 8192+4 {
			EmitError("bad pkt length: " + strconv.Itoa(len(pkt)))
		}
		offset := binary.LittleEndian.Uint32(pkt[0:4])
		if expected != offset {
			expected = 0
			if once {
				once = false
				EmitError(fmt.Sprintf("bad offset: %d, expected: %d", offset, expected))
			}
		} else {
			once = true
			copy(cur.Raw[expected:expected+8192], pkt[4:])

			expected += 8192
			if expected == 8*1024*1024 {
				expected = 0
				now := time.Now().UnixMilli()
				cur.Timestamp = now
				if err := cur.VerifyPulseOut(); nil != err {
					EmitError(fmt.Sprintf("bad pulse out: %d, err: %s", now, err))
				} else {
					cur.Parse()

					m.ChOutput <- cur
					m.ChOutput1 <- cur
					EmitError(fmt.Sprintf("%d Merger Idx: %d", now, m.filling))
					m.filling = (m.filling + 1) % NBuffer

					fps := fps.tick(time.Now().UnixMilli())
					runtime.WindowSetTitle(ctx, "fmcwbb-test-v3 fps: "+strconv.Itoa(fps))
				}
			}
		}
	}

}

func (m *Merged) Parse() {
	p := &m.Parsed
	*p = Parsed{}

	for idx := range NSamplePerChannel {
		offset := idx * NRxChannel * NBytePerSample

		for c := range NRxChannel {
			v := uint16(m.Raw[offset+c*NBytePerSample])<<0 + uint16(m.Raw[offset+c*NBytePerSample+1])<<8
			v >>= 2 // 去掉前两位

			p[idx/NSamplePerPeriod][c][idx%NSamplePerPeriod] = int16(v)
		}
	}
}

var (
	ErrPulseOutMismatch    = fmt.Errorf("pulse out mismatch")
	ErrBadPulseOutPosition = fmt.Errorf("bad pulse out position")
)

// 一帧 2125 个样本
// 其中 [0, 1753] SYNC 为高电平
// SYNC 采样可能有误，允许 ±1 范围
func (m *Merged) VerifyPulseOut() error {
	// pos := [NSamplePerChannel * 2 * 2]byte{}
	// for idx := range pos {
	// 	pos[idx] = '\n'
	// }
	// posIdx := 0
	for idxPeriod := range NPeriod - 1 {
		// EmitError("idxPeriod: " + strconv.Itoa(idxPeriod))
		for idxCh := 6; idxCh < 8; idxCh += 1 {
			for idx := range NSamplePerPeriod {
				offset := (idxPeriod*NSamplePerPeriod+idx)*NRxChannel*NBytePerSample + idxCh*NBytePerSample

				po := m.Raw[offset] & 0b10
				if 1 < idx && idx < 1753-1 {
					if po == 0 {
						return errors.Wrap(ErrBadSync, "idx: "+strconv.Itoa(idx)+", po is 0")
					}
				} else if 1753+1 < idx && idx < 2125-1 {
					if po != 0 {
						return errors.Wrap(ErrBadSync, "idx: "+strconv.Itoa(idx)+", po is not 0")
					}
				}
				// if po != 0 {
				// 	pos[posIdx] = '1'
				// } else {
				// 	pos[posIdx] = '0'
				// }
				// posIdx += 2
			}
		}
	}
	// os.WriteFile("pos.csv", pos[:], 0644)

	return nil
}

var (
	ErrDuplicatedTx      = fmt.Errorf("duplicated tx")
	ErrTxEnabledPreamble = fmt.Errorf("tx enabled preamble")
	ErrBadTx             = fmt.Errorf("bad tx")
	ErrBadSync           = fmt.Errorf("bad sync")
)
