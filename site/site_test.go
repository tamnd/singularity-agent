package site

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func content(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	put(t, root, "site/home.md", "---\ntitle: Resolution in characteristic p\nlead: The open case is $\\dim X \\ge 4$.\ndoors:\n  - title: The problem\n    path: problem/\n    text: What is open.\n---\nThe text of the main page.\n")
	put(t, root, "site/problem.md", "---\ntitle: The problem\nlead: What is open.\n---\nSee [the statement](node:SG00000001) and [Hironaka](paper:Hir64). The formula $$x^p + y^q = 0$$ shows.\n")
	put(t, root, "site/map.md", "---\ntitle: The map\n---\nThe nodes.\n")
	put(t, root, "site/papers.md", "---\ntitle: The papers\n---\nThe corpus.\n")
	put(t, root, "site/routes.md", "---\ntitle: The routes\n---\nThe routes.\n")
	put(t, root, "site/trust.md", "---\ntitle: Trust\n---\nThe checks.\n")
	for _, name := range []string{"learn", "handbook", "code", "about"} {
		put(t, root, "site/"+name+".md", "---\ntitle: "+name+"\n---\nText.\n")
	}
	put(t, root, "blueprint/routes.yaml", "- id: R1\n  title: Polyhedra\n  weight: 0.25\n  idea: i\n  obstruction: o\n  known: k\n  first: f\n")
	put(t, root, "blueprint/nodes/SG00000001.yaml", "tag: SG00000001\nkind: conjecture\ntitle: Dimension 4\nstatement: Each variety $X$ of dimension 4 has a resolution.\ntier: S\nroute: [R1]\nsources:\n  - paper: Hir64\n    loc: Main theorem\nlean:\n  statement: Singularity.ResolutionDimFourCharP\n")
	put(t, root, "papers/Hir64.yaml", "key: Hir64\ntitle: Resolution of singularities\nauthors: [H. Hironaka]\nyear: 1964\ntag: P\ntier: T8\nwhy: The char 0 case.\nuse: [read]\n")
	put(t, root, "records/lean.json", `{"commit":"0123456789abcdef","toolchain":"leanprover/lean4:v4.35.0","decls":{"Singularity.ResolutionDimFourCharP":{"kind":"def","file":"Singularity/Statements/Main.lean","line":12,"axioms":[]}}}`)
	return root
}

func TestBuild(t *testing.T) {
	out := filepath.Join(t.TempDir(), "public")
	res, err := Build(context.Background(), Options{Content: content(t), Out: out, Commit: "abc", Date: "2026-11-23"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Pages < 9 {
		t.Errorf("built %d pages", res.Pages)
	}
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for name, want := range map[string]string{
		"index.html":                 "<math",
		"problem/index.html":         `href="/node/SG00000001/"`,
		"node/SG00000001/index.html": "Singularity/Statements/Main.lean#L12",
		"paper/hir64/index.html":     "Dimension 4",
		"routes/r1/index.html":       "Dimension 4",
		"map/index.html":             `class="mark open"`,
		"api/search.json":            "SG00000001",
		"llms.txt":                   "No proof of the open case exists",
		"problem/index.md":           "# The problem",
		"404.html":                   "Page not found",
	} {
		if got := read(name); !strings.Contains(got, want) {
			t.Errorf("%s does not have %q", name, want)
		}
	}
	r, err := Check(out)
	if err != nil {
		t.Fatal(err)
	}
	if !r.OK() {
		t.Errorf("the check finds problems: %v", r.Problems)
	}
}

func TestBuildStopsOnABadMap(t *testing.T) {
	root := content(t)
	put(t, root, "blueprint/nodes/SG00000002.yaml", "tag: SG00000002\nkind: lemma\ntitle: t\nstatement: s\ntier: T1\nuses: [SG0000000Z]\n")
	if _, err := Build(context.Background(), Options{Content: root, Out: filepath.Join(t.TempDir(), "p")}); err == nil {
		t.Fatal("a map with a bad link builds")
	}
}
