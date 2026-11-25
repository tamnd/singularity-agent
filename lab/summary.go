package lab

import (
	"fmt"
	"strings"
)

func short(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// plural writes n and the noun, with an s when n is not 1.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func header(b *strings.Builder, s *Spec, rec *Record) {
	fmt.Fprintf(b, "# %s\n\n", s.Name)
	fmt.Fprintf(b, "%s\n\n", s.Question)
	fmt.Fprintf(b, "- Experiment: %s. Node: %s.\n", s.Experiment, s.Node)
	tool := "sing-lab, in sing " + rec.Sing.Version
	if rec.Sing.Commit != "" {
		tool += ", commit " + short(rec.Sing.Commit, 12)
		if rec.Sing.Modified {
			tool += " with local changes"
		}
	}
	fmt.Fprintf(b, "- Tool: %s.\n", tool)
	fmt.Fprintf(b, "- Spec: `%s`, sha256 %s.\n", rec.Spec, short(rec.SpecSHA256, 12))
	if rec.WalksFrom != nil {
		fmt.Fprintf(b, "- Walks: the walks of `%s`, sha256 %s. The run makes the same walks again from the same seed.\n", rec.WalksFrom.Spec, short(rec.WalksFrom.SHA256, 12))
		if len(s.Inputs.Select) > 0 {
			var parts []string
			for _, sel := range s.Inputs.Select {
				parts = append(parts, fmt.Sprintf("n = %d and q = %d", sel.N, sel.Q))
			}
			fmt.Fprintf(b, "- Cases: only the cases of that spec with %s.\n", strings.Join(parts, ", or "))
		}
	}
	if rec.Seed != 0 {
		fmt.Fprintf(b, "- Seed: %d.\n", rec.Seed)
	}
	if rec.WalksLimit > 0 {
		fmt.Fprintf(b, "- This is a short test run: each case has at most %d walks.\n", rec.WalksLimit)
	}
	b.WriteString("- The host and the wall time are in `record.json`.\n\n")
}

// changeText writes a histogram of changes, for example "-2: 10, -1: 31, 0: 400, +1: 7".
func changeText(bs []Bin) string {
	var parts []string
	for _, x := range bs {
		v := fmt.Sprint(x.Value)
		if x.Value > 0 {
			v = "+" + v
		}
		parts = append(parts, fmt.Sprintf("%s: %d", v, x.Count))
	}
	return strings.Join(parts, ", ")
}

const caveats = `## What the data does not show

- The lab computes the residual order in the given coordinates x1, ..., xn, after it removes the q-th powers from F (the cleaning of z). The bound of Moh is about well chosen coordinates. So a step over the bound in this data is not a counterexample to the theorem of Moh. It is an example to check in other coordinates.
- The walks use only the points over F_p of the exceptional divisor, in the n charts. The same point can be in more than one chart.
- A walk stops at a cap of the spec. So the data has fewer large polynomials than the family.
- This is data from examples. It does not prove a statement.
`

