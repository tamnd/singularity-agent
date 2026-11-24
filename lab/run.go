package lab

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ToolInfo is the version of sing that runs a spec.
type ToolInfo struct {
	Version  string `json:"version"`
	Commit   string `json:"commit,omitempty"`
	Modified bool   `json:"modified,omitempty"`
	Go       string `json:"go,omitempty"`
}

// Options are the options of Run.
type Options struct {
	Runs    string // the directory of the runs, usually lab/runs of the content repository
	Threads int    // the number of goroutines; the outputs do not depend on it
	Walks   int    // when positive, the largest number of walks of each case (for a quick test; the record says so)
	Force   bool   // replace a run with the same id
	Host    string
	Tool    ToolInfo
	Now     func() time.Time
}

// Record is record.json of a run.
type Record struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Experiment  string            `json:"experiment"`
	Question    string            `json:"question"`
	Tool        string            `json:"tool"`
	Node        string            `json:"node"`
	Nodes       []string          `json:"nodes,omitempty"`
	Spec        string            `json:"spec"`
	SpecSHA256  string            `json:"spec_sha256"`
	WalksFrom   *SpecRef          `json:"walks_from,omitempty"`
	Sing        ToolInfo          `json:"sing"`
	Seed        uint64            `json:"seed,omitempty"`
	Host        string            `json:"host"`
	Started     string            `json:"started"`
	WallSeconds float64           `json:"wall_seconds"`
	WalksLimit  int               `json:"walks_limit,omitempty"`
	Counts      Counts            `json:"counts"`
	Files       map[string]string `json:"files"`
	Raw         string            `json:"raw,omitempty"`
}

// SpecRef names a spec and its hash.
type SpecRef struct {
	Spec   string `json:"spec"`
	SHA256 string `json:"sha256"`
}

// Counts are the main counts of a run.
type Counts struct {
	Cases         int `json:"cases"`
	Walks         int `json:"walks"`
	Steps         int `json:"steps"`
	Kangaroo      int `json:"kangaroo"`
	MohViolations int `json:"moh_violations"`
	Stability     int `json:"stability_violations"`
	Failures      int `json:"candidate_failures,omitempty"`
}

// Bin is one value of a histogram.
type Bin struct {
	Value int `json:"value"`
	Count int `json:"count"`
}

func bins(m map[int]int) []Bin {
	out := []Bin{}
	for _, k := range sortedKeys(m) {
		out = append(out, Bin{k, m[k]})
	}
	return out
}

// CaseOut is one case of histogram.json.
type CaseOut struct {
	P           int            `json:"p"`
	N           int            `json:"n"`
	Q           int            `json:"q"`
	E           int            `json:"e"`
	Walks       int            `json:"walks"`
	Depth       int            `json:"depth"`
	Degree      []int          `json:"degree"`
	Terms       []int          `json:"terms"`
	Divisor     string         `json:"divisor"`
	Steps       int            `json:"steps"`
	Stops       map[string]int `json:"stops"`
	Start       []Bin          `json:"start_residual_order"`
	Change      []Bin          `json:"change"`
	Kangaroo    int            `json:"kangaroo"`
	MaxIncrease int            `json:"max_increase"`
	MohBound    int            `json:"moh_bound"`
	Moh         int            `json:"moh_violations"`
	Stability   int            `json:"stability_violations"`
	MaxTerms    int            `json:"max_terms"`
	Shifted     int            `json:"z_shift_steps"`
}

// Histogram is histogram.json.
type Histogram struct {
	Spec  string    `json:"spec"`
	Cases []CaseOut `json:"cases"`
}

// ExampleOut is one step of a walk, with all the data to check it again with sing lab step.
type ExampleOut struct {
	Walk      int            `json:"walk"`
	Step      int            `json:"step"`
	F         string         `json:"F"`
	Terms     int            `json:"terms"`
	Divisor   []string       `json:"divisor"`
	Chart     string         `json:"chart"`
	Translate map[string]int `json:"translate"`
	Before    int            `json:"d_before"`
	After     int            `json:"d_after"`
}

