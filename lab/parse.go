package lab

import (
	"fmt"
	"strconv"
	"unicode"
)

// Parse reads a polynomial over F_p in the variables names. The syntax has integers, the names, +, -, *, ^ with an integer exponent, and parentheses, for example x^2*y^2*w^3*(w*(x+y)^4 + x^13).
func Parse(p int, names []string, src string) (Poly, error) {
	if len(names) == 0 || len(names) > MaxVars {
		return Poly{}, fmt.Errorf("lab: a polynomial needs 1 to %d variables, not %d", MaxVars, len(names))
	}
	idx := map[string]int{}
	for i, n := range names {
		if _, dup := idx[n]; dup {
			return Poly{}, fmt.Errorf("lab: the variable %s is in the list twice", n)
		}
		idx[n] = i
	}
	ps := &parser{p: p, n: len(names), idx: idx, src: []rune(src)}
	f, err := ps.sum()
	if err != nil {
		return Poly{}, err
	}
	ps.space()
	if ps.pos < len(ps.src) {
		return Poly{}, fmt.Errorf("lab: unexpected %q at position %d of %q", string(ps.src[ps.pos]), ps.pos, src)
	}
	return f, nil
}

type parser struct {
	p, n int
	idx  map[string]int
	src  []rune
	pos  int
}

func (ps *parser) space() {
	for ps.pos < len(ps.src) && unicode.IsSpace(ps.src[ps.pos]) {
		ps.pos++
	}
}

func (ps *parser) peek() rune {
	ps.space()
	if ps.pos < len(ps.src) {
		return ps.src[ps.pos]
	}
	return 0
}

func (ps *parser) sum() (Poly, error) {
	neg := false
	if ps.peek() == '-' {
		neg = true
		ps.pos++
	} else if ps.peek() == '+' {
		ps.pos++
	}
	f, err := ps.product()
	if err != nil {
		return Poly{}, err
	}
	if neg {
		f = f.Neg()
	}
	for {
		switch ps.peek() {
		case '+':
			ps.pos++
			g, err := ps.product()
			if err != nil {
				return Poly{}, err
			}
			f = f.Add(g)
		case '-':
			ps.pos++
			g, err := ps.product()
			if err != nil {
				return Poly{}, err
			}
			f = f.Add(g.Neg())
		default:
			return f, nil
		}
	}
}

func (ps *parser) product() (Poly, error) {
	f, err := ps.power()
	if err != nil {
		return Poly{}, err
	}
	for ps.peek() == '*' {
		ps.pos++
		g, err := ps.power()
		if err != nil {
			return Poly{}, err
		}
		f = f.Mul(g)
	}
	return f, nil
}

func (ps *parser) power() (Poly, error) {
	f, err := ps.atom()
	if err != nil {
		return Poly{}, err
	}
	if ps.peek() == '^' {
		ps.pos++
		ps.space()
		k, ok := ps.integer()
		if !ok {
			return Poly{}, fmt.Errorf("lab: an exponent must be an integer, at position %d", ps.pos)
		}
		f = f.Pow(k)
	}
	return f, nil
}

func (ps *parser) integer() (int, bool) {
	start := ps.pos
	for ps.pos < len(ps.src) && unicode.IsDigit(ps.src[ps.pos]) {
		ps.pos++
	}
	if start == ps.pos {
		return 0, false
	}
	k, err := strconv.Atoi(string(ps.src[start:ps.pos]))
	return k, err == nil
}

func (ps *parser) atom() (Poly, error) {
	r := ps.peek()
	switch {
	case r == '(':
		ps.pos++
		f, err := ps.sum()
		if err != nil {
			return Poly{}, err
		}
		if ps.peek() != ')' {
			return Poly{}, fmt.Errorf("lab: a parenthesis is not closed, at position %d", ps.pos)
		}
		ps.pos++
		return f, nil
	case unicode.IsDigit(r):
		k, _ := ps.integer()
		return Constant(ps.p, ps.n, k%ps.p), nil
	case unicode.IsLetter(r):
		start := ps.pos
		for ps.pos < len(ps.src) && (unicode.IsLetter(ps.src[ps.pos]) || unicode.IsDigit(ps.src[ps.pos]) || ps.src[ps.pos] == '_') {
			ps.pos++
		}
		name := string(ps.src[start:ps.pos])
		i, ok := ps.idx[name]
		if !ok {
			return Poly{}, fmt.Errorf("lab: unknown variable %s", name)
		}
		return Var(ps.p, ps.n, i), nil
	}
	if r == 0 {
		return Poly{}, fmt.Errorf("lab: the polynomial ends too early")
	}
	return Poly{}, fmt.Errorf("lab: unexpected %q at position %d", string(r), ps.pos)
}