func summaryE1(s *Spec, rec *Record, h Histogram) string {
	var b strings.Builder
	header(&b, s, rec)
	fmt.Fprintf(&b, "The run did %d random walks with %d point blowups in %d cases. At each step the order of f stays q.\n\n", rec.Counts.Walks, rec.Counts.Steps, rec.Counts.Cases)
	b.WriteString("## Results\n\n")
	b.WriteString("A kangaroo point is a step at which the residual order goes up. The bound of Moh for one step is p^(e-1). The stability bound is d0 + p^(e-1), with d0 the residual order at the start of the walk.\n\n")
	b.WriteString("| p | n | q | walks | steps | kangaroo points | largest increase | p^(e-1) | steps over p^(e-1) | walks over d0 + p^(e-1) |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range h.Cases {
		fmt.Fprintf(&b, "| %d | %d | %d | %d | %d | %d | %d | %d | %d | %d |\n", c.P, c.N, c.Q, c.Walks, c.Steps, c.Kangaroo, c.MaxIncrease, c.MohBound, c.Moh, c.Stability)
	}
	shifted := 0
	for _, c := range h.Cases {
		shifted += c.Shifted
	}
	if shifted > 0 {
		fmt.Fprintf(&b, "\nIn %d steps the transform F' had a constant term c. The point of the strict transform is then at z = c^(1/q), and the cleaning removes c with the change z -> z + c^(1/q). histogram.json gives the number of these steps in each case as z_shift_steps.\n", shifted)
	}
	b.WriteString("\n## The change of the residual order\n\nEach entry is a change d' - d and the number of steps with that change.\n\n| p | n | q | changes |\n|---|---|---|---|\n")
	for _, c := range h.Cases {
		fmt.Fprintf(&b, "| %d | %d | %d | %s |\n", c.P, c.N, c.Q, changeText(c.Change))
	}
	b.WriteString("\n## How the walks stop\n\n| p | n | q | depth | the order drops at each point | residual order 0 | cap |\n|---|---|---|---|---|---|---|\n")
	for _, c := range h.Cases {
		fmt.Fprintf(&b, "| %d | %d | %d | %d | %d | %d | %d |\n", c.P, c.N, c.Q, c.Stops[StopDepth], c.Stops[StopOrderDrops], c.Stops[StopResidualZero], c.Stops[StopCap])
	}
	b.WriteString("\n## The files\n\n- `histogram.json`: the counts of each case.\n- `examples.json`: the first examples of each increase in each case, and the first walks over the stability bound. Each example has the polynomial, the divisor, the chart and the translations, so you can check it again with `sing lab step`.\n\n")
	if s.Notes != "" {
		fmt.Fprintf(&b, "## Notes\n\n%s\n\n", strings.TrimSpace(s.Notes))
	}
	b.WriteString(caveats)
	return b.String()
}