// IncreaseGroup is the first examples of one increase of the residual order.
type IncreaseGroup struct {
	Increase     int          `json:"increase"`
	Count        int          `json:"count"`
	OverMohBound bool         `json:"over_moh_bound"`
	Examples     []ExampleOut `json:"examples"`
}

// WalkOut is a walk in short form: the start and the charts. The walk of an example of the stability bound.
type WalkOut struct {
	Walk    int           `json:"walk"`
	F       string        `json:"F"`
	Divisor []string      `json:"divisor"`
	Start   int           `json:"d_start"`
	Bound   int           `json:"bound"`
	Steps   []WalkStepOut `json:"steps"`
}

// WalkStepOut is one blowup of a WalkOut.
type WalkStepOut struct {
	Chart     string         `json:"chart"`
	Translate map[string]int `json:"translate"`
	D         int            `json:"d"`
}

// CaseExamples is one case of examples.json.
type CaseExamples struct {
	P         int             `json:"p"`
	N         int             `json:"n"`
	Q         int             `json:"q"`
	Vars      []string        `json:"vars"`
	Increases []IncreaseGroup `json:"increases"`
	Stability []WalkOut       `json:"stability"`
}

// Examples is examples.json.
type Examples struct {
	Spec  string         `json:"spec"`
	Note  string         `json:"note"`
	Cases []CaseExamples `json:"cases"`
}

// Failure is the first failure of a candidate.
type Failure struct {
	ExampleOut
	StateBefore State  `json:"state_before"`
	StateAfter  State  `json:"state_after"`
	ValueBefore string `json:"value_before"`
	ValueAfter  string `json:"value_after"`
}

// CandResult is the result of one candidate in one case.
type CandResult struct {
	ID    string   `json:"id"`
	Steps int      `json:"steps"`
	Fail  int      `json:"fail"`
	Equal int      `json:"equal,omitempty"`
	Down  int      `json:"down,omitempty"`
	First *Failure `json:"first_failure,omitempty"`
}

// CaseCands is one case of candidates.json.
type CaseCands struct {
	P       int          `json:"p"`
	N       int          `json:"n"`
	Q       int          `json:"q"`
	Steps   int          `json:"steps"`
	Results []CandResult `json:"results"`
}

// CandDef is the definition of a candidate.
type CandDef struct {
	ID         string `json:"id"`
	Definition string `json:"definition"`
	Bound      bool   `json:"bound,omitempty"`
}

// CandTotal is the result of one candidate on all cases.
type CandTotal struct {
	ID        string `json:"id"`
	Steps     int    `json:"steps"`
	Fail      int    `json:"fail"`
	FirstCase string `json:"first_failure_case,omitempty"`
}

// CandOut is candidates.json.
type CandOut struct {
	Spec       string      `json:"spec"`
	WalksFrom  string      `json:"walks_from"`
	Candidates []CandDef   `json:"candidates"`
	Totals     []CandTotal `json:"totals"`
	Cases      []CaseCands `json:"cases"`
}

// FixedOut is steps.json of a fixed spec.
type FixedOut struct {
	Spec   string     `json:"spec"`
	P      int        `json:"p"`
	Q      int        `json:"q"`
	Vars   []string   `json:"vars"`
	Source string     `json:"source,omitempty"`
	Reduce bool       `json:"reduce"`
	Steps  []FixedRow `json:"steps"`
	Match  bool       `json:"match"`
}

// FixedRow is one state of a fixed sequence: the start or the state after a blowup.
type FixedRow struct {
	Index     int            `json:"index"`
	Label     string         `json:"label,omitempty"`
	Chart     string         `json:"chart,omitempty"`
	Translate map[string]int `json:"translate,omitempty"`
	F         string         `json:"F"`
	Terms     int            `json:"terms"`
	Order     int            `json:"order"`
	Divisor   []string       `json:"divisor"`
	State     State          `json:"state"`
	Expect    *int           `json:"expect,omitempty"`
	Match     *bool          `json:"match,omitempty"`
}

