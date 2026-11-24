// Package site builds singularity.tamnd.com from the content repository tamnd/singularity.
//
// The site has the main page, the pages of site/, the map with one page for each node, the papers with one page for each record, the routes, the runs of the lab, the trust report and a search page. People read the HTML. Agents read the same text as Markdown next to each page, the JSON in api/ and llms.txt. The style is the style of Math 2.0. The build renders the math to MathML with the katex package of math2-agent, so a page shows its math with JavaScript off.
package site

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tamnd/math2-agent/katex"
	"github.com/tamnd/math2-agent/markup"
	"go.yaml.in/yaml/v3"

	"github.com/tamnd/singularity-agent/blueprint"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed all:assets
var assetFS embed.FS

// Repositories of the project.
const (
	ContentRepo  = "https://github.com/tamnd/singularity"
	LeanRepo     = "https://github.com/tamnd/singularity-lean"
	AgentRepo    = "https://github.com/tamnd/singularity-agent"
	HandbookRepo = "https://github.com/tamnd/singularity-handbook"
)

// Options are the inputs of Build.
type Options struct {
	Content string // the root of the content repository
	Out     string // the output directory; Build empties it first
	URL     string // the public address of the site
	Commit  string // the commit of the content repository, or ""
	Date    string // the build date, or ""
	Version string // the version of sing
}

// MenuItem is an entry of the main menu. Question is the question that the section answers.
type MenuItem struct {
	Path     string
	Section  string
	Question string
	Text     string
}

// Menu is the main menu.
var Menu = []MenuItem{
	{Path: "problem/", Section: "problem", Question: "What is the open problem?", Text: "Problem"},
	{Path: "map/", Section: "map", Question: "What is stated and what is proved?", Text: "Map"},
	{Path: "routes/", Section: "routes", Question: "How could the open case be attacked?", Text: "Routes"},
	{Path: "paper/", Section: "paper", Question: "Which papers does the project use?", Text: "Papers"},
	{Path: "lab/", Section: "lab", Question: "What do the computations show?", Text: "Lab"},
	{Path: "learn/", Section: "learn", Question: "What do I need to know first?", Text: "Learn"},
	{Path: "trust/", Section: "trust", Question: "What is checked?", Text: "Trust"},
}

// Info is the data of the site that each page shows.
type Info struct {
	Name        string
	URL         string
	Menu        []MenuItem
	Trust       Trust
	Counts      []Count
	ContentRepo string
	LeanRepo    string
	AgentRepo   string
	HandbookRep string
	LeanCommit  string
	Version     string
	Commit      string
	Built       string
}

// Canonical returns the public address of a page.
func (i *Info) Canonical(p string) string { return strings.TrimSuffix(i.URL, "/") + "/" + p }

// Count is the number of nodes with one status.
type Count struct {
	Status string
	N      int
}

// Crumb is a step of the trail at the top of a page.
type Crumb struct {
	Text    string
	URL     string
	Current bool
}

// Door is an entry point on the home page.
type Door struct {
	Title string `yaml:"title"`
	Path  string `yaml:"path"`
	Text  string `yaml:"text"`
}

// Front is the front matter of a page of site/.
type Front struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Lead        string `yaml:"lead"`
	Eyebrow     string `yaml:"eyebrow"`
	Doors       []Door `yaml:"doors"`
}

// Trust is records/trust.json.
type Trust struct {
	Updated string       `json:"updated,omitempty"`
	Checks  []TrustCheck `json:"checks"`
}

// TrustCheck is one row of the trust report, spec 14.
type TrustCheck struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	State    string `json:"state"` // yes, no, open or "not run"
	Note     string `json:"note,omitempty"`
}

// Class returns the CSS class of the state.
func (c TrustCheck) Class() string {
	switch c.State {
	case "yes", "no", "open":
		return c.State
	}
	return ""
}

// NodeLink is a link to a node, for lists.
type NodeLink struct {
	Tag, Title, Status, Kind, URL string
}

// TierGroup is the nodes or papers of one tier.
type TierGroup struct {
	Tier   string
	Name   string
	Nodes  []NodeLink
	Papers []*blueprint.Paper
}

