package lab

import (
	"context"
	"errors"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync"
)

// The reasons why a walk stops.
const (
	StopDepth        = "depth"         // the walk did all its blowups
	StopOrderDrops   = "order-drops"   // the order of f drops at each F_p-rational point of the exceptional divisor
	StopResidualZero = "residual-zero" // the residual order is 0: F is a monomial times a unit along E
	StopCap          = "cap"           // a transform is larger than a cap of the spec
)

// Stops lists the reasons in the order of the outputs.
var Stops = []string{StopDepth, StopOrderDrops, StopResidualZero, StopCap}

// stepRec is one blowup of a walk.
type stepRec struct {
	walk, step int
	f          Poly
	d          Divisor
	pt         Point
	a, b       State
}

// walkRec is a walk in short form, for the examples of the stability bound.
type walkRec struct {
	walk   int
	f      Poly
	d      Divisor
	start  int
	points []Point
	orders []int
}

// Tally is the result of the walks of one case, or of one batch of walks.
type Tally struct {
	Walks, Steps int
	Stops        map[string]int
	Start        map[int]int // the residual order at the start of each walk
	Change       map[int]int // the change d' - d of each step
	Kangaroo     int         // the steps with d' > d
	MaxIncrease  int
	Moh          int // the steps with d' - d > p^(e-1)
	Stability    int // the walks with some d_k > d_0 + p^(e-1)
	MaxTerms     int
	Shifted      int // the steps where the transform had a constant term, which the cleaning removes with a change of z

	examples  map[int][]stepRec // the first examples of each increase
	stability []walkRec
	cand      []CandTally
	raw       []stepRec
}

// CandTally counts the steps of one candidate of E2.
type CandTally struct {
	Fail, Equal, Down int
	first             *stepRec
}

func newTally(ncand int) *Tally {
	return &Tally{Stops: map[string]int{}, Start: map[int]int{}, Change: map[int]int{}, examples: map[int][]stepRec{}, cand: make([]CandTally, ncand)}
}

// walker runs the walks of one case.
type walker struct {
	c        Case
	index    int // the index of the case in the spec
	seed     uint64
	opts     WalkOpts
	cands    []*Candidate
	examples int
	stabEx   int
	raw      bool
}

// rng returns the random source of a walk. It depends only on the seed of the spec, the index of the case and the index of the walk. So a walk is the same for each number of threads and for each spec that uses the same inputs.
func (w *walker) rng(walk int) *rand.Rand {
	return rand.New(rand.NewPCG(w.seed, uint64(w.index)<<40|uint64(walk))) // #nosec G404 G115 -- a reproducible experiment, not a secret
}

// family draws the start of a walk.
func (w *walker) family(r *rand.Rand, en *Engine) (Poly, Divisor) {
	n := w.c.N
	a := newAcc(w.c.P)
	for {
		k := w.c.Terms[0] + r.IntN(w.c.Terms[1]-w.c.Terms[0]+1)
		for range k {
			deg := w.c.Degree[0] + r.IntN(w.c.Degree[1]-w.c.Degree[0]+1)
			a.add(composition(r, deg, n), uint32(1+r.IntN(w.c.P-1))) // #nosec G115 -- in 1..p-1
		}
		f := a.poly(n).Clean(en.Q)
		if f.IsZero() || f.Ord() < en.Q {
			continue
		}
		var d Divisor
		switch w.c.Divisor {
		case "all":
			d = All(n)
		case "random":
			for i := range n {
				if r.IntN(2) == 1 {
					d |= 1 << i
				}
			}
		}
		return f, d
	}
}

// composition returns an exponent vector of total degree deg in n variables, uniform among all such vectors. It chooses n-1 bars among deg+n-1 places (Knuth's selection sampling).
func composition(r *rand.Rand, deg, n int) Exp {
	var e Exp
	need, total := n-1, deg+n-1
	i, cur := 0, uint32(0)
	for place := 0; place < total; place++ {
		if need > 0 && r.IntN(total-place) < need {
			e[i] = cur
			i++
			cur = 0
			need--
		} else {
			cur++
		}
	}
	e[i] = cur
	return e
}

// walk runs walk number k and adds it to the tally.
func (w *walker) walk(en *Engine, k int, t *Tally) {
	r := w.rng(k)
	f, d := w.family(r, en)
	st := en.Measure(f, d)
	t.Walks++
	t.Start[st.D]++
	t.MaxTerms = max(t.MaxTerms, len(f.Terms))
	rec := walkRec{walk: k, f: f, d: d, start: st.D}
	bound := st.D + en.MohBound()
	unstable := false
	stop := StopDepth
	for step := 0; step < w.c.Depth; step++ {
		if st.D == 0 {
			stop = StopResidualZero
			break
		}
		pts := en.Points(f)
		if len(pts) == 0 {
			stop = StopOrderDrops
			break
		}
		pt := pts[r.IntN(len(pts))]
		g, d2, err := en.Step(f, d, pt)
		if errors.Is(err, ErrCap) {
			stop = StopCap
			break
		}
		if err != nil {
			panic("lab: " + err.Error()) // OrderAtLeast and Blowup disagree: a bug
		}
		if en.Shifted {
			t.Shifted++
		}
		st2 := en.Measure(g, d2)
		w.record(en, t, stepRec{walk: k, step: step, f: f, d: d, pt: pt, a: st, b: st2})
		t.MaxTerms = max(t.MaxTerms, len(g.Terms))
		rec.points = append(rec.points, pt)
		rec.orders = append(rec.orders, st2.D)
		if st2.D > bound {
			unstable = true
		}
		f, d, st = g, d2, st2
		if (w.opts.MaxTerms > 0 && len(g.Terms) > w.opts.MaxTerms) || (w.opts.MaxDegree > 0 && g.MaxDeg() > w.opts.MaxDegree) {
			stop = StopCap
			break
		}
	}
	t.Stops[stop]++
	if unstable {
		t.Stability++
		if len(t.stability) < w.stabEx {
			t.stability = append(t.stability, rec)
		}
	}
}