// specRel returns the path of a spec from the root of the content repository, when the path has lab/specs/ in it.
func specRel(path string) string {
	p := filepath.ToSlash(path)
	if i := strings.LastIndex(p, "lab/specs/"); i >= 0 {
		return p[i:]
	}
	return filepath.Base(p)
}

// RunsDir returns lab/runs next to lab/specs of a spec path, or "" when the spec is not in lab/specs.
func RunsDir(spec string) string {
	abs, err := filepath.Abs(spec)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(abs)
	if filepath.Base(dir) == "specs" && filepath.Base(filepath.Dir(dir)) == "lab" {
		return filepath.Join(filepath.Dir(dir), "runs")
	}
	return ""
}

// Run runs a spec and writes the run directory. It returns the record.
func Run(ctx context.Context, s *Spec, opt Options) (*Record, error) {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Runs == "" {
		return nil, errors.New("lab: the directory of the runs is not set")
	}
	start := opt.Now().UTC()
	rec := &Record{
		Name: s.Name, Experiment: s.Experiment, Question: s.Question, Tool: s.Tool, Node: s.Node, Nodes: s.Nodes,
		Spec: specRel(s.Path), SpecSHA256: s.Hash, Sing: opt.Tool, Host: opt.Host,
		Started: start.Format(time.RFC3339), WalksLimit: opt.Walks, Files: map[string]string{},
	}
	rec.ID = start.Format("2006-01-02") + "-" + s.Name + "-" + s.Hash[:8]
	dir := filepath.Join(opt.Runs, rec.ID)
	if _, err := os.Stat(dir); err == nil && !opt.Force {
		return nil, fmt.Errorf("lab: the run %s exists; use -force to replace it", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	var summary string
	var err error
	switch {
	case s.Inputs.Fixed != nil:
		summary, err = runFixed(s, rec, files)
	default:
		summary, err = runWalks(ctx, s, opt, rec, files, dir)
	}
	if err != nil {
		return nil, err
	}
	files["summary.md"] = []byte(summary)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), files[n], 0o644); err != nil { // #nosec G306 -- public data
			return nil, err
		}
		sum := sha256.Sum256(files[n])
		rec.Files[n] = hex.EncodeToString(sum[:])
	}
	rec.WallSeconds = float64(opt.Now().Sub(start).Milliseconds()) / 1000
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "record.json"), append(data, '\n'), 0o644); err != nil { // #nosec G306 -- public data
		return nil, err
	}
	return rec, nil
}

func marshal(v any) []byte {
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		panic(err) // the types of this file always marshal
	}
	return append(data, '\n')
}