// NodeData is the data of a node page.
type NodeData struct {
	Node       *blueprint.Node
	Statement  template.HTML
	Notes      template.HTML
	Uses       []NodeLink
	UsedBy     []NodeLink
	Hypotheses []NodeLink
	Sources    []SourceLink
	Lean       []LeanLink
	Routes     []*blueprint.Route
}

// SourceLink is a source of a node.
type SourceLink struct {
	Key, Title, Loc, URL string
}

// LeanLink is a Lean declaration of a node.
type LeanLink struct {
	Role, Name, URL, Axioms string
	Found                   bool
}

// PaperData is the data of a paper page.
type PaperData struct {
	Paper *blueprint.Paper
	Why   template.HTML
	Notes template.HTML
	Nodes []NodeLink
	Links []SourceLink
}

// RouteData is the data of a route page and of the list of routes.
type RouteData struct {
	Route       *blueprint.Route
	Idea        template.HTML
	Obstruction template.HTML
	Known       template.HTML
	First       template.HTML
	Nodes       []NodeLink
	URL         string
}

// view is the data of one page for the templates.
type view struct {
	Site        *Info
	Path        string
	Title       string
	Description string
	Lead        string
	LeadHTML    template.HTML
	Section     string
	Trail       []Crumb
	Home        *Front
	Groups      []TierGroup
	Node        *NodeData
	Paper       *PaperData
	Route       *RouteData
	Routes      []RouteData
	Lab         *LabRun
	LabRuns     []LabRun
	Body        template.HTML
	Content     template.HTML
	SourceURL   string
	MarkdownURL string

	tmpl     string
	markdown string
}

type builder struct {
	opt   Options
	info  *Info
	proj  *blueprint.Project
	tmpl  *template.Template
	md    *markup.Markdown
	tex   *katex.Renderer
	views []*view
	runs  []LabRun
}

// Result tells what Build wrote.
type Result struct {
	Pages int
	Files int
	Bytes int64
}

