package test

import (
	"fmt"
	"math"
	"math/cmplx"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"radar/lib/dataprocess"
)

// Independent O(N^2) reference, used only on small test inputs.
func referenceDFT(input []complex128, removeDC bool) []complex128 {
	var mean complex128
	if removeDC {
		for _, v := range input {
			mean += v
		}
		mean /= complex(float64(len(input)), 0)
	}
	result := make([]complex128, len(input))
	for k := range result {
		for j, v := range input {
			result[k] += (v - mean) * cmplx.Exp(complex(0, -2*math.Pi*float64(j*k)/float64(len(input))))
		}
	}
	return result
}

func assertSpectrum(t *testing.T, got, want []complex128) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("spectrum lengths %d != %d", len(got), len(want))
	}
	for k := range want {
		if cmplx.Abs(got[k]-want[k]) > 1e-9*(1+cmplx.Abs(want[k])) {
			t.Fatalf("bin %d: got %v, want %v", k, got[k], want[k])
		}
	}
}

func TestFFTVectors(t *testing.T) {
	rng := rand.New(rand.NewSource(17))
	for _, n := range []int{1, 2, 3, 7, 8, 15, 16, 31} {
		realInput := make([]float64, n)
		complexInput := make([]complex128, n)
		realAsComplex := make([]complex128, n)
		for i := range realInput {
			realInput[i] = rng.Float64()*10 - 5
			realAsComplex[i] = complex(realInput[i], 0)
			complexInput[i] = complex(realInput[i], rng.Float64()*10-5)
		}
		realCopy := append([]float64(nil), realInput...)
		complexCopy := append([]complex128(nil), complexInput...)
		for _, dc := range []bool{false, true} {
			got, err := dataprocess.FFTReal(realInput, dc)
			if err != nil {
				t.Fatal(err)
			}
			assertSpectrum(t, got, referenceDFT(realAsComplex, dc))
			got, err = dataprocess.FFTComplex(complexInput, dc)
			if err != nil {
				t.Fatal(err)
			}
			assertSpectrum(t, got, referenceDFT(complexInput, dc))
		}
		if !reflect.DeepEqual(realInput, realCopy) || !reflect.DeepEqual(complexInput, complexCopy) {
			t.Fatal("input modified")
		}
	}
}

func TestFFTRealRows(t *testing.T) {
	a := [][]float64{{1, 2, 3, 4, 5}, {20, 20, 20, 20, 20}}
	for _, dc := range []bool{false, true} {
		got, err := dataprocess.FFTRealRows(a, dc)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(a) {
			t.Fatal("row count changed")
		}
		for i, row := range a {
			seq := make([]complex128, len(row))
			for j, v := range row {
				seq[j] = complex(v, 0)
			}
			assertSpectrum(t, got[i], referenceDFT(seq, dc))
		}
	}
	if !reflect.DeepEqual(a, [][]float64{{1, 2, 3, 4, 5}, {20, 20, 20, 20, 20}}) {
		t.Fatal("matrix modified")
	}
}

func TestFFTRadarLength(t *testing.T) {
	const n, bin = 2125, 37
	input := make([]float64, n)
	for i := range input {
		input[i] = 10 + 2*math.Cos(2*math.Pi*bin*float64(i)/n)
	}
	a, err := dataprocess.FFTReal(input, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := dataprocess.FFTReal(input, true)
	if err != nil {
		t.Fatal(err)
	}
	for k := range a {
		want := complex(0, 0)
		if k == bin || k == n-bin {
			want = n
		}
		if cmplx.Abs(b[k]-want) > 1e-7 {
			t.Fatalf("demeaned bin %d: %v", k, b[k])
		}
		if k == 0 {
			want = 10 * n
		}
		if cmplx.Abs(a[k]-want) > 1e-7 {
			t.Fatalf("raw bin %d: %v", k, a[k])
		}
	}
	dir := complexPartsResults(t)
	if err := dataprocess.SaveComplexMatrixParts(filepath.Join(dir, "fft_without_dc_removal"), [][]complex128{a}); err != nil {
		t.Fatal(err)
	}
	if err := dataprocess.SaveComplexMatrixParts(filepath.Join(dir, "fft_with_dc_removal"), [][]complex128{b}); err != nil {
		t.Fatal(err)
	}
	report := fmt.Sprintf("N=%d\r\ntone_bin=%d\r\nDC_before=%v\r\nDC_after=%v\r\ntone_amplitude=%v\r\n", n, bin, a[0], b[0], b[bin])
	if err := os.WriteFile(filepath.Join(dir, "comparison.txt"), []byte(report), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFFTComplexRows(t *testing.T) {
	a := [][]complex128{{1 + 2i, 3 - 4i, 5 + 6i}, {10 + 20i, 10 + 20i, 10 + 20i}}
	before := make([][]complex128, len(a))
	for i := range a {
		before[i] = append([]complex128(nil), a[i]...)
	}
	for _, dc := range []bool{false, true} {
		got, err := dataprocess.FFTComplexRows(a, dc)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(a) {
			t.Fatal("row count changed")
		}
		for i := range a {
			assertSpectrum(t, got[i], referenceDFT(a[i], dc))
		}
		// Output rows are independent from input and from each other.
		other := got[1][0]
		got[0][0] = 999
		if got[1][0] != other {
			t.Fatal("output rows share storage")
		}
	}
	if !reflect.DeepEqual(a, before) {
		t.Fatal("complex matrix modified")
	}
}

func TestFFTComplexToneAndDC(t *testing.T) {
	const n, bin = 25, 4
	input := make([]complex128, n)
	for i := range input {
		input[i] = 7 - 3i + cmplx.Exp(complex(0, 2*math.Pi*bin*float64(i)/n))
	}
	for _, dc := range []bool{false, true} {
		got, err := dataprocess.FFTComplex(input, dc)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]complex128, n)
		want[bin] = n
		if !dc {
			want[0] = n * (7 - 3i)
		}
		assertSpectrum(t, got, want)
	}
}

func TestFFTInvalidInput(t *testing.T) {
	for _, a := range [][]float64{nil, {}, {math.NaN()}, {math.Inf(1)}} {
		if _, err := dataprocess.FFTReal(a, true); err == nil {
			t.Fatal("invalid real vector accepted")
		}
	}
	for _, a := range [][]complex128{nil, {}, {complex(0, math.Inf(-1))}, {complex(math.NaN(), 0)}} {
		if _, err := dataprocess.FFTComplex(a, false); err == nil {
			t.Fatal("invalid complex vector accepted")
		}
	}
	for _, a := range [][][]float64{nil, {{}}, {{1}, {1, 2}}, {{1}, {math.NaN()}}} {
		if _, err := dataprocess.FFTRealRows(a, false); err == nil {
			t.Fatal("invalid real matrix accepted")
		}
	}
	for _, a := range [][][]complex128{nil, {{}}, {{1}, {1, 2}}, {{1}, {complex(0, math.NaN())}}} {
		if _, err := dataprocess.FFTComplexRows(a, true); err == nil {
			t.Fatal("invalid complex matrix accepted")
		}
	}
}
