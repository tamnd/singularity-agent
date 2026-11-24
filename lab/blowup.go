package lab

import (
	"errors"
	"fmt"
	"math/bits"
	"sort"
	"strings"
)

// Divisor is a set of the variables x_i, as a bit mask. It stands for the normal crossings divisor E = V(prod_{i in Delta} x_i).
type Divisor uint32

// Has reports whether x_i is in the divisor.
func (d Divisor) Has(i int) bool { return d&(1<<i) != 0 }

// All returns the divisor of all n variables.
func All(n int) Divisor { return Divisor(1<<n - 1) }

// Size returns the number of components.
func (d Divisor) Size() int { return bits.OnesCount32(uint32(d)) }

// Format writes the divisor as a list of names, for example [x, w].
func (d Divisor) Format(names []string) []string {
	out := []string{}
	for i, n := range names {
		if d.Has(i) {
			out = append(out, n)
		}
	}
	return out
}

// Point is a point blowup in the x_J chart with the translations x_i -> x_i + T[i] for i != J. The map is z -> x_J z, x_J -> x_J and x_i -> x_J (x_i + T[i]) for i != J. T[J] is 0.
type Point struct {
	J int
	T Exp
}

// Divisor returns the divisor at the new origin: {J} with each i of d where T[i] = 0.
func (pt Point) Divisor(d Divisor, n int) Divisor {
	out := Divisor(1 << pt.J)
	for i := range n {
		if d.Has(i) && pt.T[i] == 0 {
			out |= 1 << i
		}
	}
	return out
}

// Format writes the chart in the words of Hauser and Perlega, for example "x-chart, y -> y+1".
func (pt Point) Format(names []string) string {
	var b strings.Builder
	b.WriteString(names[pt.J] + "-chart")
	for i, n := range names {
		if i != pt.J && pt.T[i] != 0 {
			fmt.Fprintf(&b, ", %s -> %s+%d", n, n, pt.T[i])
		}
	}
	return b.String()
}

// Translations returns the non-zero translations by name.
func (pt Point) Translations(names []string) map[string]int {
	out := map[string]int{}
	for i, n := range names {
		if i != pt.J && pt.T[i] != 0 {
			out[n] = int(pt.T[i])
		}
	}
	return out
}

// ErrCap is the error of a blowup whose expansion has more terms than the cap.
var ErrCap = errors.New("lab: the expansion is larger than the cap")

// kc is a term c*x^k of (x + t)^m.
type kc struct{ k, c uint32 }

// Engine does the computations for one characteristic p and one q = p^e. An Engine is not safe for use by more than one goroutine.
type Engine struct {
	P, Q, E int
	// Reduce removes q-th power units from the transform (see Blowup). It does not change any invariant that the lab computes.
	Reduce bool
	// MaxExpand is the largest number of terms that Blowup expands before it adds them up. Zero means no cap.
	MaxExpand int

	small [][]uint32      // small[a][b] = binomial(a, b) mod p for a, b < p
	exps  map[uint64][]kc // the terms of (x + t)^m, by m*p + t
	acc   *acc            // the collector of Blowup
	low   *acc            // the collector of OrderAtLeast

	// Shifted tells whether the transform of the last Step had a constant term. The point of the strict transform is then at z = c^(1/q), and the change z -> z + c^(1/q) of the cleaning removes the constant c.
	Shifted bool
}

// NewEngine returns an engine for p and q. q must be a power p^e with e >= 1.
func NewEngine(p, q int) (*Engine, error) {
	if p < 2 || p > 251 || !prime(p) {
		return nil, fmt.Errorf("lab: p = %d is not a prime from 2 to 251", p)
	}
	e, qq := 0, 1
	for qq < q {
		qq *= p
		e++
	}
	if qq != q || e == 0 {
		return nil, fmt.Errorf("lab: q = %d is not a power p^e of p = %d with e >= 1", q, p)
	}
	en := &Engine{P: p, Q: q, E: e, Reduce: true, exps: map[uint64][]kc{}, acc: newAcc(p), low: newAcc(p)}
	en.small = make([][]uint32, p)
	for a := range p {
		en.small[a] = make([]uint32, p)
		en.small[a][0] = 1
		for b := 1; b <= a; b++ {
			en.small[a][b] = (en.small[a-1][b-1] + en.small[a-1][b]) % uint32(p) // #nosec G115 -- p is small
		}
	}
	return en, nil
}

func prime(p int) bool {
	for d := 2; d*d <= p; d++ {
		if p%d == 0 {
			return false
		}
	}
	return true
}

// MohBound returns p^(e-1), the bound of Moh for the increase of the residual order under one blowup.
func (en *Engine) MohBound() int { return en.Q / en.P }

