package dataprocess

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// SaveComplexMatrix writes a nonempty rectangular complex128 matrix as UTF-8
// CSV, with one matrix row per line and no header. Values use Go's (real+imagi)
// syntax and enough digits to round-trip each finite float64 component exactly.
// CRLF line endings make the text readable in older Windows Notepad versions.
// Existing files are not overwritten; the parent directory must already exist.
func SaveComplexMatrix(path string, matrix [][]complex128) error {
	if err := validateComplexMatrix(matrix); err != nil {
		return err
	}
	width := len(matrix[0])
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(f)
	cw.UseCRLF = true
	record := make([]string, width)
	var writeErr error
	for _, row := range matrix {
		for j, value := range row {
			record[j] = strconv.FormatComplex(value, 'g', -1, 128)
		}
		if writeErr = cw.Write(record); writeErr != nil {
			break
		}
	}
	cw.Flush()
	if writeErr == nil {
		writeErr = cw.Error()
	}
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

func validateComplexMatrix(matrix [][]complex128) error {
	if len(matrix) == 0 || len(matrix[0]) == 0 {
		return fmt.Errorf("complex matrix must contain at least one row and column")
	}
	width := len(matrix[0])
	for i, row := range matrix {
		if len(row) != width {
			return fmt.Errorf("complex matrix row %d has %d columns, want %d", i, len(row), width)
		}
	}
	return nil
}

// ReadComplexMatrix reads the headerless format produced by SaveComplexMatrix.
// It also accepts LF line endings, a UTF-8 BOM, and whitespace around fields.
// Empty files, inconsistent row widths and invalid complex numbers are errors;
// a failed read returns no partial matrix. NaN and Inf components are supported.
func ReadComplexMatrix(path string) ([][]complex128, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	var matrix [][]complex128
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("complex CSV row %d: %w", len(matrix), err)
		}
		row := make([]complex128, len(record))
		for j, field := range record {
			if len(matrix) == 0 && j == 0 {
				field = strings.TrimPrefix(field, "\ufeff")
			}
			value, err := strconv.ParseComplex(strings.TrimSpace(field), 128)
			if err != nil {
				return nil, fmt.Errorf("complex CSV row %d column %d: %w", len(matrix), j, err)
			}
			row[j] = value
		}
		matrix = append(matrix, row)
	}
	if len(matrix) == 0 {
		return nil, fmt.Errorf("complex CSV contains no data")
	}
	return matrix, nil
}
