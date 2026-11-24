package lab

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// Spec is a lab job, a file lab/specs/<name>.yaml of the content repository (spec 13.2).
type Spec struct {
	Name       string   `yaml:"name" json:"name"`
	Experiment string   `yaml:"experiment" json:"experiment"`
	Question   string   `yaml:"question" json:"question"`
	Tool       string   `yaml:"tool" json:"tool"`
	Node       string   `yaml:"node" json:"node"`
	Nodes      []string `yaml:"nodes,omitempty" json:"nodes,omitempty"`
	Inputs     Inputs   `yaml:"inputs" json:"inputs"`
	Outputs    []string `yaml:"outputs" json:"outputs"`
	Notes      string   `yaml:"notes,omitempty" json:"notes,omitempty"`

	// Path is the file of the spec, Hash the sha256 of its bytes. They are not in the YAML.
	Path string `yaml:"-" json:"-"`
	Hash string `yaml:"-" json:"-"`
	// From is the spec of walks_from, for experiment E2.
	From *Spec `yaml:"-" json:"-"`
}

// Inputs are the inputs of a spec. A spec has cases (random walks), walks_from (the walks of another spec) or fixed (one given equation and one given sequence of blowups).
type Inputs struct {
	Seed       uint64   `yaml:"seed,omitempty" json:"seed,omitempty"`
	Family     Family   `yaml:"family,omitempty" json:"family"`
	Walk       WalkOpts `yaml:"walk,omitempty" json:"walk"`
	Examples   int      `yaml:"examples,omitempty" json:"examples,omitempty"`
	Stability  int      `yaml:"stability_examples,omitempty" json:"stability_examples,omitempty"`
	Raw        bool     `yaml:"raw,omitempty" json:"raw,omitempty"`
	Cases      []Case   `yaml:"cases,omitempty" json:"cases,omitempty"`
	WalksFrom  string   `yaml:"walks_from,omitempty" json:"walks_from,omitempty"`
	Candidates []string `yaml:"candidates,omitempty" json:"candidates,omitempty"`
	Fixed      *Fixed   `yaml:"fixed,omitempty" json:"fixed,omitempty"`
}

// Family is the family of the start polynomials F of random walks.
type Family struct {
	// Name is random-sparse: a random number of terms in Terms, each with a random total degree in Degree, a random exponent vector of that degree (uniform among the vectors of that degree) and a random coefficient in 1..p-1. The family cleans the sum and draws again when the sum is zero or has order less than q.
	Name    string `yaml:"name,omitempty" json:"name,omitempty"`
	Degree  []int  `yaml:"degree,omitempty" json:"degree,omitempty"`
	Terms   []int  `yaml:"terms,omitempty" json:"terms,omitempty"`
	Divisor string `yaml:"divisor,omitempty" json:"divisor,omitempty"` // all, none or random (each x_i with probability 1/2)
}

// WalkOpts are the rules of a random walk.
type WalkOpts struct {
	Depth     int `yaml:"depth,omitempty" json:"depth,omitempty"`           // the largest number of blowups
	MaxTerms  int `yaml:"max_terms,omitempty" json:"max_terms,omitempty"`   // the walk stops after a transform with more terms
	MaxExpand int `yaml:"max_expand,omitempty" json:"max_expand,omitempty"` // the walk stops before a transform whose expansion has more terms
	MaxDegree int `yaml:"max_degree,omitempty" json:"max_degree,omitempty"` // the walk stops after a transform with a term of a larger total degree
}

// Case is one (p, n, q) of a spec. Degree, Terms, Divisor and Depth replace the values of the family and the walk when they are set.
type Case struct {
	P       int    `yaml:"p" json:"p"`
	N       int    `yaml:"n" json:"n"`
	Q       int    `yaml:"q" json:"q"`
	Walks   int    `yaml:"walks" json:"walks"`
	Degree  []int  `yaml:"degree,omitempty" json:"degree,omitempty"`
	Terms   []int  `yaml:"terms,omitempty" json:"terms,omitempty"`
	Divisor string `yaml:"divisor,omitempty" json:"divisor,omitempty"`
	Depth   int    `yaml:"depth,omitempty" json:"depth,omitempty"`
}