func runWalks(ctx context.Context, s *Spec, opt Options, rec *Record, files map[string][]byte, dir string) (string, error) {
	walkSpec := s
	var cands []*Candidate
	if s.From != nil {
		walkSpec = s.From
		rec.WalksFrom = &SpecRef{Spec: specRel(s.From.Path), SHA256: s.From.Hash}
		for _, id := range s.Inputs.Candidates {
			cands = append(cands, CandidateByID(id))
		}
	}
	in := walkSpec.Inputs
	rec.Seed = in.Seed
	examples, stabEx := in.Examples, in.Stability
	if s.From != nil {
		examples, stabEx = 0, 0
	}
	var ws []*walker
	for i := range in.Cases {
		c := in.Case(i)
		if opt.Walks > 0 {
			c.Walks = min(c.Walks, opt.Walks)
		}
		ws = append(ws, &walker{c: c, index: i, seed: in.Seed, opts: in.Walk, cands: cands, examples: examples, stabEx: stabEx, raw: in.Raw && s.From == nil})
	}
	var sink func(int, *Tally)
	var raws []*rawFile
	if in.Raw && s.From == nil {
		if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o750); err != nil {
			return "", err
		}
		for i := range ws {
			rf, err := newRawFile(filepath.Join(dir, "raw", fmt.Sprintf("kangaroo-%02d.jsonl.gz", i+1)))
			if err != nil {
				return "", err
			}
			raws = append(raws, rf)
		}
		sink = func(ci int, t *Tally) { raws[ci].write(t.raw, Names(ws[ci].c.N)) }
		rec.Raw = "raw/: every kangaroo point of each case, one JSON line each, gzip. The directory stays on the machine of the run and is not in git."
	}
	res, err := runCases(ctx, ws, opt.Threads, sink)
	for _, rf := range raws {
		if cerr := rf.close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		return "", err
	}
	hist := Histogram{Spec: walkSpec.Name}
	exs := Examples{Spec: walkSpec.Name, Note: "Each example is one point blowup: z^q + F with F clean, the divisor, the chart and the translations. Check it with: sing lab step -p P -q Q -vars VARS -f F -divisor DIVISOR -chart CHART -t VAR=VALUE. The polynomials are up to a unit factor that is a q-th power, which does not change the invariants (see the README of lab/)."}
	co := CandOut{Spec: s.Name, WalksFrom: walkSpec.Name}
	for _, c := range cands {
		co.Candidates = append(co.Candidates, CandDef{ID: c.ID, Definition: c.Definition, Bound: c.Bound})
		co.Totals = append(co.Totals, CandTotal{ID: c.ID})
	}
	rec.Counts.Cases = len(ws)
	for ci, w := range ws {
		t := res[ci]
		en, _ := NewEngine(w.c.P, w.c.Q)
		names := Names(w.c.N)
		stops := map[string]int{}
		for _, k := range Stops {
			stops[k] = t.Stops[k]
		}
		hist.Cases = append(hist.Cases, CaseOut{
			P: w.c.P, N: w.c.N, Q: w.c.Q, E: en.E, Walks: t.Walks, Depth: w.c.Depth, Degree: w.c.Degree, Terms: w.c.Terms, Divisor: w.c.Divisor,
			Steps: t.Steps, Stops: stops, Start: bins(t.Start), Change: bins(t.Change), Kangaroo: t.Kangaroo, MaxIncrease: t.MaxIncrease,
			MohBound: en.MohBound(), Moh: t.Moh, Stability: t.Stability, MaxTerms: t.MaxTerms, Shifted: t.Shifted,
		})
		rec.Counts.Walks += t.Walks
		rec.Counts.Steps += t.Steps
		rec.Counts.Kangaroo += t.Kangaroo
		rec.Counts.MohViolations += t.Moh
		rec.Counts.Stability += t.Stability
		ce := CaseExamples{P: w.c.P, N: w.c.N, Q: w.c.Q, Vars: names, Increases: []IncreaseGroup{}, Stability: []WalkOut{}}
		for _, inc := range sortedKeys(t.Change) {
			if inc <= 0 {
				continue
			}
			g := IncreaseGroup{Increase: inc, Count: t.Change[inc], OverMohBound: inc > en.MohBound(), Examples: []ExampleOut{}}
			for _, sr := range t.examples[inc] {
				g.Examples = append(g.Examples, exampleOut(sr, names))
			}
			ce.Increases = append(ce.Increases, g)
		}
		for _, wr := range t.stability {
			wo := WalkOut{Walk: wr.walk, F: wr.f.Format(names), Divisor: wr.d.Format(names), Start: wr.start, Bound: wr.start + en.MohBound()}
			for k, pt := range wr.points {
				wo.Steps = append(wo.Steps, WalkStepOut{Chart: names[pt.J], Translate: pt.Translations(names), D: wr.orders[k]})
			}
			ce.Stability = append(ce.Stability, wo)
		}
		exs.Cases = append(exs.Cases, ce)
		if cands != nil {
			cc := CaseCands{P: w.c.P, N: w.c.N, Q: w.c.Q, Steps: t.Steps}
			for i, c := range cands {
				ct := t.cand[i]
				r := CandResult{ID: c.ID, Steps: t.Steps, Fail: ct.Fail}
				if !c.Bound {
					r.Equal, r.Down = ct.Equal, ct.Down
				}
				if ct.first != nil {
					f := ct.first
					r.First = &Failure{ExampleOut: exampleOut(*f, names), StateBefore: f.a, StateAfter: f.b, ValueBefore: c.Value(f.a, w.c.Q), ValueAfter: c.Value(f.b, w.c.Q)}
					if co.Totals[i].FirstCase == "" {
						co.Totals[i].FirstCase = caseName(w.c)
					}
				}
				co.Totals[i].Steps += t.Steps
				co.Totals[i].Fail += ct.Fail
				rec.Counts.Failures += ct.Fail
				cc.Results = append(cc.Results, r)
			}
			co.Cases = append(co.Cases, cc)
		}
	}
	files["histogram.json"] = marshal(hist)
	if cands != nil {
		files["candidates.json"] = marshal(co)
		return summaryE2(s, rec, hist, co), nil
	}
	files["examples.json"] = marshal(exs)
	return summaryE1(s, rec, hist), nil
}

