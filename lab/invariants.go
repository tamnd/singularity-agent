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

	// CP has the invariants of Cossart and Piltant. The engine computes them only when q = p, the case of their proof. It is the zero value when q != p.
	CP CPState `json:"cp,omitzero"`
}

// CPState is the part of the invariants of Cossart and Piltant (arXiv:1412.0868, the paper [CP19] of the map) that the engine computes for h = Z^p + F at the origin, with F clean and ord F >= p. The sources are [CP19, Definitions 2.11, 2.25 and 2.68, (1.11), (2.44) and (2.76)] and Volume 6, Section 3.2, of the handbook (Definitions 3.2.1, 3.2.3 and 3.2.5 and Proposition 3.2.4).
//
// The coordinates are well adapted in the sense of [CP19, Definition 2.24]: for a clean F the polyhedron Delta(h; x; Z) is minimal (handbook, Proposition 3.2.4(1)). So the engine reads off the invariants from F. The residue field is F_p, which is perfect, so the derivatives of a p-basis of the residue field do not occur, and the equation is purely inseparable (i0 = p in the words of [CP19]).
//
//   - m(x) = p, the multiplicity. It is p at each point of a walk, because the walks keep ord F >= p.
//   - delta(x) = ord F / p [CP19, Definition 2.11]; OrdF is p delta(x).
//   - epsilon(x) = ord F - sum_{j in E} r_j = ord G [CP19, Definition 2.25]. This is the residual order D.
//   - omega(x) = epsilon(x) - 1 if dF/dU_j != 0 for some j not in E, and omega(x) = epsilon(x) otherwise, where F is the initial form of the clean F [CP19, (1.11) and Definition 2.68, with V(F, E, m_S) of (2.44) spanned by the dF/dU_j for j not in E]. In characteristic p, dF/dU_j != 0 if and only if some term of in(F) has an exponent of x_j that is prime to p.
//   - kappa(x) = 1 if omega(x) = 0, and kappa(x) >= 2 otherwise [CP19, Definition 2.68, with i0 = p]. The engine does not compute the values 2, 3 and 4 of kappa for n = 3 [CP19, Definition 5.1], because they need the projection of the polyhedron with a well chosen coordinate v = u_3 - phi_2, which the engine cannot choose.
//
// The invariant of [CP19, (2.76)] is iota(x) = (m(x), omega(x), kappa(x)), in the lexicographic order. On a walk m = p at each point and kappa = 1 exactly when omega = 0, so iota goes up at a step exactly when omega goes up. By [CP19, Remark 2.69] omega and kappa do not depend on the choice of the well adapted coordinates.
type CPState struct {
	OrdF  int    `json:"ordF"`  // ord F = p delta(x)
	Omega int    `json:"omega"` // omega(x)
	Kappa string `json:"kappa"` // "1" or ">=2"
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
	if en.Q == en.P {
		st.CP = cpState(in, d, dd, en.P)
	}
	return st
}

// cpState returns the invariants of Cossart and Piltant at the origin (see CPState), from the initial form in of the clean F, the divisor d and the residual order eps.
func cpState(in Poly, d Divisor, eps, p int) CPState {
	cp := CPState{OrdF: in.Ord(), Omega: eps, Kappa: "1"}
	if omegaDrop(in, d, p) {
		cp.Omega = eps - 1
	}
	if cp.Omega > 0 {
		cp.Kappa = ">=2"
	}
	return cp
}

// omegaDrop reports whether dF/dU_j != 0 for some variable x_j that is not in the divisor d, with F = in. The derivative of c x^a is c a_j x^(a - e_j), and different terms have different derivatives, so this holds if and only if some term of in has an exponent of such an x_j that is prime to p.
func omegaDrop(in Poly, d Divisor, p int) bool {
	for _, t := range in.Terms {
		for j := range in.N {
			if !d.Has(j) && int(t.E[j])%p != 0 {
				return true
			}
		}
	}
	return false
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

// Candidate is a candidate invariant of experiments E2 and E4. Fails reports whether the step from a to b breaks the rule of the candidate.
type Candidate struct {
	ID         string
	Definition string
	Bound      bool // a one-step bound, not a value that should go down
	Fails      func(a, b State, q int) bool
	Value      func(s State, q int) string
	// QIsP marks a candidate of Cossart and Piltant. It needs q = p, so a spec can use it only on cases with q = p, and it is not in the default list of E2.
	QIsP bool
	// Applies, when it is not nil, reports whether a step from a is in the scope of the candidate, for example the hypothesis omega(x) > 0 of [CP19, Theorem 3.13]. The run counts the other steps as out of scope.
	Applies func(a State) bool
	// Scope says in words which steps Applies keeps.
	Scope string
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
	{
		ID:         "cp-omega",
		QIsP:       true,
		Definition: "omega of Cossart-Piltant [CP19, (1.11) and Definition 2.68]: omega = epsilon - 1 if dF/dU_j != 0 for some x_j not in E, with F the initial form of the clean F, and omega = epsilon otherwise. Here epsilon = d is the residual order. The candidate fails when omega goes up, at any step.",
		Fails:      func(a, b State, _ int) bool { return b.CP.Omega > a.CP.Omega },
		Value:      cpValue,
	},
	{
		ID:         "cp-iota",
		QIsP:       true,
		Definition: "iota = (m, omega, kappa) of Cossart-Piltant [CP19, (2.76) and Definition 2.68] in the lexicographic order, with kappa = 1 if omega = 0 and kappa >= 2 otherwise. On a walk m = p, so iota goes up exactly when omega goes up. [CP19, Theorem 3.13] says that iota does not go up under a permissible blowup when m(x) = p and omega(x) > 0, and a point is a permissible centre.",
		Scope:      "the steps from a point with omega(x) > 0, the hypothesis of [CP19, Theorem 3.13]",
		Fails: func(a, b State, _ int) bool {
			return lexGreater(b.CP.Omega, kappaRank(b), a.CP.Omega, kappaRank(a))
		},
		Applies: func(a State) bool { return a.CP.Omega > 0 },
		Value: func(s State, q int) string {
			return fmt.Sprintf("(%d, %d, %s)", q, s.CP.Omega, s.CP.Kappa)
		},
	},
	{
		ID:         "cp-eps-jump",
		QIsP:       true,
		Bound:      true,
		Definition: "the condition of [CP19, Theorem 3.13(1)] for a point centre: if epsilon goes up at a step from a point with omega(x) > 0, then omega(x) = epsilon(x) and delta(x) = ord F / p is an integer. The candidate fails at a step where epsilon goes up and one of the two conditions does not hold.",
		Scope:      "the steps from a point with omega(x) > 0, the hypothesis of [CP19, Theorem 3.13]",
		Fails: func(a, b State, q int) bool {
			return b.D > a.D && (a.CP.Omega != a.D || a.CP.OrdF%q != 0)
		},
		Applies: func(a State) bool { return a.CP.Omega > 0 },
		Value: func(s State, q int) string {
			return fmt.Sprintf("epsilon = %d, omega = %d, delta = %d/%d", s.D, s.CP.Omega, s.CP.OrdF, q)
		},
	},
}

// kappaRank orders the two values of kappa: 1 < (>= 2).
func kappaRank(s State) int {
	if s.CP.Kappa == "1" {
		return 1
	}
	return 2
}

func cpValue(s State, _ int) string {
	return fmt.Sprintf("epsilon = %d, omega = %d", s.D, s.CP.Omega)
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
