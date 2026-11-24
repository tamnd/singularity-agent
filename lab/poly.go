// Package lab is sing-lab, the inner loop of the computation lab of singularity (spec 13 of tamnd/singularity).
//
// The package works in the setting of Hauser and Perlega, arXiv:1802.05010, Section 2, over the prime field F_p. A purely inseparable equation is f = z^q + F(x_1, ..., x_n) with q = p^e and ord F >= q. The package has sparse polynomials over F_p, the cleaning of F, the residual order of f along a normal crossings divisor, the chart maps of a point blowup, random sparse families and random blowup walks.
//
// The results of the lab are data, not proofs.
package lab

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MaxVars is the largest number of variables x_1, ..., x_n.
const MaxVars = 8

// Exp is the exponent vector of a monomial. Only the first n entries are used.
type Exp [MaxVars]uint32

// Deg returns the total degree of the monomial.
func (e Exp) Deg() int {
	d := 0
	for _, a := range e {
		d += int(a)
	}
	return d
}

// Term is a monomial with a coefficient in F_p. The coefficient is in 1..p-1.
type Term struct {
	E Exp
	C uint32
}

// Poly is a polynomial over F_p in N variables. The terms are sorted by less and have no zero coefficient.
type Poly struct {
	P     int
	N     int
	Terms []Term
}