// Fixed is one given equation z^q + F with a divisor and a given sequence of point blowups.
type Fixed struct {
	P       int         `yaml:"p" json:"p"`
	Q       int         `yaml:"q" json:"q"`
	Vars    []string    `yaml:"vars" json:"vars"`
	F       string      `yaml:"f" json:"f"`
	Divisor []string    `yaml:"divisor" json:"divisor"`
	Expect  *int        `yaml:"expect,omitempty" json:"expect,omitempty"` // the residual order at the start, from the source
	Reduce  bool        `yaml:"reduce,omitempty" json:"reduce,omitempty"` // drop q-th power units; the default writes the literal strict transforms
	Steps   []FixedStep `yaml:"steps" json:"steps"`
	Source  string      `yaml:"source,omitempty" json:"source,omitempty"`
}

// FixedStep is one point blowup of a fixed sequence, done Repeat times (1 when Repeat is 0).
type FixedStep struct {
	Label     string         `yaml:"label,omitempty" json:"label,omitempty"`
	Chart     string         `yaml:"chart" json:"chart"`
	Translate map[string]int `yaml:"translate,omitempty" json:"translate,omitempty"`
	Repeat    int            `yaml:"repeat,omitempty" json:"repeat,omitempty"`
	Expect    *int           `yaml:"expect,omitempty" json:"expect,omitempty"` // the residual order after the step (after the last repeat), from the source
}

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var tagRE = regexp.MustCompile(`^SG[0-9A-HJKMNP-TV-Z]{8}$`)

// LoadSpec reads and checks a spec. A spec with walks_from also loads that spec, from the same directory.
func LoadSpec(path string) (*Spec, error) {
	s, err := loadOne(path)
	if err != nil {
		return nil, err
	}
	if s.Inputs.WalksFrom != "" {
		from := filepath.Join(filepath.Dir(path), s.Inputs.WalksFrom)
		if s.From, err = loadOne(from); err != nil {
			return nil, err
		}
		if s.From.Inputs.WalksFrom != "" || len(s.From.Inputs.Cases) == 0 {
			return nil, fmt.Errorf("%s: walks_from must name a spec with cases", path)
		}
	}
	return s, nil
}

func loadOne(path string) (*Spec, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- a spec that the user names
	if err != nil {
		return nil, err
	}
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	s.Path, s.Hash = path, hex.EncodeToString(sum[:])
	if err := s.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &s, nil
}

