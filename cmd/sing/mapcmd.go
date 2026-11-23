package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/singularity-agent/blueprint"
)

func runCheck(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	content := fs.String("content", ".", "the root of the content repository")
	if err := parse(fs, args); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(*content, "blueprint")); err != nil {
		return fmt.Errorf("%s is not the content repository: %w", *content, err)
	}
	p, err := blueprint.Load(*content)
	if err != nil {
		return err
	}
	probs := p.Check()
	for _, s := range probs {
		fmt.Fprintln(stdout, "problem:", s)
	}
	if len(probs) > 0 {
		return fmt.Errorf("the map has %d problems", len(probs))
	}
	fmt.Fprintf(stdout, "the map is good: %d nodes, %d papers, %d routes\n", len(p.Nodes), len(p.Papers), len(p.Routes))
	return nil
}

func runStatus(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	content := fs.String("content", ".", "the root of the content repository")
	only := fs.String("only", "", "print only the nodes with this status, for example stated")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *only != "" && !contains(blueprint.Statuses, *only) {
		return fmt.Errorf("%w: unknown status %q; use one of %s", errUsage, *only, strings.Join(blueprint.Statuses, ", "))
	}
	p, err := blueprint.Load(*content)
	if err != nil {
		return err
	}
	type row struct {
		Tag    string `json:"tag"`
		Status string `json:"status"`
		Tier   string `json:"tier"`
		Title  string `json:"title"`
		Why    string `json:"why"`
	}
	var rows []row
	for _, n := range p.Nodes {
		if *only == "" || n.Status == *only {
			rows = append(rows, row{n.Tag, n.Status, n.Tier, n.Title, n.Why})
		}
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"counts": p.Counts(), "nodes": rows})
	}
	for _, r := range rows {
		fmt.Fprintf(stdout, "%s  %-10s  %-3s  %s\n", r.Tag, r.Status, r.Tier, r.Title)
	}
	counts := p.Counts()
	var parts []string
	for _, s := range blueprint.Statuses {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[s], s))
		}
	}
	fmt.Fprintf(stdout, "%d nodes: %s\n", len(p.Nodes), strings.Join(parts, ", "))
	return nil
}

// runIndex reads the record that the export script of singularity-lean writes, checks it and writes it to records/lean.json of the content repository. A record with a declaration that uses sorry is accepted, because the status rules read it as stated.
func runIndex(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sing index", flag.ContinueOnError)
	fs.SetOutput(stderr)
	content := fs.String("content", ".", "the root of the content repository")
	lean := fs.String("lean", "", "the record of the Lean build, from lake exe export in singularity-lean")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *lean == "" {
		return fmt.Errorf("%w: index needs -lean", errUsage)
	}
	data, err := os.ReadFile(*lean)
	if err != nil {
		return err
	}
	var rec blueprint.LeanRecord
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rec); err != nil {
		return fmt.Errorf("%s: %w", *lean, err)
	}
	if rec.Commit == "" || rec.Toolchain == "" {
		return fmt.Errorf("%s: the record has no commit or no toolchain", *lean)
	}
	if len(rec.Decls) == 0 {
		return fmt.Errorf("%s: the record has no declarations", *lean)
	}
	var names []string
	sorry := 0
	for name, d := range rec.Decls {
		names = append(names, name)
		if d.Sorry() {
			sorry++
		}
	}
	sort.Strings(names)
	out, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	dst := filepath.Join(*content, "records", "lean.json")
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(dst, append(out, '\n'), 0o644); err != nil { // #nosec G306 -- a public record
		return err
	}
	fmt.Fprintf(stdout, "wrote %s: %d declarations from commit %.7s, %d with sorry\n", dst, len(names), rec.Commit, sorry)
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