// Build builds the site in opt.Out.
func Build(ctx context.Context, opt Options) (*Result, error) {
	if opt.Content == "" || opt.Out == "" {
		return nil, errors.New("site: the content directory and the output directory are necessary")
	}
	if opt.URL == "" {
		opt.URL = "https://singularity.tamnd.com"
	}
	if opt.Version == "" {
		opt.Version = "dev"
	}
	proj, err := blueprint.Load(opt.Content)
	if err != nil {
		return nil, fmt.Errorf("site: %w", err)
	}
	if probs := proj.Check(); len(probs) > 0 {
		return nil, fmt.Errorf("site: the map has %d problems; run sing check. The first: %s", len(probs), probs[0])
	}
	trust, err := readTrust(opt.Content)
	if err != nil {
		return nil, fmt.Errorf("site: %w", err)
	}
	b := &builder{opt: opt, proj: proj}
	b.info = &Info{
		Name: "singularity", URL: strings.TrimSuffix(opt.URL, "/"), Menu: Menu, Trust: trust,
		ContentRepo: ContentRepo, LeanRepo: LeanRepo, AgentRepo: AgentRepo, HandbookRep: HandbookRepo,
		Version: opt.Version, Commit: opt.Commit, Built: opt.Date,
	}
	if proj.Lean != nil {
		b.info.LeanCommit = proj.Lean.Commit
	}
	counts := proj.Counts()
	for _, s := range blueprint.Statuses {
		if counts[s] > 0 {
			b.info.Counts = append(b.info.Counts, Count{Status: s, N: counts[s]})
		}
	}
	b.tmpl, err = template.New("").Funcs(template.FuncMap{
		"url":       func(p string) string { return "/" + p },
		"mark":      markClass,
		"short":     func(s string) string { return shortText(s, 7) },
		"tierName":  func(t string) string { return blueprint.TierNames[t] },
		"join":      strings.Join,
		"percent":   func(w float64) string { return fmt.Sprintf("%.0f%%", w*100) },
		"nodeCount": func() int { return len(proj.Nodes) },
		"paperURL":  PaperURL,
		"routeURL":  RouteURL,
		"deref":     func(b *bool) bool { return b != nil && *b },
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("site: templates: %w", err)
	}
	if b.tex, err = katex.New(macros); err != nil {
		return nil, fmt.Errorf("site: katex: %w", err)
	}
	b.md = markup.New(b.math)
	if err := b.collect(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return b.write()
}

// macros are the TeX macros of the site.
var macros = map[string]string{
	`\N`:    `\mathbb{N}`,
	`\Z`:    `\mathbb{Z}`,
	`\Q`:    `\mathbb{Q}`,
	`\F`:    `\mathbb{F}`,
	`\ord`:  `\operatorname{ord}`,
	`\Spec`: `\operatorname{Spec}`,
	`\Reg`:  `\operatorname{Reg}`,
}

func shortText(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// markClass is the CSS class of a status mark.
func markClass(status string) string {
	switch status {
	case blueprint.Proved, blueprint.Defined:
		return "yes"
	case blueprint.Refuted:
		return "no"
	case blueprint.Open, blueprint.ProvedMod:
		return "open"
	}
	return ""
}

// math returns the MathML of a formula. A formula that KaTeX cannot parse shows as its TeX source, with the error as the title.
func (b *builder) math(tex string, display bool) string {
	out, err := b.tex.Render(tex, display)
	if err != nil {
		return `<code class="tex-error" title="` + html.EscapeString(err.Error()) + `">` + html.EscapeString(tex) + `</code>`
	}
	if m := katex.MathML(out); m != "" {
		if display {
			return `<span class="katex-display"><span class="katex">` + m + `</span></span>`
		}
		return `<span class="katex">` + m + `</span>`
	}
	return out
}

// link resolves a link of the Markdown. node:TAG goes to a node page, paper:KEY to a paper page and lean:Path/File.lean#L4 to singularity-lean.
func (b *builder) link(dest string) (markup.Target, bool) {
	switch {
	case strings.HasPrefix(dest, "node:"):
		tag := strings.TrimPrefix(dest, "node:")
		if n := b.proj.Node(tag); n != nil {
			return markup.Target{Dest: NodeURL(tag), Title: n.Title}, true
		}
		return markup.Target{Dest: "/map/"}, false
	case strings.HasPrefix(dest, "paper:"):
		key := strings.TrimPrefix(dest, "paper:")
		if p := b.proj.Paper(key); p != nil {
			return markup.Target{Dest: PaperURL(key), Title: p.Title}, true
		}
		return markup.Target{Dest: "/paper/"}, false
	case strings.HasPrefix(dest, "lean:"):
		return markup.Target{Dest: b.leanURL(strings.TrimPrefix(dest, "lean:")), Class: "lean"}, true
	}
	return markup.Target{Dest: dest}, true
}

func (b *builder) leanURL(file string) string {
	ref := "main"
	if b.info.LeanCommit != "" {
		ref = b.info.LeanCommit
	}
	return LeanRepo + "/blob/" + ref + "/" + strings.TrimPrefix(file, "/")
}

func (b *builder) render(src string) (template.HTML, error) {
	out, err := b.md.Render(src, b.link)
	if err != nil {
		return "", err
	}
	return template.HTML(out), nil // #nosec G203 -- the content repository is reviewed
}

func (b *builder) inline(src string) (template.HTML, error) {
	h, err := b.render(src)
	return inline(h), err
}

// NodeURL is the address of a node page.
func NodeURL(tag string) string { return "/node/" + tag + "/" }

// PaperURL is the address of a paper page.
func PaperURL(key string) string { return "/paper/" + strings.ToLower(key) + "/" }

// RouteURL is the address of a route page.
func RouteURL(id string) string { return "/routes/" + strings.ToLower(id) + "/" }

func (b *builder) nodeLink(tag string) NodeLink {
	n := b.proj.Node(tag)
	if n == nil {
		return NodeLink{Tag: tag, Title: tag, URL: "/map/"}
	}
	return NodeLink{Tag: tag, Title: n.Title, Status: n.Status, Kind: n.Kind, URL: NodeURL(tag)}
}

func (b *builder) nodeLinks(tags []string) []NodeLink {
	var out []NodeLink
	for _, t := range tags {
		out = append(out, b.nodeLink(t))
	}
	return out
}

func home() []Crumb { return []Crumb{{Text: "Home", URL: "/"}} }

// collect reads the content and makes the views.
func (b *builder) collect() error {
	pages, err := sitePages(b.opt.Content)
	if err != nil {
		return fmt.Errorf("site: %w", err)
	}
	if b.runs, err = b.labRuns(); err != nil {
		return err
	}
	hasLab := false
	for _, rel := range pages {
		front, body, err := readDoc(b.opt.Content, rel)
		if err != nil {
			return fmt.Errorf("site: %w", err)
		}
		name := strings.TrimSuffix(path.Base(rel), ".md")
		v := &view{Title: front.Title, Description: front.Description, Lead: front.Lead, markdown: body, SourceURL: ContentRepo + "/blob/main/" + rel}
		switch name {
		case "home":
			v.tmpl = "home"
			v.Home = &front
		case "trust":
			v.Path, v.Section, v.tmpl = "trust/", "trust", "trust"
			v.Trail = append(home(), Crumb{Text: front.Title, Current: true})
		case "map":
			v.Path, v.Section, v.tmpl = "map/", "map", "map"
			v.Trail = append(home(), Crumb{Text: "Map", Current: true})
			v.Groups = b.nodeGroups()
		case "papers":
			v.Path, v.Section, v.tmpl = "paper/", "paper", "papers"
			v.Trail = append(home(), Crumb{Text: "Papers", Current: true})
			v.Groups = b.paperGroups()
		case "routes":
			v.Path, v.Section, v.tmpl = "routes/", "routes", "routes"
			v.Trail = append(home(), Crumb{Text: "Routes", Current: true})
			if v.Routes, err = b.routeData(); err != nil {
				return err
			}
		case "lab":
			v.Path, v.Section, v.tmpl = "lab/", "lab", "lab"
			v.Trail = append(home(), Crumb{Text: "Lab", Current: true})
			v.LabRuns = b.runs
			hasLab = true
		default:
			v.Path, v.Section, v.tmpl = name+"/", name, "page"
			v.Trail = append(home(), Crumb{Text: front.Title, Current: true})
		}
		b.add(v)
	}
	if !hasLab {
		b.add(&view{
			Path: "lab/", Section: "lab", tmpl: "lab", Title: "Lab", LabRuns: b.runs, markdown: labDefault,
			Lead:  "The runs of the computation lab: the question, the counts and the outputs of each run.",
			Trail: append(home(), Crumb{Text: "Lab", Current: true}),
		})
	}
	b.labViews(b.runs)
	if err := b.nodeViews(); err != nil {
		return err
	}
	if err := b.paperViews(); err != nil {
		return err
	}
	if err := b.routeViews(); err != nil {
		return err
	}
	b.add(&view{Path: "search/", Title: "Search", Description: "Search the statements, the papers and the pages of singularity.", Section: "search", tmpl: "search"})
	b.add(&view{Path: "404.html", Title: "Page not found", Description: "This page does not exist.", tmpl: "notfound"})
	return b.unique()
}

func (b *builder) nodeGroups() []TierGroup {
	by := map[string][]NodeLink{}
	for _, n := range b.proj.Nodes {
		by[n.Tier] = append(by[n.Tier], b.nodeLink(n.Tag))
	}
	var out []TierGroup
	for _, t := range append([]string{"S"}, blueprint.Tiers[:len(blueprint.Tiers)-1]...) {
		if len(by[t]) == 0 {
			continue
		}
		sort.SliceStable(by[t], func(i, j int) bool { return by[t][i].Title < by[t][j].Title })
		out = append(out, TierGroup{Tier: t, Name: blueprint.TierNames[t], Nodes: by[t]})
	}
	return out
}

func (b *builder) paperGroups() []TierGroup {
	by := map[string][]*blueprint.Paper{}
	for _, p := range b.proj.Papers {
		by[p.Tier] = append(by[p.Tier], p)
	}
	var out []TierGroup
	for _, t := range blueprint.Tiers {
		if len(by[t]) == 0 {
			continue
		}
		out = append(out, TierGroup{Tier: t, Name: blueprint.TierNames[t], Papers: by[t]})
	}
	return out
}

func (b *builder) nodeViews() error {
	for _, n := range b.proj.Nodes {
		d := &NodeData{Node: n, Uses: b.nodeLinks(n.Uses), UsedBy: b.nodeLinks(n.UsedBy), Hypotheses: b.nodeLinks(n.Hypotheses)}
		var err error
		if d.Statement, err = b.render(n.Statement); err != nil {
			return fmt.Errorf("site: %s: %w", n.File, err)
		}
		if n.Notes != "" {
			if d.Notes, err = b.render(n.Notes); err != nil {
				return fmt.Errorf("site: %s: %w", n.File, err)
			}
		}
		for _, s := range n.Sources {
			p := b.proj.Paper(s.Paper)
			d.Sources = append(d.Sources, SourceLink{Key: s.Paper, Title: p.Title, Loc: s.Loc, URL: PaperURL(s.Paper)})
		}
		for _, l := range []struct{ role, name string }{{"Statement", n.Lean.Statement}, {"Proof", n.Lean.Proof}, {"Negation", n.Lean.Negation}} {
			if l.name == "" {
				continue
			}
			ll := LeanLink{Role: l.role, Name: l.name, URL: LeanRepo}
			if b.proj.Lean != nil {
				if decl, ok := b.proj.Lean.Decls[l.name]; ok {
					ll.Found = true
					if decl.File != "" {
						ll.URL = b.leanURL(decl.File)
						if decl.Line > 0 {
							ll.URL += fmt.Sprintf("#L%d", decl.Line)
						}
					}
					ll.Axioms = strings.Join(decl.Axioms, ", ")
					if ll.Axioms == "" {
						ll.Axioms = "none"
					}
				}
			}
			d.Lean = append(d.Lean, ll)
		}
		for _, r := range n.Route {
			for _, rr := range b.proj.Routes {
				if rr.ID == r {
					d.Routes = append(d.Routes, rr)
				}
			}
		}
		v := &view{
			Path: strings.TrimPrefix(NodeURL(n.Tag), "/"), Title: n.Title, Section: "map", tmpl: "node", Node: d,
			Description: fmt.Sprintf("A %s of the map of singularity, tier %s. Status: %s.", n.Kind, n.Tier, n.Status),
			SourceURL:   ContentRepo + "/blob/main/" + n.File,
			Trail:       append(home(), Crumb{Text: "Map", URL: "/map/"}, Crumb{Text: n.Tag, Current: true}),
			markdown:    n.Statement,
		}
		b.add(v)
	}
	return nil
}

func (b *builder) paperViews() error {
	for _, p := range b.proj.Papers {
		d := &PaperData{Paper: p, Nodes: b.nodeLinks(p.Nodes)}
		var err error
		if d.Why, err = b.inline(p.Why); err != nil {
			return fmt.Errorf("site: %s: %w", p.File, err)
		}
		if p.Notes != "" {
			if d.Notes, err = b.render(p.Notes); err != nil {
				return fmt.Errorf("site: %s: %w", p.File, err)
			}
		}
		for _, a := range p.Arxiv {
			d.Links = append(d.Links, SourceLink{Title: "arXiv:" + a, URL: "https://arxiv.org/abs/" + a})
		}
		if p.DOI != "" {
			d.Links = append(d.Links, SourceLink{Title: "doi:" + p.DOI, URL: "https://doi.org/" + p.DOI})
		}
		v := &view{
			Path: strings.TrimPrefix(PaperURL(p.Key), "/"), Title: p.Title, Section: "paper", tmpl: "paper", Paper: d,
			Description: fmt.Sprintf("The record of %s in the corpus of singularity, tier %s.", p.Key, p.Tier),
			SourceURL:   ContentRepo + "/blob/main/" + p.File,
			Trail:       append(home(), Crumb{Text: "Papers", URL: "/paper/"}, Crumb{Text: p.Key, Current: true}),
			markdown:    p.Why,
		}
		b.add(v)
	}
	return nil
}

func (b *builder) routeData() ([]RouteData, error) {
	var out []RouteData
	for _, r := range b.proj.Routes {
		d := RouteData{Route: r, URL: RouteURL(r.ID)}
		var err error
		for _, f := range []struct {
			dst *template.HTML
			src string
		}{{&d.Idea, r.Idea}, {&d.Obstruction, r.Obstruction}, {&d.Known, r.Known}, {&d.First, r.First}} {
			if *f.dst, err = b.render(f.src); err != nil {
				return nil, fmt.Errorf("site: route %s: %w", r.ID, err)
			}
		}
		for _, n := range b.proj.Nodes {
			for _, id := range n.Route {
				if id == r.ID {
					d.Nodes = append(d.Nodes, b.nodeLink(n.Tag))
				}
			}
		}
		out = append(out, d)
	}
	return out, nil
}

func (b *builder) routeViews() error {
	routes, err := b.routeData()
	if err != nil {
		return err
	}
	for i := range routes {
		r := routes[i]
		b.add(&view{
			Path: strings.TrimPrefix(r.URL, "/"), Title: r.Route.ID + ". " + r.Route.Title, Section: "routes", tmpl: "route", Route: &r,
			Description: "Route " + r.Route.ID + " to resolution in dimension 4: the idea, the known results and the key obstruction.",
			SourceURL:   ContentRepo + "/blob/main/blueprint/routes.yaml",
			Trail:       append(home(), Crumb{Text: "Routes", URL: "/routes/"}, Crumb{Text: r.Route.ID, Current: true}),
			markdown:    "## Idea\n\n" + r.Route.Idea + "\n\n## Key obstruction\n\n" + r.Route.Obstruction + "\n",
		})
	}
	return nil
}

func (b *builder) add(v *view) {
	v.Site = b.info
	if v.Description == "" {
		v.Description = strings.ReplaceAll(v.Lead, "`", "")
	}
	if strings.ContainsAny(v.Lead, "`$*[") {
		if h, err := b.inline(v.Lead); err == nil {
			v.LeadHTML = h
		}
	}
	if v.markdown != "" && !strings.HasSuffix(v.Path, ".html") {
		v.MarkdownURL = "/" + v.Path + "index.md"
	}
	b.views = append(b.views, v)
}

func (b *builder) unique() error {
	seen := map[string]bool{}
	for _, v := range b.views {
		if seen[v.Path] {
			return fmt.Errorf("site: two pages have the path /%s", v.Path)
		}
		seen[v.Path] = true
	}
	if !seen[""] {
		return errors.New("site: the content has no site/home.md")
	}
	return nil
}

// write empties the output directory and writes the site.
func (b *builder) write() (*Result, error) {
	out := b.opt.Out
	if err := os.RemoveAll(out); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(out, 0o750); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(out)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	w := &writer{root: root}
	for _, v := range b.views {
		if v.markdown != "" && v.tmpl != "node" && v.tmpl != "paper" && v.tmpl != "route" {
			body, err := b.render(v.markdown)
			if err != nil {
				return nil, fmt.Errorf("site: /%s: %w", v.Path, err)
			}
			v.Body = body
		}
		var inner bytes.Buffer
		if err := b.tmpl.ExecuteTemplate(&inner, v.tmpl, v); err != nil {
			return nil, fmt.Errorf("site: /%s: %w", v.Path, err)
		}
		v.Content = template.HTML(inner.String()) // #nosec G203 -- the output of a template
		var page bytes.Buffer
		if err := b.tmpl.ExecuteTemplate(&page, "layout", v); err != nil {
			return nil, fmt.Errorf("site: /%s: %w", v.Path, err)
		}
		file := v.Path
		if file == "" || strings.HasSuffix(file, "/") {
			file += "index.html"
		}
		w.file(file, page.Bytes())
		if v.MarkdownURL != "" {
			w.file(strings.TrimPrefix(v.MarkdownURL, "/"), []byte("# "+v.Title+"\n\n"+strings.TrimLeft(v.markdown, "\n")))
		}
	}
	if err := fs.WalkDir(assetFS, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(p, ".md") {
			return err
		}
		data, err := assetFS.ReadFile(p)
		if err != nil {
			return err
		}
		w.file(p, data)
		return nil
	}); err != nil {
		return nil, err
	}
	b.labCopy(w, b.runs)
	b.machine(w)
	if w.err != nil {
		return nil, fmt.Errorf("site: %w", w.err)
	}
	return &Result{Pages: len(b.views), Files: w.files, Bytes: w.bytes}, nil
}

// inline removes the paragraph around a short text.
func inline(h template.HTML) template.HTML {
	s := strings.TrimSpace(string(h))
	if strings.HasPrefix(s, "<p>") && strings.HasSuffix(s, "</p>") && strings.Count(s, "<p>") == 1 {
		s = s[3 : len(s)-4]
	}
	return template.HTML(s) // #nosec G203 -- the output of the Markdown renderer
}

// PageRecord is an entry of api/pages.json.
type PageRecord struct {
	Path        string `json:"path"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Section     string `json:"section,omitempty"`
	Markdown    string `json:"markdown,omitempty"`
}

// searchEntry is an entry of api/search.json, in the form that site.js of Math 2.0 reads.
type searchEntry struct {
	T string   `json:"t"`
	N string   `json:"n"`
	K string   `json:"k"`
	U string   `json:"u"`
	A []string `json:"a,omitempty"`
	M []string `json:"m,omitempty"`
}

var wordRE = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]+`)

// machine writes the files for agents and for the host: the JSON in api/, llms.txt, the sitemap, robots.txt and _headers.
func (b *builder) machine(w *writer) {
	var pages []PageRecord
	var search []searchEntry
	for _, v := range b.views {
		if strings.HasSuffix(v.Path, ".html") {
			continue
		}
		pages = append(pages, PageRecord{Path: "/" + v.Path, URL: b.info.Canonical(v.Path), Title: v.Title, Description: v.Description, Section: v.Section, Markdown: v.MarkdownURL})
		switch {
		case v.Node != nil:
			n := v.Node.Node
			search = append(search, searchEntry{T: n.Tag, N: n.Title, K: n.Kind, U: v.Path, A: []string{n.Status, "tier " + n.Tier}, M: []string{n.Lean.Statement, n.Lean.Proof}})
		case v.Paper != nil:
			p := v.Paper.Paper
			search = append(search, searchEntry{T: p.Key, N: p.Title, K: "paper", U: v.Path, A: append([]string{"tier " + p.Tier}, p.Authors...), M: []string{p.Why}})
		default:
			kind := v.Section
			if kind == "" {
				kind = "page"
			}
			var a []string
			if v.Lead != "" {
				a = []string{strings.Join(wordRE.FindAllString(v.Lead, -1), " ")}
			}
			search = append(search, searchEntry{T: v.Path, N: v.Title, K: kind, U: v.Path, A: a})
		}
	}
	w.json("api/pages.json", map[string]any{"name": "singularity", "commit": b.opt.Commit, "built": b.opt.Date, "pages": pages})
	w.json("api/search.json", search)
	w.json("api/trust.json", b.info.Trust)
	w.json("api/nodes.json", map[string]any{"lean": b.proj.Lean, "nodes": b.proj.Nodes})
	w.json("api/papers.json", b.proj.Papers)
	w.json("api/routes.json", b.proj.Routes)

	var l strings.Builder
	l.WriteString("# singularity\n\n> Resolution of singularities in characteristic p, in public. The open problem is resolution in dimension 4 and more over a field of characteristic p > 0. The project states the problem in Lean 4, formalizes the theory that it rests on and works on the open case. No proof of the open case exists. Each status on this site comes from the Lean build of singularity-lean.\n\n")
	l.WriteString("Each page has a Markdown copy at index.md next to it. The JSON is in /api/: nodes.json has the map with the computed status of each node.\n\n## Pages\n\n")
	for _, p := range pages {
		if strings.HasPrefix(p.Path, "/node/") || strings.HasPrefix(p.Path, "/paper/") && p.Path != "/paper/" {
			continue
		}
		u := p.URL
		if p.Markdown != "" {
			u = b.info.Canonical(strings.TrimPrefix(p.Markdown, "/"))
		}
		fmt.Fprintf(&l, "- [%s](%s)", p.Title, u)
		if p.Description != "" {
			fmt.Fprintf(&l, ": %s", p.Description)
		}
		l.WriteString("\n")
	}
	fmt.Fprintf(&l, "\n## Data\n\n- [The map](%s)\n- [The papers](%s)\n- [The trust report](%s)\n- [The Lean source](%s)\n", b.info.Canonical("api/nodes.json"), b.info.Canonical("api/papers.json"), b.info.Canonical("api/trust.json"), LeanRepo)
	w.file("llms.txt", []byte(l.String()))

	var s strings.Builder
	s.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range pages {
		if p.Path == "/search/" {
			continue
		}
		fmt.Fprintf(&s, "<url><loc>%s</loc></url>\n", html.EscapeString(p.URL))
	}
	s.WriteString("</urlset>\n")
	w.file("sitemap.xml", []byte(s.String()))
	w.file("robots.txt", []byte("User-agent: *\nAllow: /\n\nSitemap: "+b.info.Canonical("sitemap.xml")+"\n"))
	w.file("_headers", []byte(headers))
}

// headers are the HTTP headers of the _headers file, which Cloudflare static assets read.
const headers = `/*
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  X-Frame-Options: DENY
  Permissions-Policy: interest-cohort=()

/assets/fonts/*
  Cache-Control: public, max-age=31536000, immutable

/*.md
  Content-Type: text/markdown; charset=utf-8

/llms.txt
  Content-Type: text/plain; charset=utf-8
`

// writer writes files under a root and keeps the first error.
type writer struct {
	root  *os.Root
	files int
	bytes int64
	err   error
}

func (w *writer) file(name string, data []byte) {
	if w.err != nil {
		return
	}
	if dir := path.Dir(name); dir != "." {
		if err := w.root.MkdirAll(dir, 0o750); err != nil {
			w.err = err
			return
		}
	}
	if err := w.root.WriteFile(name, data, 0o644); err != nil { // #nosec G306 -- the files of a public site
		w.err = err
		return
	}
	w.files++
	w.bytes += int64(len(data))
}

func (w *writer) json(name string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		w.err = err
		return
	}
	w.file(name, append(data, '\n'))
}

// DefaultTrust is the trust report before any check runs. The rows are those of spec 14.
func DefaultTrust() Trust {
	return Trust{Checks: []TrustCheck{
		{ID: "Q0", Question: "Is the statement the one in the pinned statement file?", State: "not run"},
		{ID: "Q1", Question: "Does the project build with the pinned Lean and Mathlib?", State: "not run"},
		{ID: "Q2", Question: "Which axioms do the top theorems use?", State: "not run"},
		{ID: "Q3", Question: "Does a second kernel accept the proofs?", State: "not run"},
		{ID: "Q4", Question: "Does lake comparator match each proof to the pinned statement?", State: "not run"},
		{ID: "Q5", Question: "Did the statements pass the fidelity battery F1 to F7?", State: "not run"},
		{ID: "Q6", Question: "Is each hypothesis of a proved-mod result named and listed?", State: "not run"},
		{ID: "Q7", Question: "Have two outside experts read the statements?", State: "not run"},
		{ID: "Q8", Question: "Have two outside experts read the written proof of the open case?", State: "not run"},
	}}
}

func readTrust(root string) (Trust, error) {
	data, err := os.ReadFile(filepath.Join(root, "records", "trust.json")) // #nosec G304 -- a file of the content repository
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultTrust(), nil
	}
	if err != nil {
		return Trust{}, err
	}
	var t Trust
	if err := json.Unmarshal(data, &t); err != nil {
		return Trust{}, fmt.Errorf("records/trust.json: %w", err)
	}
	return t, nil
}

// sitePages lists the Markdown files in site/ of the content repository, sorted.
func sitePages(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "site"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, "site/"+e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func readDoc(root, rel string) (Front, string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a file of the content repository
	if err != nil {
		return Front{}, "", err
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	var f Front
	body := string(data)
	if bytes.HasPrefix(data, []byte("---\n")) {
		rest := data[4:]
		end := bytes.Index(rest, []byte("\n---\n"))
		if end < 0 {
			return Front{}, "", fmt.Errorf("%s: the front matter has no end line ---", rel)
		}
		dec := yaml.NewDecoder(bytes.NewReader(rest[:end]))
		dec.KnownFields(true)
		if err := dec.Decode(&f); err != nil {
			return Front{}, "", fmt.Errorf("%s: front matter: %w", rel, err)
		}
		body = string(rest[end+5:])
	}
	if f.Title == "" {
		return Front{}, "", fmt.Errorf("%s: the front matter has no title", rel)
	}
	return f, body, nil
}
