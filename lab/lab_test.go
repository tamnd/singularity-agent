package lab

import (
	"testing"
)

func mustParse(t *testing.T, p int, names []string, src string) Poly {
	t.Helper()
	f, err := Parse(p, names, src)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func mustEngine(t *testing.T, p, q int) *Engine {
	t.Helper()
	en, err := NewEngine(p, q)
	if err != nil {
		t.Fatal(err)
	}
	return en
}

func TestParseFormat(t *testing.T) {
	names := []string{"x", "y"}
	f := mustParse(t, 2, names, "(x+y)^4 + x*y - 3*y^2 + 1")
	if got, want := f.Format(names), "1 + x*y + y^2 + x^4 + y^4"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	g := mustParse(t, 2, names, f.Format(names))
	if !g.Equal(f) {
		t.Errorf("the formatted polynomial does not parse back to the same polynomial")
	}
	for _, bad := range []string{"x +", "(x", "z", "x^y", "x ) "} {
		if _, err := Parse(2, names, bad); err == nil {
			t.Errorf("Parse(%q) has no error", bad)
		}
	}
}

func TestClean(t *testing.T) {
	names := []string{"x", "y"}
	f := mustParse(t, 3, names, "x^3*y^6 + x^3*y + x^9 + y^2")
	got := f.Clean(3).Format(names)
	if want := "y^2 + x^3*y"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestExpansion(t *testing.T) {
	en := mustEngine(t, 3, 3)
	// (x+2)^4 over F_3 = x^4 + 8x^3 + 24x^2 + 32x + 16 = x^4 + 2x^3 + 2x + 1
	got := en.expansion(4, 2)
	want := []kc{{0, 1}, {1, 2}, {3, 2}, {4, 1}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestHauserPerlegaSection3 is the example of Section 3 of Hauser-Perlega, arXiv:1802.05010: p = 2, q = 4, f = z^4 + x^2 y^2 w^3 (w (x+y)^4 + x^13), E = V(x y w). In the x-chart with y -> y+1 the residual order goes from 5 to 7.
func TestHauserPerlegaSection3(t *testing.T) {
	names := []string{"x", "y", "w"}
	en := mustEngine(t, 2, 4)
	f := mustParse(t, 2, names, "x^2*y^2*w^3*(w*(x+y)^4 + x^13)")
	if !f.IsClean(4) {
		t.Fatal("the start polynomial is not clean")
	}
	d := All(3)
	if r, _ := Residual(f, d); r != 5 {
		t.Errorf("the residual order at the start is %d, want 5", r)
	}
	if st := en.Measure(f, d); st.PF != 2 {
		t.Errorf("p^r of Moh is %d, want 2", st.PF)
	}
	pt := Point{J: 0, T: Exp{0, 1, 0}}
	if !en.OrderAtLeast(f, pt) {
		t.Fatal("OrderAtLeast says that the order drops")
	}
	g, d2, err := en.Step(f, d, pt)
	if err != nil {
		t.Fatal(err)
	}
	want := mustParse(t, 2, names, "x^8*w^3*(w*y^6 + x^8 + x^8*y^2)")
	if !g.Equal(want) {
		t.Errorf("the transform is %s, want %s", g.Format(names), want.Format(names))
	}
	if got := d2.Format(names); len(got) != 2 || got[0] != "x" || got[1] != "w" {
		t.Errorf("the new divisor is %v, want [x w]", got)
	}
	if r, _ := Residual(g, d2); r != 7 {
		t.Errorf("the residual order after the blowup is %d, want 7", r)
	}
	// The bound of the false step of Moh: 7 > floor(5/2)*2 + 2 = 6.
	if !CandidateByID("moh-rounded").Fails(en.Measure(f, d), en.Measure(g, d2), 4) {
		t.Error("the rounded bound of Moh does not fail on the example")
	}
}

// TestHauserPerlegaSection4 checks the first two steps of the first example of Section 4 of Hauser-Perlega, with d = 2, a = b = r = 0, s = 1, lambda = 1, A = 1 and Q = 0. Then p = 2, q = 8, n = 5 and F = x^4 y^4 w^6 (w^2 + x^3 u^10).
func TestHauserPerlegaSection4(t *testing.T) {
	names := []string{"x", "y", "u", "v", "w"}
	en := mustEngine(t, 2, 8)
	f := mustParse(t, 2, names, "x^4*y^4*w^6*(w^2*(1 + 0*u^10) + x^3*u^10*1)")
	d := All(5)
	if r, _ := Residual(f, d); r != 2 {
		t.Fatalf("the residual order at the start is %d, want d = 2", r)
	}
	// (1) x-chart, y -> y+1, u -> u+1: the residual order goes up to d + 4 = 6.
	f1, d1, err := en.Step(f, d, Point{J: 0, T: Exp{0, 1, 1, 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := Residual(f1, d1); r != 6 {
		t.Errorf("after step (1) the residual order is %d, want 6", r)
	}
	// (2) x-chart: the residual order goes down to d + 3 = 5.
	f2, d2, err := en.Step(f1, d1, Point{J: 0})
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := Residual(f2, d2); r != 5 {
		t.Errorf("after step (2) the residual order is %d, want 5", r)
	}
}

// TestReduce checks that the reduction of q-th power units does not change the invariants along a walk.
func TestReduce(t *testing.T) {
	names := []string{"x", "y", "w"}
	f := mustParse(t, 3, names, "x^7*y^5*w^4*(w*(x+y)^3 + x^2*y^4 + 2*y^7)")
	pts := []Point{{J: 0, T: Exp{0, 1, 0}}, {J: 2, T: Exp{2, 0, 0}}, {J: 1, T: Exp{1, 0, 1}}, {J: 0}}
	run := func(reduce bool) []State {
		en := mustEngine(t, 3, 3)
		en.Reduce = reduce
		g, d := f.Clean(3), All(3)
		out := []State{en.Measure(g, d)}
		for _, pt := range pts {
			if !en.OrderAtLeast(g, pt) {
				t.Fatalf("the order drops at %s", pt.Format(names))
			}
			var err error
			g, d, err = en.Step(g, d, pt)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, en.Measure(g, d))
		}
		return out
	}
	a, b := run(true), run(false)
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("step %d: %+v with the reduction, %+v without", i, a[i], b[i])
		}
	}
}

// TestShift checks a point where the transform has a constant term: the order of f stays q after the change of z.
func TestShift(t *testing.T) {
	names := []string{"x", "y", "w"}
	en := mustEngine(t, 2, 2)
	f := mustParse(t, 2, names, "x*y + x*w + y*w + x^3*y")
	pt := Point{J: 0, T: Exp{0, 1, 1}}
	if !en.OrderAtLeast(f, pt) {
		t.Fatal("OrderAtLeast says that the order drops")
	}
	g, _, err := en.Step(f, All(3), pt)
	if err != nil {
		t.Fatal(err)
	}
	if !en.Shifted || g.Format(names) != "y*w + x^2*y" {
		t.Errorf("the step gives %s, shifted %v", g.Format(names), en.Shifted)
	}
}

// TestOrderAtLeast compares the fast test of the order with the full transform.
func TestOrderAtLeast(t *testing.T) {
	names := []string{"x", "y", "w"}
	for _, src := range []string{"x*y + x*w + y*w + x^3*y", "x^2*y + y^3*w + x*w^2", "x^3*y + y^2*w^2 + x*y*w^2 + w^5", "x*y^2*w + x^4 + y^5"} {
		for _, p := range []int{2, 3} {
			q := p
			en := mustEngine(t, p, q)
			f := mustParse(t, p, names, src).Clean(q)
			if f.Ord() < q {
				continue
			}
			for _, pt := range allPoints(3, p) {
				g, err := en.Blowup(f, pt)
				if err != nil {
					t.Fatal(err)
				}
				g = g.Clean(q)
				if got, want := en.OrderAtLeast(f, pt), g.Ord() >= q; got != want {
					t.Errorf("p = %d, %s, %s: OrderAtLeast is %v, the order of the cleaned transform is %d", p, src, pt.Format(names), got, g.Ord())
				}
			}
		}
	}
}

func allPoints(n, p int) []Point {
	var out []Point
	total := 1
	for range n - 1 {
		total *= p
	}
	for j := range n {
		for k := range total {
			var t Exp
			x := k
			for i := n - 1; i >= 0; i-- {
				if i == j {
					continue
				}
				t[i] = uint32(x % p) // #nosec G115 -- small
				x /= p
			}
			out = append(out, Point{J: j, T: t})
		}
	}
	return out
}