// less orders the terms: by total degree, and in one degree by the exponent vector in decreasing lexicographic order, so x^2 comes before x*y and x*y before y^2.
func less(a, b Exp) bool {
	da, db := a.Deg(), b.Deg()
	if da != db {
		return da < db
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// acc collects terms. The coefficients are reduced mod p.
type acc struct {
	p int
	m map[Exp]uint32
}

func newAcc(p int) *acc { return &acc{p: p, m: make(map[Exp]uint32)} }

func (a *acc) add(e Exp, c uint32) {
	if c == 0 {
		return
	}
	v := (a.m[e] + c) % uint32(a.p) // #nosec G115 -- p is small and positive
	if v == 0 {
		delete(a.m, e)
		return
	}
	a.m[e] = v
}

// poly returns the collected terms as a polynomial and empties the collector.
func (a *acc) poly(n int) Poly {
	terms := make([]Term, 0, len(a.m))
	for e, c := range a.m {
		terms = append(terms, Term{E: e, C: c})
	}
	clear(a.m)
	sort.Slice(terms, func(i, j int) bool { return less(terms[i].E, terms[j].E) })
	return Poly{P: a.p, N: n, Terms: terms}
}

// IsZero reports whether f is the zero polynomial.
func (f Poly) IsZero() bool { return len(f.Terms) == 0 }

// Ord returns the order of f at the origin: the smallest total degree of a term. The order of the zero polynomial is -1.
func (f Poly) Ord() int {
	if f.IsZero() {
		return -1
	}
	return f.Terms[0].E.Deg() // the terms are sorted by degree
}

// MaxDeg returns the largest total degree of a term.
func (f Poly) MaxDeg() int {
	if f.IsZero() {
		return -1
	}
	return f.Terms[len(f.Terms)-1].E.Deg()
}

// MinExp returns the smallest exponent of x_i in the terms of f. This is the order of f along x_i = 0.
func (f Poly) MinExp(i int) uint32 {
	if f.IsZero() {
		return 0
	}
	m := f.Terms[0].E[i]
	for _, t := range f.Terms[1:] {
		m = min(m, t.E[i])
	}
	return m
}

// Clean removes from f each term whose exponent vector is in qN^n. Over F_p each coefficient is a q-th power, because the Frobenius map is bijective on F_p. So such a term is a q-th power, and the change of coordinates z -> z - g removes it from z^q + F.
func (f Poly) Clean(q int) Poly {
	out := Poly{P: f.P, N: f.N, Terms: make([]Term, 0, len(f.Terms))}
	for _, t := range f.Terms {
		if !qPower(t.E, f.N, q) {
			out.Terms = append(out.Terms, t)
		}
	}
	return out
}

// IsClean reports whether f has no term whose exponent vector is in qN^n.
func (f Poly) IsClean(q int) bool {
	for _, t := range f.Terms {
		if qPower(t.E, f.N, q) {
			return false
		}
	}
	return true
}

func qPower(e Exp, n, q int) bool {
	for i := range n {
		if int(e[i])%q != 0 {
			return false
		}
	}
	return true
}

// Initial returns the initial form of f: the terms of the smallest total degree.
func (f Poly) Initial() Poly {
	out := Poly{P: f.P, N: f.N}
	if f.IsZero() {
		return out
	}
	d := f.Ord()
	for _, t := range f.Terms {
		if t.E.Deg() != d {
			break
		}
		out.Terms = append(out.Terms, t)
	}
	return out
}

// Equal reports whether f and g are the same polynomial.
func (f Poly) Equal(g Poly) bool {
	if f.P != g.P || f.N != g.N || len(f.Terms) != len(g.Terms) {
		return false
	}
	for i := range f.Terms {
		if f.Terms[i] != g.Terms[i] {
			return false
		}
	}
	return true
}

// Add returns f + g.
func (f Poly) Add(g Poly) Poly {
	a := newAcc(f.P)
	for _, t := range f.Terms {
		a.add(t.E, t.C)
	}
	for _, t := range g.Terms {
		a.add(t.E, t.C)
	}
	return a.poly(f.N)
}

// Neg returns -f.
func (f Poly) Neg() Poly {
	out := Poly{P: f.P, N: f.N, Terms: make([]Term, len(f.Terms))}
	for i, t := range f.Terms {
		out.Terms[i] = Term{E: t.E, C: uint32(f.P) - t.C} // #nosec G115 -- p is small and positive
	}
	return out
}

// Mul returns f * g.
func (f Poly) Mul(g Poly) Poly {
	a := newAcc(f.P)
	p := uint64(f.P) // #nosec G115 -- p is small and positive
	for _, s := range f.Terms {
		for _, t := range g.Terms {
			var e Exp
			for i := range e {
				e[i] = s.E[i] + t.E[i]
			}
			a.add(e, uint32(uint64(s.C)*uint64(t.C)%p)) // #nosec G115 -- the value is less than p
		}
	}
	return a.poly(f.N)
}

// Pow returns f^k.
func (f Poly) Pow(k int) Poly {
	out := Constant(f.P, f.N, 1)
	base := f
	for k > 0 {
		if k&1 == 1 {
			out = out.Mul(base)
		}
		k >>= 1
		if k > 0 {
			base = base.Mul(base)
		}
	}
	return out
}

// Constant returns the constant polynomial c.
func Constant(p, n, c int) Poly {
	c = ((c % p) + p) % p
	if c == 0 {
		return Poly{P: p, N: n}
	}
	return Poly{P: p, N: n, Terms: []Term{{C: uint32(c)}}} // #nosec G115 -- 0 < c < p
}

// Var returns the polynomial x_i.
func Var(p, n, i int) Poly {
	var e Exp
	e[i] = 1
	return Poly{P: p, N: n, Terms: []Term{{E: e, C: 1}}}
}

// Names returns the default names x1, ..., xn of n variables.
func Names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "x" + strconv.Itoa(i+1)
	}
	return out
}

// Format writes f with the variable names, for example 2*x1^3*x2 + x2^5. The zero polynomial is 0.
func (f Poly) Format(names []string) string {
	if f.IsZero() {
		return "0"
	}
	var b strings.Builder
	for k, t := range f.Terms {
		if k > 0 {
			b.WriteString(" + ")
		}
		mono := monomial(t.E, f.N, names)
		switch {
		case mono == "":
			b.WriteString(strconv.Itoa(int(t.C)))
		case t.C == 1:
			b.WriteString(mono)
		default:
			b.WriteString(strconv.Itoa(int(t.C)))
			b.WriteString("*")
			b.WriteString(mono)
		}
	}
	return b.String()
}

func monomial(e Exp, n int, names []string) string {
	var parts []string
	for i := range n {
		switch e[i] {
		case 0:
		case 1:
			parts = append(parts, names[i])
		default:
			parts = append(parts, fmt.Sprintf("%s^%d", names[i], e[i]))
		}
	}
	return strings.Join(parts, "*")
}

// String writes f with the names x1, ..., xn.
func (f Poly) String() string { return f.Format(Names(f.N)) }