// expansion returns the non-zero terms of (x + t)^m over F_p, with t in 1..p-1, by increasing k. By the theorem of Lucas, binomial(m, k) mod p is the product of the binomials of the base p digits, and it is non-zero only when each digit of k is at most the digit of m.
func (en *Engine) expansion(m, t uint32) []kc {
	p := uint32(en.P) // #nosec G115 -- p is small
	key := uint64(m)*uint64(p) + uint64(t)
	if v, ok := en.exps[key]; ok {
		return v
	}
	var digits []uint32
	for x := m; x > 0; x /= p {
		digits = append(digits, x%p)
	}
	// tp[i] = t^i mod p for i < p-1; t^(p-1) = 1.
	tp := make([]uint32, p-1)
	tp[0] = 1
	for i := 1; i < len(tp); i++ {
		tp[i] = tp[i-1] * t % p
	}
	out := []kc{{k: 0, c: 1}}
	place := uint32(1)
	for _, dm := range digits {
		next := make([]kc, 0, len(out)*int(dm+1))
		for dk := uint32(0); dk <= dm; dk++ {
			b := en.small[dm][dk]
			for _, x := range out {
				next = append(next, kc{k: x.k + dk*place, c: x.c * b % p})
			}
		}
		out = next
		place *= p
	}
	for i := range out {
		out[i].c = out[i].c * tp[(m-out[i].k)%(p-1)] % p
	}
	sortKC(out)
	en.exps[key] = out
	return out
}

func sortKC(s []kc) { sort.Slice(s, func(i, j int) bool { return s[i].k < s[j].k }) }

// reductions returns, for each translated variable x_i, the multiple s_i of q that Blowup removes from the exponents of x_i. The exponent of x_i in each term of f is at least s_i.
func (en *Engine) reductions(f Poly, pt Point) Exp {
	var s Exp
	if !en.Reduce {
		return s
	}
	for i := range f.N {
		if i != pt.J && pt.T[i] != 0 {
			g := f.MinExp(i)
			s[i] = g - g%uint32(en.Q) // #nosec G115 -- q is small
		}
	}
	return s
}

// Blowup returns the strict transform F' = x_J^(-q) pi(F) of f = z^q + F in the chart pt. The order of F must be at least q. The result is not cleaned.
//
// When en.Reduce is true, Blowup drops a unit factor that is a q-th power. Write s_i for the largest multiple of q that divides the order of F along x_i = 0, for a translated variable x_i. Then F = x_i^(s_i) F_1 and pi(F) = x_J^(s_i) (x_i + t_i)^(s_i) pi(F_1). The factor (x_i + t_i)^(s_i) is a unit that is a q-th power. Multiplication by such a unit keeps a polynomial clean, and it does not change the order, the orders along the x_i, the residual order or the number of terms of an initial form, at this point and at each later point of the walk. So the walk sees the same invariants, with much smaller polynomials.
func (en *Engine) Blowup(f Poly, pt Point) (Poly, error) {
	if f.Ord() < en.Q {
		return Poly{}, fmt.Errorf("lab: the order of F is %d, less than q = %d", f.Ord(), en.Q)
	}
	n := f.N
	s := en.reductions(f, pt)
	var tr []int
	for i := range n {
		if i != pt.J && pt.T[i] != 0 {
			tr = append(tr, i)
		}
	}
	if en.MaxExpand > 0 {
		est := 0
		for _, t := range f.Terms {
			prod := 1
			for _, i := range tr {
				prod *= len(en.expansion(t.E[i]-s[i], pt.T[i]))
				if prod > en.MaxExpand {
					return Poly{}, ErrCap
				}
			}
			est += prod
			if est > en.MaxExpand {
				return Poly{}, ErrCap
			}
		}
	}
	p := uint32(en.P) // #nosec G115 -- p is small
	lists := make([][]kc, len(tr))
	for _, t := range f.Terms {
		var base Exp
		base[pt.J] = uint32(t.E.Deg() - en.Q) // #nosec G115 -- the degree is at least q
		for i := range n {
			if i != pt.J && pt.T[i] == 0 {
				base[i] = t.E[i]
			}
		}
		for k, i := range tr {
			lists[k] = en.expansion(t.E[i]-s[i], pt.T[i])
		}
		en.product(tr, lists, 0, base, t.C, p)
	}
	return en.acc.poly(n), nil
}

func (en *Engine) product(tr []int, lists [][]kc, k int, e Exp, c, p uint32) {
	if k == len(tr) {
		en.acc.add(e, c)
		return
	}
	i := tr[k]
	for _, x := range lists[k] {
		e[i] = x.k
		en.product(tr, lists, k+1, e, c*x.c%p, p)
	}
}

// OrderAtLeast reports whether the order of f stays q at the point pt of the exceptional divisor. The test is ord F' >= q for the cleaned transform F'. A constant term c of F' does not count: it is a q-th power, and the cleaning removes it with z -> z + c^(1/q), which moves the point to the point of the strict transform over pt. The test computes only the terms of degree less than q of the transform, so it is much faster than Blowup. The order of F must be at least q.
func (en *Engine) OrderAtLeast(f Poly, pt Point) bool {
	return en.orderAtLeast(f.N, lowTerms(f, en.Q), minExps(f), pt)
}

