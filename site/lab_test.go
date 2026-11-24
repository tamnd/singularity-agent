package site

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/singularity-agent/lab"
)

func TestBuildLab(t *testing.T) {
	root := content(t)
	put(t, root, "lab/specs/e1-walks.yaml", "name: e1-walks\nexperiment: E1\nquestion: How does the residual order change?\ntool: sing-lab\nnode: SG00000001\ninputs:\n  seed: 3\n  family: { name: random-sparse }\n  walk: { depth: 6, max_terms: 100 }\n  examples: 2\n  cases:\n    - { p: 2, n: 3, q: 2, walks: 300, degree: [3, 6], terms: [2, 4] }\noutputs: [histogram, examples]\n")
	put(t, root, "lab/specs/e1-fixed.yaml", "name: e1-fixed\nexperiment: E1\nquestion: Does the example match?\ntool: sing-lab\nnode: SG00000001\ninputs:\n  fixed:\n    p: 2\n    q: 4\n    vars: [x, y, w]\n    f: \"x^2*y^2*w^3*(w*(x+y)^4 + x^13)\"\n    divisor: [x, y, w]\n    expect: 5\n    steps:\n      - { chart: x, translate: { y: 1 }, expect: 7 }\noutputs: [steps]\n")
	var ids []string
	for _, name := range []string{"e1-walks", "e1-fixed"} {
		s, err := lab.LoadSpec(filepath.Join(root, "lab", "specs", name+".yaml"))
		if err != nil {
			t.Fatal(err)
		}
		rec, err := lab.Run(context.Background(), s, lab.Options{Runs: filepath.Join(root, "lab", "runs"), Threads: 2, Host: "test", Tool: lab.ToolInfo{Version: "test", Commit: "0123456789abcdef"}, Now: func() time.Time { return time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC) }})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, rec.ID)
	}
	out := filepath.Join(t.TempDir(), "public")
	if _, err := Build(context.Background(), Options{Content: root, Out: out}); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	idx := read("lab/index.html")
	for _, want := range []string{ids[0], ids[1], "How does the residual order change?", `href="/node/SG00000001/"`, "kangaroo points", "/lab/" + ids[0] + "/examples.json", "steps with d' = d"} {
		if !strings.Contains(idx, want) {
			t.Errorf("lab/index.html does not have %q", want)
		}
	}
	if got := read("lab/" + ids[1] + "/index.html"); !strings.Contains(got, "<td>7</td>") || strings.Contains(got, "no match") {
		t.Error("the page of the fixed run does not show the residual order 7")
	}
	if got := read("lab/" + ids[0] + "/record.json"); !strings.Contains(got, `"host": "test"`) {
		t.Error("the site does not copy record.json")
	}
	if got := read("lab/" + ids[1] + "/index.md"); !strings.Contains(got, "The engine gives the value of the source") {
		t.Error("the Markdown copy of a run has no summary")
	}
	r, err := Check(out)
	if err != nil {
		t.Fatal(err)
	}
	if !r.OK() {
		t.Errorf("the check finds problems: %v", r.Problems)
	}
}