func caseName(c Case) string { return fmt.Sprintf("p = %d, n = %d, q = %d", c.P, c.N, c.Q) }

func exampleOut(s stepRec, names []string) ExampleOut {
	return ExampleOut{
		Walk: s.walk, Step: s.step, F: s.f.Format(names), Terms: len(s.f.Terms), Divisor: s.d.Format(names),
		Chart: names[s.pt.J], Translate: s.pt.Translations(names), Before: s.a.D, After: s.b.D,
	}
}

// rawFile is a gzip file of JSON lines.
type rawFile struct {
	f   *os.File
	gz  *gzip.Writer
	err error
}

func newRawFile(path string) (*rawFile, error) {
	f, err := os.Create(path) // #nosec G304 -- a file in the run directory
	if err != nil {
		return nil, err
	}
	return &rawFile{f: f, gz: gzip.NewWriter(f)}, nil
}

func (r *rawFile) write(steps []stepRec, names []string) {
	for _, s := range steps {
		if r.err != nil {
			return
		}
		data, _ := json.Marshal(exampleOut(s, names))
		_, r.err = r.gz.Write(append(data, '\n'))
	}
}

func (r *rawFile) close() error {
	err := r.gz.Close()
	if cerr := r.f.Close(); err == nil {
		err = cerr
	}
	if r.err != nil {
		return r.err
	}
	return err
}

// Replay does the fixed sequence of point blowups of fx. It returns the start and the state after each blowup. It checks the input first.
func Replay(fx *Fixed) ([]FixedRow, error) {
	if err := fx.check(); err != nil {
		return nil, err
	}
	en, err := NewEngine(fx.P, fx.Q)
	if err != nil {
		return nil, err
	}
	en.Reduce = fx.Reduce
	f, err := fx.Poly()
	if err != nil {
		return nil, err
	}
	if f.Ord() < fx.Q {
		return nil, fmt.Errorf("lab: the order of F is %d, less than q = %d", f.Ord(), fx.Q)
	}
	f = f.Clean(fx.Q)
	d, err := fx.divisor()
	if err != nil {
		return nil, err
	}
	row := func(i int, label string, pt *Point, expect *int) FixedRow {
		st := en.Measure(f, d)
		r := FixedRow{Index: i, Label: label, F: f.Format(fx.Vars), Terms: len(f.Terms), Order: f.Ord(), Divisor: d.Format(fx.Vars), State: st, Expect: expect}
		if pt != nil {
			r.Chart, r.Translate = fx.Vars[pt.J], pt.Translations(fx.Vars)
		}
		if expect != nil {
			m := *expect == st.D
			r.Match = &m
		}
		return r
	}
	rows := []FixedRow{row(0, "start", nil, fx.Expect)}
	k := 0
	for _, st := range fx.Steps {
		pt, err := fx.point(st)
		if err != nil {
			return nil, err
		}
		reps := max(1, st.Repeat)
		for r := range reps {
			k++
			if !en.OrderAtLeast(f, pt) {
				return nil, fmt.Errorf("lab: step %d (%s): the order of f drops below q in the chart %s", k, st.Label, pt.Format(fx.Vars))
			}
			if f, d, err = en.Step(f, d, pt); err != nil {
				return nil, err
			}
			var expect *int
			if r == reps-1 {
				expect = st.Expect
			}
			rows = append(rows, row(k, st.Label, &pt, expect))
		}
	}
	return rows, nil
}

