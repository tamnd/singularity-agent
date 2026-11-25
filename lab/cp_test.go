package lab

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cpAt returns epsilon, omega and kappa of z^p + F along the divisor of the named variables.
func cpAt(t *testing.T, p int, names []string, src string, div ...string) (State, Divisor) {
	t.Helper()
	en := mustEngine(t, p, p)
	f := mustParse(t, p, names, src)
	if !f.IsClean(p) {
		t.Fatalf("%s is not clean", src)
	}
	var d Divisor
	for _, v := range div {
		for i, n := range names {
			if n == v {
				d |= 1 << i
			}
		}
	}
	return en.Measure(f, d), d
}

// TestCPRemark270 is the example of [CP19, Remark 2.70] for p = 3 (the handbook, Volume 6, Example 3.3.2): two equations with the same polyhedron and omega = p + 2 and p + 1.
func TestCPRemark270(t *testing.T) {
	names := []string{"u1", "u2", "u3"}
	for _, tc := range []struct {
		f     string
		omega int
	}{
		{"u1*u2*u3^3 + u1^5 + u2^5", 5},
		{"u1*u2*u3^3 + u1^5 + u2^5 + u1^3*u2*u3", 4},
	} {
		st, _ := cpAt(t, 3, names, tc.f, "u1", "u2")
		want := CPState{OrdF: 5, Omega: tc.omega, Kappa: ">=2"}
		if st.D != 5 || st.CP != want {
			t.Errorf("%s: epsilon %d and %+v, want 5 and %+v", tc.f, st.D, st.CP, want)
		}
	}
}

// TestCPExample247 is [CP19, Example 2.47] at W = m_S with p = 3 and a = b = 1: for u1 u2 u3^(p+1) the module V is <U3^p> and omega = epsilon - 1 = p; for u1 u2 u3^p it is 0 and omega = epsilon = p.
func TestCPExample247(t *testing.T) {
	names := []string{"u1", "u2", "u3"}
	st, _ := cpAt(t, 3, names, "u1*u2*u3^4", "u1", "u2")
	if st.D != 4 || st.CP.Omega != 3 {
		t.Errorf("u1 u2 u3^4: epsilon %d, omega %d, want 4 and 3", st.D, st.CP.Omega)
	}
	st, _ = cpAt(t, 3, names, "u1*u2*u3^3", "u1", "u2")
	if st.D != 3 || st.CP.Omega != 3 {
		t.Errorf("u1 u2 u3^3: epsilon %d, omega %d, want 3 and 3", st.D, st.CP.Omega)
	}
	// The handbook, Volume 6, Example 3.2.8: z^3 + u1 u2 u3 with E = div(u1 u2 u3) has epsilon = omega = 0 and kappa = 1.
	st, _ = cpAt(t, 3, names, "u1*u2*u3", "u1", "u2", "u3")
	if st.D != 0 || st.CP != (CPState{OrdF: 3, Omega: 0, Kappa: "1"}) {
		t.Errorf("u1 u2 u3: epsilon %d and %+v, want 0, omega 0 and kappa 1", st.D, st.CP)
	}
}

// TestCPExample331 is the table of the handbook, Volume 6, Example 3.3.1: p = 2, f = u1 u2^3 u3^2 + u1^3 u2 u3^2 + u1^8 u2 u3 and E = div(u1 u2 u3), with epsilon = omega = 3 at the origin, and the seven points of the exceptional plane with coordinates 0 or 1.
func TestCPExample331(t *testing.T) {
	names := []string{"u1", "u2", "u3"}
	src := "u1*u2^3*u3^2 + u1^3*u2*u3^2 + u1^8*u2*u3"
	st, d := cpAt(t, 2, names, src, "u1", "u2", "u3")
	if st.D != 3 || st.CP != (CPState{OrdF: 6, Omega: 3, Kappa: ">=2"}) {
		t.Fatalf("at the origin: epsilon %d and %+v, want 3, ord 6 and omega 3", st.D, st.CP)
	}
	f := mustParse(t, 2, names, src)
	rows := []struct {
		pt              Point
		ord, eps, omega int
	}{
		{Point{J: 0}, 7, 1, 1},
		{Point{J: 0, T: Exp{0, 0, 1}}, 5, 0, 0},
		{Point{J: 0, T: Exp{0, 1, 0}}, 9, 4, 3},
		{Point{J: 0, T: Exp{0, 1, 1}}, 7, 3, 2},
		{Point{J: 1}, 7, 1, 1},
		{Point{J: 1, T: Exp{0, 0, 1}}, 5, 0, 0},
		{Point{J: 2}, 8, 2, 2},
	}
	iota, jump := CandidateByID("cp-iota"), CandidateByID("cp-eps-jump")
	for _, en := range []*Engine{mustEngine(t, 2, 2), mustEngine(t, 2, 2)} {
		en.Reduce = !en.Reduce // once with the literal transforms, once with the reduction of the walks
		for _, r := range rows {
			g, d2, err := en.Step(f, d, r.pt)
			if err != nil {
				t.Fatal(err)
			}
			b := en.Measure(g, d2)
			if b.CP.OrdF != r.ord || b.D != r.eps || b.CP.Omega != r.omega {
				t.Errorf("%s: ord %d, epsilon %d, omega %d, want %d, %d, %d", r.pt.Format(names), b.CP.OrdF, b.D, b.CP.Omega, r.ord, r.eps, r.omega)
			}
			if iota.Fails(st, b, 2) {
				t.Errorf("%s: iota goes up", r.pt.Format(names))
			}
			if jump.Fails(st, b, 2) {
				t.Errorf("%s: the condition of Theorem 3.13(1) fails", r.pt.Format(names))
			}
		}
	}
}

