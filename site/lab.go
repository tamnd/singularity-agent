package site

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/singularity-agent/lab"
)

// LabURL is the address of the page of a run of the lab.
func LabURL(id string) string { return "/lab/" + id + "/" }

// LabRun is the data of a run of the lab, for the list of runs and for the page of the run.
type LabRun struct {
	Rec      *lab.Record
	URL      string
	Node     NodeLink
	Nodes    []NodeLink
	Cases    []lab.CaseOut
	Cands    *lab.CandOut
	Fixed    *lab.FixedOut
	Files    []LabFile
	Wall     string
	SpecURL  string
	ToolURL  string
	Examples string // the address of the file with the examples, or ""
	Changes  []ChangeRow

	dir string // the directory of the run in the content repository
	md  string // summary.md
}

// ChangeRow is the histogram of the change d' - d of the residual order in one case, in short form.
type ChangeRow struct {
	P, N, Q    int
	Down, Zero int
	Up         string // each increase and its count, for example "+1: 13, +3: 1"
}

func changeRow(c lab.CaseOut) ChangeRow {
	row := ChangeRow{P: c.P, N: c.N, Q: c.Q}
	var up []string
	for _, x := range c.Change {
		switch {
		case x.Value < 0:
			row.Down += x.Count
		case x.Value == 0:
			row.Zero += x.Count
		default:
			up = append(up, fmt.Sprintf("+%d: %d", x.Value, x.Count))
		}
	}
	row.Up = strings.Join(up, ", ")
	if row.Up == "" {
		row.Up = "none"
	}
	return row
}

// LabFile is a file of a run that the site copies.
type LabFile struct {
	Name, URL string
	Size      string
}

// labDefault is the text of the lab page when the content has no site/lab.md.
const labDefault = "Each run of the lab answers one question with a computation. The spec is in `lab/specs/` and the outputs are in `lab/runs/` of the content repository. A run is data from examples. It does not prove a statement.\n"

// labRuns reads lab/runs of the content repository.
func (b *builder) labRuns() ([]LabRun, error) {
	runs, err := lab.LoadRuns(filepath.Join(b.opt.Content, "lab", "runs"))
	if err != nil {
		return nil, fmt.Errorf("site: %w", err)
	}
	var out []LabRun
	for _, r := range runs {
		rec := r.Record
		lr := LabRun{
			Rec: rec, URL: LabURL(rec.ID), Node: b.nodeLink(rec.Node), Nodes: b.nodeLinks(rec.Nodes),
			Cands: r.Candidates, Fixed: r.Fixed, Wall: wallText(rec.WallSeconds),
			SpecURL: ContentRepo + "/blob/main/" + rec.Spec, dir: r.Dir, md: r.Summary,
		}
		if r.Histogram != nil {
			lr.Cases = r.Histogram.Cases
			for _, c := range lr.Cases {
				lr.Changes = append(lr.Changes, changeRow(c))
			}
		}
		if rec.Sing.Commit != "" {
			lr.ToolURL = AgentRepo + "/commit/" + rec.Sing.Commit
		}
		names := make([]string, 0, len(rec.Files)+1)
		for n := range rec.Files {
			names = append(names, n)
		}
		names = append(names, "record.json")
		sort.Strings(names)
		for _, n := range names {
			info, err := os.Stat(filepath.Join(r.Dir, n))
			if err != nil {
				return nil, fmt.Errorf("site: lab run %s: %w", rec.ID, err)
			}
			f := LabFile{Name: n, URL: lr.URL + n, Size: sizeText(info.Size())}
			lr.Files = append(lr.Files, f)
			if n == "examples.json" || n == "examples.json.gz" {
				lr.Examples = f.URL
			}
		}
		out = append(out, lr)
	}
	return out, nil
}

func wallText(s float64) string {
	switch {
	case s < 60:
		return fmt.Sprintf("%.1f s", s)
	case s < 3600:
		return fmt.Sprintf("%.1f min", s/60)
	}
	return fmt.Sprintf("%.2f h", s/3600)
}

func sizeText(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

// labViews adds the page of each run. The list of runs is the page lab/, from site/lab.md or from labDefault.
func (b *builder) labViews(runs []LabRun) {
	for i := range runs {
		r := runs[i]
		summary := r.summary()
		b.add(&view{
			Path: strings.TrimPrefix(r.URL, "/"), Title: r.Rec.ID, Section: "lab", tmpl: "labrun", Lab: &r,
			Description: r.Rec.Experiment + ". " + r.Rec.Question,
			SourceURL:   ContentRepo + "/tree/main/lab/runs/" + r.Rec.ID,
			Trail:       append(home(), Crumb{Text: "Lab", URL: "/lab/"}, Crumb{Text: r.Rec.ID, Current: true}),
			markdown:    summary,
		})
	}
}

// summary returns summary.md of the run without its title, which the page shows.
func (r *LabRun) summary() string {
	s := r.md
	if strings.HasPrefix(s, "# ") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
	}
	return strings.TrimLeft(s, "\n")
}

// labCopy writes the files of each run next to its page.
func (b *builder) labCopy(w *writer, runs []LabRun) {
	for _, r := range runs {
		for _, f := range r.Files {
			data, err := os.ReadFile(filepath.Join(r.dir, f.Name)) // #nosec G304 -- a file of the content repository
			if err != nil {
				w.err = err
				return
			}
			w.file(strings.TrimPrefix(f.URL, "/"), data)
		}
	}
}
