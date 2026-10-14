package dataprocess

import (
	"fmt"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/dsp/fourier"
)

// FFTReal computes the unnormalized forward FFT of a real vector. The result
// contains all N complex bins in standard order (DC at index 0), with no padding
// or fftshift. If removeDC is true, the vector mean is subtracted first.
// Input is never modified. Empty input and nonfinite values return an error.
func FFTReal(input []float64, removeDC bool) ([]complex128, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("FFT input must not be empty")
	}
	return fftReal(fourier.NewFFT(len(input)), input, removeDC)
}

// FFTRealRows transforms each row independently along the second dimension.
// Output has the same shape as input. removeDC subtracts each row's own mean.
// No transform is performed across rows. The matrix must be nonempty/rectangular.
func FFTRealRows(input [][]float64, removeDC bool) ([][]complex128, error) {
	if len(input) == 0 || len(input[0]) == 0 {
		return nil, fmt.Errorf("FFT matrix must contain at least one row and column")
	}
	n := len(input[0])
	plan := fourier.NewFFT(n)
	output := make([][]complex128, len(input))
	for i, row := range input {
		if len(row) != n {
			return nil, fmt.Errorf("FFT row %d has %d samples, want %d", i, len(row), n)
		}
		var err error
		output[i], err = fftReal(plan, row, removeDC)
		if err != nil {
			return nil, fmt.Errorf("FFT row %d: %w", i, err)
		}
	}
	return output, nil
}

// FFTComplex computes the unnormalized forward FFT of a complex vector.
// removeDC subtracts its complex mean (real and imaginary parts) before FFT.
// The N output bins are not shifted or padded, and input is never modified.
func FFTComplex(input []complex128, removeDC bool) ([]complex128, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("FFT input must not be empty")
	}
	return fftComplex(fourier.NewCmplxFFT(len(input)), input, removeDC)
}

// FFTComplexRows transforms each complex row along the second dimension only.
// removeDC subtracts each row's own complex mean. It does not perform a full
// two-dimensional FFT. Input is unchanged; output retains the original shape.
func FFTComplexRows(input [][]complex128, removeDC bool) ([][]complex128, error) {
	if err := validateComplexMatrix(input); err != nil {
		return nil, err
	}
	plan := fourier.NewCmplxFFT(len(input[0]))
	output := make([][]complex128, len(input))
	for i, row := range input {
		var err error
		output[i], err = fftComplex(plan, row, removeDC)
		if err != nil {
			return nil, fmt.Errorf("FFT row %d: %w", i, err)
		}
	}
	return output, nil
}

func fftReal(plan *fourier.FFT, input []float64, removeDC bool) ([]complex128, error) {
	for i, value := range input {
		if !finite(value) {
			return nil, fmt.Errorf("nonfinite sample at index %d", i)
		}
	}
	seq := input
	if removeDC {
		seq = append([]float64(nil), input...)
		var mean, correction float64
		for _, value := range seq {
			// Divide before summing to avoid overflow in the mean calculation.
			y := value/float64(len(seq)) - correction
			sum := mean + y
			correction = (sum - mean) - y
			mean = sum
		}
		for i := range seq {
			seq[i] -= mean
		}
	}
	n := len(seq)
	result := make([]complex128, n)
	plan.Coefficients(result[:n/2+1], seq)
	for k := n/2 + 1; k < n; k++ {
		result[k] = cmplx.Conj(result[n-k])
	}
	return checkedSpectrum(result)
}

func fftComplex(plan *fourier.CmplxFFT, input []complex128, removeDC bool) ([]complex128, error) {
	for i, value := range input {
		if !finite(real(value)) || !finite(imag(value)) {
			return nil, fmt.Errorf("nonfinite sample at index %d", i)
		}
	}
	seq := input
	if removeDC {
		seq = append([]complex128(nil), input...)
		var mean, correction complex128
		for _, value := range seq {
			y := value/complex(float64(len(seq)), 0) - correction
			sum := mean + y
			correction = (sum - mean) - y
			mean = sum
		}
		for i := range seq {
			seq[i] -= mean
		}
	}
	return checkedSpectrum(plan.Coefficients(nil, seq))
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func checkedSpectrum(output []complex128) ([]complex128, error) {
	for i, value := range output {
		if !finite(real(value)) || !finite(imag(value)) {
			return nil, fmt.Errorf("FFT numerical overflow at bin %d", i)
		}
	}
	return output, nil
}