func runFixed(s *Spec, rec *Record, files map[string][]byte) (string, error) {
	fx := s.Inputs.Fixed
	en, err := NewEngine(fx.P, fx.Q)
	if err != nil {
		return "", err
	}
	rows, err := Replay(fx)
	if err != nil {
		return "", err
	}
	out := FixedOut{Spec: s.Name, P: fx.P, Q: fx.Q, Vars: fx.Vars, Source: fx.Source, Reduce: fx.Reduce, Steps: rows, Match: true}
	for _, r := range rows {
		if r.Match != nil && !*r.Match {
			out.Match = false
		}
	}
	rec.Counts.Cases = 1
	rec.Counts.Walks = 1
	rec.Counts.Steps = len(rows) - 1
	for i := 1; i < len(rows); i++ {
		ch := rows[i].State.D - rows[i-1].State.D
		if ch > 0 {
			rec.Counts.Kangaroo++
			if ch > en.MohBound() {
				rec.Counts.MohViolations++
			}
		}
	}
	files["steps.json"] = marshal(out)
	return summaryFixed(s, rec, out, en), nil
}

// RunData is a run as the site reads it.
type RunData struct {
	Dir        string
	Record     *Record
	Histogram  *Histogram
	Candidates *CandOut
	Fixed      *FixedOut
	Summary    string
}

// LoadRuns reads each run in dir (lab/runs of the content repository), sorted by id. A missing dir has no runs.
func LoadRuns(dir string) ([]*RunData, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*RunData
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rd := &RunData{Dir: filepath.Join(dir, e.Name())}
		read := func(name string, v any) error {
			data, err := os.ReadFile(filepath.Join(rd.Dir, name)) // #nosec G304 -- a file of the content repository
			if errors.Is(err, fs.ErrNotExist) && name != "record.json" {
				return nil
			}
			if err != nil {
				return err
			}
			if err := json.Unmarshal(data, v); err != nil {
				return fmt.Errorf("%s/%s: %w", e.Name(), name, err)
			}
			return nil
		}
		rd.Record = &Record{}
		if err := read("record.json", rd.Record); err != nil {
			return nil, err
		}
		if rd.Record.ID != e.Name() {
			return nil, fmt.Errorf("lab: the run %s has the id %q in record.json", e.Name(), rd.Record.ID)
		}
		if _, ok := rd.Record.Files["histogram.json"]; ok {
			rd.Histogram = &Histogram{}
			if err := read("histogram.json", rd.Histogram); err != nil {
				return nil, err
			}
		}
		if _, ok := rd.Record.Files["candidates.json"]; ok {
			rd.Candidates = &CandOut{}
			if err := read("candidates.json", rd.Candidates); err != nil {
				return nil, err
			}
		}
		if _, ok := rd.Record.Files["steps.json"]; ok {
			rd.Fixed = &FixedOut{}
			if err := read("steps.json", rd.Fixed); err != nil {
				return nil, err
			}
		}
		if data, err := os.ReadFile(filepath.Join(rd.Dir, "summary.md")); err == nil { // #nosec G304 -- a file of the content repository
			rd.Summary = string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
		}
		out = append(out, rd)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Record.ID < out[j].Record.ID })
	return out, nil
}
