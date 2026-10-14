package test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"radar/lib/dataprocess"
	"radar/lib/radar"
)

func writeInput(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.csv")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDataProcessPipeline(t *testing.T) {
	var input strings.Builder
	// Header mapping supports BOM, whitespace, reordered and extra columns.
	input.WriteString("\ufeffch7,period_index, ch6 ,sample_in_period,extra\n")
	for i := 0; i < 2*radar.SamplesPerPeriod+17; i++ {
		fmt.Fprintf(&input, "%d,%d,%d,%d,ignored\n", i+9000, i/radar.SamplesPerPeriod, i, i%radar.SamplesPerPeriod)
	}
	samples, err := dataprocess.ReadCSV(writeInput(t, input.String()))
	if err != nil {
		t.Fatal(err)
	}
	m, err := dataprocess.ToMatrices(samples)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Ch6) != 3 || len(m.Ch7[2]) != 17 || m.Ch6[1][0] != 2125 || m.Ch7[2][16] != 13266 {
		t.Fatal("incorrect period/channel mapping")
	}
	full, err := dataprocess.SelectSamples(m, 0, radar.SamplesPerPeriod-1)
	if err != nil || len(full.Ch6) != 2 || len(full.Ch6[1]) != 2125 {
		t.Fatalf("full range: %v", err)
	}
	selected, err := dataprocess.SelectSamples(m, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selected.Ch6, [][]float64{{1, 2, 3}, {2126, 2127, 2128}}) || !reflect.DeepEqual(selected.Ch7, [][]float64{{9001, 9002, 9003}, {11126, 11127, 11128}}) {
		t.Fatalf("incorrect inclusive range: %+v", selected)
	}
	dir := t.TempDir()
	if err := dataprocess.SaveMatrices(dir, selected); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"datach6.csv": "1,2,3\n2126,2127,2128\n",
		"datach7.csv": "9001,9002,9003\n11126,11127,11128\n",
	} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q %v", name, got, err)
		}
	}
	selected.Ch6[0][0] = -1
	if m.Ch6[0][1] != 1 || full.Ch6[0][1] != 1 || selected.Ch7[0][0] != 9001 {
		t.Fatal("selected matrices share sample storage")
	}
	single, err := dataprocess.SelectSamples(m, 2124, 2124)
	if err != nil || single.Ch6[1][0] != 4249 || len(single.Ch7[0]) != 1 {
		t.Fatalf("inclusive last index: %v", err)
	}
}

func TestDataProcessBadInput(t *testing.T) {
	header := "period_index,sample_in_period,ch6,ch7\n"
	for _, input := range []string{
		"", header, "ch6,ch7\n1,2\n", "period_index,sample_in_period,ch6,ch6\n0,0,1,2\n",
		header + "0,0,1\n", header + "-1,0,1,2\n", header + "0,1.5,1,2\n",
		header + "0,0,NaN,2\n", header + "0,0,1,Inf\n", header + "0,0,foo,2\n",
	} {
		if _, err := dataprocess.ReadCSV(writeInput(t, input)); err == nil {
			t.Fatalf("accepted invalid input %q", input)
		}
	}
	for _, samples := range [][]dataprocess.Sample{
		nil,
		{{PeriodIndex: 1}},
		{{SampleInPeriod: 1}},
		{{}, {}},
		{{}, {SampleInPeriod: 2}},
		{{}, {PeriodIndex: 2}},
		{{SampleInPeriod: 2125}},
		{{PeriodIndex: -1}},
	} {
		if _, err := dataprocess.ToMatrices(samples); err == nil {
			t.Fatalf("accepted nonconsecutive indices: %+v", samples)
		}
	}
	m := dataprocess.ChannelMatrices{Ch6: [][]float64{make([]float64, 2125)}, Ch7: [][]float64{make([]float64, 2125)}}
	for _, bounds := range [][2]int{{-1, 1}, {2, 1}, {0, 2125}} {
		if _, err := dataprocess.SelectSamples(m, bounds[0], bounds[1]); err == nil {
			t.Fatalf("accepted range %v", bounds)
		}
	}
	partial := dataprocess.ChannelMatrices{Ch6: [][]float64{{1, 2}}, Ch7: [][]float64{{3, 4}}}
	if _, err := dataprocess.SelectSamples(partial, 0, 1); err == nil {
		t.Fatal("partial period retained just because it covers the selected range")
	}
	m.Ch7[0] = m.Ch7[0][:1]
	if _, err := dataprocess.SelectSamples(m, 0, 1); err == nil {
		t.Fatal("mismatched channels accepted")
	}
}

func TestDataProcessOutputProtection(t *testing.T) {
	m := dataprocess.ChannelMatrices{Ch6: [][]float64{{1.25, -2}}, Ch7: [][]float64{{3, 4.5}}}
	dir := t.TempDir()
	preserved := filepath.Join(dir, "datach7.csv")
	if err := os.WriteFile(preserved, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := dataprocess.SaveMatrices(dir, m); err == nil {
		t.Fatal("overwrote existing output")
	}
	if _, err := os.Stat(filepath.Join(dir, "datach6.csv")); !os.IsNotExist(err) {
		t.Fatal("left partial output pair")
	}
	if b, err := os.ReadFile(preserved); err != nil || string(b) != "existing" {
		t.Fatal("changed existing output")
	}
	dir = t.TempDir()
	if err := dataprocess.SaveMatrices(dir, m); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "datach6.csv"))
	if err != nil || string(b) != "1.25,-2\n" {
		t.Fatalf("numeric precision: %q %v", b, err)
	}
}

func TestReceiveDataCSVCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.csv")
	if err := radar.SaveCSV(path, validFrame()); err != nil {
		t.Fatal(err)
	}
	samples, err := dataprocess.ReadCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := dataprocess.ToMatrices(samples)
	if err != nil || len(m.Ch6) != 247 || len(m.Ch6[246]) != 1538 {
		t.Fatalf("receiveData frame conversion: %v", err)
	}
	selected, err := dataprocess.SelectSamples(m, 0, radar.SamplesPerPeriod-1)
	if err != nil || len(selected.Ch6) != 246 {
		t.Fatalf("complete periods: %v", err)
	}
	dir := t.TempDir()
	if err := dataprocess.SaveMatrices(dir, selected); err != nil {
		t.Fatal(err)
	}
	for name, ch := range map[string]int{"datach6.csv": 6, "datach7.csv": 7} {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(f).ReadAll()
		f.Close()
		if err != nil || len(rows) != 246 || len(rows[245]) != 2125 {
			t.Fatalf("%s dimensions: %v", name, err)
		}
		want := fmt.Sprint((245*2125 + 2124 + ch) % 16384)
		if rows[245][2124] != want {
			t.Fatalf("%s last sample: %s, want %s", name, rows[245][2124], want)
		}
	}
}