func (s *Spec) check() error {
	switch {
	case !nameRE.MatchString(s.Name):
		return fmt.Errorf("the name %q must have only lower case letters, digits and hyphens", s.Name)
	case s.Tool != "sing-lab":
		return fmt.Errorf("the tool is %q; sing lab runs only the tool sing-lab", s.Tool)
	case s.Question == "":
		return fmt.Errorf("the spec has no question")
	case !tagRE.MatchString(s.Node):
		return fmt.Errorf("the node %q is not a tag", s.Node)
	}
	for _, t := range s.Nodes {
		if !tagRE.MatchString(t) {
			return fmt.Errorf("the node %q is not a tag", t)
		}
	}
	in := &s.Inputs
	kinds := 0
	if len(in.Cases) > 0 {
		kinds++
	}
	if in.WalksFrom != "" {
		kinds++
	}
	if in.Fixed != nil {
		kinds++
	}
	if kinds != 1 {
		return fmt.Errorf("the inputs must have exactly one of cases, walks_from and fixed")
	}
	if in.WalksFrom != "" {
		if len(in.Candidates) == 0 {
			in.Candidates = candidateIDs()
		}
		for _, c := range in.Candidates {
			if CandidateByID(c) == nil {
				return fmt.Errorf("unknown candidate %q; the candidates are %v", c, candidateIDs())
			}
		}
		return nil
	}
	if in.Fixed != nil {
		return in.Fixed.check()
	}
	if in.Walk.Depth <= 0 {
		return fmt.Errorf("walk.depth must be positive")
	}
	if in.Family.Name != "random-sparse" {
		return fmt.Errorf("the family %q is not known; use random-sparse", in.Family.Name)
	}
	for i := range in.Cases {
		c := in.Case(i)
		if _, err := NewEngine(c.P, c.Q); err != nil {
			return fmt.Errorf("case %d: %w", i+1, err)
		}
		switch {
		case c.N < 1 || c.N > MaxVars:
			return fmt.Errorf("case %d: n = %d is not in 1..%d", i+1, c.N, MaxVars)
		case c.Walks <= 0:
			return fmt.Errorf("case %d: the number of walks must be positive", i+1)
		case len(c.Degree) != 2 || c.Degree[0] < c.Q || c.Degree[1] < c.Degree[0]:
			return fmt.Errorf("case %d: degree must be [lo, hi] with q <= lo <= hi", i+1)
		case len(c.Terms) != 2 || c.Terms[0] < 1 || c.Terms[1] < c.Terms[0]:
			return fmt.Errorf("case %d: terms must be [lo, hi] with 1 <= lo <= hi", i+1)
		case c.Divisor != "all" && c.Divisor != "none" && c.Divisor != "random":
			return fmt.Errorf("case %d: the divisor must be all, none or random", i+1)
		case c.Depth <= 0:
			return fmt.Errorf("case %d: the depth must be positive", i+1)
		}
	}
	return nil
}

// Case returns case i with the values of the family and the walk filled in.
func (in *Inputs) Case(i int) Case {
	c := in.Cases[i]
	if c.Degree == nil {
		c.Degree = in.Family.Degree
	}
	if c.Terms == nil {
		c.Terms = in.Family.Terms
	}
	if c.Divisor == "" {
		c.Divisor = in.Family.Divisor
	}
	if c.Divisor == "" {
		c.Divisor = "all"
	}
	if c.Depth == 0 {
		c.Depth = in.Walk.Depth
	}
	return c
}

func (f *Fixed) check() error {
	if _, err := NewEngine(f.P, f.Q); err != nil {
		return err
	}
	if _, err := f.Poly(); err != nil {
		return err
	}
	if _, err := f.divisor(); err != nil {
		return err
	}
	for i, st := range f.Steps {
		if _, err := f.point(st); err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}
		if st.Repeat < 0 {
			return fmt.Errorf("step %d: repeat must not be negative", i+1)
		}
	}
	return nil
}

// Poly returns the polynomial F of the fixed input.
func (f *Fixed) Poly() (Poly, error) { return Parse(f.P, f.Vars, f.F) }

func (f *Fixed) index(name string) (int, error) {
	for i, v := range f.Vars {
		if v == name {
			return i, nil
		}
	}
	return 0, fmt.Errorf("unknown variable %s", name)
}

func (f *Fixed) divisor() (Divisor, error) {
	var d Divisor
	for _, v := range f.Divisor {
		i, err := f.index(v)
		if err != nil {
			return 0, err
		}
		d |= 1 << i
	}
	return d, nil
}

func (f *Fixed) point(st FixedStep) (Point, error) {
	j, err := f.index(st.Chart)
	if err != nil {
		return Point{}, err
	}
	pt := Point{J: j}
	for v, t := range st.Translate {
		i, err := f.index(v)
		if err != nil {
			return Point{}, err
		}
		if i == j {
			return Point{}, fmt.Errorf("the chart variable %s cannot have a translation", v)
		}
		pt.T[i] = uint32(((t % f.P) + f.P) % f.P) // #nosec G115 -- in 0..p-1
	}
	return pt, nil
}

func candidateIDs() []string {
	var out []string
	for _, c := range Candidates {
		out = append(out, c.ID)
	}
	return out
}
