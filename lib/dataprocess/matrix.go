package dataprocess

import (
	"fmt"
	"math"

	"radar/lib/radar"
)

// ChannelMatrices stores channels as [period][sample], sharing no sample storage.
type ChannelMatrices struct {
	Ch6 [][]float64
	Ch7 [][]float64
}

// ToMatrices preserves all input periods, including a final partial period.
// Rows must start at period 0/sample 0 and be ordered with no missing periods,
// duplicate indices or gaps within a period. Partial periods remain short rows.
func ToMatrices(samples []Sample) (ChannelMatrices, error) {
	var result ChannelMatrices
	if len(samples) == 0 {
		return result, fmt.Errorf("no samples to convert")
	}
	for i, s := range samples {
		if s.PeriodIndex < 0 || s.SampleInPeriod < 0 || s.SampleInPeriod >= radar.SamplesPerPeriod {
			return ChannelMatrices{}, fmt.Errorf("sample row %d: invalid period/sample index", i)
		}
		if s.PeriodIndex == len(result.Ch6) && s.SampleInPeriod == 0 {
			result.Ch6 = append(result.Ch6, make([]float64, 0, radar.SamplesPerPeriod))
			result.Ch7 = append(result.Ch7, make([]float64, 0, radar.SamplesPerPeriod))
		}
		if s.PeriodIndex != len(result.Ch6)-1 || s.SampleInPeriod != len(result.Ch6[s.PeriodIndex]) {
			return ChannelMatrices{}, fmt.Errorf("sample row %d: indices are not consecutive (period=%d sample=%d)", i, s.PeriodIndex, s.SampleInPeriod)
		}
		result.Ch6[s.PeriodIndex] = append(result.Ch6[s.PeriodIndex], s.Ch6)
		result.Ch7[s.PeriodIndex] = append(result.Ch7[s.PeriodIndex], s.Ch7)
	}
	if err := result.validate(); err != nil {
		return ChannelMatrices{}, err
	}
	return result, nil
}

// SelectSamples keeps only complete original periods (2125 samples), then
// copies the inclusive zero-based [start, end] range into new independent slices.
// Retained periods remain in input order; incomplete periods are discarded.
func SelectSamples(matrices ChannelMatrices, start, end int) (ChannelMatrices, error) {
	var result ChannelMatrices
	if start < 0 || end < start || end >= radar.SamplesPerPeriod {
		return result, fmt.Errorf("sample range must satisfy 0 <= start <= end < %d", radar.SamplesPerPeriod)
	}
	if err := matrices.validate(); err != nil {
		return result, err
	}
	for p := range matrices.Ch6 {
		n := len(matrices.Ch6[p])
		if n > radar.SamplesPerPeriod {
			return ChannelMatrices{}, fmt.Errorf("period %d has too many samples: %d", p, n)
		}
		if n != radar.SamplesPerPeriod {
			continue
		}
		result.Ch6 = append(result.Ch6, append([]float64(nil), matrices.Ch6[p][start:end+1]...))
		result.Ch7 = append(result.Ch7, append([]float64(nil), matrices.Ch7[p][start:end+1]...))
	}
	if len(result.Ch6) == 0 {
		return result, fmt.Errorf("no complete periods (%d samples each)", radar.SamplesPerPeriod)
	}
	return result, nil
}

func (m ChannelMatrices) validate() error {
	if len(m.Ch6) == 0 || len(m.Ch6) != len(m.Ch7) {
		return fmt.Errorf("channel matrices must have the same nonzero period count")
	}
	for p := range m.Ch6 {
		if len(m.Ch6[p]) == 0 || len(m.Ch6[p]) != len(m.Ch7[p]) {
			return fmt.Errorf("period %d: channel sample counts must match and be nonzero", p)
		}
		for s := range m.Ch6[p] {
			for _, v := range []float64{m.Ch6[p][s], m.Ch7[p][s]} {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return fmt.Errorf("period %d sample %d: nonfinite value", p, s)
				}
			}
		}
	}
	return nil
}