// TestCPCandidates checks the rules of the candidates on states made by hand.
func TestCPCandidates(t *testing.T) {
	a := State{D: 3, CP: CPState{OrdF: 7, Omega: 3, Kappa: ">=2"}}
	up := State{D: 4, CP: CPState{OrdF: 9, Omega: 4, Kappa: ">=2"}}
	same := State{D: 4, CP: CPState{OrdF: 9, Omega: 3, Kappa: ">=2"}}
	zero := State{D: 1, CP: CPState{OrdF: 5, Omega: 0, Kappa: "1"}}
	if !CandidateByID("cp-omega").Fails(a, up, 2) || !CandidateByID("cp-iota").Fails(a, up, 2) {
		t.Error("omega from 3 to 4 is not a failure")
	}
	if CandidateByID("cp-iota").Fails(a, same, 2) || CandidateByID("cp-iota").Fails(a, zero, 2) {
		t.Error("omega that does not go up is a failure")
	}
	if !CandidateByID("cp-iota").Fails(zero, a, 2) {
		t.Error("iota from (2, 0, 1) to (2, 3, >=2) is not a failure")
	}
	// epsilon goes up from 3 to 4, omega(x) = epsilon(x), but delta(x) = 7/2 is not an integer.
	if !CandidateByID("cp-eps-jump").Fails(a, same, 2) {
		t.Error("an increase of epsilon with delta = 7/2 is not a failure")
	}
	b := State{D: 3, CP: CPState{OrdF: 6, Omega: 2, Kappa: ">=2"}}
	if !CandidateByID("cp-eps-jump").Fails(b, same, 2) {
		t.Error("an increase of epsilon with omega(x) = epsilon(x) - 1 is not a failure")
	}
	if c := CandidateByID("cp-iota"); c.Applies(zero) || !c.Applies(a) {
		t.Error("the scope of cp-iota is not omega(x) > 0")
	}
	// The engine computes the invariants of Cossart and Piltant only for q = p.
	en := mustEngine(t, 2, 4)
	if st := en.Measure(mustParse(t, 2, []string{"x", "y"}, "x^3*y^2"), All(2)); st.CP != (CPState{}) {
		t.Errorf("q = 4: the state has the invariants %+v", st.CP)
	}
}

const testE4 = `name: e4-test
experiment: E4
question: A test.
tool: sing-lab
node: SG00000014
inputs:
  walks_from: e1-test.yaml
  select: [{ n: 3, q: 2 }]
  candidates: [cp-omega, cp-iota, cp-eps-jump, d-over-q]
outputs: [histogram, candidates]
`

// TestE4Select checks that an E4 spec makes the walks of the selected case of the E1 spec again, and that a candidate of Cossart and Piltant needs q = p.
func TestE4Select(t *testing.T) {
	root := t.TempDir()
	specs := filepath.Join(root, "lab", "specs")
	if err := os.MkdirAll(specs, 0o750); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(testE4, "select: [{ n: 3, q: 2 }]", "select: [{ n: 3, q: 4 }]", 1)
	for name, src := range map[string]string{"e1-test.yaml": testE1, "e4-test.yaml": testE4, "e4-bad.yaml": strings.Replace(bad, "e4-test", "e4-bad", 1)} {
		if err := os.WriteFile(filepath.Join(specs, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := LoadSpec(filepath.Join(specs, "e4-bad.yaml")); err == nil || !strings.Contains(err.Error(), "needs q = p") {
		t.Errorf("a candidate of Cossart and Piltant on q = 4: the error is %v", err)
	}
	hists := map[string]Histogram{}
	for _, name := range []string{"e1-test.yaml", "e4-test.yaml"} {
		s, err := LoadSpec(filepath.Join(specs, name))
		if err != nil {
			t.Fatal(err)
		}
		opt := Options{Runs: RunsDir(s.Path), Threads: 3, Host: "h", Tool: ToolInfo{Version: "test"}, Now: func() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) }}
		rec, err := Run(context.Background(), s, opt)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(opt.Runs, rec.ID, "histogram.json")) // #nosec G304 -- a test file
		if err != nil {
			t.Fatal(err)
		}
		var h Histogram
		if err := json.Unmarshal(data, &h); err != nil {
			t.Fatal(err)
		}
		hists[name] = h
		if name == "e4-test.yaml" {
			sum, err := os.ReadFile(filepath.Join(opt.Runs, rec.ID, "summary.md")) // #nosec G304 -- a test file
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"## Dimension 3 and dimension 4", "`cp-iota`", "condition (E)", "only the cases of that spec with n = 3 and q = 2"} {
				if !bytes.Contains(sum, []byte(want)) {
					t.Errorf("the summary has no %q", want)
				}
			}
			var co CandOut
			data, _ := os.ReadFile(filepath.Join(opt.Runs, rec.ID, "candidates.json")) // #nosec G304 -- a test file
			if err := json.Unmarshal(data, &co); err != nil {
				t.Fatal(err)
			}
			r := co.Cases[0].Results[1] // cp-iota
			if r.Steps+r.Skip != co.Cases[0].Steps || r.Steps != r.Fail+r.Equal+r.Down {
				t.Errorf("cp-iota: %d in the scope and %d out of it, of %d steps", r.Steps, r.Skip, co.Cases[0].Steps)
			}
		}
	}
	e1, e4 := hists["e1-test.yaml"], hists["e4-test.yaml"]
	if len(e4.Cases) != 1 {
		t.Fatalf("the E4 run has %d cases, want 1", len(e4.Cases))
	}
	a, _ := json.Marshal(e1.Cases[0])
	b, _ := json.Marshal(e4.Cases[0])
	if !bytes.Equal(a, b) {
		t.Error("the E4 run does not make the walks of the case n = 3, q = 2 of the E1 run")
	}
}