func summaryE2(s *Spec, rec *Record, h Histogram, co CandOut) string {
	var b strings.Builder
	header(&b, s, rec)
	fmt.Fprintf(&b, "The run tests %d candidates on %d steps of %d walks in %d cases. A candidate fails at a step when its value after the blowup is larger than its value before.\n\n", len(co.Candidates), rec.Counts.Steps, rec.Counts.Walks, rec.Counts.Cases)
	b.WriteString("## The candidates\n\n")
	for _, c := range co.Candidates {
		fmt.Fprintf(&b, "- `%s`: %s\n", c.ID, c.Definition)
	}
	b.WriteString("\nIn these definitions, F is clean, E is the divisor, F = prod x_i^(r_i) G along E, d = ord G is the residual order, and in(.) is the initial form.\n\n")
	b.WriteString("## Results\n\n| candidate | steps | failures | the first failure |\n|---|---|---|---|\n")
	for i, t := range co.Totals {
		first := "none"
		if t.FirstCase != "" {
			for _, cc := range co.Cases {
				r := cc.Results[i]
				if r.First != nil {
					first = fmt.Sprintf("%s, walk %d, step %d: %s to %s", caseName(Case{P: cc.P, N: cc.N, Q: cc.Q}), r.First.Walk, r.First.Step, r.First.ValueBefore, r.First.ValueAfter)
					break
				}
			}
		}
		fmt.Fprintf(&b, "| `%s` | %d | %d | %s |\n", t.ID, t.Steps, t.Fail, first)
	}
	b.WriteString("\n## Failures in each case\n\n| p | n | q | steps |")
	for _, c := range co.Candidates {
		fmt.Fprintf(&b, " `%s` |", c.ID)
	}
	b.WriteString("\n|---|---|---|---|" + strings.Repeat("---|", len(co.Candidates)) + "\n")
	for _, cc := range co.Cases {
		fmt.Fprintf(&b, "| %d | %d | %d | %d |", cc.P, cc.N, cc.Q, cc.Steps)
		for _, r := range cc.Results {
			fmt.Fprintf(&b, " %d |", r.Fail)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## The files\n\n- `candidates.json`: the counts of each candidate in each case, with the first failure. A failure has the polynomial, the divisor, the chart and the translations, so you can check it again with `sing lab step`.\n- `histogram.json`: the counts of the walks. It is the same file as in the run of the walks, because the walks are the same.\n\n")
	if s.Notes != "" {
		fmt.Fprintf(&b, "## Notes\n\n%s\n\n", strings.TrimSpace(s.Notes))
	}
	b.WriteString("## What the data does not show\n\n- A candidate with no failure on these walks is not proved to go down. The walks are a small sample, in the given coordinates, at the points over F_p only.\n- A failure is in the given coordinates. It shows that the candidate in this form does not work. It does not show that no form of the idea works.\n- This is data from examples. It does not prove a statement.\n")
	return b.String()
}

func summaryFixed(s *Spec, rec *Record, out FixedOut, en *Engine) string {
	var b strings.Builder
	header(&b, s, rec)
	if out.Source != "" {
		fmt.Fprintf(&b, "Source: %s\n\n", out.Source)
	}
	fmt.Fprintf(&b, "The run starts from one given equation z^%d + F over F_%d and does %s. At each step the order of f stays q.\n\n", out.Q, out.P, plural(len(out.Steps)-1, "given point blowup"))
	b.WriteString("| step | label | chart | residual order | the source | terms of F |\n|---|---|---|---|---|---|\n")
	for _, r := range out.Steps {
		chart := "start"
		if r.Chart != "" {
			chart = r.Chart + "-chart"
			for _, v := range out.Vars {
				if t, ok := r.Translate[v]; ok {
					chart += fmt.Sprintf(", %s -> %s+%d", v, v, t)
				}
			}
		}
		src := ""
		if r.Expect != nil {
			src = fmt.Sprint(*r.Expect)
			if r.Match != nil && !*r.Match {
				src += " (no match)"
			}
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %d | %s | %d |\n", r.Index, r.Label, chart, r.State.D, src, r.Terms)
	}
	b.WriteString("\n")
	if out.Match {
		b.WriteString("The engine gives the value of the source at each step where the source gives a value.\n\n")
	} else {
		b.WriteString("The engine does not give the value of the source at each step where the source gives a value. See the rows with no match.\n\n")
	}
	fmt.Fprintf(&b, "The bound of Moh for one step is p^(e-1) = %d. The sequence has %s where the residual order goes up, and %d of them go up by more than the bound.\n\n", en.MohBound(), plural(rec.Counts.Kangaroo, "step"), rec.Counts.MohViolations)
	b.WriteString("`steps.json` has the polynomial, the divisor and the invariants after each step.\n\n")
	if s.Notes != "" {
		fmt.Fprintf(&b, "## Notes\n\n%s\n\n", strings.TrimSpace(s.Notes))
	}
	b.WriteString("This is a computation on one example. It does not prove a statement.\n")
	return b.String()
}

// pct writes a/b as a percentage with one decimal, or "-" when b is 0.
func pct(a, b int) string {
	if b == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(a)/float64(b))
}

const caveatsE4 = `## What the data does not show

- This is data from examples. It does not prove a statement.
- The walks use only point centres, and only the points over F_p of the exceptional divisor, in the n charts. [CP19, Theorem 3.13] is about all points over x and all permissible centres, also curves and surfaces.
- The engine does not check condition (E) of [CP19, Definition 2.32]: that the points of multiplicity p of the hypersurface lie over E. Theorem 3.13 assumes it. A failure of cp-iota or cp-eps-jump at a point where (E) fails is not a counterexample to the theorem.
- The values of omega and epsilon do not depend on the well adapted coordinates [CP19, Remark 2.69, and the handbook, Volume 6, Proposition 3.2.4(5)]. So, unlike the bound of Moh in E1, a failure is not only a matter of coordinates. Each failure must be checked by hand, with condition (E), before it counts for anything.
- The engine computes kappa only as 1 or >= 2. The values 2, 3 and 4 of kappa for n = 3 [CP19, Definition 5.1] and the invariants of the projection need a well chosen coordinate, which the engine cannot choose. So the run does not test the decrease of the Projection Theorem [CP19, Theorem 5.5].
- A candidate with no failure on these walks is not proved to go down or to stay. The walks are a small sample, and a walk stops at a depth or a cap.
`

func summaryE4(s *Spec, rec *Record, h Histogram, co CandOut) string {
	var b strings.Builder
	header(&b, s, rec)
	fmt.Fprintf(&b, "The run tests %d invariants of Cossart and Piltant [CP19] on %d steps of %d walks in %d cases, with q = p. A candidate fails at a step when its value after the blowup is larger than its value before, or, for a bound, when the step breaks the bound.\n\n", len(co.Candidates), rec.Counts.Steps, rec.Counts.Walks, rec.Counts.Cases)
	b.WriteString("## The invariants\n\nThe equation is h = Z^p + F with F clean, so the coordinates are well adapted [CP19, Definition 2.24]. E is the divisor, F = prod_{j in E} x_j^(r_j) G, and in(F) is the initial form of F. The engine computes these invariants at each point of a walk.\n\n")
	b.WriteString("- m(x) = p, the multiplicity. A walk keeps the order p, so m = p at each point.\n- delta(x) = ord F / p [CP19, Definition 2.11].\n- epsilon(x) = ord F - sum r_j = ord G, the residual order d [CP19, Definition 2.25].\n- omega(x) = epsilon(x) - 1 if dF/dU_j != 0 for some x_j not in E, with F = in(F), and omega(x) = epsilon(x) otherwise [CP19, (1.11) and Definition 2.68].\n- kappa(x) = 1 if omega(x) = 0, and kappa(x) >= 2 otherwise [CP19, Definition 2.68]. The engine does not split the values >= 2.\n\n")
	b.WriteString("## The candidates\n\n")
	for _, c := range co.Candidates {
		fmt.Fprintf(&b, "- `%s`: %s", c.ID, c.Definition)
		if c.Scope != "" {
			fmt.Fprintf(&b, " The scope is %s.", c.Scope)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Results\n\n| candidate | steps in the scope | steps out of the scope | failures | the first failure |\n|---|---|---|---|---|\n")
	for i, t := range co.Totals {
		first := "none"
		for _, cc := range co.Cases {
			if r := cc.Results[i]; r.First != nil {
				first = fmt.Sprintf("%s, walk %d, step %d: %s to %s", caseName(Case{P: cc.P, N: cc.N, Q: cc.Q}), r.First.Walk, r.First.Step, r.First.ValueBefore, r.First.ValueAfter)
				break
			}
		}
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %s |\n", t.ID, t.Steps, t.Skip, t.Fail, first)
	}
	b.WriteString("\n## Dimension 3 and dimension 4\n\nThe case n = 3 is a threefold z^p + g(x1, x2, x3), the case of the theorem of Cossart and Piltant. The case n = 4 is its analogue in dimension 4. For each candidate that is a value, the table gives the steps in the scope where the value goes up, stays the same and goes down, and the share of the steps where it goes down.\n\n")
	for i, c := range co.Candidates {
		if c.Bound {
			continue
		}
		fmt.Fprintf(&b, "`%s`:\n\n| p | n | q | steps in the scope | out of the scope | up | same | down | share down |\n|---|---|---|---|---|---|---|---|---|\n", c.ID)
		for _, cc := range co.Cases {
			r := cc.Results[i]
			fmt.Fprintf(&b, "| %d | %d | %d | %d | %d | %d | %d | %d | %s |\n", cc.P, cc.N, cc.Q, r.Steps, r.Skip, r.Fail, r.Equal, r.Down, pct(r.Down, r.Steps))
		}
		b.WriteString("\n")
	}
	b.WriteString("The change of epsilon, the residual order, in each case (each entry is a change and the number of steps):\n\n| p | n | q | walks | steps | kangaroo points | changes |\n|---|---|---|---|---|---|---|\n")
	for _, c := range h.Cases {
		fmt.Fprintf(&b, "| %d | %d | %d | %d | %d | %d | %s |\n", c.P, c.N, c.Q, c.Walks, c.Steps, c.Kangaroo, changeText(c.Change))
	}
	b.WriteString("\n## The files\n\n- `candidates.json`: the counts of each candidate in each case, with the first failure. A failure has the polynomial, the divisor, the chart and the translations, and the invariants before and after the step (the field `cp`), so you can check it again with `sing lab step`.\n- `histogram.json`: the counts of the walks. They are the counts of the same cases in the run of the walks, because the walks are the same.\n\n")
	if s.Notes != "" {
		fmt.Fprintf(&b, "## Notes\n\n%s\n\n", strings.TrimSpace(s.Notes))
	}
	b.WriteString(caveatsE4)
	return b.String()
}
