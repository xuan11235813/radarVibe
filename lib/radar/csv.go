package radar

import (
	"encoding/csv"
	"io"
	"os"
	"strconv"
)

// WriteCSV exports zero-based period and within-period indices, then ch6 and ch7.
// All actual samples are retained, including the final partial period.
func WriteCSV(w io.Writer, frame *Frame) error {
	if err := frame.Verify(); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	row := []string{"period_index", "sample_in_period", "ch6", "ch7"}
	if err := cw.Write(row); err != nil {
		return err
	}
	for i := 0; i < SamplesPerChannel; i++ {
		row[0] = strconv.Itoa(i / SamplesPerPeriod)
		row[1] = strconv.Itoa(i % SamplesPerPeriod)
		row[2] = strconv.Itoa(int(frame.Sample(i, 6)))
		row[3] = strconv.Itoa(int(frame.Sample(i, 7)))
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// SaveCSV refuses to overwrite an existing file and removes incomplete output
// on write failure. Parent directories must already exist.
func SaveCSV(path string, frame *Frame) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	writeErr := WriteCSV(f, frame)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}
