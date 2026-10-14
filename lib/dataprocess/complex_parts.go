package dataprocess

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// SaveComplexMatrixParts saves matrix to prefix+"_real.csv" and prefix+"_img.csv".
// Prefix is a path without an extension. Each file contains numeric values only,
// one matrix row per CSV row, with no header. Finite components round-trip exactly.
// The matrix must be nonempty and rectangular. Existing files are not overwritten.
func SaveComplexMatrixParts(prefix string, matrix [][]complex128) error {
	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("complex matrix filename prefix must not be empty")
	}
	if err := validateComplexMatrix(matrix); err != nil {
		return err
	}
	realMatrix := make([][]float64, len(matrix))
	imgMatrix := make([][]float64, len(matrix))
	for i, row := range matrix {
		realMatrix[i] = make([]float64, len(row))
		imgMatrix[i] = make([]float64, len(row))
		for j, value := range row {
			realMatrix[i][j] = real(value)
			imgMatrix[i][j] = imag(value)
		}
	}
	return saveMatrixPair([]string{prefix + "_real.csv", prefix + "_img.csv"}, [][][]float64{realMatrix, imgMatrix})
}

// ReadComplexMatrixParts combines two numeric CSV files, in real/imaginary order.
// Both files must be nonempty rectangular matrices with identical dimensions.
// UTF-8 BOM, surrounding whitespace, LF/CRLF and NaN/Inf are supported.
// NaN payload bits are not preserved. Errors return no partial matrix.
func ReadComplexMatrixParts(realPath, imgPath string) ([][]complex128, error) {
	realMatrix, err := readNumericMatrix(realPath)
	if err != nil {
		return nil, fmt.Errorf("read real matrix: %w", err)
	}
	imgMatrix, err := readNumericMatrix(imgPath)
	if err != nil {
		return nil, fmt.Errorf("read imaginary matrix: %w", err)
	}
	if len(realMatrix) != len(imgMatrix) || len(realMatrix[0]) != len(imgMatrix[0]) {
		return nil, fmt.Errorf("matrix dimensions differ: real=%dx%d, img=%dx%d", len(realMatrix), len(realMatrix[0]), len(imgMatrix), len(imgMatrix[0]))
	}
	matrix := make([][]complex128, len(realMatrix))
	for i, row := range realMatrix {
		matrix[i] = make([]complex128, len(row))
		for j, value := range row {
			matrix[i][j] = complex(value, imgMatrix[i][j])
		}
	}
	return matrix, nil
}

func readNumericMatrix(path string) ([][]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	var matrix [][]float64
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s row %d: %w", path, len(matrix), err)
		}
		row := make([]float64, len(record))
		for j, field := range record {
			if len(matrix) == 0 && j == 0 {
				field = strings.TrimPrefix(field, "\ufeff")
			}
			value, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
			if err != nil {
				return nil, fmt.Errorf("%s row %d column %d: %w", path, len(matrix), j, err)
			}
			row[j] = value
		}
		matrix = append(matrix, row)
	}
	if len(matrix) == 0 {
		return nil, fmt.Errorf("%s contains no matrix data", path)
	}
	return matrix, nil
}
