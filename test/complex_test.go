package test

import (
	"errors"
	"fmt"
	"math"
	"math/cmplx"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"radar/lib/dataprocess"
)

func TestComplexMatrixRandom100x100RoundTrip(t *testing.T) {
	const rows, cols = 100, 100
	const seed int64 = 20261014
	// A fixed seed keeps random test failures reproducible. Both components
	// are independently distributed over [-1e6, 1e6).
	rng := rand.New(rand.NewSource(seed))
	original := make([][]complex128, rows)
	for i := range original {
		original[i] = make([]complex128, cols)
		for j := range original[i] {
			original[i][j] = complex((2*rng.Float64()-1)*1e6, (2*rng.Float64()-1)*1e6)
		}
	}

	// Keep each run's files for manual inspection; do not register cleanup.
	outputRoot, err := filepath.Abs("results")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputRoot, 0755); err != nil {
		t.Fatal(err)
	}
	outputDir, err := os.MkdirTemp(outputRoot, "complex_100x100-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("persistent test output: %s", outputDir)
	path := filepath.Join(outputDir, "complex_100x100.txt")
	if err := dataprocess.SaveComplexMatrix(path, original); err != nil {
		t.Fatal(err)
	}
	restored, err := dataprocess.ReadComplexMatrix(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != rows {
		t.Fatalf("restored row count = %d, want %d", len(restored), rows)
	}
	var maxRealDiff, maxImagDiff, maxAbsDiff float64
	mismatches := 0
	firstRow, firstCol := -1, -1
	for i := range original {
		if len(restored[i]) != cols {
			t.Fatalf("restored row %d column count = %d, want %d", i, len(restored[i]), cols)
		}
		for j, want := range original[i] {
			got := restored[i][j]
			maxRealDiff = math.Max(maxRealDiff, math.Abs(real(got)-real(want)))
			maxImagDiff = math.Max(maxImagDiff, math.Abs(imag(got)-imag(want)))
			maxAbsDiff = math.Max(maxAbsDiff, cmplx.Abs(got-want))
			// Require exact component bits, rather than accepting a tolerance.
			if math.Float64bits(real(got)) != math.Float64bits(real(want)) || math.Float64bits(imag(got)) != math.Float64bits(imag(want)) {
				if mismatches == 0 {
					firstRow, firstCol = i, j
				}
				mismatches++
			}
		}
	}
	t.Logf("seed=%d; matrix=%dx%d; compared=%d; mismatches=%d", seed, rows, cols, rows*cols, mismatches)
	t.Logf("max real difference=%g; max imaginary difference=%g; max complex absolute difference=%g", maxRealDiff, maxImagDiff, maxAbsDiff)
	t.Logf("saved and reloaded: %s (retained for manual inspection)", path)
	report := fmt.Sprintf("seed=%d\r\nmatrix=%dx%d\r\ncompared=%d\r\nmismatches=%d\r\nmax_real_difference=%g\r\nmax_imaginary_difference=%g\r\nmax_complex_absolute_difference=%g\r\n", seed, rows, cols, rows*cols, mismatches, maxRealDiff, maxImagDiff, maxAbsDiff)
	if err := os.WriteFile(filepath.Join(outputDir, "comparison.txt"), []byte(report), 0644); err != nil {
		t.Fatal(err)
	}
	if mismatches != 0 {
		t.Fatalf("first mismatch [%d][%d]: got %v, want %v", firstRow, firstCol, restored[firstRow][firstCol], original[firstRow][firstCol])
	}
}

func TestComplexMatrixRoundTrip(t *testing.T) {
	matrix := [][]complex128{
		{1 + 2i, -3 - 4i, 0, complex(math.Copysign(0, -1), math.Copysign(0, -1))},
		{complex(math.Pi, -math.E), complex(math.SmallestNonzeroFloat64, math.MaxFloat64), complex(1e-200, -1e200), complex(math.Inf(1), math.Inf(-1))},
	}
	path := filepath.Join(t.TempDir(), "复数矩阵.txt")
	if err := dataprocess.SaveComplexMatrix(path, matrix); err != nil {
		t.Fatal(err)
	}
	got, err := dataprocess.ReadComplexMatrix(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(matrix) {
		t.Fatal("row count changed")
	}
	for i := range matrix {
		if len(got[i]) != len(matrix[i]) {
			t.Fatalf("row %d width changed", i)
		}
		for j, want := range matrix[i] {
			value := got[i][j]
			if math.Float64bits(real(value)) != math.Float64bits(real(want)) || math.Float64bits(imag(value)) != math.Float64bits(imag(want)) {
				t.Fatalf("[%d][%d] lost precision: got %v, want %v", i, j, value, want)
			}
		}
	}
}

func TestComplexMatrixTextAndProtection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matrix.csv")
	if err := dataprocess.SaveComplexMatrix(path, [][]complex128{{1 + 2i, 3 - 4i}, {5, -6i}}); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(path)
	if err != nil || string(text) != "(1+2i),(3-4i)\r\n(5+0i),(0-6i)\r\n" {
		t.Fatalf("unexpected readable format: %q, %v", text, err)
	}
	if err := dataprocess.SaveComplexMatrix(path, [][]complex128{{99}}); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing file not protected: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(text) {
		t.Fatal("existing matrix changed")
	}
	path = writeInput(t, "\ufeff (1+2i) , -3i\n4, (NaN+Infi)\n")
	got, err := dataprocess.ReadComplexMatrix(path)
	if err != nil {
		t.Fatal(err)
	}
	if got[0][0] != 1+2i || got[0][1] != -3i || got[1][0] != 4 || !math.IsNaN(real(got[1][1])) || !math.IsInf(imag(got[1][1]), 1) {
		t.Fatalf("incorrect manual text decoding: %v", got)
	}
}

func TestComplexMatrixInvalidInput(t *testing.T) {
	for _, matrix := range [][][]complex128{nil, {}, {{}}, {{1}, {}}, {{1}, {2, 3}}} {
		path := filepath.Join(t.TempDir(), "invalid.txt")
		if err := dataprocess.SaveComplexMatrix(path, matrix); err == nil {
			t.Fatalf("accepted invalid matrix %v", matrix)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("invalid matrix created output")
		}
	}
	for _, input := range []string{"", "\n", "ch6,ch7\n", "(1+2i),\n", "(1+2i),(3+4i)\n5\n", "(1+2i)\ninvalid\n", "1e999+2i\n"} {
		if got, err := dataprocess.ReadComplexMatrix(writeInput(t, input)); err == nil || got != nil {
			t.Fatalf("accepted invalid file %q or returned partial data: %v, %v", input, got, err)
		}
	}
	if _, err := dataprocess.ReadComplexMatrix(filepath.Join(t.TempDir(), "missing.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
}
