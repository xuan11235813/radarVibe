package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"radar/lib/dataprocess"
	"radar/lib/radar"
)

const usage = `Usage:
  dataProcess filename.csv splitChannel [start end]
  dataProcess filename.csv fftChannel [start end] [removeDC]
Indices are zero-based and inclusive; default range is the full period.
removeDC is true (default) or false; it may also be specified without a range.
All outputs are saved in the current working directory; existing files are not overwritten.`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dataProcess:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Println(usage)
		return nil
	}
	if len(args) < 2 {
		return fmt.Errorf("%s", usage)
	}
	if args[1] != "splitChannel" && args[1] != "fftChannel" {
		return fmt.Errorf("unknown operation %q\n%s", args[1], usage)
	}
	fftMode := args[1] == "fftChannel"
	if (!fftMode && len(args) != 2 && len(args) != 4) || (fftMode && (len(args) < 2 || len(args) > 5)) {
		return fmt.Errorf("%s", usage)
	}
	removeDC := true
	if fftMode && (len(args) == 3 || len(args) == 5) {
		value := args[len(args)-1]
		if value != "true" && value != "false" {
			return fmt.Errorf("removeDC must be true or false, got %q", value)
		}
		removeDC = value == "true"
	}
	start, end := 0, radar.SamplesPerPeriod-1
	if len(args) >= 4 {
		var err error
		start, err = strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid start index %q", args[2])
		}
		end, err = strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("invalid end index %q", args[3])
		}
	}
	if start < 0 || end < start || end >= radar.SamplesPerPeriod {
		return fmt.Errorf("indices must satisfy 0 <= start <= end < %d", radar.SamplesPerPeriod)
	}
	samples, err := dataprocess.ReadCSV(args[0])
	if err != nil {
		return fmt.Errorf("read CSV: %w", err)
	}
	matrices, err := dataprocess.ToMatrices(samples)
	if err != nil {
		return fmt.Errorf("convert matrices: %w", err)
	}
	selected, err := dataprocess.SelectSamples(matrices, start, end)
	if err != nil {
		return fmt.Errorf("select samples: %w", err)
	}
	cwd, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	if fftMode {
		if err := fftChannel(selected, removeDC, cwd); err != nil {
			return err
		}
		fmt.Printf("FFT: %d periods x %d bins per channel; range %d..%d; removeDC=%t; discarded %d incomplete periods.\n", len(selected.Ch6), end-start+1, start, end, removeDC, len(matrices.Ch6)-len(selected.Ch6))
		return nil
	}
	if err := dataprocess.SaveMatrices(".", selected); err != nil {
		return fmt.Errorf("save matrices: %w", err)
	}
	fmt.Printf("Saved %d periods x %d samples per channel (range %d..%d); discarded %d incomplete periods.\n", len(selected.Ch6), end-start+1, start, end, len(matrices.Ch6)-len(selected.Ch6))
	fmt.Println(filepath.Join(cwd, "datach6.csv"))
	fmt.Println(filepath.Join(cwd, "datach7.csv"))
	return nil
}

// fftChannel only orchestrates library algorithms and file writers.
func fftChannel(selected dataprocess.ChannelMatrices, removeDC bool, dir string) error {
	prefixes := []string{filepath.Join(dir, "fftch6"), filepath.Join(dir, "fftch7")}
	suffixes := []string{".txt", "_real.csv", "_img.csv"}
	// Check all destinations before creating any files, including the last one.
	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			path := prefix + suffix
			if _, err := os.Lstat(path); err == nil {
				return fmt.Errorf("output already exists: %s", path)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}
	channels := [][][]float64{selected.Ch6, selected.Ch7}
	spectra := make([][][]complex128, len(channels))
	for i, channel := range channels {
		var err error
		spectra[i], err = dataprocess.FFTRealRows(channel, removeDC)
		if err != nil {
			return fmt.Errorf("FFT ch%d: %w", i+6, err)
		}
	}
	for i, spectrum := range spectra {
		if err := dataprocess.SaveComplexMatrix(prefixes[i]+".txt", spectrum); err != nil {
			return fmt.Errorf("save ch%d complex spectrum: %w", i+6, err)
		}
		if err := dataprocess.SaveComplexMatrixParts(prefixes[i], spectrum); err != nil {
			return fmt.Errorf("save ch%d spectrum parts: %w", i+6, err)
		}
	}
	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			fmt.Println(prefix + suffix)
		}
	}
	return nil
}
