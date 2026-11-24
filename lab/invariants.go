package lab

import "fmt"

// State is the data of z^q + F with the divisor E at the origin that the candidates of experiment E2 use. F is clean and ord F >= q.
type State struct {
	D  int `json:"d"`  // the residual order: the order of G in F = prod_{i in E} x_i^(r_i) G
	N  int `json:"n"`  // the number of terms of in(G), the initial form of G
	RF int `json:"rF"` // Moh's r: the largest r such that in(F) is a p^r-th power
	RG int `json:"rG"` // the largest r such that in(G) is a p^r-th power, and 0 when d = 0
	PF int `json:"pF"` // p^rF
	PG int `json:"pG"` // p^rG
}

// Measure returns the state of f along d.
func (en *Engine) Measure(f Poly, d Divisor) State {
	dd, r := Residual(f, d)
	in := f.Initial()
	st := State{D: dd, N: len(in.Terms)}
	st.RF = pthPower(in, Exp{}, en.P)
	if dd > 0 {
		st.RG = pthPower(in, r, en.P)
	}
	st.PF, st.PG = ipow(en.P, st.RF), ipow(en.P, st.RG)
	return st
}

// pthPower returns the largest r such that each exponent of each term of in, less r0, is divisible by p^r. Over F_p each coefficient is a p^r-th power, so this is the largest r such that in / x^r0 is a p^r-th power. At least one shifted exponent must be non-zero.
func pthPower(in Poly, r0 Exp, p int) int {
	r, pr := 0, p
	for {
		for _, t := range in.Terms {
			for i := range in.N {
				if int(t.E[i]-r0[i])%pr != 0 {
					return r
				}
			}
		}
		r++
		pr *= p
		if r > 40 {
			return r // only when all the shifted exponents are zero, which the callers exclude
		}
	}
}

func ipow(b, e int) int {
	out := 1
	for range e {
		out *= b
	}
	return out
}

// Candidate is a candidate secondary invariant of experiment E2. Fails reports whether the step from a to b breaks the rule of the candidate.
type Candidate struct {
	ID         string
	Definition string
	Bound      bool // a one-step bound, not a value that should go down
	Fails      func(a, b State, q int) bool
	Value      func(s State, q int) string
}

// lexGreater reports whether (a1, a2) > (b1, b2) in the lexicographic order.
func lexGreater(a1, a2, b1, b2 int) bool { return a1 > b1 || a1 == b1 && a2 > b2 }

// Candidates are the candidates of experiment E2, by id. Each one but moh-rounded is a value that should not go up under a point blowup at which the order stays q. moh-rounded is the bound of the false step in the proof of Moh, which Hauser and Perlega found.
var Candidates = []Candidate{
	{
		ID:         "d-over-q",
		Definition: "d/q, the residual order divided by q. Along one walk q does not change, so this candidate fails exactly at the kangaroo points.",
		Fails:      func(a, b State, q int) bool { return b.D > a.D },
		Value:      func(s State, q int) string { return fmt.Sprintf("%d/%d", s.D, q) },
	},
	{
		ID:         "d-n",
		Definition: "the pair (d, N) in the lexicographic order, with N the number of terms of in(G), the initial form of the residual factor G.",
		Fails:      func(a, b State, _ int) bool { return lexGreater(b.D, b.N, a.D, a.N) },
		Value:      func(s State, _ int) string { return fmt.Sprintf("(%d, %d)", s.D, s.N) },
	},
	{
		ID:         "d-minus-pr-moh",
		Definition: "d - p^rF, with rF the r of Moh: the largest r such that in(F) is a p^r-th power.",
		Fails:      func(a, b State, _ int) bool { return b.D-b.PF > a.D-a.PF },
		Value:      func(s State, _ int) string { return fmt.Sprintf("%d - %d = %d", s.D, s.PF, s.D-s.PF) },
	},
	{
		ID:         "d-minus-pr-g",
		Definition: "d - p^rG, with rG the largest r such that in(G) is a p^r-th power (rG = 0 when d = 0).",
		Fails:      func(a, b State, _ int) bool { return b.D-b.PG > a.D-a.PG },
		Value:      func(s State, _ int) string { return fmt.Sprintf("%d - %d = %d", s.D, s.PG, s.D-s.PG) },
	},
	{
		ID:         "d-minus-r",
		Definition: "the pair (d, -rF) in the lexicographic order, with rF the r of Moh.",
		Fails:      func(a, b State, _ int) bool { return lexGreater(b.D, -b.RF, a.D, -a.RF) },
		Value:      func(s State, _ int) string { return fmt.Sprintf("(%d, %d)", s.D, -s.RF) },
	},
	{
		ID:         "moh-rounded",
		Bound:      true,
		Definition: "the one-step bound d' <= floor(d/p^rF) p^rF + p^rF that the proof of Moh uses (Hauser-Perlega, Section 3). This is a bound, not a value that goes down.",
		Fails:      func(a, b State, _ int) bool { return b.D > a.D/a.PF*a.PF+a.PF },
		Value:      func(s State, _ int) string { return fmt.Sprintf("d = %d, p^rF = %d", s.D, s.PF) },
	},
}

// CandidateByID returns the candidate with the id, or nil.
func CandidateByID(id string) *Candidate {
	for i := range Candidates {
		if Candidates[i].ID == id {
			return &Candidates[i]
		}
	}
	return nil
}