// lowTerms returns the terms of f of degree less than 2q. Only these terms can give a term of degree less than q in a transform, because the transform of x^a has degree at least |a| - q.
func lowTerms(f Poly, q int) []Term {
	k := 0
	for k < len(f.Terms) && f.Terms[k].E.Deg() < 2*q {
		k++
	}
	return f.Terms[:k]
}

// minExps returns the order of f along each x_i = 0.
func minExps(f Poly) Exp {
	var g Exp
	for i := range f.N {
		g[i] = f.MinExp(i)
	}
	return g
}

func (en *Engine) orderAtLeast(n int, low []Term, g Exp, pt Point) bool {
	q := en.Q
	var s Exp
	var tr []int
	for i := range n {
		if i != pt.J && pt.T[i] != 0 {
			tr = append(tr, i)
			if en.Reduce {
				s[i] = g[i] - g[i]%uint32(q) // #nosec G115 -- q is small
			}
		}
	}
	p := uint32(en.P) // #nosec G115 -- p is small
	// The transform has order at least q when its parts of degree 1 to q-1 are zero. The test computes them one degree at a time and stops at the first part that is not zero, so a point where the order drops costs little. The part of degree 0 is the constant term, which does not count.
	bases := make([]Exp, len(low))
	bdegs := make([]int, len(low))
	for k, t := range low {
		deg := t.E.Deg()
		bases[k][pt.J] = uint32(deg - q) // #nosec G115 -- the degree is at least q
		bdegs[k] = deg - q
		for i := range n {
			if i != pt.J && pt.T[i] == 0 {
				bases[k][i] = t.E[i]
				bdegs[k] += int(t.E[i])
			}
		}
	}
	for deg := 1; deg < q; deg++ {
		for k, t := range low {
			if bdegs[k] <= deg {
				en.lowProduct(tr, t.E, s, pt, 0, bases[k], deg-bdegs[k], t.C, p)
			}
		}
		if len(en.low.m) > 0 {
			clear(en.low.m)
			return false
		}
	}
	return true
}

// lowProduct adds the terms of degree exactly rest in the translated variables tr[k:] of the product of the expansions of (x_i + t_i)^(a_i - s_i), times c x^e.
func (en *Engine) lowProduct(tr []int, a, s Exp, pt Point, k int, e Exp, rest int, c, p uint32) {
	if k == len(tr) {
		if rest == 0 {
			en.low.add(e, c)
		}
		return
	}
	i := tr[k]
	for _, x := range en.expansion(a[i]-s[i], pt.T[i]) {
		if int(x.k) > rest {
			break // the terms are sorted by k
		}
		if k == len(tr)-1 && int(x.k) != rest {
			continue
		}
		e[i] = x.k
		en.lowProduct(tr, a, s, pt, k+1, e, rest-int(x.k), c*x.c%p, p)
	}
}

// Points returns each chart and translation over F_p at which the order of f stays q, in a fixed order: by the chart J, then by the translations as a number in base p with the first variable as the most significant digit.
func (en *Engine) Points(f Poly) []Point {
	n := f.N
	low, g := lowTerms(f, en.Q), minExps(f)
	var out []Point
	for j := range n {
		var t Exp
		for {
			pt := Point{J: j, T: t}
			if len(low) == 0 || en.orderAtLeast(n, low, g, pt) {
				out = append(out, pt)
			}
			// the next translation vector, skipping the entry j
			i := n - 1
			for ; i >= 0; i-- {
				if i == j {
					continue
				}
				t[i]++
				if int(t[i]) < en.P {
					break
				}
				t[i] = 0
			}
			if i < 0 {
				break
			}
		}
	}
	return out
}

// Step does one point blowup: it returns the cleaned strict transform and the new divisor. The cleaning removes a constant term of the transform (see OrderAtLeast), and Shifted tells whether it did.
func (en *Engine) Step(f Poly, d Divisor, pt Point) (Poly, Divisor, error) {
	g, err := en.Blowup(f, pt)
	if err != nil {
		return Poly{}, 0, err
	}
	en.Shifted = len(g.Terms) > 0 && g.Terms[0].E == Exp{}
	g = g.Clean(en.Q)
	if g.Ord() < en.Q {
		return Poly{}, 0, fmt.Errorf("lab: the order drops to %d in the chart %s", g.Ord(), pt.Format(Names(f.N)))
	}
	return g, pt.Divisor(d, f.N), nil
}

// Residual returns the residual order of z^q + F along the divisor d, with the orders r_i of F along the x_i of d. F must be clean. Write F = prod_{i in d} x_i^(r_i) G with r_i maximal; the residual order is the order of G.
func Residual(f Poly, d Divisor) (int, Exp) {
	var r Exp
	sum := 0
	for i := range f.N {
		if d.Has(i) {
			r[i] = f.MinExp(i)
			sum += int(r[i])
		}
	}
	return f.Ord() - sum, r
}
