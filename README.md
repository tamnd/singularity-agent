# singularity-agent

The toolchain of [singularity](https://github.com/tamnd/singularity), the public project on resolution of singularities in characteristic p. The command is `sing`. It checks the map of the content repository, gives the status of each node from the record of the Lean build, and builds the site [singularity.tamnd.com](https://singularity.tamnd.com).

The open problem is resolution of singularities for varieties of dimension 4 and more over a field of characteristic p > 0. No proof of it exists. This toolchain does not decide that a result is proved. The Lean kernel decides that, and `sing` only reads the record of the build.

## Install

You need Go 1.27 or later.

```sh
go install github.com/tamnd/singularity-agent/cmd/sing@latest
```

Each release on GitHub also has binaries for Linux, macOS and Windows, with checksums and a build provenance attestation.

## Use

Run the commands in a clone of `tamnd/singularity`.

```sh
sing check                      # check the map: tags, links, papers, routes and Lean names
sing status                     # print the status of each node
sing status -only stated        # print only the stated nodes
sing index -lean lean.json      # copy the record of the Lean build to records/lean.json
sing site build -content . -out public
sing site serve -dir public     # serve the site at http://127.0.0.1:8080/
sing lab run lab/specs/e1-p2.yaml   # run an experiment of the lab
sing lab list                   # list the runs in lab/runs
sing lab step -p 2 -q 4 -vars x,y,w -f "x^2*y^2*w^3*(w*(x+y)^4+x^13)" -divisor x,y,w -chart x -t y=1
```

The exit codes are 0 for success, 1 for a failed check, 2 for a usage error and 3 for a command that is not written yet. The help text gives the milestone of each command that is not written yet.

## The status rules

The status of a node comes from `records/lean.json` and from the nodes that it uses. The rules are in `blueprint/blueprint.go`.

| Status | When |
|---|---|
| planned | The node has no Lean statement in the build. |
| stated | The statement builds, but the proof uses `sorry` or an axiom that is not standard. |
| defined | A definition that builds with no `sorry`. |
| proved | The proof uses only `propext`, `Classical.choice` and `Quot.sound`, and each node that it uses is proved or defined. |
| proved-mod | The proof is clean but it takes named hypotheses, or a node that it uses is not proved. |
| open | A conjecture. No proof is claimed. |
| refuted | The negation is proved. |

## The lab

`sing lab` is the computation lab of spec 13 of the content repository. The engine is in Go, in the package `lab`. Spec 13 planned it in Rust. It is in Go so that it is one binary with the rest of `sing`. The specs still name the tool `sing-lab`.

The engine works with the equations f = z^q + F over F_p, with q = p^e and ord F >= q, as in section 2 of Hauser and Perlega, arXiv:1802.05010. It cleans F (it removes the monomials with exponents in qN^n), computes the residual order along the divisor and does point blowups in the charts x_j with translations over F_p. A step keeps the order q of f. When the transform has a constant term c, the point of the strict transform is at z = c^(1/q), and the cleaning removes c with the change z -> z + c^(1/q). The centres are points only. `lab step` does one blowup, so you can check an example of a run by hand.

`sing lab run <spec.yaml>` writes `lab/runs/<id>/`. The id is the date, the name of the spec and the first 8 characters of the sha256 of the spec. The run has:

- `record.json`: the sha256 of the spec, the version and the commit of sing, the seed, the host, the wall time, the counts and the sha256 of each output.
- the outputs: `histogram.json` and `examples.json` for random walks (E1), `candidates.json` for the candidate invariants (E2) and `steps.json` for a fixed sequence of blowups.
- `summary.md`: a short summary in plain English.

A run is reproducible from the seed. Each walk has its own random source that depends only on the seed, the case and the number of the walk, and the walks are added up in order. So the outputs do not depend on the number of threads. Two runs of the same spec differ only in the host and the times in `record.json`.

The lab runs the engine as a process on the host, with no container. Container isolation is for the external tools of spec 13 (Singular, Macaulay2, SageMath). The lab does not run those tools yet. The containers are planned for the milestone that adds them.

## The site

The site uses the style and the math renderer of [Math 2.0](https://github.com/tamnd/math2-agent). The math is rendered to MathML at build time, so a page shows its math with JavaScript off. Each page has a Markdown copy next to it, and `/api/` has the map, the papers and the trust report as JSON. The page `/lab/` lists the runs of `lab/runs/`, and each run has a page with its summary and its files.

## Develop

```sh
make check      # gofmt, vet, the tests with the race detector, and golangci-lint
```

Read [CONTRIBUTING.md](CONTRIBUTING.md) before you open a pull request.

## Licence

MIT. See [LICENSE](LICENSE). The files in `site/assets/` come from Math 2.0. See [site/assets/UPSTREAM.md](site/assets/UPSTREAM.md) for their sources and licences.
