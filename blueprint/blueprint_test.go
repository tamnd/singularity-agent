package blueprint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "blueprint/routes.yaml", "- id: R1\n  title: Polyhedra\n  weight: 0.25\n  idea: i\n  obstruction: o\n  known: k\n  first: f\n")
	write(t, root, "blueprint/nodes/SG00000001.yaml", "tag: SG00000001\nkind: definition\ntitle: Regular\nstatement: A regular scheme.\ntier: S\nlean:\n  statement: Singularity.IsRegularScheme\n")
	write(t, root, "blueprint/nodes/SG00000002.yaml", "tag: SG00000002\nkind: theorem\ntitle: Dimension 0\nstatement: Dim 0.\ntier: S\nuses: [SG00000001]\nlean:\n  statement: Singularity.ResolutionInDim\n  proof: Singularity.resolutionInDim_zero\nsources:\n  - paper: Hir64\n")
	write(t, root, "blueprint/nodes/SG00000003.yaml", "tag: SG00000003\nkind: theorem\ntitle: Dimension 1\nstatement: Dim 1.\ntier: T5\nuses: [SG00000004]\nroute: [R1]\nlean:\n  statement: Singularity.ResolutionInDim\n  proof: Singularity.resolutionInDim_one\n")
	write(t, root, "blueprint/nodes/SG00000004.yaml", "tag: SG00000004\nkind: hypothesis\ntitle: Normalization\nstatement: Finite.\ntier: T1\nlean:\n  statement: Singularity.Normalization\n")
	write(t, root, "blueprint/nodes/SG00000005.yaml", "tag: SG00000005\nkind: conjecture\ntitle: Dimension 4\nstatement: Open.\ntier: S\nlean:\n  statement: Singularity.ResolutionDimFourCharP\n")
	write(t, root, "papers/Hir64.yaml", "key: Hir64\ntitle: Resolution of singularities\ntag: P\ntier: T8\nwhy: char 0\nuse: [read]\n")
	write(t, root, "records/lean.json", `{"commit":"abc","decls":{
"Singularity.IsRegularScheme":{"kind":"def","axioms":["propext"]},
"Singularity.ResolutionInDim":{"kind":"def","axioms":[]},
"Singularity.resolutionInDim_zero":{"kind":"theorem","axioms":["propext","Classical.choice","Quot.sound"]},
"Singularity.resolutionInDim_one":{"kind":"theorem","axioms":["propext"]},
"Singularity.Normalization":{"kind":"def","axioms":[]},
"Singularity.ResolutionDimFourCharP":{"kind":"def","axioms":[]}}}`)
	return root
}

func TestLoadAndStatus(t *testing.T) {
	p, err := Load(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if probs := p.Check(); len(probs) != 0 {
		t.Fatalf("problems: %v", probs)
	}
	want := map[string]string{
		"SG00000001": Defined,
		"SG00000002": Proved,
		"SG00000003": ProvedMod,
		"SG00000004": Stated,
		"SG00000005": Open,
	}
	for tag, st := range want {
		if got := p.Node(tag).Status; got != st {
			t.Errorf("%s: status %s, want %s (%s)", tag, got, st, p.Node(tag).Why)
		}
	}
	if got := p.Paper("Hir64").Nodes; len(got) != 1 || got[0] != "SG00000002" {
		t.Errorf("Hir64 nodes = %v", got)
	}
	if got := p.Node("SG00000001").UsedBy; len(got) != 1 {
		t.Errorf("used by = %v", got)
	}
}

func TestSorryIsNotAProof(t *testing.T) {
	root := fixture(t)
	write(t, root, "records/lean.json", `{"decls":{
"Singularity.IsRegularScheme":{"kind":"def","axioms":[]},
"Singularity.ResolutionInDim":{"kind":"def","axioms":[]},
"Singularity.resolutionInDim_zero":{"kind":"theorem","axioms":["sorryAx"]},
"Singularity.resolutionInDim_one":{"kind":"theorem","axioms":["propext","Singularity.myAxiom"]},
"Singularity.Normalization":{"kind":"def","axioms":[]},
"Singularity.ResolutionDimFourCharP":{"kind":"def","axioms":[]}}}`)
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Node("SG00000002").Status; got != Stated {
		t.Errorf("a proof with sorry has status %s", got)
	}
	if got := p.Node("SG00000003").Status; got != Stated {
		t.Errorf("a proof with a new axiom has status %s", got)
	}
}

func TestCheckFindsFaults(t *testing.T) {
	root := fixture(t)
	write(t, root, "blueprint/nodes/SG00000006.yaml", "tag: SG00000006\nkind: guess\ntitle: Bad\nstatement: x\ntier: T9\nuses: [SG00000007, SG00000006]\nsources:\n  - paper: Nobody\nroute: [R7]\n")
	write(t, root, "blueprint/nodes/SGbad.yaml", "tag: SGbad\nkind: lemma\ntitle: t\nstatement: s\ntier: T1\n")
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	probs := strings.Join(p.Check(), "\n")
	for _, want := range []string{"kind \"guess\"", "tier \"T9\"", "SG00000007, which is not a node", "Nobody", "route \"R7\"", "cycle", "SGbad"} {
		if !strings.Contains(probs, want) {
			t.Errorf("the check does not report %q:\n%s", want, probs)
		}
	}
}

func TestUnknownFieldIsAnError(t *testing.T) {
	root := fixture(t)
	write(t, root, "papers/Bad.yaml", "key: Bad\ntitle: t\ntag: P\ntier: T1\nwhy: w\nstatus: proved\n")
	if _, err := Load(root); err == nil {
		t.Fatal("a paper record with an unknown field loads with no error")
	}
}
