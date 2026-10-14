// Package dataprocess provides reusable processing for captured radar samples.
package dataprocess

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Sample is one input row. Floating-point channel values support later algorithms.
type Sample struct {
	PeriodIndex    int
	SampleInPeriod int
	Ch6            float64
	Ch7            float64
}

// ReadCSV loads a receiveData CSV into memory. Required columns may appear in
// any order; additional columns are ignored. Invalid numbers are never skipped.
func ReadCSV(path string) ([]Sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	columns := make(map[string]int, len(header))
	for i, name := range header {
		name = strings.TrimSpace(strings.TrimPrefix(name, "\ufeff"))
		if _, exists := columns[name]; exists {
			return nil, fmt.Errorf("duplicate CSV column %q", name)
		}
		columns[name] = i
	}
	for _, name := range []string{"period_index", "sample_in_period", "ch6", "ch7"} {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("missing CSV column %q", name)
		}
	}
	var samples []Sample
	for row := 2; ; row++ {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV record %d: %w", row, err)
		}
		var s Sample
		for name, dst := range map[string]*int{"period_index": &s.PeriodIndex, "sample_in_period": &s.SampleInPeriod} {
			value, err := strconv.Atoi(strings.TrimSpace(record[columns[name]]))
			if err != nil || value < 0 {
				return nil, fmt.Errorf("CSV record %d: invalid nonnegative integer in %s", row, name)
			}
			*dst = value
		}
		for name, dst := range map[string]*float64{"ch6": &s.Ch6, "ch7": &s.Ch7} {
			value, err := strconv.ParseFloat(strings.TrimSpace(record[columns[name]]), 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("CSV record %d: invalid finite number in %s", row, name)
			}
			*dst = value
		}
		samples = append(samples, s)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("CSV contains no samples")
	}
	return samples, nil
}

// SaveMatrices writes one period per row, without headers or indices, to
// datach6.csv and datach7.csv in dir. Existing files are never overwritten.
// If either output fails, both files created by this call are removed.
func SaveMatrices(dir string, matrices ChannelMatrices) error {
	if err := matrices.validate(); err != nil {
		return err
	}
	width := len(matrices.Ch6[0])
	for p := range matrices.Ch6 {
		if len(matrices.Ch6[p]) != width {
			return fmt.Errorf("matrix row %d has a different sample count", p)
		}
	}
	paths := []string{filepath.Join(dir, "datach6.csv"), filepath.Join(dir, "datach7.csv")}
	return saveMatrixPair(paths, [][][]float64{matrices.Ch6, matrices.Ch7})
}

// saveMatrixPair writes two already-validated rectangular matrices and protects
// existing outputs. On failure it removes only files created by this call.
func saveMatrixPair(paths []string, matrices [][][]float64) error {
	var files []*os.File
	cleanup := func() {
		for i, f := range files {
			_ = f.Close()
			_ = os.Remove(paths[i])
		}
	}
	for _, path := range paths {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			cleanup()
			return fmt.Errorf("create %s: %w", path, err)
		}
		files = append(files, f)
	}
	for i, matrix := range matrices {
		if err := writeMatrixCSV(files[i], matrix); err != nil {
			cleanup()
			return fmt.Errorf("write %s: %w", paths[i], err)
		}
	}
	if err := errors.Join(files[0].Close(), files[1].Close()); err != nil {
		cleanup()
		return err
	}
	return nil
}

func writeMatrixCSV(w io.Writer, matrix [][]float64) error {
	cw := csv.NewWriter(w)
	row := make([]string, len(matrix[0]))
	for _, samples := range matrix {
		for i, value := range samples {
			row[i] = strconv.FormatFloat(value, 'g', -1, 64)
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
