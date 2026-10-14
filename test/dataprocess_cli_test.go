package test

import (
	"fmt"
	"math/cmplx"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"radar/lib/dataprocess"
	"radar/lib/radar"
)

func TestDataProcessFFTCLI(t *testing.T) {
	dir := complexPartsResults(t)
	exe := filepath.Join(dir, "dataProcess")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build := exec.Command("go", "build", "-o", exe, "../toolbox/dataProcess")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	var input strings.Builder
	input.WriteString("period_index,sample_in_period,ch6,ch7\n")
	for i := 0; i < radar.SamplesPerPeriod+5; i++ {
		v := 1
		if i == 0 {
			v = 2
		}
		fmt.Fprintf(&input, "%d,%d,%d,5\n", i/radar.SamplesPerPeriod, i%radar.SamplesPerPeriod, v)
	}
	inputPath := filepath.Join(dir, "input.csv")
	if err := os.WriteFile(inputPath, []byte(input.String()), 0644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		n    int
		dc   bool
	}{
		{"default", nil, 2125, true},
		{"full_keep_dc", []string{"false"}, 2125, false},
		{"crop", []string{"0", "3"}, 4, true},
		{"crop_keep_dc", []string{"0", "3", "false"}, 4, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outDir := filepath.Join(dir, tc.name)
			if err := os.Mkdir(outDir, 0755); err != nil {
				t.Fatal(err)
			}
			args := append([]string{inputPath, "fftChannel"}, tc.args...)
			cmd := exec.Command(exe, args...)
			cmd.Dir = outDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run: %v\n%s", err, output)
			}
			if err := os.WriteFile(filepath.Join(outDir, "run.txt"), output, 0644); err != nil {
				t.Fatal(err)
			}
			for _, ch := range []int{6, 7} {
				prefix := filepath.Join(outDir, fmt.Sprintf("fftch%d", ch))
				full, err := dataprocess.ReadComplexMatrix(prefix + ".txt")
				if err != nil {
					t.Fatal(err)
				}
				parts, err := dataprocess.ReadComplexMatrixParts(prefix+"_real.csv", prefix+"_img.csv")
				if err != nil {
					t.Fatal(err)
				}
				if len(full) != 1 || len(parts) != 1 || len(full[0]) != tc.n || len(parts[0]) != tc.n {
					t.Fatal("wrong shape or partial period retained")
				}
				for k, v := range full[0] {
					want := complex(0, 0)
					if ch == 6 {
						want = 1
					}
					if k == 0 {
						want = 0
						if !tc.dc {
							if ch == 6 {
								want = complex(float64(tc.n+1), 0)
							} else {
								want = complex(float64(tc.n*5), 0)
							}
						}
					}
					if cmplx.Abs(v-want) > 1e-8 || parts[0][k] != v {
						t.Fatalf("ch%d bin%d: got %v, want %v; parts %v", ch, k, v, want, parts[0][k])
					}
				}
			}
		})
	}
	// A collision in the final destination must prevent all other output writes.
	blocked := filepath.Join(dir, "existing_output")
	if err := os.Mkdir(blocked, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "fftch7_img.csv"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, inputPath, "fftChannel")
	cmd.Dir = blocked
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "already exists") {
		t.Fatalf("collision: %v %s", err, out)
	}
	files, err := os.ReadDir(blocked)
	if err != nil || len(files) != 1 {
		t.Fatal("collision created partial outputs")
	}
	for _, args := range [][]string{{inputPath, "fftChannel", "maybe"}, {inputPath, "fftChannel", "3", "0"}, {inputPath, "splitChannel", "true"}} {
		cmd := exec.Command(exe, args...)
		cmd.Dir = blocked
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("accepted bad args %v: %s", args, out)
		}
	}
	// Existing splitChannel behavior is still available.
	splitDir := filepath.Join(dir, "split")
	if err := os.Mkdir(splitDir, 0755); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(exe, inputPath, "splitChannel", "0", "3")
	cmd.Dir = splitDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("splitChannel: %v %s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(splitDir, "datach6.csv"))
	if err != nil || string(data) != "2,1,1,1\n" {
		t.Fatalf("splitChannel output: %q %v", data, err)
	}
}