func (w *walker) record(en *Engine, t *Tally, s stepRec) {
	t.Steps++
	ch := s.b.D - s.a.D
	t.Change[ch]++
	if ch > 0 {
		t.Kangaroo++
		t.MaxIncrease = max(t.MaxIncrease, ch)
		if ch > en.MohBound() {
			t.Moh++
		}
		if len(t.examples[ch]) < w.examples {
			t.examples[ch] = append(t.examples[ch], s)
		}
		if w.raw {
			t.raw = append(t.raw, s)
		}
	}
	for i, c := range w.cands {
		ct := &t.cand[i]
		switch {
		case c.Fails(s.a, s.b, en.Q):
			ct.Fail++
			if ct.first == nil {
				cp := s
				ct.first = &cp
			}
		case c.Fails(s.b, s.a, en.Q):
			ct.Down++
		default:
			ct.Equal++
		}
	}
}

// merge adds the batch u, which comes after the walks of t, to t.
func (t *Tally) merge(u *Tally, examples, stabEx int) {
	t.Walks += u.Walks
	t.Steps += u.Steps
	for k, v := range u.Stops {
		t.Stops[k] += v
	}
	for k, v := range u.Start {
		t.Start[k] += v
	}
	for k, v := range u.Change {
		t.Change[k] += v
	}
	t.Kangaroo += u.Kangaroo
	t.MaxIncrease = max(t.MaxIncrease, u.MaxIncrease)
	t.Moh += u.Moh
	t.Stability += u.Stability
	t.MaxTerms = max(t.MaxTerms, u.MaxTerms)
	t.Shifted += u.Shifted
	for k, v := range u.examples {
		for _, s := range v {
			if len(t.examples[k]) < examples {
				t.examples[k] = append(t.examples[k], s)
			}
		}
	}
	for _, s := range u.stability {
		if len(t.stability) < stabEx {
			t.stability = append(t.stability, s)
		}
	}
	for i := range u.cand {
		a, b := &t.cand[i], u.cand[i]
		a.Fail += b.Fail
		a.Equal += b.Equal
		a.Down += b.Down
		if a.first == nil {
			a.first = b.first
		}
	}
	t.raw = append(t.raw, u.raw...)
}

// batchSize is the number of walks of one task. The tasks are added up in their order, so the result does not depend on the number of threads.
const batchSize = 256

// runCases runs the walks of all cases with threads goroutines. sink, when it is not nil, gets each batch in order, and the batch is then dropped from memory: this keeps the raw output small in memory.
func runCases(ctx context.Context, ws []*walker, threads int, sink func(ci int, t *Tally)) ([]*Tally, error) {
	if threads <= 0 {
		threads = min(4, runtime.NumCPU())
	}
	type task struct{ ci, bi, lo, hi int }
	var tasks []task
	nb := make([]int, len(ws))
	for ci, w := range ws {
		for lo := 0; lo < w.c.Walks; lo += batchSize {
			tasks = append(tasks, task{ci, nb[ci], lo, min(lo+batchSize, w.c.Walks)})
			nb[ci]++
		}
	}
	type done struct {
		task
		t *Tally
	}
	in := make(chan task)
	out := make(chan done, threads)
	var wg sync.WaitGroup
	for range threads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			engines := map[int]*Engine{}
			for tk := range in {
				w := ws[tk.ci]
				en := engines[tk.ci]
				if en == nil {
					var err error
					if en, err = NewEngine(w.c.P, w.c.Q); err != nil {
						panic(err) // the spec check made sure that this does not happen
					}
					en.MaxExpand = w.opts.MaxExpand
					engines[tk.ci] = en
				}
				t := newTally(len(w.cands))
				for k := tk.lo; k < tk.hi; k++ {
					w.walk(en, k, t)
				}
				if len(en.exps) > 1<<16 {
					clear(en.exps) // keep the cache of expansions small
				}
				out <- done{tk, t}
			}
		}()
	}
	go func() {
		defer close(in)
		for _, tk := range tasks {
			select {
			case in <- tk:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(out) }()

	res := make([]*Tally, len(ws))
	for ci, w := range ws {
		res[ci] = newTally(len(w.cands))
	}
	next := make([]int, len(ws))
	pending := map[[2]int]*Tally{}
	for d := range out {
		pending[[2]int{d.ci, d.bi}] = d.t
		for ci := range ws {
			for {
				t, ok := pending[[2]int{ci, next[ci]}]
				if !ok {
					break
				}
				delete(pending, [2]int{ci, next[ci]})
				if sink != nil {
					sink(ci, t)
				}
				t.raw = nil
				res[ci].merge(t, ws[ci].examples, ws[ci].stabEx)
				next[ci]++
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// sortedKeys returns the keys of m in increasing order.
func sortedKeys(m map[int]int) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
