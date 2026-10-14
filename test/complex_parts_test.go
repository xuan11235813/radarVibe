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

func complexPartsResults(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("results")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(root, t.Name()+"-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained output: %s", dir)
	return dir
}

func TestComplexMatrixPartsRandomRoundTrip(t *testing.T) {
	dir := complexPartsResults(t)
	rng := rand.New(rand.NewSource(20261014))
	a := make([][]complex128, 100)
	for i := range a {
		a[i] = make([]complex128, 100)
		for j := range a[i] {
			a[i][j] = complex((rng.Float64()*2-1)*1e6, (rng.Float64()*2-1)*1e6)
		}
	}
	prefix := filepath.Join(dir, "matrix")
	if err := dataprocess.SaveComplexMatrixParts(prefix, a); err != nil {
		t.Fatal(err)
	}
	b, err := dataprocess.ReadComplexMatrixParts(prefix+"_real.csv", prefix+"_img.csv")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 100 {
		t.Fatalf("row count = %d", len(b))
	}
	mismatches := 0
	var maxDiff float64
	for i := range a {
		if len(b[i]) != 100 {
			t.Fatalf("row %d column count = %d", i, len(b[i]))
		}
		for j, want := range a[i] {
			got := b[i][j]
			maxDiff = math.Max(maxDiff, cmplx.Abs(got-want))
			if math.Float64bits(real(got)) != math.Float64bits(real(want)) || math.Float64bits(imag(got)) != math.Float64bits(imag(want)) {
				mismatches++
			}
		}
	}
	report := fmt.Sprintf("matrix=100x100\r\ncompared=10000\r\nmismatches=%d\r\nmax_absolute_difference=%g\r\n", mismatches, maxDiff)
	if err := os.WriteFile(filepath.Join(dir, "comparison.txt"), []byte(report), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("compared=10000; mismatches=%d; max absolute difference=%g", mismatches, maxDiff)
	if mismatches != 0 {
		t.Fatal("component precision changed")
	}
}

func TestComplexMatrixPartsValidation(t *testing.T) {
	dir := complexPartsResults(t)
	prefix := filepath.Join(dir, "example")
	if err := dataprocess.SaveComplexMatrixParts(prefix, [][]complex128{{1 + 2i, 3 - 4i}, {5, -6i}}); err != nil {
		t.Fatal(err)
	}
	for suffix, want := range map[string]string{"_real.csv": "1,3\n5,0\n", "_img.csv": "2,-4\n0,-6\n"} {
		got, err := os.ReadFile(prefix + suffix)
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q %v", suffix, got, err)
		}
	}
	// Failure opening the second file must not overwrite it or leave the first.
	protected := filepath.Join(dir, "protected")
	if err := os.WriteFile(protected+"_img.csv", []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := dataprocess.SaveComplexMatrixParts(protected, [][]complex128{{1}}); err == nil {
		t.Fatal("existing output was accepted")
	}
	if _, err := os.Stat(protected + "_real.csv"); !os.IsNotExist(err) {
		t.Fatal("partial output pair left behind")
	}
	if got, err := os.ReadFile(protected + "_img.csv"); err != nil || string(got) != "original" {
		t.Fatal("existing file changed")
	}
	for i, text := range []string{"", "1\n", "1,2\n", "1,2\n3\n", "1,2\n3,invalid\n", "1,2\n3,4+5i\n"} {
		path := filepath.Join(dir, fmt.Sprintf("invalid_%d.csv", i))
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		if got, err := dataprocess.ReadComplexMatrixParts(prefix+"_real.csv", path); err == nil || got != nil {
			t.Fatalf("accepted bad imaginary matrix %q", text)
		}
	}
	if _, err := dataprocess.ReadComplexMatrixParts(filepath.Join(dir, "missing.csv"), prefix+"_img.csv"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing input: %v", err)
	}
	if err := dataprocess.SaveComplexMatrixParts(filepath.Join(dir, "ragged"), [][]complex128{{1}, {2, 3}}); err == nil {
		t.Fatal("accepted ragged matrix")
	}
	if err := dataprocess.SaveComplexMatrixParts(filepath.Join(dir, "empty"), nil); err == nil {
		t.Fatal("accepted empty matrix")
	}
}

func TestComplexMatrixPartsSpecialValues(t *testing.T) {
	dir := complexPartsResults(t)
	a := [][]complex128{{complex(math.SmallestNonzeroFloat64, math.MaxFloat64), complex(math.Copysign(0, -1), math.Inf(-1)), complex(math.NaN(), math.Inf(1))}}
	prefix := filepath.Join(dir, "special")
	if err := dataprocess.SaveComplexMatrixParts(prefix, a); err != nil {
		t.Fatal(err)
	}
	b, err := dataprocess.ReadComplexMatrixParts(prefix+"_real.csv", prefix+"_img.csv")
	if err != nil {
		t.Fatal(err)
	}
	if b[0][0] != a[0][0] || !math.Signbit(real(b[0][1])) || !math.IsInf(imag(b[0][1]), -1) || !math.IsNaN(real(b[0][2])) || !math.IsInf(imag(b[0][2]), 1) {
		t.Fatalf("special values changed: %v", b)
	}
}
