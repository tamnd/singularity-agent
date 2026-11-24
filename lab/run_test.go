package lab

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testE1 = `name: e1-test
experiment: E1
question: A test.
tool: sing-lab
node: SG00000016
inputs:
  seed: 7
  family: { name: random-sparse, terms: [2, 5], divisor: random }
  walk: { depth: 8, max_terms: 200, max_expand: 50000, max_degree: 120 }
  examples: 5
  stability_examples: 3
  raw: true
  cases:
    - { p: 2, n: 3, q: 2, walks: 700, degree: [3, 8] }
    - { p: 2, n: 3, q: 4, walks: 600, degree: [5, 12] }
    - { p: 3, n: 2, q: 9, walks: 300, degree: [10, 27] }
outputs: [histogram, examples]
`

const testE2 = `name: e2-test
experiment: E2
question: A test.
tool: sing-lab
node: SG00000013
inputs:
  walks_from: e1-test.yaml
outputs: [histogram, candidates]
`

// runAll runs the two test specs with the given number of threads and returns the files of the runs.
func runAll(t *testing.T, threads int, host string, now time.Time) map[string][]byte {
	t.Helper()
	root := t.TempDir()
	specs := filepath.Join(root, "lab", "specs")
	if err := os.MkdirAll(specs, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{"e1-test.yaml": testE1, "e2-test.yaml": testE2} {
		if err := os.WriteFile(filepath.Join(specs, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out := map[string][]byte{}
	for _, name := range []string{"e1-test.yaml", "e2-test.yaml"} {
		s, err := LoadSpec(filepath.Join(specs, name))
		if err != nil {
			t.Fatal(err)
		}
		clock := now
		opt := Options{Runs: RunsDir(s.Path), Threads: threads, Host: host, Tool: ToolInfo{Version: "test"}, Now: func() time.Time { clock = clock.Add(time.Duration(threads) * time.Second); return clock }}
		rec, err := Run(context.Background(), s, opt)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(opt.Runs, rec.ID)
		err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(path) // #nosec G304 -- a test file
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(opt.Runs, path)
			out[rel] = data
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// TestReproducible checks that a run gives the same files for each number of threads, apart from the host and the times in record.json.
func TestReproducible(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	a := runAll(t, 1, "host-a", now)
	b := runAll(t, 4, "host-b", now.Add(time.Hour))
	if len(a) != len(b) {
		t.Fatalf("the runs have %d and %d files", len(a), len(b))
	}
	raw := 0
	for name, x := range a {
		y, ok := b[name]
		if !ok {
			t.Fatalf("the second run has no %s", name)
		}
		if filepath.Base(filepath.Dir(name)) == "raw" {
			raw++
		}
		if filepath.Base(name) == "record.json" {
			x, y = normRecord(t, x), normRecord(t, y)
		}
		if !bytes.Equal(x, y) {
			t.Errorf("%s is not the same in the two runs", name)
		}
	}
	if raw == 0 {
		t.Error("the E1 run wrote no raw files")
	}
	var h1, h2 []byte
	for name, x := range a {
		switch filepath.Base(name) {
		case "histogram.json":
			if filepath.Base(filepath.Dir(name))[11:18] == "e1-test" {
				h1 = x
			} else {
				h2 = x
			}
		}
	}
	if h1 == nil || !bytes.Equal(stripSpec(t, h1), stripSpec(t, h2)) {
		t.Error("the E2 run does not make the same walks as the E1 run")
	}
}

func normRecord(t *testing.T, data []byte) []byte {
	t.Helper()
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Host == "" || r.WallSeconds == 0 {
		t.Error("the record has no host or no wall time")
	}
	r.Host, r.Started, r.WallSeconds = "", "", 0
	return marshal(r)
}

// stripSpec removes the name of the spec from a histogram.
func stripSpec(t *testing.T, data []byte) []byte {
	t.Helper()
	var h Histogram
	if err := json.Unmarshal(data, &h); err != nil {
		t.Fatal(err)
	}
	h.Spec = ""
	return marshal(h)
}
