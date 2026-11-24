package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/tamnd/singularity-agent/lab"
)

func runLab(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: lab needs a subcommand: run, list or step", errUsage)
	}
	switch args[0] {
	case "run":
		return labRun(args[1:], stdout, stderr)
	case "list":
		return labList(args[1:], stdout, stderr)
	case "step":
		return labStep(args[1:], stdout, stderr)
	}
	return fmt.Errorf("%w: unknown subcommand %q; use run, list or step", errUsage, args[0])
}

// toolInfo returns the version and the commit of this binary.
func toolInfo() lab.ToolInfo {
	t := lab.ToolInfo{Version: Version, Go: runtime.Version()}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				t.Commit = s.Value
			case "vcs.modified":
				t.Modified = s.Value == "true"
			}
		}
	}
	return t
}

func labRun(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing lab run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runs := fs.String("runs", "", "the directory of the runs (default: lab/runs next to the lab/specs of the spec)")
	threads := fs.Int("threads", min(4, runtime.NumCPU()), "the number of threads; the outputs do not depend on it")
	walks := fs.Int("walks", 0, "when positive, run at most this number of walks in each case (a short test run)")
	force := fs.Bool("force", false, "replace a run with the same id")
	host := fs.String("host", "", "the name of the host in the record (default: the host name)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: sing lab run [flags] <spec.yaml>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("%w: lab run needs one spec file", errUsage)
	}
	spec, err := lab.LoadSpec(fs.Arg(0))
	if err != nil {
		return err
	}
	if *runs == "" {
		if *runs = lab.RunsDir(fs.Arg(0)); *runs == "" {
			return fmt.Errorf("%w: the spec is not in lab/specs, so lab run needs -runs", errUsage)
		}
	}
	if *threads < 1 {
		return fmt.Errorf("%w: -threads must be positive", errUsage)
	}
	if *host == "" {
		*host, _ = os.Hostname()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rec, err := lab.Run(ctx, spec, lab.Options{Runs: *runs, Threads: *threads, Walks: *walks, Force: *force, Host: *host, Tool: toolInfo()})
	if err != nil {
		return err
	}
	c := rec.Counts
	fmt.Fprintf(stdout, "wrote %s/%s in %.1f s: %d cases, %d walks, %d steps, %d kangaroo points, %d steps over the bound of Moh\n", *runs, rec.ID, rec.WallSeconds, c.Cases, c.Walks, c.Steps, c.Kangaroo, c.MohViolations)
	return nil
}

func labList(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing lab list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	content := fs.String("content", ".", "the root of the content repository")
	runs := fs.String("runs", "", "the directory of the runs (default: lab/runs of the content repository)")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *runs == "" {
		*runs = *content + "/lab/runs"
	}
	rs, err := lab.LoadRuns(*runs)
	if err != nil {
		return err
	}
	for _, r := range rs {
		rec := r.Record
		c := rec.Counts
		fmt.Fprintf(stdout, "%s  %-3s  %s  walks %d, steps %d, kangaroo %d, over Moh %d, %.0f s on %s\n", rec.ID, rec.Experiment, rec.Node, c.Walks, c.Steps, c.Kangaroo, c.MohViolations, rec.WallSeconds, rec.Host)
	}
	fmt.Fprintf(stdout, "%d runs in %s\n", len(rs), *runs)
	return nil
}

// labStep does one point blowup and prints the result, to check an example of a run.
func labStep(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing lab step", flag.ContinueOnError)
	fs.SetOutput(stderr)
	p := fs.Int("p", 2, "the characteristic")
	q := fs.Int("q", 0, "the power q = p^e of z (default p)")
	vars := fs.String("vars", "", "the variables, for example x,y,w or x1,x2,x3")
	f := fs.String("f", "", "the polynomial F of f = z^q + F")
	divisor := fs.String("divisor", "", "the variables of the divisor E, for example x,w; empty for no divisor")
	chart := fs.String("chart", "", "the chart variable x_j; empty to print only the invariants of F")
	var trans multiFlag
	fs.Var(&trans, "t", "a translation VAR=VALUE; give -t again for more than one")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *q == 0 {
		*q = *p
	}
	if *vars == "" || *f == "" {
		return fmt.Errorf("%w: lab step needs -vars and -f", errUsage)
	}
	fx := &lab.Fixed{P: *p, Q: *q, Vars: splitList(*vars), F: *f, Divisor: splitList(*divisor), Reduce: false}
	if *chart != "" {
		st := lab.FixedStep{Chart: *chart, Translate: map[string]int{}}
		for _, t := range trans {
			name, val, ok := strings.Cut(t, "=")
			var v int
			if _, err := fmt.Sscan(val, &v); !ok || err != nil {
				return fmt.Errorf("%w: the translation %q is not VAR=VALUE", errUsage, t)
			}
			st.Translate[strings.TrimSpace(name)] = v
		}
		fx.Steps = []lab.FixedStep{st}
	}
	rows, err := lab.Replay(fx)
	if err != nil {
		return err
	}
	for i, r := range rows {
		if i == 0 {
			fmt.Fprintln(stdout, "start:")
		} else {
			fmt.Fprintf(stdout, "after the %s-chart%s:\n", r.Chart, transText(r.Translate, fx.Vars))
		}
		fmt.Fprintf(stdout, "  F = %s\n  terms %d, divisor %v, order %d\n  residual order %d, terms of in(G) %d, Moh r %d, r of in(G) %d\n", r.F, r.Terms, r.Divisor, r.Order, r.State.D, r.State.N, r.State.RF, r.State.RG)
	}
	return nil
}

func transText(t map[string]int, vars []string) string {
	var b strings.Builder
	for _, v := range vars {
		if x, ok := t[v]; ok {
			fmt.Fprintf(&b, ", %s -> %s+%d", v, v, x)
		}
	}
	return b.String()
}

func splitList(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(s string) error {
	*m = append(*m, s)
	return nil
}
